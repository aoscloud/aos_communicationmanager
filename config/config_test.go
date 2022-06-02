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

package config_test

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path"
	"reflect"
	"testing"
	"time"

	"github.com/aoscloud/aos_common/aoserrors"
	"github.com/aoscloud/aos_common/aostypes"

	"github.com/aoscloud/aos_communicationmanager/config"
)

/***********************************************************************************************************************
 * Consts
 **********************************************************************************************************************/

const testConfigContent = `{
	"fcrypt" : {
		"CACert" : "CACert",
		"tpmDevice": "/dev/tpmrm0",
		"pkcs11Library": "/path/to/pkcs11/library"
	},
	"certStorage": "/var/aos/crypt/cm/",
	"serviceDiscoveryUrl" : "www.aos.com",
	"iamServerUrl" : "localhost:8089",
	"iamPublicServerUrl" : "localhost:8090",
	"fileServerUrl":"localhost:8092",
	"cmServerUrl":"localhost:8094",
	"workingDir" : "workingDir",
	"boardConfigFile" : "/var/aos/aos_board.cfg",
	"downloader": {
		"downloadDir": "/path/to/download",
		"decryptDir": "/path/to/decrypt",
		"maxConcurrentDownloads": 10,
		"retryDelay": "10s",
		"maxRetryDelay": "30s",
		"downloadPartLimit": 57
	},
	"monitoring": {
		"monitorConfig": {
			"sendPeriod": "5m",
			"pollPeriod": "1s",
			"ram": {
				"minTimeout": "10s",
				"minThreshold": 10,
				"maxThreshold": 150
			},
			"outTraffic": {
				"minTimeout": "20s",
				"minThreshold": 10,
				"maxThreshold": 150
			}
		},
		"maxOfflineMessages": 25
	},
	"alerts": {		
		"sendPeriod": "20s",
		"maxMessageSize": 1024,
		"maxOfflineMessages": 32,
		"journalAlerts": {
			"filter": ["(test)", "(regexp)"]
		}
	},
	"migration": {
		"migrationPath" : "/usr/share/aos_communicationmanager/migration",
		"mergedMigrationPath" : "/var/aos/communicationmanager/migration"
	},
	"smController": {
		"smList": [
			{
				"smId": "sm0",
				"serverUrl": "localhost:8888",
				"isLocal": true
			},
			{
				"smId": "sm1",
				"serverUrl": "remotehost:8888"
			}
		],
		"updateTTL": "30h"
	},
	"umController": {
		"serverUrl": "localhost:8091",
		"umClients": [{
			"umId": "um",
			"priority": 0,
			"isLocal": true
		}],
		"updateTTL": "100h"
	}
}`

/***********************************************************************************************************************
 * Vars
 **********************************************************************************************************************/

var (
	tmpDir  string
	testCfg *config.Config
)

/***********************************************************************************************************************
 * Main
 **********************************************************************************************************************/

func TestMain(m *testing.M) {
	if err := setup(); err != nil {
		log.Fatalf("Error creating service images: %s", err)
	}

	ret := m.Run()

	if err := cleanup(); err != nil {
		log.Fatalf("Error cleaning up: %s", err)
	}

	os.Exit(ret)
}

/***********************************************************************************************************************
 * Tests
 **********************************************************************************************************************/

func TestGetCrypt(t *testing.T) {
	if testCfg.Crypt.TpmDevice != "/dev/tpmrm0" {
		t.Errorf("Wrong TPMEngine Interface value: %s", testCfg.Crypt.TpmDevice)
	}

	if testCfg.Crypt.CACert != "CACert" {
		t.Errorf("Wrong CACert value: %s", testCfg.Crypt.CACert)
	}

	if testCfg.Crypt.Pkcs11Library != "/path/to/pkcs11/library" {
		t.Errorf("Wrong PKCS11 library value: %s", testCfg.Crypt.Pkcs11Library)
	}
}

func TestGetServiceDiscoveryURL(t *testing.T) {
	if testCfg.ServiceDiscoveryURL != "www.aos.com" {
		t.Errorf("Wrong server URL value: %s", testCfg.ServiceDiscoveryURL)
	}
}

func TestGetWorkingDir(t *testing.T) {
	if testCfg.WorkingDir != "workingDir" {
		t.Errorf("Wrong working directory value: %s", testCfg.WorkingDir)
	}
}

func TestGetBoardConfigFile(t *testing.T) {
	if testCfg.BoardConfigFile != "/var/aos/aos_board.cfg" {
		t.Errorf("Wrong board config file value: %s", testCfg.BoardConfigFile)
	}
}

func TestGetIAMServerURL(t *testing.T) {
	if testCfg.IAMServerURL != "localhost:8089" {
		t.Errorf("Wrong IAM server value: %s", testCfg.IAMServerURL)
	}
}

func TestGetIAMPublicServerURL(t *testing.T) {
	if testCfg.IAMPublicServerURL != "localhost:8090" {
		t.Errorf("wrong IAM public server value: %s", testCfg.IAMPublicServerURL)
	}
}

func TestDurationMarshal(t *testing.T) {
	d := aostypes.Duration{Duration: 32 * time.Second}

	result, err := json.Marshal(d)
	if err != nil {
		t.Errorf("Can't marshal: %s", err)
	}

	if string(result) != `"32s"` {
		t.Errorf("Wrong value: %s", result)
	}
}

