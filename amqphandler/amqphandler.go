// SPDX-License-Identifier: Apache-2.0
//
// Copyright (C) 2021 Renesas Electronics Corporation.
// Copyright (C) 2021 EPAM Systems, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package amqphandler

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/aoscloud/aos_common/aoserrors"
	"github.com/aoscloud/aos_common/api/cloudprotocol"
	log "github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
)

/***********************************************************************************************************************
 * Consts
 **********************************************************************************************************************/

const (
	sendChannelSize    = 32
	sendMaxTry         = 3
	sendTimeout        = 1 * time.Minute
	receiveChannelSize = 16
)

const (
	amqpSecureScheme   = "amqps"
	amqpInsecureScheme = "amqp"
)

/***********************************************************************************************************************
 * Types
 **********************************************************************************************************************/

// AmqpHandler structure with all amqp connection info.
type AmqpHandler struct { // nolint:stylecheck
	sync.Mutex

	// MessageChannel channel for amqp messages
	MessageChannel chan Message

	sendChannel    chan Message
	pendingChannel chan Message
	sendTry        int

	sendConnection    *amqp.Connection
	receiveConnection *amqp.Connection

	cryptoContext CryptoContext

	systemID string

	cancelFunc context.CancelFunc

	wg sync.WaitGroup

	isConnected               bool
	connectionEventsConsumers []ConnectionEventsConsumer
}

// CryptoContext interface to access crypto functions.
type CryptoContext interface {
	GetTLSConfig() (*tls.Config, error)
	DecryptMetadata(input []byte) ([]byte, error)
}

// Message AMQP message.
type Message interface{}

// ConnectionEventsConsumer connection events consumer interface.
type ConnectionEventsConsumer interface {
	CloudConnected()
	CloudDisconnected()
}

/***********************************************************************************************************************
 * Variables
 **********************************************************************************************************************/

var messageMap = map[string]func() interface{}{ // nolint:gochecknoglobals
	cloudprotocol.DesiredStatusType: func() interface{} {
		return &cloudprotocol.DesiredStatus{}
	},
	cloudprotocol.RequestServiceCrashLogType: func() interface{} {
		return &cloudprotocol.RequestServiceCrashLog{}
	},
	cloudprotocol.RequestServiceLogType: func() interface{} {
		return &cloudprotocol.RequestServiceLog{}
	},
	cloudprotocol.RequestSystemLogType: func() interface{} {
		return &cloudprotocol.RequestSystemLog{}
	},
	cloudprotocol.StateAcceptanceType: func() interface{} {
		return &cloudprotocol.StateAcceptance{}
	},
	cloudprotocol.UpdateStateType: func() interface{} {
		return &cloudprotocol.UpdateState{}
	},
	cloudprotocol.RenewCertsNotificationType: func() interface{} {
		return &cloudprotocol.RenewCertsNotification{}
	},
	cloudprotocol.IssuedUnitCertsType: func() interface{} {
		return &cloudprotocol.IssuedUnitCerts{}
	},
	cloudprotocol.OverrideEnvVarsType: func() interface{} {
		return &cloudprotocol.OverrideEnvVars{}
	},
}

var (
	// ErrNotConnected indicates AMQP client is not connected.
	ErrNotConnected = errors.New("not connected")
	// ErrSendChannelFull indicates AMQP send channel is full.
	ErrSendChannelFull = errors.New("send channel full")
)

/***********************************************************************************************************************
 * Public
 **********************************************************************************************************************/

// New creates new amqp object.
func New() (*AmqpHandler, error) {
	log.Debug("New AMQP")

	handler := &AmqpHandler{
		sendChannel:    make(chan Message, sendChannelSize),
		pendingChannel: make(chan Message, 1),
	}

	return handler, nil
}

