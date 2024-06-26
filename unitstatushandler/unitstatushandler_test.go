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

package unitstatushandler_test

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/aosedge/aos_common/aoserrors"
	"github.com/aosedge/aos_common/aostypes"
	"github.com/aosedge/aos_common/api/cloudprotocol"

	"github.com/aosedge/aos_communicationmanager/config"
	"github.com/aosedge/aos_communicationmanager/unitstatushandler"
)

/***********************************************************************************************************************
 * Consts
 **********************************************************************************************************************/

const (
	waitStatusTimeout      = 5 * time.Second
	waitRunInstanceTimeout = 5 * time.Second
)

/***********************************************************************************************************************
 * Vars
 **********************************************************************************************************************/

var cfg = &config.Config{UnitStatusSendTimeout: aostypes.Duration{Duration: 3 * time.Second}}

/***********************************************************************************************************************
 * Tests
 **********************************************************************************************************************/

func TestSendInitialStatus(t *testing.T) {
	expectedUnitStatus := cloudprotocol.UnitStatus{
		UnitSubjects: []string{"subject1"},
		UnitConfig: []cloudprotocol.UnitConfigStatus{
			{VendorVersion: "1.0", Status: cloudprotocol.InstalledStatus},
		},
		Components: []cloudprotocol.ComponentStatus{
			{ID: "comp0", VendorVersion: "1.0", Status: cloudprotocol.InstalledStatus},
			{ID: "comp1", VendorVersion: "1.1", Status: cloudprotocol.InstalledStatus},
			{ID: "comp2", VendorVersion: "1.2", Status: cloudprotocol.InstalledStatus},
		},
		Layers: []cloudprotocol.LayerStatus{
			{ID: "layer0", Digest: "digest0", AosVersion: 1, Status: cloudprotocol.InstalledStatus},
			{ID: "layer1", Digest: "digest1", AosVersion: 2, Status: cloudprotocol.InstalledStatus},
			{ID: "layer2", Digest: "digest2", AosVersion: 3, Status: cloudprotocol.InstalledStatus},
		},
		Services: []cloudprotocol.ServiceStatus{
			{ID: "service0", AosVersion: 1, Status: cloudprotocol.InstalledStatus},
			{ID: "service1", AosVersion: 1, Status: cloudprotocol.InstalledStatus},
			{ID: "service2", AosVersion: 1, Status: cloudprotocol.InstalledStatus},
		},
	}

	initialServices := []unitstatushandler.ServiceStatus{
		{ServiceStatus: cloudprotocol.ServiceStatus{
			ID: "service0", AosVersion: 1, Status: cloudprotocol.InstalledStatus,
		}},

		{ServiceStatus: cloudprotocol.ServiceStatus{
			ID: "service1", AosVersion: 1, Status: cloudprotocol.InstalledStatus,
		}},
		{ServiceStatus: cloudprotocol.ServiceStatus{
			ID: "service2", AosVersion: 1, Status: cloudprotocol.InstalledStatus,
		}},
		{
			ServiceStatus: cloudprotocol.ServiceStatus{
				ID: "service3", AosVersion: 1, Status: cloudprotocol.InstalledStatus,
			},
			Cached: true,
		},
	}

	initialLayers := []unitstatushandler.LayerStatus{
		{LayerStatus: cloudprotocol.LayerStatus{
			ID: "layer0", Digest: "digest0", AosVersion: 1, Status: cloudprotocol.InstalledStatus,
		}},
		{LayerStatus: cloudprotocol.LayerStatus{
			ID: "layer1", Digest: "digest1", AosVersion: 2, Status: cloudprotocol.InstalledStatus,
		}},
		{LayerStatus: cloudprotocol.LayerStatus{
			ID: "layer2", Digest: "digest2", AosVersion: 3, Status: cloudprotocol.InstalledStatus,
		}},
	}

	unitConfigUpdater := unitstatushandler.NewTestUnitConfigUpdater(expectedUnitStatus.UnitConfig[0])
	fotaUpdater := unitstatushandler.NewTestFirmwareUpdater(expectedUnitStatus.Components)
	sotaUpdater := unitstatushandler.NewTestSoftwareUpdater(initialServices, initialLayers)
	instanceRunner := unitstatushandler.NewTestInstanceRunner()
	sender := unitstatushandler.NewTestSender()

	statusHandler, err := unitstatushandler.New(
		cfg, unitConfigUpdater, fotaUpdater, sotaUpdater, instanceRunner, unitstatushandler.NewTestDownloader(),
		unitstatushandler.NewTestStorage(), sender)
	if err != nil {
		t.Fatalf("Can't create unit status handler: %s", err)
	}
	defer statusHandler.Close()

	sender.Consumer.CloudConnected()

	if err := statusHandler.SendUnitStatus(); err != nil {
		t.Fatalf("Can't send unit status: %v", err)
	}

	if err := statusHandler.ProcessRunStatus(
		unitstatushandler.RunInstancesStatus{UnitSubjects: []string{"subject1"}}); err != nil {
		t.Fatalf("Can't process run status: %v", err)
	}

	receivedUnitStatus, err := sender.WaitForStatus(waitStatusTimeout)
	if err != nil {
		t.Fatalf("Can't receive unit status: %s", err)
	}

	if err = compareUnitStatus(receivedUnitStatus, expectedUnitStatus); err != nil {
		t.Errorf("Wrong unit status received: %v, expected: %v", receivedUnitStatus, expectedUnitStatus)
	}

	sender.Consumer.CloudDisconnected()

	if err := statusHandler.ProcessRunStatus(
		unitstatushandler.RunInstancesStatus{UnitSubjects: []string{"subject10"}}); err != nil {
		t.Fatalf("Can't process run status: %v", err)
	}

	if _, err := sender.WaitForStatus(time.Second); err == nil {
		t.Fatal("Should be receive status timeout")
	}
}