func TestGetMonitoringConfig(t *testing.T) {
	if testCfg.Monitoring.MonitorConfig.SendPeriod.Duration != 5*time.Minute {
		t.Errorf("Wrong send period value: %s", testCfg.Monitoring.MonitorConfig.SendPeriod)
	}

	if testCfg.Monitoring.MonitorConfig.PollPeriod.Duration != 1*time.Second {
		t.Errorf("Wrong poll period value: %s", testCfg.Monitoring.MonitorConfig.PollPeriod)
	}

	if testCfg.Monitoring.MonitorConfig.RAM.MinTimeout.Duration != 10*time.Second {
		t.Errorf("Wrong value: %s", testCfg.Monitoring.MonitorConfig.RAM.MinTimeout)
	}

	if testCfg.Monitoring.MonitorConfig.OutTraffic.MinTimeout.Duration != 20*time.Second {
		t.Errorf("Wrong value: %s", testCfg.Monitoring.MonitorConfig.RAM.MinTimeout)
	}
}

func TestGetAlertsConfig(t *testing.T) {
	if testCfg.Alerts.SendPeriod.Duration != 20*time.Second {
		t.Errorf("Wrong poll period value: %s", testCfg.Alerts.SendPeriod)
	}

	if testCfg.Alerts.MaxMessageSize != 1024 {
		t.Errorf("Wrong max message size value: %d", testCfg.Alerts.MaxMessageSize)
	}

	if testCfg.Alerts.MaxOfflineMessages != 32 {
		t.Errorf("Wrong max offline message value: %d", testCfg.Alerts.MaxOfflineMessages)
	}

	filter := []string{"(test)", "(regexp)"}

	if !reflect.DeepEqual(testCfg.Alerts.JournalAlerts.Filter, filter) {
		t.Errorf("Wrong filter value: %v", testCfg.Alerts.JournalAlerts.Filter)
	}
}

func TestUMControllerConfig(t *testing.T) {
	umClient := config.UMClientConfig{UMID: "um", Priority: 0, IsLocal: true}

	originalConfig := config.UMController{
		ServerURL: "localhost:8091",
		UMClients: []config.UMClientConfig{umClient},
		UpdateTTL: aostypes.Duration{Duration: 100 * time.Hour},
	}

	if !reflect.DeepEqual(originalConfig, testCfg.UMController) {
		t.Errorf("Wrong UM controller value: %v", testCfg.UMController)
	}
}

func TestDownloaderConfig(t *testing.T) {
	originalConfig := config.Downloader{
		DownloadDir:            "/path/to/download",
		DecryptDir:             "/path/to/decrypt",
		MaxConcurrentDownloads: 10,
		RetryDelay:             aostypes.Duration{Duration: 10 * time.Second},
		MaxRetryDelay:          aostypes.Duration{Duration: 30 * time.Second},
		DownloadPartLimit:      57,
	}

	if !reflect.DeepEqual(originalConfig, testCfg.Downloader) {
		t.Errorf("Wrong downloader config value: %v", testCfg.Downloader)
	}
}

func TestSMControllerConfig(t *testing.T) {
	originalConfig := config.SMController{
		SMList: []config.SMConfig{
			{SMID: "sm0", ServerURL: "localhost:8888", IsLocal: true},
			{SMID: "sm1", ServerURL: "remotehost:8888"},
		},
		UpdateTTL: aostypes.Duration{Duration: 30 * time.Hour},
	}

	if !reflect.DeepEqual(originalConfig, testCfg.SMController) {
		t.Errorf("Wrong SM controller value: %v", testCfg.SMController)
	}
}

func TestDatabaseMigration(t *testing.T) {
	if testCfg.Migration.MigrationPath != "/usr/share/aos_communicationmanager/migration" {
		t.Errorf("Wrong migration path value: %s", testCfg.Migration.MigrationPath)
	}

	if testCfg.Migration.MergedMigrationPath != "/var/aos/communicationmanager/migration" {
		t.Errorf("Wrong merged migration path value: %s", testCfg.Migration.MergedMigrationPath)
	}
}

func TestCertStorage(t *testing.T) {
	if testCfg.CertStorage != "/var/aos/crypt/cm/" {
		t.Errorf("Wrong certificate storage value: %s", testCfg.CertStorage)
	}
}

func TestFileServer(t *testing.T) {
	if testCfg.FileServerURL != "localhost:8092" {
		t.Errorf("Wrong file server URL value: %s", testCfg.FileServerURL)
	}
}

func TestCMServer(t *testing.T) {
	if testCfg.CMServerURL != "localhost:8094" {
		t.Errorf("Wrong cm server URL value: %s", testCfg.CMServerURL)
	}
}

/***********************************************************************************************************************
 * Private
 **********************************************************************************************************************/

func createConfigFile(fileName string) (err error) {
	if err := ioutil.WriteFile(fileName, []byte(testConfigContent), 0o600); err != nil {
		return aoserrors.Wrap(err)
	}

	return nil
}

func setup() (err error) {
	if tmpDir, err = ioutil.TempDir("", "aos_"); err != nil {
		return aoserrors.Wrap(err)
	}

	fileName := path.Join(tmpDir, "aos_communicationmanager.cfg")

	if err = createConfigFile(fileName); err != nil {
		return aoserrors.Wrap(err)
	}

	if testCfg, err = config.New(fileName); err != nil {
		return aoserrors.Wrap(err)
	}

	return nil
}

func cleanup() (err error) {
	if err := os.RemoveAll(tmpDir); err != nil {
		return aoserrors.Wrap(err)
	}

	return nil
}