// Connect connects to cloud.
func (handler *AmqpHandler) Connect(cryptoContext CryptoContext, sdURL, systemID string, insecure bool) error {
	handler.Lock()
	defer handler.Unlock()

	log.WithFields(log.Fields{"url": sdURL}).Debug("AMQP connect")

	handler.cryptoContext = cryptoContext
	handler.systemID = systemID

	tlsConfig, err := handler.cryptoContext.GetTLSConfig()
	if err != nil {
		return aoserrors.Wrap(err)
	}

	var (
		connectionInfo cloudprotocol.ConnectionInfo
		ctx            context.Context
	)

	ctx, handler.cancelFunc = context.WithCancel(context.Background())

	if connectionInfo, err = getConnectionInfo(ctx, sdURL,
		handler.createCloudMessage(cloudprotocol.ServiceDiscoveryType,
			cloudprotocol.ServiceDiscoveryRequest{}), tlsConfig); err != nil {
		return aoserrors.Wrap(err)
	}

	scheme := amqpSecureScheme

	if insecure {
		scheme = amqpInsecureScheme
	}

	if err := handler.setupConnections(scheme, connectionInfo, tlsConfig); err != nil {
		return aoserrors.Wrap(err)
	}

	handler.isConnected = true

	handler.notifyCloudConnected()

	return nil
}

// Disconnect disconnects from cloud.
func (handler *AmqpHandler) Disconnect() error {
	handler.Lock()
	defer handler.Unlock()

	log.Debug("AMQP disconnect")

	if handler.cancelFunc != nil {
		handler.cancelFunc()
	}

	if handler.sendConnection != nil {
		handler.sendConnection.Close()
	}

	if handler.receiveConnection != nil {
		handler.receiveConnection.Close()
	}

	handler.wg.Wait()

	handler.isConnected = false

	handler.notifyCloudDisconnected()

	return nil
}

// SendUnitStatus sends unit status.
func (handler *AmqpHandler) SendUnitStatus(unitStatus cloudprotocol.UnitStatus) error {
	handler.Lock()
	defer handler.Unlock()

	return handler.scheduleMessage(cloudprotocol.UnitStatusType, unitStatus, false)
}

// SendMonitoringData sends monitoring data.
func (handler *AmqpHandler) SendMonitoringData(monitoringData cloudprotocol.MonitoringData) error {
	handler.Lock()
	defer handler.Unlock()

	return handler.scheduleMessage(cloudprotocol.MonitoringDataType, monitoringData, false)
}

// SendServiceNewState sends new state message.
func (handler *AmqpHandler) SendInstanceNewState(newState cloudprotocol.NewState) error {
	handler.Lock()
	defer handler.Unlock()

	return handler.scheduleMessage(cloudprotocol.NewStateType, newState, false)
}

// SendServiceStateRequest sends state request message.
func (handler *AmqpHandler) SendInstanceStateRequest(request cloudprotocol.StateRequest) error {
	handler.Lock()
	defer handler.Unlock()

	return handler.scheduleMessage(cloudprotocol.StateRequestType, request, true)
}

// SendLog sends system or service logs.
func (handler *AmqpHandler) SendLog(serviceLog cloudprotocol.PushLog) error {
	handler.Lock()
	defer handler.Unlock()

	return handler.scheduleMessage(cloudprotocol.PushLogType, serviceLog, true)
}

// SendAlerts sends alerts message.
func (handler *AmqpHandler) SendAlerts(alerts cloudprotocol.Alerts) error {
	handler.Lock()
	defer handler.Unlock()

	return handler.scheduleMessage(cloudprotocol.AlertsType, alerts, true)
}

// SendIssueUnitCerts sends request to issue new certificates.
func (handler *AmqpHandler) SendIssueUnitCerts(requests []cloudprotocol.IssueCertData) error {
	handler.Lock()
	defer handler.Unlock()

	return handler.scheduleMessage(
		cloudprotocol.IssueUnitCertsType, cloudprotocol.IssueUnitCerts{Requests: requests}, true)
}

// SendInstallCertsConfirmation sends install certificates confirmation.
func (handler *AmqpHandler) SendInstallCertsConfirmation(confirmations []cloudprotocol.InstallCertData) error {
	handler.Lock()
	defer handler.Unlock()

	return handler.scheduleMessage(
		cloudprotocol.InstallUnitCertsConfirmationType,
		cloudprotocol.InstallUnitCertsConfirmation{Certificates: confirmations}, true)
}