func TestUpdateUnitConfig(t *testing.T) {
	unitConfigUpdater := unitstatushandler.NewTestUnitConfigUpdater(
		cloudprotocol.UnitConfigStatus{VendorVersion: "1.0", Status: cloudprotocol.InstalledStatus})
	fotaUpdater := unitstatushandler.NewTestFirmwareUpdater(nil)
	sotaUpdater := unitstatushandler.NewTestSoftwareUpdater(nil, nil)
	instanceRunner := unitstatushandler.NewTestInstanceRunner()
	sender := unitstatushandler.NewTestSender()

	statusHandler, err := unitstatushandler.New(
		cfg, unitConfigUpdater, fotaUpdater, sotaUpdater, instanceRunner, unitstatushandler.NewTestDownloader(),
		unitstatushandler.NewTestStorage(), sender)
	if err != nil {
		t.Fatalf("Can't create unit status handler: %s", err)
	}
	defer statusHandler.Close()

	sender.Consumer.CloudConnected()

	go handleUpdateStatus(statusHandler)

	if err := statusHandler.ProcessRunStatus(unitstatushandler.RunInstancesStatus{}); err != nil {
		t.Fatalf("Can't process run status: %v", err)
	}

	if _, err = sender.WaitForStatus(waitStatusTimeout); err != nil {
		t.Fatalf("Can't receive unit status: %s", err)
	}

	// success update

	unitConfigUpdater.UnitConfigStatus = cloudprotocol.UnitConfigStatus{
		VendorVersion: "1.1", Status: cloudprotocol.InstalledStatus,
	}
	expectedUnitStatus := cloudprotocol.UnitStatus{
		UnitConfig: []cloudprotocol.UnitConfigStatus{unitConfigUpdater.UnitConfigStatus},
		Components: []cloudprotocol.ComponentStatus{},
		Layers:     []cloudprotocol.LayerStatus{},
		Services:   []cloudprotocol.ServiceStatus{},
	}

	unitConfigUpdater.UpdateVersion = "1.1"

	statusHandler.ProcessDesiredStatus(cloudprotocol.DesiredStatus{UnitConfig: json.RawMessage("{}")})

	receivedUnitStatus, err := sender.WaitForStatus(waitStatusTimeout)
	if err != nil {
		t.Fatalf("Can't receive unit status: %s", err)
	}

	if err = compareUnitStatus(receivedUnitStatus, expectedUnitStatus); err != nil {
		t.Errorf("Wrong unit status received: %v, expected: %v", receivedUnitStatus, expectedUnitStatus)
	}

	// failed update

	unitConfigUpdater.UpdateVersion = "1.2"
	unitConfigUpdater.UpdateError = aoserrors.New("some error occurs")

	unitConfigUpdater.UnitConfigStatus = cloudprotocol.UnitConfigStatus{
		VendorVersion: "1.2", Status: cloudprotocol.ErrorStatus,
		ErrorInfo: &cloudprotocol.ErrorInfo{Message: unitConfigUpdater.UpdateError.Error()},
	}
	expectedUnitStatus.UnitConfig = append(expectedUnitStatus.UnitConfig, unitConfigUpdater.UnitConfigStatus)

	statusHandler.ProcessDesiredStatus(cloudprotocol.DesiredStatus{UnitConfig: json.RawMessage("{}")})

	if receivedUnitStatus, err = sender.WaitForStatus(waitStatusTimeout); err != nil {
		t.Fatalf("Can't receive unit status: %s", err)
	}

	if err = compareUnitStatus(receivedUnitStatus, expectedUnitStatus); err != nil {
		t.Errorf("Wrong unit status received: %v, expected: %v", receivedUnitStatus, expectedUnitStatus)
	}
}

func TestUpdateComponents(t *testing.T) {
	unitConfigUpdater := unitstatushandler.NewTestUnitConfigUpdater(cloudprotocol.UnitConfigStatus{
		VendorVersion: "1.0", Status: cloudprotocol.InstalledStatus,
	})
	firmwareUpdater := unitstatushandler.NewTestFirmwareUpdater([]cloudprotocol.ComponentStatus{
		{ID: "comp0", VendorVersion: "1.0", Status: cloudprotocol.InstalledStatus},
		{ID: "comp1", VendorVersion: "1.0", Status: cloudprotocol.InstalledStatus},
		{ID: "comp2", VendorVersion: "1.0", Status: cloudprotocol.InstalledStatus},
	})
	softwareUpdater := unitstatushandler.NewTestSoftwareUpdater(nil, nil)
	instanceRunner := unitstatushandler.NewTestInstanceRunner()
	sender := unitstatushandler.NewTestSender()

	statusHandler, err := unitstatushandler.New(cfg,
		unitConfigUpdater, firmwareUpdater, softwareUpdater, instanceRunner, unitstatushandler.NewTestDownloader(),
		unitstatushandler.NewTestStorage(), sender)
	if err != nil {
		t.Fatalf("Can't create unit status handler: %s", err)
	}
	defer statusHandler.Close()

	sender.Consumer.CloudConnected()

	go handleUpdateStatus(statusHandler)

	if err := statusHandler.ProcessRunStatus(unitstatushandler.RunInstancesStatus{}); err != nil {
		t.Fatalf("Can't process run status: %v", err)
	}

	if _, err = sender.WaitForStatus(waitStatusTimeout); err != nil {
		t.Fatalf("Can't receive unit status: %s", err)
	}

	// success update

	expectedUnitStatus := cloudprotocol.UnitStatus{
		UnitConfig: []cloudprotocol.UnitConfigStatus{unitConfigUpdater.UnitConfigStatus},
		Components: []cloudprotocol.ComponentStatus{
			{ID: "comp0", VendorVersion: "2.0", Status: cloudprotocol.InstalledStatus},
			{ID: "comp1", VendorVersion: "1.0", Status: cloudprotocol.InstalledStatus},
			{ID: "comp2", VendorVersion: "2.0", Status: cloudprotocol.InstalledStatus},
		},
		Layers:   []cloudprotocol.LayerStatus{},
		Services: []cloudprotocol.ServiceStatus{},
	}

	firmwareUpdater.UpdateComponentsInfo = expectedUnitStatus.Components

	statusHandler.ProcessDesiredStatus(cloudprotocol.DesiredStatus{
		Components: []cloudprotocol.ComponentInfo{
			{ID: "comp0", VersionInfo: aostypes.VersionInfo{VendorVersion: "2.0"}},
			{ID: "comp2", VersionInfo: aostypes.VersionInfo{VendorVersion: "2.0"}},
		},
	})

	receivedUnitStatus, err := sender.WaitForStatus(waitStatusTimeout)
	if err != nil {
		t.Fatalf("Can't receive unit status: %s", err)
	}

	if err = compareUnitStatus(receivedUnitStatus, expectedUnitStatus); err != nil {
		t.Errorf("Wrong unit status received: %v, expected: %v", receivedUnitStatus, expectedUnitStatus)
	}

	// failed update

	firmwareUpdater.UpdateError = aoserrors.New("some error occurs")

	expectedUnitStatus = cloudprotocol.UnitStatus{
		UnitConfig: []cloudprotocol.UnitConfigStatus{unitConfigUpdater.UnitConfigStatus},
		Components: []cloudprotocol.ComponentStatus{
			{ID: "comp0", VendorVersion: "2.0", Status: cloudprotocol.InstalledStatus},
			{ID: "comp1", VendorVersion: "1.0", Status: cloudprotocol.InstalledStatus},
			{
				ID: "comp1", VendorVersion: "2.0", Status: cloudprotocol.ErrorStatus,
				ErrorInfo: &cloudprotocol.ErrorInfo{Message: firmwareUpdater.UpdateError.Error()},
			},
			{ID: "comp2", VendorVersion: "2.0", Status: cloudprotocol.InstalledStatus},
		},
		Layers:   []cloudprotocol.LayerStatus{},
		Services: []cloudprotocol.ServiceStatus{},
	}

	firmwareUpdater.UpdateComponentsInfo = expectedUnitStatus.Components

	statusHandler.ProcessDesiredStatus(cloudprotocol.DesiredStatus{
		Components: []cloudprotocol.ComponentInfo{
			{ID: "comp1", VersionInfo: aostypes.VersionInfo{VendorVersion: "2.0"}},
		},
	})

	if receivedUnitStatus, err = sender.WaitForStatus(waitStatusTimeout); err != nil {
		t.Fatalf("Can't receive unit status: %s", err)
	}

	if err = compareUnitStatus(receivedUnitStatus, expectedUnitStatus); err != nil {
		t.Errorf("Wrong unit status received: %v, expected: %v", receivedUnitStatus, expectedUnitStatus)
	}
}

