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

package cloudprotocol

import (
	"time"

	"github.com/aoscloud/aos_common/aostypes"
)

/***********************************************************************************************************************
 * Consts
 **********************************************************************************************************************/

// AlertsMessageType alerts message type.
const AlertsMessageType = "alerts"

// Alert tags.
const (
	AlertTagSystemError      = "systemAlert"
	AlertTagAosCore          = "coreAlert"
	AlertTagResourceValidate = "resourceValidateAlert"
	AlertTagDeviceAllocate   = "deviceAllocateAlert"
	AlertTagSystemQuota      = "systemQuotaAlert"
	AlertTagInstanceQuota    = "instanceQuotaAlert"
	AlertTagDownloadProgress = "downloadProgressAlert"
	AlertTagServiceInstance  = "serviceInstanceAlert"
)

// Download target types.
const (
	DownloadTargetComponent = "component"
	DownloadTargetLayer     = "layer"
	DownloadTargetService   = "service"
)

/***********************************************************************************************************************
 * Types
 **********************************************************************************************************************/

// SystemAlert system alert structure.
type SystemAlert struct {
	NodeID  string `json:"nodeId"`
	Message string `json:"message"`
}

// CoreAlert system alert structure.
type CoreAlert struct {
	NodeID        string `json:"nodeId"`
	CoreComponent string `json:"coreComponent"`
	Message       string `json:"message"`
}

// DownloadAlert download alert structure.
type DownloadAlert struct {
	TargetType          string `json:"targetType"`
	TargetID            string `json:"targetId"`
	TargetAosVersion    uint64 `json:"targetAosVersion"`
	TargetVendorVersion string `json:"targetVendorVersion"`
	Message             string `json:"message"`
	Progress            string `json:"progress"`
	URL                 string `json:"url"`
	DownloadedBytes     string `json:"downloadedBytes"`
	TotalBytes          string `json:"totalBytes"`
}

// SystemQuotaAlert system quota alert structure.
type SystemQuotaAlert struct {
	NodeID    string `json:"nodeId"`
	Parameter string `json:"parameter"`
	Value     uint64 `json:"value"`
}

// InstanceQuotaAlert instance quota alert structure.
type InstanceQuotaAlert struct {
	aostypes.InstanceIdent
	Parameter string `json:"parameter"`
	Value     uint64 `json:"value"`
}

// DeviceAllocateAlert device allocate alert structure.
type DeviceAllocateAlert struct {
	aostypes.InstanceIdent
	NodeID  string `json:"nodeId"`
	Device  string `json:"device"`
	Message string `json:"message"`
}

// ResourceValidateError resource validate error structure.
type ResourceValidateError struct {
	Name   string   `json:"name"`
	Errors []string `json:"error"`
}

// ResourceValidateAlert resource validate alert structure.
type ResourceValidateAlert struct {
	NodeID          string                  `json:"nodeId"`
	ResourcesErrors []ResourceValidateError `json:"resourcesErrors"`
}

// ServiceInstanceAlert system alert structure.
type ServiceInstanceAlert struct {
	aostypes.InstanceIdent
	AosVersion uint64 `json:"aosVersion"`
	Message    string `json:"message"`
}

// AlertItem alert item structure.
type AlertItem struct {
	Timestamp time.Time   `json:"timestamp"`
	Tag       string      `json:"tag"`
	Payload   interface{} `json:"payload"`
}

// Alerts alerts message structure.
type Alerts struct {
	MessageType string `json:"messageType"`
	Items       []AlertItem
}