// SendOverrideEnvVarsStatus overrides env vars status.
func (handler *AmqpHandler) SendOverrideEnvVarsStatus(envs cloudprotocol.OverrideEnvVarsStatus) error {
	handler.Lock()
	defer handler.Unlock()

	return handler.scheduleMessage(cloudprotocol.OverrideEnvVarsStatusType, envs, true)
}

// SubscribeForConnectionEvents subscribes for connection events.
func (handler *AmqpHandler) SubscribeForConnectionEvents(consumer ConnectionEventsConsumer) error {
	handler.Lock()
	defer handler.Unlock()

	for _, subscribedConsumer := range handler.connectionEventsConsumers {
		if subscribedConsumer == consumer {
			return aoserrors.New("already subscribed")
		}
	}

	handler.connectionEventsConsumers = append(handler.connectionEventsConsumers, consumer)

	return nil
}

// UnsubscribeFromConnectionEvents unsubscribes from connection events.
func (handler *AmqpHandler) UnsubscribeFromConnectionEvents(consumer ConnectionEventsConsumer) error {
	handler.Lock()
	defer handler.Unlock()

	for i, subscribedConsumer := range handler.connectionEventsConsumers {
		if subscribedConsumer == consumer {
			handler.connectionEventsConsumers = append(handler.connectionEventsConsumers[:i],
				handler.connectionEventsConsumers[i+1:]...)

			return nil
		}
	}

	return aoserrors.New("not subscribed")
}

// Close closes all amqp connection.
func (handler *AmqpHandler) Close() {
	log.Info("Close AMQP")

	if handler.cancelFunc != nil {
		handler.cancelFunc()
	}

	if handler.isConnected {
		if err := handler.Disconnect(); err != nil {
			log.Errorf("Can't disconnect from AMQP server: %s", err)
		}
	}
}

/***************************************************************************************************
 * Private
 **************************************************************************************************/

// service discovery implementation.
func getConnectionInfo(
	ctx context.Context, url string, request cloudprotocol.Message, tlsConfig *tls.Config,
) (info cloudprotocol.ConnectionInfo, err error) {
	reqJSON, err := json.Marshal(request)
	if err != nil {
		return info, aoserrors.Wrap(err)
	}

	log.WithField("request", string(reqJSON)).Info("AMQP service discovery request")

	transport := &http.Transport{TLSClientConfig: tlsConfig}
	client := &http.Client{Transport: transport}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqJSON))
	if err != nil {
		return info, aoserrors.Wrap(err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return info, aoserrors.Wrap(err)
	}
	defer resp.Body.Close()

	htmlData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return info, aoserrors.Wrap(err)
	}

	if resp.StatusCode != http.StatusOK {
		return info, aoserrors.Errorf("%s: %s", resp.Status, string(htmlData))
	}

	var jsonResp cloudprotocol.ServiceDiscoveryResponse

	err = json.Unmarshal(htmlData, &jsonResp)
	if err != nil {
		return info, aoserrors.Wrap(err)
	}

	return jsonResp.Connection, nil
}

func (handler *AmqpHandler) setupConnections(
	scheme string, info cloudprotocol.ConnectionInfo, tlsConfig *tls.Config,
) error {
	handler.MessageChannel = make(chan Message, receiveChannelSize)

	if err := handler.setupSendConnection(scheme, info.SendParams, tlsConfig); err != nil {
		return aoserrors.Wrap(err)
	}

	if err := handler.setupReceiveConnection(scheme, info.ReceiveParams, tlsConfig); err != nil {
		return aoserrors.Wrap(err)
	}

	return nil
}