func TestUpdateLayers(t *testing.T) {
	layerStatuses := []unitstatushandler.LayerStatus{
		{LayerStatus: cloudprotocol.LayerStatus{
			ID: "layer0", Digest: "digest0", AosVersion: 0, Status: cloudprotocol.InstalledStatus,
		}},
		{LayerStatus: cloudprotocol.LayerStatus{
			ID: "layer1", Digest: "digest1", AosVersion: 0, Status: cloudprotocol.InstalledStatus,
		}},
		{LayerStatus: cloudprotocol.LayerStatus{
			ID: "layer2", Digest: "digest2", AosVersion: 0, Status: cloudprotocol.InstalledStatus,
		}},
	}
	unitConfigUpdater := unitstatushandler.NewTestUnitConfigUpdater(
		cloudprotocol.UnitConfigStatus{VendorVersion: "1.0", Status: cloudprotocol.InstalledStatus})
	firmwareUpdater := unitstatushandler.NewTestFirmwareUpdater(nil)
	softwareUpdater := unitstatushandler.NewTestSoftwareUpdater(nil, layerStatuses)
	instanceRunner := unitstatushandler.NewTestInstanceRunner()
	sender := unitstatushandler.NewTestSender()

	statusHandler, err := unitstatushandler.New(
		cfg, unitConfigUpdater, firmwareUpdater, softwareUpdater, instanceRunner, unitstatushandler.NewTestDownloader(),
		unitstatushandler.NewTestStorage(), sender)
	if err != nil {
		t.Fatalf("Can't create unit status handler: %s", err)
	}
	defer statusHandler.Close()

	sender.Consumer.CloudConnected()

	go handleUpdateStatus(statusHandler)

	if err := statusHandler.ProcessRunStatus(unitstatushandler.RunInstancesStatus{}); err != nil {
		t.Fatalf("Can't process run status: %v", err)
	}

	if _, err = sender.WaitForStatus(waitStatusTimeout); err != nil {
		t.Fatalf("Can't receive unit status: %s", err)
	}

	// success update

	expectedUnitStatus := cloudprotocol.UnitStatus{
		UnitConfig: []cloudprotocol.UnitConfigStatus{unitConfigUpdater.UnitConfigStatus},
		Components: []cloudprotocol.ComponentStatus{},
		Layers: []cloudprotocol.LayerStatus{
			{ID: "layer0", Digest: "digest0", AosVersion: 0, Status: cloudprotocol.RemovedStatus},
			{ID: "layer1", Digest: "digest1", AosVersion: 0, Status: cloudprotocol.InstalledStatus},
			{ID: "layer2", Digest: "digest2", AosVersion: 0, Status: cloudprotocol.RemovedStatus},
			{ID: "layer3", Digest: "digest3", AosVersion: 1, Status: cloudprotocol.InstalledStatus},
			{ID: "layer4", Digest: "digest4", AosVersion: 1, Status: cloudprotocol.InstalledStatus},
		},
		Services: []cloudprotocol.ServiceStatus{},
	}

	statusHandler.ProcessDesiredStatus(cloudprotocol.DesiredStatus{
		Layers: []cloudprotocol.LayerInfo{
			{
				ID: "layer1", Digest: "digest1", VersionInfo: aostypes.VersionInfo{AosVersion: 0},
				DecryptDataStruct: cloudprotocol.DecryptDataStruct{Sha256: []byte{1}},
			},
			{
				ID: "layer3", Digest: "digest3", VersionInfo: aostypes.VersionInfo{AosVersion: 1},
				DecryptDataStruct: cloudprotocol.DecryptDataStruct{Sha256: []byte{3}},
			},
			{
				ID: "layer4", Digest: "digest4", VersionInfo: aostypes.VersionInfo{AosVersion: 1},
				DecryptDataStruct: cloudprotocol.DecryptDataStruct{Sha256: []byte{4}},
			},
		},
	})

	if _, err := instanceRunner.WaitForRunInstance(waitRunInstanceTimeout); err != nil {
		t.Errorf("Wait run instances error: %v", err)
	}

	if err := statusHandler.ProcessRunStatus(unitstatushandler.RunInstancesStatus{}); err != nil {
		t.Fatalf("Can't process run status: %v", err)
	}

	receivedUnitStatus, err := sender.WaitForStatus(waitStatusTimeout)
	if err != nil {
		t.Fatalf("Can't receive unit status: %s", err)
	}

	if err = compareUnitStatus(receivedUnitStatus, expectedUnitStatus); err != nil {
		t.Errorf("Wrong unit status received: %v, expected: %v", receivedUnitStatus, expectedUnitStatus)
	}

	softwareUpdater.AllLayers = []unitstatushandler.LayerStatus{
		{LayerStatus: cloudprotocol.LayerStatus{
			ID: "layer0", Digest: "digest0", AosVersion: 0, Status: cloudprotocol.RemovedStatus,
		}},
		{LayerStatus: cloudprotocol.LayerStatus{
			ID: "layer1", Digest: "digest1", AosVersion: 0, Status: cloudprotocol.InstalledStatus,
		}},
		{LayerStatus: cloudprotocol.LayerStatus{
			ID: "layer2", Digest: "digest2", AosVersion: 0, Status: cloudprotocol.RemovedStatus,
		}},
		{LayerStatus: cloudprotocol.LayerStatus{
			ID: "layer3", Digest: "digest3", AosVersion: 1, Status: cloudprotocol.InstalledStatus,
		}},
		{LayerStatus: cloudprotocol.LayerStatus{
			ID: "layer4", Digest: "digest4", AosVersion: 1, Status: cloudprotocol.InstalledStatus,
		}},
	}

	// failed update

	softwareUpdater.UpdateError = aoserrors.New("some error occurs")

	expectedUnitStatus = cloudprotocol.UnitStatus{
		UnitConfig: []cloudprotocol.UnitConfigStatus{unitConfigUpdater.UnitConfigStatus},
		Components: []cloudprotocol.ComponentStatus{},
		Layers: []cloudprotocol.LayerStatus{
			{ID: "layer0", Digest: "digest0", AosVersion: 0, Status: cloudprotocol.RemovedStatus},
			{ID: "layer1", Digest: "digest1", AosVersion: 0, Status: cloudprotocol.RemovedStatus},
			{ID: "layer2", Digest: "digest2", AosVersion: 0, Status: cloudprotocol.RemovedStatus},
			{ID: "layer3", Digest: "digest3", AosVersion: 1, Status: cloudprotocol.InstalledStatus},
			{ID: "layer4", Digest: "digest4", AosVersion: 1, Status: cloudprotocol.InstalledStatus},
			{
				ID: "layer5", Digest: "digest5", AosVersion: 1, Status: cloudprotocol.ErrorStatus,
				ErrorInfo: &cloudprotocol.ErrorInfo{Message: softwareUpdater.UpdateError.Error()},
			},
		},
		Services: []cloudprotocol.ServiceStatus{},
	}

	statusHandler.ProcessDesiredStatus(cloudprotocol.DesiredStatus{
		Layers: []cloudprotocol.LayerInfo{
			{
				ID: "layer3", Digest: "digest3", VersionInfo: aostypes.VersionInfo{AosVersion: 1},
				DecryptDataStruct: cloudprotocol.DecryptDataStruct{Sha256: []byte{3}},
			},
			{
				ID: "layer4", Digest: "digest4", VersionInfo: aostypes.VersionInfo{AosVersion: 1},
				DecryptDataStruct: cloudprotocol.DecryptDataStruct{Sha256: []byte{4}},
			},
			{
				ID: "layer5", Digest: "digest5", VersionInfo: aostypes.VersionInfo{AosVersion: 1},
				DecryptDataStruct: cloudprotocol.DecryptDataStruct{Sha256: []byte{5}},
			},
		},
	})

	if _, err := instanceRunner.WaitForRunInstance(waitRunInstanceTimeout); err != nil {
		t.Errorf("Wait run instances error: %v", err)
	}

	if receivedUnitStatus, err = sender.WaitForStatus(waitStatusTimeout); err != nil {
		t.Fatalf("Can't receive unit status: %s", err)
	}

	if err = compareUnitStatus(receivedUnitStatus, expectedUnitStatus); err != nil {
		t.Errorf("Wrong unit status received: %v, expected: %v", receivedUnitStatus, expectedUnitStatus)
	}
}

