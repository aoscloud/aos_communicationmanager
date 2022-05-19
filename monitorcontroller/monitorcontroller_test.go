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

package monitorcontroller_test

import (
	"os"
	"testing"
	"time"

	"github.com/aoscloud/aos_common/api/cloudprotocol"
	"github.com/aoscloud/aos_communicationmanager/config"
	"github.com/aoscloud/aos_communicationmanager/monitorcontroller"
	log "github.com/sirupsen/logrus"
)

/***********************************************************************************************************************
 * Types
 **********************************************************************************************************************/

type testMonitoringSender struct {
	monitoringData chan cloudprotocol.MonitoringData
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
 * Main
 **********************************************************************************************************************/

func TestMain(m *testing.M) {
	ret := m.Run()

	os.Exit(ret)
}

/***********************************************************************************************************************
 * Tests
 **********************************************************************************************************************/

func TestSendMonitorData(t *testing.T) {
	duration := 100 * time.Millisecond

	sender := &testMonitoringSender{
		monitoringData: make(chan cloudprotocol.MonitoringData),
	}

	controller, err := monitorcontroller.New(&config.Config{
		Monitoring: config.Monitoring{
			MaxOfflineMessages: 25,
		},
	}, sender)
	if err != nil {
		t.Fatalf("Can't create monitoring controller: %v", err)
	}

	monitoringData := cloudprotocol.MonitoringData{
		Global: cloudprotocol.GlobalMonitoringData{
			RAM:        1100,
			CPU:        35,
			UsedDisk:   2300,
			InTraffic:  150,
			OutTraffic: 150,
		},
	}

	controller.SendMonitoringData(monitoringData)

	select {
	case receivedMonitoringData := <-sender.monitoringData:
		if receivedMonitoringData.Global != monitoringData.Global {
			t.Errorf("Incorrect system monitoring data: %v", receivedMonitoringData.Global)
		}

	case <-time.After(duration * 2):
		t.Fatal("Monitoring data timeout")
	}

	controller.Close()
}

/***********************************************************************************************************************
 * Interfaces
 **********************************************************************************************************************/

func (sender *testMonitoringSender) SendMonitoringData(monitoringData cloudprotocol.MonitoringData) (err error) {
	sender.monitoringData <- monitoringData

	return nil
}