func (handler *AmqpHandler) setupSendConnection(
	scheme string, params cloudprotocol.SendParams, tlsConfig *tls.Config,
) error {
	urlRabbitMQ := url.URL{
		Scheme: scheme,
		User:   url.UserPassword(params.User, params.Password),
		Host:   params.Host,
	}

	log.WithField("url", urlRabbitMQ.String()).Debug("Sender connection url")

	connection, err := amqp.DialConfig(urlRabbitMQ.String(), amqp.Config{
		TLSClientConfig: tlsConfig,
		SASL:            nil,
		Heartbeat:       10 * time.Second,
	})
	if err != nil {
		return aoserrors.Wrap(err)
	}

	amqpChannel, err := connection.Channel()
	if err != nil {
		return aoserrors.Wrap(err)
	}

	handler.sendConnection = connection

	if err = amqpChannel.Confirm(false); err != nil {
		return aoserrors.Wrap(err)
	}

	handler.wg.Add(1)

	go handler.runSender(amqpChannel, params)

	return nil
}

func (handler *AmqpHandler) runSender(amqpChannel *amqp.Channel, params cloudprotocol.SendParams) {
	log.Info("Start AMQP sender")

	defer func() {
		log.Info("AMQP sender closed")

		handler.wg.Done()
	}()

	errorChannel := handler.sendConnection.NotifyClose(make(chan *amqp.Error, 1))
	confirmChannel := amqpChannel.NotifyPublish(make(chan amqp.Confirmation, 1))
	sendChannel := handler.sendChannel

	if len(handler.pendingChannel) > 0 {
		sendChannel = nil
	}

	for {
		select {
		case err := <-errorChannel:
			if err != nil {
				handler.MessageChannel <- aoserrors.New(err.Reason)
			}

			return

		case message := <-sendChannel:
			handler.sendTry = 0
			sendChannel = nil
			handler.pendingChannel <- message

		case message := <-handler.pendingChannel:
			if err := handler.sendMessage(message, amqpChannel, params); err != nil {
				log.Warnf("Can't send message: %v", err)

				sendChannel = handler.sendChannel

				break
			}

			if confirm, ok := <-confirmChannel; !ok || !confirm.Ack {
				handler.pendingChannel <- message

				break
			}

			sendChannel = handler.sendChannel
		}
	}
}

func (handler *AmqpHandler) setupReceiveConnection(
	scheme string, params cloudprotocol.ReceiveParams, tlsConfig *tls.Config,
) error {
	urlRabbitMQ := url.URL{
		Scheme: scheme,
		User:   url.UserPassword(params.User, params.Password),
		Host:   params.Host,
	}

	log.WithField("url", urlRabbitMQ.String()).Debug("Consumer connection url")

	connection, err := amqp.DialConfig(urlRabbitMQ.String(), amqp.Config{
		TLSClientConfig: tlsConfig,
		SASL:            nil,
		Heartbeat:       10 * time.Second,
	})
	if err != nil {
		return aoserrors.Wrap(err)
	}

	amqpChannel, err := connection.Channel()
	if err != nil {
		return aoserrors.Wrap(err)
	}

	deliveryChannel, err := amqpChannel.Consume(
		params.Queue.Name, // queue
		params.Consumer,   // consumer
		true,              // auto-ack param.AutoAck
		params.Exclusive,  // exclusive
		params.NoLocal,    // no-local
		params.NoWait,     // no-wait
		nil,               // args
	)
	if err != nil {
		return aoserrors.Wrap(err)
	}

	handler.receiveConnection = connection

	handler.wg.Add(1)

	go handler.runReceiver(deliveryChannel, params)

	return nil
}

func (handler *AmqpHandler) runReceiver(deliveryChannel <-chan amqp.Delivery, param cloudprotocol.ReceiveParams) {
	log.Info("Start AMQP receiver")

	defer func() {
		log.Info("AMQP receiver closed")

		handler.wg.Done()
	}()

	errorChannel := handler.receiveConnection.NotifyClose(make(chan *amqp.Error, 1))

	for {
		select {
		case err := <-errorChannel:
			if err != nil {
				handler.MessageChannel <- aoserrors.New(err.Reason)
			}

			return

		case delivery, ok := <-deliveryChannel:
			if !ok {
				handler.MessageChannel <- aoserrors.New("delivery channel is closed")
				return
			}

			var (
				rawData     json.RawMessage
				incomingMsg = cloudprotocol.Message{Data: &rawData}
			)

			if err := json.Unmarshal(delivery.Body, &incomingMsg); err != nil {
				log.Errorf("Can't parse message header: %s", err)
				continue
			}

			log.WithFields(log.Fields{
				"version": incomingMsg.Header.Version,
				"type":    incomingMsg.Header.MessageType,
			}).Debug("AMQP received message")

			if incomingMsg.Header.Version != cloudprotocol.ProtocolVersion {
				log.Errorf("Unsupported protocol version: %d", incomingMsg.Header.Version)
				continue
			}

			handler.processReceivedMessages(rawData, incomingMsg.Header.MessageType)
		}
	}
}