func TestUpdateServices(t *testing.T) {
	serviceStatuses := []unitstatushandler.ServiceStatus{
		{ServiceStatus: cloudprotocol.ServiceStatus{
			ID: "service0", AosVersion: 0, Status: cloudprotocol.InstalledStatus,
		}},
		{ServiceStatus: cloudprotocol.ServiceStatus{
			ID: "service1", AosVersion: 0, Status: cloudprotocol.InstalledStatus,
		}},
		{ServiceStatus: cloudprotocol.ServiceStatus{
			ID: "service2", AosVersion: 0, Status: cloudprotocol.InstalledStatus,
		}},
	}
	unitConfigUpdater := unitstatushandler.NewTestUnitConfigUpdater(
		cloudprotocol.UnitConfigStatus{VendorVersion: "1.0", Status: cloudprotocol.InstalledStatus})
	firmwareUpdater := unitstatushandler.NewTestFirmwareUpdater(nil)
	softwareUpdater := unitstatushandler.NewTestSoftwareUpdater(serviceStatuses, nil)
	instanceRunner := unitstatushandler.NewTestInstanceRunner()
	sender := unitstatushandler.NewTestSender()

	statusHandler, err := unitstatushandler.New(
		cfg, unitConfigUpdater, firmwareUpdater, softwareUpdater, instanceRunner, unitstatushandler.NewTestDownloader(),
		unitstatushandler.NewTestStorage(), sender)
	if err != nil {
		t.Fatalf("Can't create unit status handler: %s", err)
	}
	defer statusHandler.Close()

	sender.Consumer.CloudConnected()

	go handleUpdateStatus(statusHandler)

	if err := statusHandler.ProcessRunStatus(unitstatushandler.RunInstancesStatus{}); err != nil {
		t.Fatalf("Can't process run status: %v", err)
	}

	if _, err = sender.WaitForStatus(5 * time.Second); err != nil {
		t.Fatalf("Can't receive unit status: %s", err)
	}

	// success update

	expectedUnitStatus := cloudprotocol.UnitStatus{
		UnitConfig: []cloudprotocol.UnitConfigStatus{unitConfigUpdater.UnitConfigStatus},
		Components: []cloudprotocol.ComponentStatus{},
		Layers:     []cloudprotocol.LayerStatus{},
		Services: []cloudprotocol.ServiceStatus{
			{ID: "service0", AosVersion: 0, Status: cloudprotocol.InstalledStatus},
			{ID: "service1", AosVersion: 1, Status: cloudprotocol.InstalledStatus},
			{ID: "service2", Status: cloudprotocol.RemovedStatus},
			{ID: "service3", AosVersion: 1, Status: cloudprotocol.InstalledStatus},
		},
	}

	statusHandler.ProcessDesiredStatus(cloudprotocol.DesiredStatus{
		Services: []cloudprotocol.ServiceInfo{
			{
				ID: "service0", VersionInfo: aostypes.VersionInfo{AosVersion: 0},
				DecryptDataStruct: cloudprotocol.DecryptDataStruct{Sha256: []byte{0}},
			},
			{
				ID: "service1", VersionInfo: aostypes.VersionInfo{AosVersion: 1},
				DecryptDataStruct: cloudprotocol.DecryptDataStruct{Sha256: []byte{1}},
			},
			{
				ID: "service3", VersionInfo: aostypes.VersionInfo{AosVersion: 1},
				DecryptDataStruct: cloudprotocol.DecryptDataStruct{Sha256: []byte{3}},
			},
		},
	})

	if _, err := instanceRunner.WaitForRunInstance(waitRunInstanceTimeout); err != nil {
		t.Errorf("Wait run instances error: %v", err)
	}

	if err := statusHandler.ProcessRunStatus(unitstatushandler.RunInstancesStatus{}); err != nil {
		t.Fatalf("Can't process run status: %v", err)
	}

	receivedUnitStatus, err := sender.WaitForStatus(waitStatusTimeout)
	if err != nil {
		t.Fatalf("Can't receive unit status: %s", err)
	}

	if err = compareUnitStatus(receivedUnitStatus, expectedUnitStatus); err != nil {
		t.Errorf("Wrong unit status received: %v, expected: %v", receivedUnitStatus, expectedUnitStatus)
	}

	// failed update

	softwareUpdater.AllServices = []unitstatushandler.ServiceStatus{
		{ServiceStatus: cloudprotocol.ServiceStatus{
			ID: "service0", AosVersion: 0, Status: cloudprotocol.InstalledStatus,
		}},
		{ServiceStatus: cloudprotocol.ServiceStatus{
			ID: "service1", AosVersion: 1, Status: cloudprotocol.InstalledStatus,
		}},
		{ServiceStatus: cloudprotocol.ServiceStatus{
			ID: "service2", AosVersion: 0, Status: cloudprotocol.RemovedStatus,
		}},
		{ServiceStatus: cloudprotocol.ServiceStatus{
			ID: "service3", AosVersion: 1, Status: cloudprotocol.InstalledStatus,
		}},
	}
	softwareUpdater.UpdateError = aoserrors.New("some error occurs")

	expectedUnitStatus = cloudprotocol.UnitStatus{
		UnitConfig: []cloudprotocol.UnitConfigStatus{unitConfigUpdater.UnitConfigStatus},
		Components: []cloudprotocol.ComponentStatus{},
		Layers:     []cloudprotocol.LayerStatus{},
		Services: []cloudprotocol.ServiceStatus{
			{
				ID: "service0", AosVersion: 0, Status: cloudprotocol.ErrorStatus,
				ErrorInfo: &cloudprotocol.ErrorInfo{Message: softwareUpdater.UpdateError.Error()},
			},
			{ID: "service1", AosVersion: 1, Status: cloudprotocol.InstalledStatus},
			{ID: "service2", Status: cloudprotocol.RemovedStatus},
			{ID: "service3", AosVersion: 1, Status: cloudprotocol.InstalledStatus},
			{
				ID: "service3", AosVersion: 2, Status: cloudprotocol.ErrorStatus,
				ErrorInfo: &cloudprotocol.ErrorInfo{Message: softwareUpdater.UpdateError.Error()},
			},
			{
				ID: "service4", AosVersion: 2, Status: cloudprotocol.ErrorStatus,
				ErrorInfo: &cloudprotocol.ErrorInfo{Message: softwareUpdater.UpdateError.Error()},
			},
		},
	}

	statusHandler.ProcessDesiredStatus(cloudprotocol.DesiredStatus{
		Services: []cloudprotocol.ServiceInfo{
			{
				ID: "service1", VersionInfo: aostypes.VersionInfo{AosVersion: 1},
				DecryptDataStruct: cloudprotocol.DecryptDataStruct{Sha256: []byte{1}},
			},
			{
				ID: "service3", VersionInfo: aostypes.VersionInfo{AosVersion: 2},
				DecryptDataStruct: cloudprotocol.DecryptDataStruct{Sha256: []byte{3}},
			},
			{
				ID: "service4", VersionInfo: aostypes.VersionInfo{AosVersion: 2},
				DecryptDataStruct: cloudprotocol.DecryptDataStruct{Sha256: []byte{4}},
			},
		},
	})

	if _, err := instanceRunner.WaitForRunInstance(waitRunInstanceTimeout); err != nil {
		t.Errorf("Wait run instances error: %v", err)
	}

	if receivedUnitStatus, err = sender.WaitForStatus(waitStatusTimeout); err != nil {
		t.Fatalf("Can't receive unit status: %s", err)
	}

	if err = compareUnitStatus(receivedUnitStatus, expectedUnitStatus); err != nil {
		t.Errorf("Wrong unit status received: %v, expected: %v", receivedUnitStatus, expectedUnitStatus)
	}
}

