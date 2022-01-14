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

package umcontroller_test

import (
	"context"
	"errors"
	"io"
	"os"
	"reflect"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"github.com/aoscloud/aos_common/aoserrors"
	pb "github.com/aoscloud/aos_common/api/updatemanager/v1"

	"github.com/aoscloud/aos_communicationmanager/cloudprotocol"
	"github.com/aoscloud/aos_communicationmanager/config"
	"github.com/aoscloud/aos_communicationmanager/umcontroller"
)

/***********************************************************************************************************************
 * Consts
 **********************************************************************************************************************/

const (
	serverURL = "localhost:8091"
)

const (
	prepareStep = "prepare"
	updateStep  = "update"
	applyStep   = "apply"
	revertStep  = "revert"
	finishStep  = "finish"
	rebootStep  = "reboot"
)

type testStorage struct {
	updateInfo []umcontroller.SystemComponent
}

type testURLTranslator struct{}

type testUmConnection struct {
	stream         pb.UMService_RegisterUMClient
	notifyTestChan chan bool
	continueChan   chan bool
	step           string
	test           *testing.T
	umID           string
	components     []*pb.SystemComponent
	conn           *grpc.ClientConn
}

/***********************************************************************************************************************
 * Init
 **********************************************************************************************************************/

func init() {
	log.SetFormatter(&log.TextFormatter{
		DisableTimestamp: false,
		TimestampFormat:  "2006-01-02 15:04:05.000",
		FullTimestamp:    true,
	})
	log.SetLevel(log.DebugLevel)
	log.SetOutput(os.Stdout)
}

/***********************************************************************************************************************
 * Tests
 **********************************************************************************************************************/