func (handler *AmqpHandler) processReceivedMessages(rawData json.RawMessage, messageType string) {
	messageTypeFunc, ok := messageMap[messageType]
	if !ok {
		log.Warnf("AMQP unsupported message type: %s", messageType)
		return
	}

	decodedData := messageTypeFunc()

	if err := json.Unmarshal(rawData, decodedData); err != nil {
		log.Errorf("Can't parse message body: %s", err)
		return
	}

	switch messageType {
	case cloudprotocol.DesiredStatusType:
		encodedStatus, ok := decodedData.(*cloudprotocol.DesiredStatus)
		if !ok {
			log.Error("Wrong data type: expect desired status")
			return
		}

		decodedStatus, err := handler.decodeDesiredStatus(encodedStatus)
		if err != nil {
			log.Errorf("Can't decode desired status: %s", err)
			return
		}

		decodedData = decodedStatus

	case cloudprotocol.RenewCertsNotificationType:
		notification, ok := decodedData.(*cloudprotocol.RenewCertsNotification)
		if !ok {
			log.Error("Wrong data type: expect renew certificate notification")
			return
		}

		notificationWithPwd, err := handler.decodeRenewCertsNotification(notification)
		if err != nil {
			log.Errorf("Can't decode renew certificate notification: %s", err)
			return
		}

		decodedData = notificationWithPwd

	case cloudprotocol.OverrideEnvVarsType:
		encodedEnvVars, ok := decodedData.(*cloudprotocol.OverrideEnvVars)
		if !ok {
			log.Error("Wrong data type: expect override env")
			return
		}

		decodedEnvVars, err := handler.decodeEnvVars(encodedEnvVars)
		if err != nil {
			log.Errorf("Can't decode env vars: %s", err)
			return
		}

		decodedData = decodedEnvVars
	}

	handler.MessageChannel <- decodedData
}

func (handler *AmqpHandler) decodeDesiredStatus(
	encodedStatus *cloudprotocol.DesiredStatus,
) (*cloudprotocol.DecodedDesiredStatus, error) {
	decodedStatus := &cloudprotocol.DecodedDesiredStatus{
		CertificateChains: encodedStatus.CertificateChains,
		Certificates:      encodedStatus.Certificates,
	}

	if err := handler.decodeData(encodedStatus.BoardConfig, &decodedStatus.BoardConfig); err != nil {
		return nil, aoserrors.Wrap(err)
	}

	if err := handler.decodeData(encodedStatus.Services, &decodedStatus.Services); err != nil {
		return nil, aoserrors.Wrap(err)
	}

	if err := handler.decodeData(encodedStatus.Layers, &decodedStatus.Layers); err != nil {
		return nil, aoserrors.Wrap(err)
	}

	if err := handler.decodeData(encodedStatus.Components, &decodedStatus.Components); err != nil {
		return nil, aoserrors.Wrap(err)
	}

	if err := handler.decodeData(encodedStatus.FOTASchedule, &decodedStatus.FOTASchedule); err != nil {
		return nil, aoserrors.Wrap(err)
	}

	if err := handler.decodeData(encodedStatus.SOTASchedule, &decodedStatus.SOTASchedule); err != nil {
		return nil, aoserrors.Wrap(err)
	}

	if err := handler.decodeData(encodedStatus.Instances, &decodedStatus.Instances); err != nil {
		return nil, aoserrors.Wrap(err)
	}

	return decodedStatus, nil
}