func TestRunInstances(t *testing.T) {
	unitConfigUpdater := unitstatushandler.NewTestUnitConfigUpdater(
		cloudprotocol.UnitConfigStatus{VendorVersion: "1.0", Status: cloudprotocol.InstalledStatus})
	firmwareUpdater := unitstatushandler.NewTestFirmwareUpdater(nil)
	softwareUpdater := unitstatushandler.NewTestSoftwareUpdater(nil, nil)
	instanceRunner := unitstatushandler.NewTestInstanceRunner()
	sender := unitstatushandler.NewTestSender()

	statusHandler, err := unitstatushandler.New(
		cfg, unitConfigUpdater, firmwareUpdater, softwareUpdater, instanceRunner, unitstatushandler.NewTestDownloader(),
		unitstatushandler.NewTestStorage(), sender)
	if err != nil {
		t.Fatalf("Can't create unit status handler: %v", err)
	}
	defer statusHandler.Close()

	sender.Consumer.CloudConnected()

	go handleUpdateStatus(statusHandler)

	initialInstancesStatus := []cloudprotocol.InstanceStatus{
		{
			InstanceIdent: aostypes.InstanceIdent{ServiceID: "Serv1", SubjectID: "Subj1", Instance: 0}, AosVersion: 1,
		},
		{
			InstanceIdent: aostypes.InstanceIdent{ServiceID: "Serv1", SubjectID: "Subj1", Instance: 1}, AosVersion: 1,
		},
	}

	if err := statusHandler.ProcessRunStatus(
		unitstatushandler.RunInstancesStatus{Instances: initialInstancesStatus}); err != nil {
		t.Fatalf("Can't process run status: %v", err)
	}

	receivedUnitStatus, err := sender.WaitForStatus(waitStatusTimeout)
	if err != nil {
		t.Fatalf("Can't receive unit status: %v", err)
	}

	expectedUnitStatus := cloudprotocol.UnitStatus{
		UnitConfig: []cloudprotocol.UnitConfigStatus{unitConfigUpdater.UnitConfigStatus},
		Instances:  initialInstancesStatus,
	}

	if err = compareUnitStatus(receivedUnitStatus, expectedUnitStatus); err != nil {
		t.Errorf("Wrong unit status received: %v, expected: %v", receivedUnitStatus, expectedUnitStatus)
	}

	// success run

	expectedRunInstances := []cloudprotocol.InstanceInfo{
		{ServiceID: "Serv1", SubjectID: "Subj1", NumInstances: 3},
		{ServiceID: "Serv1", SubjectID: "Subj2", NumInstances: 1},
		{ServiceID: "Serv2", SubjectID: "Subj1", NumInstances: 1},
	}

	statusHandler.ProcessDesiredStatus(cloudprotocol.DesiredStatus{
		Instances: expectedRunInstances,
	})

	receivedRunInstances, err := instanceRunner.WaitForRunInstance(waitRunInstanceTimeout)
	if err != nil {
		t.Fatalf("Can't receive run instances: %v", err)
	}

	if !reflect.DeepEqual(receivedRunInstances, expectedRunInstances) {
		t.Error("Incorrect run instances")
	}

	updatedInstancesStatus := []cloudprotocol.InstanceStatus{
		{
			InstanceIdent: aostypes.InstanceIdent{ServiceID: "Serv1", SubjectID: "Subj1", Instance: 0}, AosVersion: 1,
		},
		{
			InstanceIdent: aostypes.InstanceIdent{ServiceID: "Serv1", SubjectID: "Subj1", Instance: 1}, AosVersion: 1,
		},
		{
			InstanceIdent: aostypes.InstanceIdent{ServiceID: "Serv1", SubjectID: "Subj1", Instance: 2}, AosVersion: 1,
		},
		{
			InstanceIdent: aostypes.InstanceIdent{ServiceID: "Serv1", SubjectID: "Subj2", Instance: 0}, AosVersion: 1,
		},
		{
			InstanceIdent: aostypes.InstanceIdent{ServiceID: "Serv2", SubjectID: "Subj1", Instance: 0}, AosVersion: 1,
		},
	}

	if err := statusHandler.ProcessRunStatus(
		unitstatushandler.RunInstancesStatus{Instances: updatedInstancesStatus}); err != nil {
		t.Fatalf("Can't process run status: %v", err)
	}

	receivedUnitStatus, err = sender.WaitForStatus(waitStatusTimeout)
	if err != nil {
		t.Fatalf("Can't receive unit status: %v", err)
	}

	expectedUnitStatus = cloudprotocol.UnitStatus{
		UnitConfig: []cloudprotocol.UnitConfigStatus{unitConfigUpdater.UnitConfigStatus},
		Instances:  updatedInstancesStatus,
	}

	if err = compareUnitStatus(receivedUnitStatus, expectedUnitStatus); err != nil {
		t.Errorf("Wrong unit status received: %v, expected: %v", receivedUnitStatus, expectedUnitStatus)
	}

	// send the same run instances
	statusHandler.ProcessDesiredStatus(cloudprotocol.DesiredStatus{
		Instances: expectedRunInstances,
	})

	if _, err := instanceRunner.WaitForRunInstance(waitRunInstanceTimeout); err == nil {
		t.Error("Should be no run instances request")
	}
}