func TestConnection(t *testing.T) {
	umCtrlConfig := config.UMController{
		ServerURL: "localhost:8091",
		UMClients: []config.UMClientConfig{
			{UMID: "umID1", Priority: 10},
			{UMID: "umID2", Priority: 0},
		},
	}
	smConfig := config.Config{UMController: umCtrlConfig}

	umCtrl, err := umcontroller.New(&smConfig, &testStorage{}, &testURLTranslator{}, true)
	if err != nil {
		t.Fatalf("Can't create: UM controller %s", err)
	}

	components := []*pb.SystemComponent{
		{Id: "component1", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
		{Id: "component2", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
	}

	streamUM1, connUM1, err := createClientConnection("umID1", pb.UmState_IDLE, components)
	if err != nil {
		t.Errorf("Error connect %s", err)
	}

	components2 := []*pb.SystemComponent{
		{Id: "component3", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
		{Id: "component4", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
	}

	streamUM2, connUM2, err := createClientConnection("umID2", pb.UmState_IDLE, components2)
	if err != nil {
		t.Errorf("Error connect %s", err)
	}

	streamUM1Copy, connUM1Copy, err := createClientConnection("umID1", pb.UmState_IDLE, components)
	if err != nil {
		t.Errorf("Error connect %s", err)
	}

	newComponents, err := umCtrl.GetStatus()
	if err != nil {
		t.Errorf("Can't get system components %s", err)
	}

	if len(newComponents) != 4 {
		t.Errorf("Incorrect count of components %d", len(newComponents))
	}

	umCtrl.Close()

	_ = streamUM1.CloseSend()

	connUM1.Close()

	_ = streamUM2.CloseSend()

	connUM2.Close()

	_ = streamUM1Copy.CloseSend()

	connUM1Copy.Close()

	time.Sleep(1 * time.Second)
}

/***********************************************************************************************************************
 * Private
 **********************************************************************************************************************/

func createClientConnection(clientID string, state pb.UmState,
	components []*pb.SystemComponent) (stream pb.UMService_RegisterUMClient, conn *grpc.ClientConn, err error) {
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithInsecure())
	opts = append(opts, grpc.WithBlock())

	conn, err = grpc.Dial(serverURL, opts...)
	if err != nil {
		return stream, nil, aoserrors.Wrap(err)
	}

	client := pb.NewUMServiceClient(conn)

	stream, err = client.RegisterUM(context.Background())
	if err != nil {
		log.Errorf("Fail call RegisterUM %s", err)
		return stream, nil, aoserrors.Wrap(err)
	}

	umMsg := &pb.UpdateStatus{UmId: clientID, UmState: state, Components: components}

	if err = stream.Send(umMsg); err != nil {
		log.Errorf("Fail send update status message %s", err)
	}

	time.Sleep(1 * time.Second)

	return stream, conn, nil
}

func TestFullUpdate(t *testing.T) {
	umCtrlConfig := config.UMController{
		ServerURL: "localhost:8091",
		UMClients: []config.UMClientConfig{
			{UMID: "testUM1", Priority: 1},
			{UMID: "testUM2", Priority: 10},
		},
	}

	smConfig := config.Config{UMController: umCtrlConfig}

	var updateStorage testStorage

	umCtrl, err := umcontroller.New(&smConfig, &updateStorage, &testURLTranslator{}, true)
	if err != nil {
		t.Errorf("Can't create: UM controller %s", err)
	}

	um1Components := []*pb.SystemComponent{
		{Id: "um1C1", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
		{Id: "um1C2", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
	}

	um1 := newTestUM("testUM1", pb.UmState_IDLE, "init", um1Components, t)
	go um1.processMessages()

	um2Components := []*pb.SystemComponent{
		{Id: "um2C1", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
		{Id: "um2C2", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
	}

	um2 := newTestUM("testUM2", pb.UmState_IDLE, "init", um2Components, t)
	go um2.processMessages()

	updateComponents := []cloudprotocol.ComponentInfoFromCloud{
		{
			ID: "um1C2", VersionFromCloud: cloudprotocol.VersionFromCloud{VendorVersion: "2"},
			DecryptDataStruct: cloudprotocol.DecryptDataStruct{URLs: []string{"someFile"}},
		},
		{
			ID: "um2C1", VersionFromCloud: cloudprotocol.VersionFromCloud{VendorVersion: "2"},
			DecryptDataStruct: cloudprotocol.DecryptDataStruct{URLs: []string{"someFile"}},
		},
		{
			ID: "um2C2", VersionFromCloud: cloudprotocol.VersionFromCloud{VendorVersion: "2"},
			DecryptDataStruct: cloudprotocol.DecryptDataStruct{URLs: []string{"someFile"}},
		},
	}

	finishChannel := make(chan bool)

	go func(finChan chan bool) {
		if _, err := umCtrl.UpdateComponents(updateComponents); err != nil {
			t.Errorf("Can't update components: %s", err)
		}
		finChan <- true
	}(finishChannel)

	um1Components = append(um1Components, &pb.SystemComponent{
		Id: "um1C2", VendorVersion: "2", Status: pb.ComponentStatus_INSTALLING,
	})
	um1.setComponents(um1Components)

	um1.step = prepareStep
	um1.continueChan <- true
	<-um1.notifyTestChan // receive prepare
	um1.sendState(pb.UmState_PREPARED)

	um2Components = append(um2Components,
		&pb.SystemComponent{Id: "um2C1", VendorVersion: "2", Status: pb.ComponentStatus_INSTALLING})
	um2Components = append(um2Components,
		&pb.SystemComponent{Id: "um2C2", VendorVersion: "2", Status: pb.ComponentStatus_INSTALLING})
	um2.setComponents(um2Components)

	um2.step = prepareStep
	um2.continueChan <- true
	<-um2.notifyTestChan
	um2.sendState(pb.UmState_PREPARED)

	um1.step = updateStep
	um1.continueChan <- true
	<-um1.notifyTestChan // um1 updated
	um1.sendState(pb.UmState_UPDATED)

	um2.step = updateStep
	um2.continueChan <- true
	<-um2.notifyTestChan // um2 updated
	um2.sendState(pb.UmState_UPDATED)

	um1Components = []*pb.SystemComponent{
		{Id: "um1C1", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
		{Id: "um1C2", VendorVersion: "2", Status: pb.ComponentStatus_INSTALLED},
	}
	um1.setComponents(um1Components)

	um1.step = applyStep
	um1.continueChan <- true
	<-um1.notifyTestChan // um1 apply
	um1.sendState(pb.UmState_IDLE)

	um2Components = []*pb.SystemComponent{
		{Id: "um2C1", VendorVersion: "2", Status: pb.ComponentStatus_INSTALLED},
		{Id: "um2C2", VendorVersion: "2", Status: pb.ComponentStatus_INSTALLED},
	}
	um2.setComponents(um2Components)

	um2.step = applyStep
	um2.continueChan <- true
	<-um2.notifyTestChan // um1 apply
	um2.sendState(pb.UmState_IDLE)

	time.Sleep(1 * time.Second)

	um1.step = finishStep
	um2.step = finishStep

	<-finishChannel

	etalonComponents := []cloudprotocol.ComponentInfo{
		{ID: "um1C1", VendorVersion: "1", Status: "installed"},
		{ID: "um1C2", VendorVersion: "2", Status: "installed"},
		{ID: "um2C1", VendorVersion: "2", Status: "installed"},
		{ID: "um2C2", VendorVersion: "2", Status: "installed"},
	}

	currentComponents, err := umCtrl.GetStatus()
	if err != nil {
		t.Fatalf("Can't get components info: %s", err)
	}

	if !reflect.DeepEqual(etalonComponents, currentComponents) {
		log.Debug(currentComponents)
		t.Error("incorrect result component list")
	}

	um1.closeConnection()
	um2.closeConnection()

	<-um1.notifyTestChan
	<-um2.notifyTestChan

	umCtrl.Close()

	time.Sleep(time.Second)
}

func TestFullUpdateWithDisconnect(t *testing.T) {
	// fix the test on CI
	if os.Getenv("CI") != "" {
		t.Skip("Skipping testing in CI environment")
	}

	umCtrlConfig := config.UMController{
		ServerURL: "localhost:8091",
		UMClients: []config.UMClientConfig{
			{UMID: "testUM3", Priority: 1},
			{UMID: "testUM4", Priority: 10},
		},
	}

	smConfig := config.Config{UMController: umCtrlConfig}

	var updateStorage testStorage

	umCtrl, err := umcontroller.New(&smConfig, &updateStorage, &testURLTranslator{}, true)
	if err != nil {
		t.Errorf("Can't create: UM controller %s", err)
	}

	um3Components := []*pb.SystemComponent{
		{Id: "um3C1", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
		{Id: "um3C2", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
	}

	um3 := newTestUM("testUM3", pb.UmState_IDLE, "init", um3Components, t)
	go um3.processMessages()

	um4Components := []*pb.SystemComponent{
		{Id: "um4C1", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
		{Id: "um4C2", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
	}

	um4 := newTestUM("testUM4", pb.UmState_IDLE, "init", um4Components, t)
	go um4.processMessages()

	updateComponents := []cloudprotocol.ComponentInfoFromCloud{
		{
			ID: "um3C2", VersionFromCloud: cloudprotocol.VersionFromCloud{VendorVersion: "2"},
			DecryptDataStruct: cloudprotocol.DecryptDataStruct{URLs: []string{"someFile"}},
		},
		{
			ID: "um4C1", VersionFromCloud: cloudprotocol.VersionFromCloud{VendorVersion: "2"},
			DecryptDataStruct: cloudprotocol.DecryptDataStruct{URLs: []string{"someFile"}},
		},
		{
			ID: "um4C2", VersionFromCloud: cloudprotocol.VersionFromCloud{VendorVersion: "2"},
			DecryptDataStruct: cloudprotocol.DecryptDataStruct{URLs: []string{"someFile"}},
		},
	}

	finishChannel := make(chan bool)

	go func() {
		if _, err := umCtrl.UpdateComponents(updateComponents); err != nil {
			t.Errorf("Can't update components: %s", err)
		}

		close(finishChannel)
	}()

	// prepare UM3
	um3Components = append(um3Components,
		&pb.SystemComponent{Id: "um3C2", VendorVersion: "2", Status: pb.ComponentStatus_INSTALLING})
	um3.setComponents(um3Components)

	um3.step = prepareStep
	um3.continueChan <- true
	<-um3.notifyTestChan // receive prepare
	um3.sendState(pb.UmState_PREPARED)

	// prepare UM4
	um4Components = append(um4Components,
		&pb.SystemComponent{Id: "um4C1", VendorVersion: "2", Status: pb.ComponentStatus_INSTALLING})
	um4Components = append(um4Components,
		&pb.SystemComponent{Id: "um4C2", VendorVersion: "2", Status: pb.ComponentStatus_INSTALLING})
	um4.setComponents(um4Components)

	um4.step = prepareStep
	um4.continueChan <- true
	<-um4.notifyTestChan
	um4.sendState(pb.UmState_PREPARED)

	um3.step = updateStep
	um3.continueChan <- true
	<-um3.notifyTestChan

	// um3  reboot
	um3.step = rebootStep
	um3.closeConnection()
	<-um3.notifyTestChan

	um3 = newTestUM("testUM3", pb.UmState_UPDATED, applyStep, um3Components, t)
	go um3.processMessages()

	um4.step = updateStep
	um4.continueChan <- true
	<-um4.notifyTestChan
	// full reboot
	um4.step = rebootStep
	um4.closeConnection()

	<-um4.notifyTestChan

	um4 = newTestUM("testUM4", pb.UmState_UPDATED, applyStep, um4Components, t)
	go um4.processMessages()

	um3.step = applyStep
	um3.continueChan <- true
	<-um3.notifyTestChan

	// um3  reboot
	um3.step = rebootStep
	um3.closeConnection()
	<-um3.notifyTestChan

	um3Components = []*pb.SystemComponent{
		{Id: "um3C1", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
		{Id: "um3C2", VendorVersion: "2", Status: pb.ComponentStatus_INSTALLED},
	}

	um3 = newTestUM("testUM3", pb.UmState_IDLE, "init", um3Components, t)
	go um3.processMessages()

	// um4  reboot
	um4.step = rebootStep
	um4.closeConnection()
	<-um4.notifyTestChan

	um4Components = []*pb.SystemComponent{
		{Id: "um4C1", VendorVersion: "2", Status: pb.ComponentStatus_INSTALLED},
		{Id: "um4C2", VendorVersion: "2", Status: pb.ComponentStatus_INSTALLED},
	}

	um4 = newTestUM("testUM4", pb.UmState_IDLE, "init", um4Components, t)
	go um4.processMessages()

	um3.step = finishStep
	um4.step = finishStep

	<-finishChannel

	etalonComponents := []cloudprotocol.ComponentInfo{
		{ID: "um3C1", VendorVersion: "1", Status: "installed"},
		{ID: "um3C2", VendorVersion: "2", Status: "installed"},
		{ID: "um4C1", VendorVersion: "2", Status: "installed"},
		{ID: "um4C2", VendorVersion: "2", Status: "installed"},
	}

	currentComponents, err := umCtrl.GetStatus()
	if err != nil {
		t.Fatalf("Can't get components info: %s", err)
	}

	if !reflect.DeepEqual(etalonComponents, currentComponents) {
		log.Debug(currentComponents)
		t.Error("incorrect result component list")
	}

	um3.closeConnection()
	um4.closeConnection()

	<-um3.notifyTestChan
	<-um4.notifyTestChan

	umCtrl.Close()

	time.Sleep(time.Second)
}

func TestFullUpdateWithReboot(t *testing.T) {
	umCtrlConfig := config.UMController{
		ServerURL: "localhost:8091",
		UMClients: []config.UMClientConfig{
			{UMID: "testUM5", Priority: 1},
			{UMID: "testUM6", Priority: 10},
		},
	}

	smConfig := config.Config{UMController: umCtrlConfig}

	var updateStorage testStorage

	umCtrl, err := umcontroller.New(&smConfig, &updateStorage, &testURLTranslator{}, true)
	if err != nil {
		t.Errorf("Can't create: UM controller %s", err)
	}

	um5Components := []*pb.SystemComponent{
		{Id: "um5C1", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
		{Id: "um5C2", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
	}

	um5 := newTestUM("testUM5", pb.UmState_IDLE, "init", um5Components, t)
	go um5.processMessages()

	um6Components := []*pb.SystemComponent{
		{Id: "um6C1", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
		{Id: "um6C2", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
	}

	um6 := newTestUM("testUM6", pb.UmState_IDLE, "init", um6Components, t)
	go um6.processMessages()

	updateComponents := []cloudprotocol.ComponentInfoFromCloud{
		{
			ID: "um5C2", VersionFromCloud: cloudprotocol.VersionFromCloud{VendorVersion: "2"},
			DecryptDataStruct: cloudprotocol.DecryptDataStruct{URLs: []string{"someFile"}},
		},
		{
			ID: "um6C1", VersionFromCloud: cloudprotocol.VersionFromCloud{VendorVersion: "2"},
			DecryptDataStruct: cloudprotocol.DecryptDataStruct{URLs: []string{"someFile"}},
		},
		{
			ID: "um6C2", VersionFromCloud: cloudprotocol.VersionFromCloud{VendorVersion: "2"},
			DecryptDataStruct: cloudprotocol.DecryptDataStruct{URLs: []string{"someFile"}},
		},
	}

	finishChannel := make(chan bool)

	go func() {
		if _, err := umCtrl.UpdateComponents(updateComponents); err != nil {
			t.Errorf("Can't update components: %s", err)
		}

		close(finishChannel)
	}()

	// prepare UM5
	um5Components = append(um5Components,
		&pb.SystemComponent{Id: "um5C2", VendorVersion: "2", Status: pb.ComponentStatus_INSTALLING})
	um5.setComponents(um5Components)

	um5.step = prepareStep
	um5.continueChan <- true
	<-um5.notifyTestChan // receive prepare
	um5.sendState(pb.UmState_PREPARED)

	// prepare UM6
	um6Components = append(um6Components,
		&pb.SystemComponent{Id: "um6C1", VendorVersion: "2", Status: pb.ComponentStatus_INSTALLING})
	um6Components = append(um6Components,
		&pb.SystemComponent{Id: "um6C2", VendorVersion: "2", Status: pb.ComponentStatus_INSTALLING})
	um6.setComponents(um6Components)

	um6.step = prepareStep
	um6.continueChan <- true
	<-um6.notifyTestChan
	um6.sendState(pb.UmState_PREPARED)

	um5.step = updateStep
	um5.continueChan <- true
	<-um5.notifyTestChan

	// full reboot
	um5.step = rebootStep
	um6.step = rebootStep

	um5.closeConnection()
	um6.closeConnection()
	umCtrl.Close()

	<-um5.notifyTestChan
	<-um6.notifyTestChan
	<-finishChannel

	umCtrl, err = umcontroller.New(&smConfig, &updateStorage, &testURLTranslator{}, true)
	if err != nil {
		t.Errorf("Can't create: UM controller %s", err)
	}

	um5 = newTestUM("testUM5", pb.UmState_UPDATED, applyStep, um5Components, t)
	go um5.processMessages()

	um6 = newTestUM("testUM6", pb.UmState_PREPARED, updateStep, um6Components, t)
	go um6.processMessages()

	um6.continueChan <- true
	<-um6.notifyTestChan

	um6.step = rebootStep
	um6.closeConnection()

	um6 = newTestUM("testUM6", pb.UmState_UPDATED, applyStep, um6Components, t)
	go um6.processMessages()

	// um5 apply and full reboot
	um5.continueChan <- true
	<-um5.notifyTestChan

	// full reboot
	um5.step = rebootStep
	um6.step = rebootStep

	um5.closeConnection()
	um6.closeConnection()
	umCtrl.Close()

	<-um5.notifyTestChan
	<-um6.notifyTestChan

	umCtrl, err = umcontroller.New(&smConfig, &updateStorage, &testURLTranslator{}, true)
	if err != nil {
		t.Errorf("Can't create: UM controller %s", err)
	}

	um5Components = []*pb.SystemComponent{
		{Id: "um5C1", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
		{Id: "um5C2", VendorVersion: "2", Status: pb.ComponentStatus_INSTALLED},
	}

	um5 = newTestUM("testUM5", pb.UmState_IDLE, "init", um5Components, t)
	go um5.processMessages()

	um6 = newTestUM("testUM6", pb.UmState_UPDATED, applyStep, um6Components, t)
	go um6.processMessages()

	um6.step = rebootStep
	um6.closeConnection()
	<-um6.notifyTestChan

	um6Components = []*pb.SystemComponent{
		{Id: "um6C1", VendorVersion: "2", Status: pb.ComponentStatus_INSTALLED},
		{Id: "um6C2", VendorVersion: "2", Status: pb.ComponentStatus_INSTALLED},
	}

	um6 = newTestUM("testUM6", pb.UmState_IDLE, "init", um6Components, t)
	go um6.processMessages()

	um5.step = finishStep
	um6.step = finishStep

	etalonComponents := []cloudprotocol.ComponentInfo{
		{ID: "um5C1", VendorVersion: "1", Status: "installed"},
		{ID: "um5C2", VendorVersion: "2", Status: "installed"},
		{ID: "um6C1", VendorVersion: "2", Status: "installed"},
		{ID: "um6C2", VendorVersion: "2", Status: "installed"},
	}

	currentComponents, err := umCtrl.GetStatus()
	if err != nil {
		t.Fatalf("Can't get components info: %s", err)
	}

	if !reflect.DeepEqual(etalonComponents, currentComponents) {
		log.Debug(currentComponents)
		t.Error("incorrect result component list")
	}

	um5.closeConnection()
	um6.closeConnection()

	<-um5.notifyTestChan
	<-um6.notifyTestChan

	umCtrl.Close()

	time.Sleep(time.Second)
}

func TestRevertOnPrepare(t *testing.T) {
	umCtrlConfig := config.UMController{
		ServerURL: "localhost:8091",
		UMClients: []config.UMClientConfig{
			{UMID: "testUM7", Priority: 1},
			{UMID: "testUM8", Priority: 10},
		},
	}

	smConfig := config.Config{UMController: umCtrlConfig}

	var updateStorage testStorage

	umCtrl, err := umcontroller.New(&smConfig, &updateStorage, &testURLTranslator{}, true)
	if err != nil {
		t.Errorf("Can't create: UM controller %s", err)
	}

	um7Components := []*pb.SystemComponent{
		{Id: "um7C1", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
		{Id: "um7C2", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
	}

	um7 := newTestUM("testUM7", pb.UmState_IDLE, "init", um7Components, t)
	go um7.processMessages()

	um8Components := []*pb.SystemComponent{
		{Id: "um8C1", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
		{Id: "um8C2", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
	}

	um8 := newTestUM("testUM8", pb.UmState_IDLE, "init", um8Components, t)
	go um8.processMessages()

	updateComponents := []cloudprotocol.ComponentInfoFromCloud{
		{
			ID: "um7C2", VersionFromCloud: cloudprotocol.VersionFromCloud{VendorVersion: "2"},
			DecryptDataStruct: cloudprotocol.DecryptDataStruct{URLs: []string{"someFile"}},
		},
		{
			ID: "um8C1", VersionFromCloud: cloudprotocol.VersionFromCloud{VendorVersion: "2"},
			DecryptDataStruct: cloudprotocol.DecryptDataStruct{URLs: []string{"someFile"}},
		},
		{
			ID: "um8C2", VersionFromCloud: cloudprotocol.VersionFromCloud{VendorVersion: "2"},
			DecryptDataStruct: cloudprotocol.DecryptDataStruct{URLs: []string{"someFile"}},
		},
	}

	finishChannel := make(chan bool)

	go func() {
		if _, err := umCtrl.UpdateComponents(updateComponents); err == nil {
			t.Errorf("Should fail")
		}

		close(finishChannel)
	}()

	um7Components = append(um7Components,
		&pb.SystemComponent{Id: "um7C2", VendorVersion: "2", Status: pb.ComponentStatus_INSTALLING})
	um7.setComponents(um7Components)

	um7.step = prepareStep
	um7.continueChan <- true
	<-um7.notifyTestChan // receive prepare
	um7.sendState(pb.UmState_PREPARED)

	um8Components = append(um8Components,
		&pb.SystemComponent{Id: "um8C1", VendorVersion: "2", Status: pb.ComponentStatus_INSTALLING})
	um8Components = append(um8Components,
		&pb.SystemComponent{Id: "um8C2", VendorVersion: "2", Status: pb.ComponentStatus_ERROR})
	um8.setComponents(um8Components)

	um8.step = prepareStep
	um8.continueChan <- true
	<-um8.notifyTestChan
	um8.sendState(pb.UmState_FAILED)

	um7.step = revertStep
	um7.continueChan <- true
	<-um7.notifyTestChan // um7 revert received
	um7.sendState(pb.UmState_IDLE)

	um8.step = revertStep
	um8.continueChan <- true
	<-um8.notifyTestChan // um8 revert received
	um8.sendState(pb.UmState_IDLE)

	um7.step = finishStep
	um8.step = finishStep

	<-finishChannel

	etalonComponents := []cloudprotocol.ComponentInfo{
		{ID: "um7C1", VendorVersion: "1", Status: "installed"},
		{ID: "um7C2", VendorVersion: "1", Status: "installed"},
		{ID: "um8C1", VendorVersion: "1", Status: "installed"},
		{ID: "um8C2", VendorVersion: "1", Status: "installed"},
		{ID: "um8C2", VendorVersion: "2", Status: "error"},
	}

	currentComponents, err := umCtrl.GetStatus()
	if err != nil {
		t.Fatalf("Can't get components info: %s", err)
	}

	if !reflect.DeepEqual(etalonComponents, currentComponents) {
		log.Debug(currentComponents)
		t.Error("incorrect result component list")
		log.Debug(etalonComponents)
	}

	um7.closeConnection()
	um8.closeConnection()

	<-um7.notifyTestChan
	<-um8.notifyTestChan

	umCtrl.Close()

	time.Sleep(time.Second)
}

func TestRevertOnUpdate(t *testing.T) {
	umCtrlConfig := config.UMController{
		ServerURL: "localhost:8091",
		UMClients: []config.UMClientConfig{
			{UMID: "testUM9", Priority: 1},
			{UMID: "testUM10", Priority: 10},
		},
	}

	smConfig := config.Config{UMController: umCtrlConfig}

	var updateStorage testStorage

	umCtrl, err := umcontroller.New(&smConfig, &updateStorage, &testURLTranslator{}, true)
	if err != nil {
		t.Errorf("Can't create: UM controller %s", err)
	}

	um9Components := []*pb.SystemComponent{
		{Id: "um9C1", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
		{Id: "um9C2", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
	}

	um9 := newTestUM("testUM9", pb.UmState_IDLE, "init", um9Components, t)
	go um9.processMessages()

	um10Components := []*pb.SystemComponent{
		{Id: "um10C1", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
		{Id: "um10C2", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
	}

	um10 := newTestUM("testUM10", pb.UmState_IDLE, "init", um10Components, t)
	go um10.processMessages()

	updateComponents := []cloudprotocol.ComponentInfoFromCloud{
		{
			ID: "um9C2", VersionFromCloud: cloudprotocol.VersionFromCloud{VendorVersion: "2"},
			DecryptDataStruct: cloudprotocol.DecryptDataStruct{URLs: []string{"someFile"}},
		},
		{
			ID: "um10C1", VersionFromCloud: cloudprotocol.VersionFromCloud{VendorVersion: "2"},
			DecryptDataStruct: cloudprotocol.DecryptDataStruct{URLs: []string{"someFile"}},
		},
		{
			ID: "um10C2", VersionFromCloud: cloudprotocol.VersionFromCloud{VendorVersion: "2"},
			DecryptDataStruct: cloudprotocol.DecryptDataStruct{URLs: []string{"someFile"}},
		},
	}

	finishChannel := make(chan bool)

	go func() {
		if _, err := umCtrl.UpdateComponents(updateComponents); err == nil {
			t.Errorf("Should fail")
		}

		close(finishChannel)
	}()

	um9Components = append(um9Components,
		&pb.SystemComponent{Id: "um9C2", VendorVersion: "2", Status: pb.ComponentStatus_INSTALLING})
	um9.setComponents(um9Components)

	um9.step = prepareStep
	um9.continueChan <- true
	<-um9.notifyTestChan // receive prepare
	um9.sendState(pb.UmState_PREPARED)

	um10Components = append(um10Components,
		&pb.SystemComponent{Id: "um10C1", VendorVersion: "2", Status: pb.ComponentStatus_INSTALLING})
	um10Components = append(um10Components,
		&pb.SystemComponent{Id: "um10C2", VendorVersion: "2", Status: pb.ComponentStatus_INSTALLING})
	um10.setComponents(um10Components)

	um10.step = prepareStep
	um10.continueChan <- true
	<-um10.notifyTestChan
	um10.sendState(pb.UmState_PREPARED)

	um9.step = updateStep
	um9.continueChan <- true
	<-um9.notifyTestChan // um9 updated
	um9.sendState(pb.UmState_UPDATED)

	um10Components = []*pb.SystemComponent{
		{Id: "um10C1", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
		{Id: "um10C2", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
		{Id: "um10C2", VendorVersion: "2", Status: pb.ComponentStatus_INSTALLING},
		{Id: "um10C2", VendorVersion: "2", Status: pb.ComponentStatus_ERROR},
	}
	um10.setComponents(um10Components)

	um10.step = updateStep
	um10.continueChan <- true
	<-um10.notifyTestChan // um10 updated
	um10.sendState(pb.UmState_FAILED)

	um9Components = []*pb.SystemComponent{
		{Id: "um9C1", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
		{Id: "um9C2", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
	}
	um9.setComponents(um9Components)

	um9.step = revertStep
	um9.continueChan <- true
	<-um9.notifyTestChan // um9 revert received
	um9.sendState(pb.UmState_IDLE)

	um10.step = revertStep
	um10.continueChan <- true
	<-um10.notifyTestChan // um10 revert received
	um10.sendState(pb.UmState_IDLE)

	um9.step = finishStep
	um10.step = finishStep

	<-finishChannel

	etalonComponents := []cloudprotocol.ComponentInfo{
		{ID: "um9C1", VendorVersion: "1", Status: "installed"},
		{ID: "um9C2", VendorVersion: "1", Status: "installed"},
		{ID: "um10C1", VendorVersion: "1", Status: "installed"},
		{ID: "um10C2", VendorVersion: "1", Status: "installed"},
		{ID: "um10C2", VendorVersion: "2", Status: "error"},
	}

	currentComponents, err := umCtrl.GetStatus()
	if err != nil {
		t.Fatalf("Can't get components info: %s", err)
	}

	if !reflect.DeepEqual(etalonComponents, currentComponents) {
		log.Debug(currentComponents)
		t.Error("incorrect result component list")
		log.Debug(etalonComponents)
	}

	um9.closeConnection()
	um10.closeConnection()

	<-um9.notifyTestChan
	<-um10.notifyTestChan

	umCtrl.Close()

	time.Sleep(time.Second)
}

func TestRevertOnUpdateWithDisconnect(t *testing.T) {
	umCtrlConfig := config.UMController{
		ServerURL: "localhost:8091",
		UMClients: []config.UMClientConfig{
			{UMID: "testUM11", Priority: 1},
			{UMID: "testUM12", Priority: 10},
		},
	}

	smConfig := config.Config{UMController: umCtrlConfig}

	var updateStorage testStorage

	umCtrl, err := umcontroller.New(&smConfig, &updateStorage, &testURLTranslator{}, true)
	if err != nil {
		t.Errorf("Can't create: UM controller %s", err)
	}

	um11Components := []*pb.SystemComponent{
		{Id: "um11C1", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
		{Id: "um11C2", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
	}

	um11 := newTestUM("testUM11", pb.UmState_IDLE, "init", um11Components, t)
	go um11.processMessages()

	um12Components := []*pb.SystemComponent{
		{Id: "um12C1", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
		{Id: "um12C2", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
	}

	um12 := newTestUM("testUM12", pb.UmState_IDLE, "init", um12Components, t)
	go um12.processMessages()

	updateComponents := []cloudprotocol.ComponentInfoFromCloud{
		{
			ID: "um11C2", VersionFromCloud: cloudprotocol.VersionFromCloud{VendorVersion: "2"},
			DecryptDataStruct: cloudprotocol.DecryptDataStruct{URLs: []string{"someFile"}},
		},
		{
			ID: "um12C1", VersionFromCloud: cloudprotocol.VersionFromCloud{VendorVersion: "2"},
			DecryptDataStruct: cloudprotocol.DecryptDataStruct{URLs: []string{"someFile"}},
		},
		{
			ID: "um12C2", VersionFromCloud: cloudprotocol.VersionFromCloud{VendorVersion: "2"},
			DecryptDataStruct: cloudprotocol.DecryptDataStruct{URLs: []string{"someFile"}},
		},
	}

	finishChannel := make(chan bool)

	go func() {
		if _, err := umCtrl.UpdateComponents(updateComponents); err != nil {
			t.Errorf("Can't update components: %s", err)
		}

		close(finishChannel)
	}()

	um11Components = append(um11Components,
		&pb.SystemComponent{Id: "um11C2", VendorVersion: "2", Status: pb.ComponentStatus_INSTALLING})
	um11.setComponents(um11Components)

	um11.step = prepareStep
	um11.continueChan <- true
	<-um11.notifyTestChan // receive prepare
	um11.sendState(pb.UmState_PREPARED)

	um12Components = append(um12Components,
		&pb.SystemComponent{Id: "um12C1", VendorVersion: "2", Status: pb.ComponentStatus_INSTALLING})
	um12Components = append(um12Components,
		&pb.SystemComponent{Id: "um12C2", VendorVersion: "2", Status: pb.ComponentStatus_INSTALLING})
	um12.setComponents(um12Components)

	um12.step = prepareStep
	um12.continueChan <- true
	<-um12.notifyTestChan
	um12.sendState(pb.UmState_PREPARED)

	um11.step = updateStep
	um11.continueChan <- true
	<-um11.notifyTestChan // um11 updated
	um11.sendState(pb.UmState_UPDATED)

	um12Components = []*pb.SystemComponent{
		{Id: "um12C1", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
		{Id: "um12C2", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
		{Id: "um12C2", VendorVersion: "2", Status: pb.ComponentStatus_ERROR},
	}
	um12.setComponents(um12Components)

	um12.step = updateStep
	um12.continueChan <- true
	<-um12.notifyTestChan // um12 updated
	// um12  reboot
	um12.step = rebootStep
	um12.closeConnection()
	<-um12.notifyTestChan

	um12 = newTestUM("testUM12", pb.UmState_FAILED, revertStep, um12Components, t)
	go um12.processMessages()

	um11Components = []*pb.SystemComponent{
		{Id: "um11C1", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
		{Id: "um11C2", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
	}
	um11.setComponents(um11Components)

	um11.step = revertStep
	um11.continueChan <- true
	<-um11.notifyTestChan // um11 revert received
	um11.sendState(pb.UmState_IDLE)

	um12.step = revertStep
	um12.continueChan <- true
	<-um12.notifyTestChan // um12 revert received
	um12.sendState(pb.UmState_IDLE)

	um11.step = finishStep
	um12.step = finishStep

	<-finishChannel

	etalonComponents := []cloudprotocol.ComponentInfo{
		{ID: "um11C1", VendorVersion: "1", Status: "installed"},
		{ID: "um11C2", VendorVersion: "1", Status: "installed"},
		{ID: "um12C1", VendorVersion: "1", Status: "installed"},
		{ID: "um12C2", VendorVersion: "1", Status: "installed"},
		{ID: "um12C2", VendorVersion: "2", Status: "error"},
	}

	currentComponents, err := umCtrl.GetStatus()
	if err != nil {
		t.Fatalf("Can't get components info: %s", err)
	}

	if !reflect.DeepEqual(etalonComponents, currentComponents) {
		log.Debug(currentComponents)
		t.Error("incorrect result component list")
		log.Debug(etalonComponents)
	}

	um11.closeConnection()
	um12.closeConnection()

	<-um11.notifyTestChan
	<-um12.notifyTestChan

	umCtrl.Close()

	time.Sleep(time.Second)
}

func TestRevertOnUpdateWithReboot(t *testing.T) {
	umCtrlConfig := config.UMController{
		ServerURL: "localhost:8091",
		UMClients: []config.UMClientConfig{
			{UMID: "testUM13", Priority: 1, IsLocal: true},
			{UMID: "testUM14", Priority: 10},
		},
	}

	smConfig := config.Config{UMController: umCtrlConfig}

	var updateStorage testStorage

	umCtrl, err := umcontroller.New(&smConfig, &updateStorage, &testURLTranslator{}, true)
	if err != nil {
		t.Errorf("Can't create: UM controller %s", err)
	}

	um13Components := []*pb.SystemComponent{
		{Id: "um13C1", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
		{Id: "um13C2", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
	}

	um13 := newTestUM("testUM13", pb.UmState_IDLE, "init", um13Components, t)
	go um13.processMessages()

	um14Components := []*pb.SystemComponent{
		{Id: "um14C1", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
		{Id: "um14C2", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
	}

	um14 := newTestUM("testUM14", pb.UmState_IDLE, "init", um14Components, t)
	go um14.processMessages()

	updateComponents := []cloudprotocol.ComponentInfoFromCloud{
		{
			ID: "um13C2", VersionFromCloud: cloudprotocol.VersionFromCloud{VendorVersion: "2"},
			DecryptDataStruct: cloudprotocol.DecryptDataStruct{URLs: []string{"someFile"}},
		},
		{
			ID: "um14C1", VersionFromCloud: cloudprotocol.VersionFromCloud{VendorVersion: "2"},
			DecryptDataStruct: cloudprotocol.DecryptDataStruct{URLs: []string{"someFile"}},
		},
		{
			ID: "um14C2", VersionFromCloud: cloudprotocol.VersionFromCloud{VendorVersion: "2"},
			DecryptDataStruct: cloudprotocol.DecryptDataStruct{URLs: []string{"someFile"}},
		},
	}

	finishChannel := make(chan bool)

	go func() {
		if _, err := umCtrl.UpdateComponents(updateComponents); err != nil {
			t.Errorf("Can't update components: %s", err)
		}

		close(finishChannel)
	}()

	um13Components = append(um13Components,
		&pb.SystemComponent{Id: "um13C2", VendorVersion: "2", Status: pb.ComponentStatus_INSTALLING})
	um13.setComponents(um13Components)

	um13.step = prepareStep
	um13.continueChan <- true
	<-um13.notifyTestChan // receive prepare
	um13.sendState(pb.UmState_PREPARED)

	um14Components = append(um14Components,
		&pb.SystemComponent{Id: "um14C1", VendorVersion: "2", Status: pb.ComponentStatus_INSTALLING})
	um14Components = append(um14Components,
		&pb.SystemComponent{Id: "um14C2", VendorVersion: "2", Status: pb.ComponentStatus_INSTALLING})
	um14.setComponents(um14Components)

	um14.step = prepareStep
	um14.continueChan <- true
	<-um14.notifyTestChan
	um14.sendState(pb.UmState_PREPARED)

	um13.step = updateStep
	um13.continueChan <- true
	<-um13.notifyTestChan // um13 updated
	um13.sendState(pb.UmState_UPDATED)

	um14Components = []*pb.SystemComponent{
		{Id: "um14C1", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
		{Id: "um14C2", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
		{Id: "um14C2", VendorVersion: "2", Status: pb.ComponentStatus_ERROR},
	}
	um14.setComponents(um14Components)

	um14.step = updateStep
	um14.continueChan <- true
	<-um14.notifyTestChan // um14 updated

	// full reboot
	um13.step = rebootStep
	um14.step = rebootStep

	um13.closeConnection()
	um14.closeConnection()
	umCtrl.Close()

	<-um13.notifyTestChan
	<-um14.notifyTestChan
	<-finishChannel
	// um14  reboot

	umCtrl, err = umcontroller.New(&smConfig, &updateStorage, &testURLTranslator{}, true)
	if err != nil {
		t.Errorf("Can't create: UM controller %s", err)
	}

	um13 = newTestUM("testUM13", pb.UmState_UPDATED, revertStep, um13Components, t)
	go um13.processMessages()

	um14 = newTestUM("testUM14", pb.UmState_FAILED, revertStep, um14Components, t)
	go um14.processMessages()

	um13Components = []*pb.SystemComponent{
		{Id: "um13C1", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
		{Id: "um13C2", VendorVersion: "1", Status: pb.ComponentStatus_INSTALLED},
	}
	um13.setComponents(um13Components)

	um13.step = revertStep
	um13.continueChan <- true
	<-um13.notifyTestChan // um13 revert received
	um13.sendState(pb.UmState_IDLE)

	um14.step = revertStep
	um14.continueChan <- true
	<-um14.notifyTestChan // um14 revert received
	um14.sendState(pb.UmState_IDLE)

	um13.step = finishStep
	um14.step = finishStep

	time.Sleep(time.Second)

	etalonComponents := []cloudprotocol.ComponentInfo{
		{ID: "um13C1", VendorVersion: "1", Status: "installed"},
		{ID: "um13C2", VendorVersion: "1", Status: "installed"},
		{ID: "um14C1", VendorVersion: "1", Status: "installed"},
		{ID: "um14C2", VendorVersion: "1", Status: "installed"},
		{ID: "um14C2", VendorVersion: "2", Status: "error"},
	}

	currentComponents, err := umCtrl.GetStatus()
	if err != nil {
		t.Fatalf("Can't get components info: %s", err)
	}

	if !reflect.DeepEqual(etalonComponents, currentComponents) {
		log.Debug(currentComponents)
		t.Error("incorrect result component list")
		log.Debug(etalonComponents)
	}

	um13.closeConnection()
	um14.closeConnection()

	<-um13.notifyTestChan
	<-um14.notifyTestChan

	umCtrl.Close()

	time.Sleep(time.Second)
}

/*******************************************************************************
 * Interfaces
 ******************************************************************************/

func (storage *testStorage) GetComponentsUpdateInfo() (updateInfo []umcontroller.SystemComponent, err error) {
	return storage.updateInfo, err
}

func (storage *testStorage) SetComponentsUpdateInfo(updateInfo []umcontroller.SystemComponent) (err error) {
	storage.updateInfo = updateInfo
	return aoserrors.Wrap(err)
}

func (um *testUmConnection) processMessages() {
	defer func() { um.notifyTestChan <- true }()

	for {
		<-um.continueChan

		msg, err := um.stream.Recv()
		if err != nil {
			return
		}

		switch um.step {
		case finishStep:
			fallthrough

		case rebootStep:
			if errors.Is(err, io.EOF) {
				log.Debug("[test] End of connection ", um.umID)
				return
			}

			if err != nil {
				log.Debug("[test] End of connection with error ", err, um.umID)
				return
			}

		case prepareStep:
			if msg.GetPrepareUpdate() == nil {
				um.test.Error("Expect prepare update request ", um.umID)
			}

		case updateStep:
			if msg.GetStartUpdate() == nil {
				um.test.Error("Expect start update ", um.umID)
			}

		case applyStep:
			if msg.GetApplyUpdate() == nil {
				um.test.Error("Expect apply update ", um.umID)
			}

		case revertStep:
			if msg.GetRevertUpdate() == nil {
				um.test.Error("Expect revert update ", um.umID)
			}

		default:
			um.test.Error("unexpected message at step", um.step)
		}
		um.notifyTestChan <- true
	}
}

/*******************************************************************************
 * Private
 ******************************************************************************/

func newTestUM(id string, umState pb.UmState, testState string, components []*pb.SystemComponent, t *testing.T) (
	umTest *testUmConnection) {
	stream, conn, err := createClientConnection(id, umState, components)
	if err != nil {
		t.Errorf("Error connect %s", err)
		return umTest
	}

	umTest = &testUmConnection{
		notifyTestChan: make(chan bool),
		continueChan:   make(chan bool),
		step:           testState,
		test:           t,
		stream:         stream,
		umID:           id,
		conn:           conn,
		components:     components,
	}

	return umTest
}

func (um *testUmConnection) setComponents(components []*pb.SystemComponent) {
	um.components = components
}

func (um *testUmConnection) sendState(state pb.UmState) {
	umMsg := &pb.UpdateStatus{UmId: um.umID, UmState: state, Components: um.components}

	if err := um.stream.Send(umMsg); err != nil {
		um.test.Errorf("Fail send update status message %s", err)
	}
}

func (um *testUmConnection) closeConnection() {
	um.continueChan <- true
	um.conn.Close()
	_ = um.stream.CloseSend()
}

func (translator *testURLTranslator) TranslateURL(isLocal bool, inURL string) (outURL string, err error) {
	return "file://" + inURL, nil
}