func (handler *AmqpHandler) decodeRenewCertsNotification(
	encodedNotification *cloudprotocol.RenewCertsNotification) (
	*cloudprotocol.RenewCertsNotificationWithPwd, error,
) {
	var secret cloudprotocol.UnitSecret

	if len(encodedNotification.UnitSecureData) > 0 {
		if err := handler.decodeData(encodedNotification.UnitSecureData, &secret); err != nil {
			return nil, aoserrors.Wrap(err)
		}

		if secret.Version != cloudprotocol.UnitSecretVersion {
			return nil, aoserrors.New("unit secure version mismatch")
		}
	}

	return &cloudprotocol.RenewCertsNotificationWithPwd{
		Certificates: encodedNotification.Certificates,
		Password:     secret.Data.OwnerPassword,
	}, nil
}

func (handler *AmqpHandler) decodeEnvVars(
	encodedEnvVars *cloudprotocol.OverrideEnvVars,
) (*cloudprotocol.DecodedOverrideEnvVars, error) {
	decodedEnvVars := &cloudprotocol.DecodedOverrideEnvVars{}

	if err := handler.decodeData(encodedEnvVars.OverrideEnvVars, decodedEnvVars); err != nil {
		return nil, aoserrors.Wrap(err)
	}

	return decodedEnvVars, nil
}

func (handler *AmqpHandler) decodeData(data []byte, result interface{}) error {
	if len(data) == 0 {
		return nil
	}

	decryptData, err := handler.cryptoContext.DecryptMetadata(data)
	if err != nil {
		return aoserrors.Wrap(err)
	}

	if err = json.Unmarshal(decryptData, result); err != nil {
		return aoserrors.Wrap(err)
	}

	if rawJSON, ok := result.(*json.RawMessage); ok {
		log.WithField("data", string(*rawJSON)).Debug("Decrypted data")
	} else {
		log.WithField("data", result).Debug("Decrypted data")
	}

	return nil
}

func (handler *AmqpHandler) createCloudMessage(messageType string, data interface{}) cloudprotocol.Message {
	return cloudprotocol.Message{
		Header: cloudprotocol.MessageHeader{
			Version:     cloudprotocol.ProtocolVersion,
			SystemID:    handler.systemID,
			MessageType: messageType,
		},
		Data: data,
	}
}

func (handler *AmqpHandler) notifyCloudConnected() {
	for _, consumer := range handler.connectionEventsConsumers {
		consumer.CloudConnected()
	}
}

func (handler *AmqpHandler) notifyCloudDisconnected() {
	for _, consumer := range handler.connectionEventsConsumers {
		consumer.CloudDisconnected()
	}
}

func (handler *AmqpHandler) scheduleMessage(messageType string, data interface{}, important bool) error {
	if !important && !handler.isConnected {
		return ErrNotConnected
	}

	select {
	case handler.sendChannel <- handler.createCloudMessage(messageType, data):
		return nil

	case <-time.After(sendTimeout):
		return ErrSendChannelFull
	}
}

func (handler *AmqpHandler) sendMessage(
	message Message, amqpChannel *amqp.Channel, params cloudprotocol.SendParams,
) error {
	data, err := json.Marshal(message)
	if err != nil {
		return aoserrors.Wrap(err)
	}

	if handler.sendTry > 1 {
		log.WithFields(log.Fields{"data": string(data)}).Debug("AMQP retry message")
	} else {
		log.WithFields(log.Fields{"data": string(data)}).Debug("AMQP send message")
	}

	if handler.sendTry++; handler.sendTry > sendMaxTry {
		return aoserrors.New("sending message max try reached")
	}

	if err := amqpChannel.Publish(
		params.Exchange.Name, // exchange
		"",                   // routing key
		params.Mandatory,     // mandatory
		params.Immediate,     // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			UserId:       params.User,
			Body:         data,
		}); err != nil {
		// Do not return error in this case for purpose rescheduling message
		log.Errorf("Error publishing AMQP message: %v", err)
	}

	return nil
}