func TestUpdateInstancesStatus(t *testing.T) {
	unitConfigUpdater := unitstatushandler.NewTestUnitConfigUpdater(
		cloudprotocol.UnitConfigStatus{VendorVersion: "1.0", Status: cloudprotocol.InstalledStatus})
	firmwareUpdater := unitstatushandler.NewTestFirmwareUpdater(nil)
	softwareUpdater := unitstatushandler.NewTestSoftwareUpdater(nil, nil)
	instanceRunner := unitstatushandler.NewTestInstanceRunner()
	sender := unitstatushandler.NewTestSender()

	statusHandler, err := unitstatushandler.New(
		cfg, unitConfigUpdater, firmwareUpdater, softwareUpdater, instanceRunner, unitstatushandler.NewTestDownloader(),
		unitstatushandler.NewTestStorage(), sender)
	if err != nil {
		t.Fatalf("Can't create unit status handler: %v", err)
	}
	defer statusHandler.Close()

	sender.Consumer.CloudConnected()

	go handleUpdateStatus(statusHandler)

	if err := statusHandler.ProcessRunStatus(
		unitstatushandler.RunInstancesStatus{Instances: []cloudprotocol.InstanceStatus{
			{
				InstanceIdent: aostypes.InstanceIdent{ServiceID: "Serv1", SubjectID: "Subj1", Instance: 0}, AosVersion: 1,
			},
			{
				InstanceIdent: aostypes.InstanceIdent{ServiceID: "Serv1", SubjectID: "Subj1", Instance: 1}, AosVersion: 1,
			},
			{
				InstanceIdent: aostypes.InstanceIdent{ServiceID: "Serv2", SubjectID: "Subj2", Instance: 1}, AosVersion: 1,
			},
		}}); err != nil {
		t.Fatalf("Can't process run status: %v", err)
	}

	if _, err := sender.WaitForStatus(waitStatusTimeout); err != nil {
		t.Fatalf("Can't receive unit status: %v", err)
	}

	expectedUnitStatus := cloudprotocol.UnitStatus{
		UnitConfig: []cloudprotocol.UnitConfigStatus{unitConfigUpdater.UnitConfigStatus},
		Instances: []cloudprotocol.InstanceStatus{
			{
				InstanceIdent: aostypes.InstanceIdent{ServiceID: "Serv1", SubjectID: "Subj1", Instance: 0}, AosVersion: 1,
				RunState: "fail", ErrorInfo: &cloudprotocol.ErrorInfo{Message: "someError"},
			},
			{
				InstanceIdent: aostypes.InstanceIdent{ServiceID: "Serv1", SubjectID: "Subj1", Instance: 1}, AosVersion: 1,
			},
			{
				InstanceIdent: aostypes.InstanceIdent{ServiceID: "Serv2", SubjectID: "Subj2", Instance: 1}, AosVersion: 1,
				StateChecksum: "newState",
			},
		},
	}

	statusHandler.ProcessUpdateInstanceStatus([]cloudprotocol.InstanceStatus{
		{
			InstanceIdent: aostypes.InstanceIdent{ServiceID: "Serv1", SubjectID: "Subj1", Instance: 0}, AosVersion: 1,
			RunState: "fail", ErrorInfo: &cloudprotocol.ErrorInfo{Message: "someError"},
		},
		{
			InstanceIdent: aostypes.InstanceIdent{ServiceID: "Serv2", SubjectID: "Subj2", Instance: 1}, AosVersion: 1,
			StateChecksum: "newState",
		},
	})

	receivedUnitStatus, err := sender.WaitForStatus(waitStatusTimeout)
	if err != nil {
		t.Fatalf("Can't receive unit status: %v", err)
	}

	if err = compareUnitStatus(receivedUnitStatus, expectedUnitStatus); err != nil {
		t.Errorf("Wrong unit status received: %v, expected: %v", receivedUnitStatus, expectedUnitStatus)
	}
}

func TestUpdateCachedSOTA(t *testing.T) {
	serviceStatuses := []unitstatushandler.ServiceStatus{
		{ServiceStatus: cloudprotocol.ServiceStatus{
			ID: "service0", AosVersion: 0, Status: cloudprotocol.InstalledStatus,
		}},
		{ServiceStatus: cloudprotocol.ServiceStatus{
			ID: "service1", AosVersion: 0, Status: cloudprotocol.InstalledStatus,
		}},
		{ServiceStatus: cloudprotocol.ServiceStatus{
			ID: "service2", AosVersion: 0, Status: cloudprotocol.InstalledStatus,
		}},
		{ServiceStatus: cloudprotocol.ServiceStatus{
			ID: "service4", AosVersion: 0, Status: cloudprotocol.InstalledStatus,
		}, Cached: true},
	}
	layerStatuses := []unitstatushandler.LayerStatus{
		{LayerStatus: cloudprotocol.LayerStatus{
			ID: "layer0", Digest: "digest0", AosVersion: 0, Status: cloudprotocol.InstalledStatus,
		}},
		{LayerStatus: cloudprotocol.LayerStatus{
			ID: "layer1", Digest: "digest1", AosVersion: 0, Status: cloudprotocol.InstalledStatus,
		}},
		{LayerStatus: cloudprotocol.LayerStatus{
			ID: "layer2", Digest: "digest2", AosVersion: 0, Status: cloudprotocol.InstalledStatus,
		}},
		{LayerStatus: cloudprotocol.LayerStatus{
			ID: "layer4", Digest: "digest4", AosVersion: 0, Status: cloudprotocol.InstalledStatus,
		}, Cached: true},
		{LayerStatus: cloudprotocol.LayerStatus{
			ID: "layer5", Digest: "digest5", AosVersion: 0, Status: cloudprotocol.InstalledStatus,
		}, Cached: true},
	}
	unitConfigUpdater := unitstatushandler.NewTestUnitConfigUpdater(
		cloudprotocol.UnitConfigStatus{VendorVersion: "1.0", Status: cloudprotocol.InstalledStatus})
	firmwareUpdater := unitstatushandler.NewTestFirmwareUpdater(nil)
	softwareUpdater := unitstatushandler.NewTestSoftwareUpdater(serviceStatuses, layerStatuses)
	instanceRunner := unitstatushandler.NewTestInstanceRunner()
	sender := unitstatushandler.NewTestSender()
	downloader := unitstatushandler.NewTestDownloader()

	statusHandler, err := unitstatushandler.New(
		cfg, unitConfigUpdater, firmwareUpdater, softwareUpdater, instanceRunner, downloader,
		unitstatushandler.NewTestStorage(), sender)
	if err != nil {
		t.Fatalf("Can't create unit status handler: %s", err)
	}
	defer statusHandler.Close()

	sender.Consumer.CloudConnected()

	go handleUpdateStatus(statusHandler)

	if err := statusHandler.ProcessRunStatus(unitstatushandler.RunInstancesStatus{}); err != nil {
		t.Fatalf("Can't process run status: %v", err)
	}

	if _, err = sender.WaitForStatus(waitStatusTimeout); err != nil {
		t.Fatalf("Can't receive unit status: %s", err)
	}

	expectedUnitStatus := cloudprotocol.UnitStatus{
		UnitConfig: []cloudprotocol.UnitConfigStatus{unitConfigUpdater.UnitConfigStatus},
		Components: []cloudprotocol.ComponentStatus{},
		Layers: []cloudprotocol.LayerStatus{
			{ID: "layer0", Digest: "digest0", AosVersion: 0, Status: cloudprotocol.InstalledStatus},
			{ID: "layer1", Digest: "digest1", AosVersion: 0, Status: cloudprotocol.InstalledStatus},
			{ID: "layer2", Digest: "digest2", AosVersion: 0, Status: cloudprotocol.InstalledStatus},
			{ID: "layer3", Digest: "digest3", AosVersion: 0, Status: cloudprotocol.InstalledStatus},
			{ID: "layer5", Digest: "digest5", AosVersion: 0, Status: cloudprotocol.InstalledStatus},
		},
		Services: []cloudprotocol.ServiceStatus{
			{ID: "service0", AosVersion: 0, Status: cloudprotocol.InstalledStatus},
			{ID: "service1", AosVersion: 0, Status: cloudprotocol.InstalledStatus},
			{ID: "service2", AosVersion: 0, Status: cloudprotocol.InstalledStatus},
			{ID: "service3", AosVersion: 0, Status: cloudprotocol.InstalledStatus},
			{ID: "service4", AosVersion: 0, Status: cloudprotocol.InstalledStatus},
		},
	}

	statusHandler.ProcessDesiredStatus(cloudprotocol.DesiredStatus{
		Services: []cloudprotocol.ServiceInfo{
			{
				ID: "service0", VersionInfo: aostypes.VersionInfo{AosVersion: 0},
				DecryptDataStruct: cloudprotocol.DecryptDataStruct{URLs: []string{"service0"}, Sha256: []byte{0}},
			},
			{
				ID: "service1", VersionInfo: aostypes.VersionInfo{AosVersion: 0},
				DecryptDataStruct: cloudprotocol.DecryptDataStruct{URLs: []string{"service1"}, Sha256: []byte{1}},
			},
			{
				ID: "service2", VersionInfo: aostypes.VersionInfo{AosVersion: 0},
				DecryptDataStruct: cloudprotocol.DecryptDataStruct{URLs: []string{"service2"}, Sha256: []byte{2}},
			},
			{
				ID: "service3", VersionInfo: aostypes.VersionInfo{AosVersion: 0},
				DecryptDataStruct: cloudprotocol.DecryptDataStruct{URLs: []string{"service3"}, Sha256: []byte{3}},
			},
			{
				ID: "service4", VersionInfo: aostypes.VersionInfo{AosVersion: 0},
				DecryptDataStruct: cloudprotocol.DecryptDataStruct{URLs: []string{"service3"}, Sha256: []byte{3}},
			},
		},
		Layers: []cloudprotocol.LayerInfo{
			{
				ID: "layer0", Digest: "digest0", VersionInfo: aostypes.VersionInfo{AosVersion: 0},
				DecryptDataStruct: cloudprotocol.DecryptDataStruct{URLs: []string{"layer0"}, Sha256: []byte{0}},
			},
			{
				ID: "layer1", Digest: "digest1", VersionInfo: aostypes.VersionInfo{AosVersion: 0},
				DecryptDataStruct: cloudprotocol.DecryptDataStruct{URLs: []string{"layer1"}, Sha256: []byte{1}},
			},
			{
				ID: "layer2", Digest: "digest2", VersionInfo: aostypes.VersionInfo{AosVersion: 0},
				DecryptDataStruct: cloudprotocol.DecryptDataStruct{URLs: []string{"layer2"}, Sha256: []byte{2}},
			},
			{
				ID: "layer3", Digest: "digest3", VersionInfo: aostypes.VersionInfo{AosVersion: 0},
				DecryptDataStruct: cloudprotocol.DecryptDataStruct{URLs: []string{"layer3"}, Sha256: []byte{3}},
			},
			{
				ID: "layer5", Digest: "digest5", VersionInfo: aostypes.VersionInfo{AosVersion: 0},
				DecryptDataStruct: cloudprotocol.DecryptDataStruct{URLs: []string{"layer5"}, Sha256: []byte{3}},
			},
		},
	})

	receivedUnitStatus, err := sender.WaitForStatus(waitStatusTimeout)
	if err != nil {
		t.Fatalf("Can't receive unit status: %s", err)
	}

	for _, url := range downloader.DownloadedURLs {
		if url == "service1" || url == "service2" || url == "layer1" || url == "layer2" {
			t.Errorf("Unexpected download URL: %s", url)
		}

		if url != "service3" && url != "layer3" {
			t.Errorf("Unexpected download URL: %s", url)
		}
	}

	if err = compareUnitStatus(receivedUnitStatus, expectedUnitStatus); err != nil {
		t.Errorf("Wrong unit status received: %v, expected: %v", receivedUnitStatus, expectedUnitStatus)
	}
}

/***********************************************************************************************************************
 * Private
 **********************************************************************************************************************/

func compareStatus(len1, len2 int, compare func(index1, index2 int) bool) (err error) {
	if len1 != len2 {
		return aoserrors.New("data mismatch")
	}

	for index1 := 0; index1 < len1; index1++ {
		found := false

		for index2 := 0; index2 < len2; index2++ {
			if compare(index1, index2) {
				found = true
				break
			}
		}

		if !found {
			return aoserrors.New("data mismatch")
		}
	}

	for index2 := 0; index2 < len2; index2++ {
		found := false

		for index1 := 0; index1 < len1; index1++ {
			if compare(index1, index2) {
				found = true
				break
			}
		}

		if !found {
			return aoserrors.New("data mismatch")
		}
	}

	return nil
}

func compareUnitStatus(status1, status2 cloudprotocol.UnitStatus) (err error) {
	if err = compareStatus(len(status1.UnitConfig), len(status2.UnitConfig),
		func(index1, index2 int) (result bool) {
			return reflect.DeepEqual(status1.UnitConfig[index1], status2.UnitConfig[index2])
		}); err != nil {
		return aoserrors.Wrap(err)
	}

	if err = compareStatus(len(status1.Components), len(status2.Components),
		func(index1, index2 int) (result bool) {
			return reflect.DeepEqual(status1.Components[index1], status2.Components[index2])
		}); err != nil {
		return aoserrors.Wrap(err)
	}

	if err = compareStatus(len(status1.Layers), len(status2.Layers),
		func(index1, index2 int) (result bool) {
			return reflect.DeepEqual(status1.Layers[index1], status2.Layers[index2])
		}); err != nil {
		return aoserrors.Wrap(err)
	}

	if err = compareStatus(len(status1.Services), len(status2.Services),
		func(index1, index2 int) (result bool) {
			return reflect.DeepEqual(status1.Services[index1], status2.Services[index2])
		}); err != nil {
		return aoserrors.Wrap(err)
	}

	return nil
}

func handleUpdateStatus(handler *unitstatushandler.Instance) {
	for {
		select {
		case _, ok := <-handler.GetFOTAStatusChannel():
			if !ok {
				return
			}

		case _, ok := <-handler.GetSOTAStatusChannel():
			if !ok {
				return
			}
		}
	}
}
