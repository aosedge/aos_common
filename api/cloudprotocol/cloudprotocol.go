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

	"github.com/aosedge/aos_common/aostypes"
)

/***********************************************************************************************************************
 * Consts
 **********************************************************************************************************************/

// ProtocolVersion specifies supported protocol version.
const ProtocolVersion = 5

// UnitSecretVersion specifies supported version of UnitSecret message.
const UnitSecretVersion = 2

// Cloud message types.
const (
	RequestLogType             = "requestLog"
	ServiceDiscoveryType       = "serviceDiscovery"
	StateAcceptanceType        = "stateAcceptance"
	UpdateStateType            = "updateState"
	DeviceErrors               = "deviceErrors"
	RenewCertsNotificationType = "renewCertificatesNotification"
	IssuedUnitCertsType        = "issuedUnitCertificates"
	OverrideEnvVarsType        = "overrideEnvVars"
)

// Device message types.
const (
	AlertsType                       = "alerts"
	NewStateType                     = "newState"
	PushLogType                      = "pushLog"
	StateRequestType                 = "stateRequest"
	IssueUnitCertsType               = "issueUnitCertificates"
	InstallUnitCertsConfirmationType = "installUnitCertificatesConfirmation"
	OverrideEnvVarsStatusType        = "overrideEnvVarsStatus"
)

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

// ReceivedMessage structure for Aos incoming messages.
type ReceivedMessage struct {
	Header MessageHeader `json:"header"`
	Data   []byte        `json:"data"`
}

// Message structure for AOS messages.
type Message struct {
	Header MessageHeader `json:"header"`
	Data   interface{}   `json:"data"`
}

// MessageHeader message header.
type MessageHeader struct {
	Version     uint64 `json:"version"`
	SystemID    string `json:"systemId"`
	MessageType string `json:"messageType"`
}

// ServiceDiscoveryRequest service discovery request.
type ServiceDiscoveryRequest struct{}

// ServiceDiscoveryResponse service discovery response.
type ServiceDiscoveryResponse struct {
	Version    uint64         `json:"version"`
	Connection ConnectionInfo `json:"connection"`
}

// ConnectionInfo AMQP connection info.
type ConnectionInfo struct {
	SendParams    SendParams    `json:"sendParams"`
	ReceiveParams ReceiveParams `json:"receiveParams"`
}

// SendParams AMQP send parameters.
type SendParams struct {
	Host      string         `json:"host"`
	User      string         `json:"user"`
	Password  string         `json:"password"`
	Mandatory bool           `json:"mandatory"`
	Immediate bool           `json:"immediate"`
	Exchange  ExchangeParams `json:"exchange"`
}

// ExchangeParams AMQP exchange parameters.
type ExchangeParams struct {
	Name       string `json:"name"`
	Durable    bool   `json:"durable"`
	AutoDetect bool   `json:"autoDetect"`
	Internal   bool   `json:"internal"`
	NoWait     bool   `json:"noWait"`
}

// ReceiveParams AMQP receive parameters.
type ReceiveParams struct {
	Host      string    `json:"host"`
	User      string    `json:"user"`
	Password  string    `json:"password"`
	Consumer  string    `json:"consumer"`
	AutoAck   bool      `json:"autoAck"`
	Exclusive bool      `json:"exclusive"`
	NoLocal   bool      `json:"noLocal"`
	NoWait    bool      `json:"noWait"`
	Queue     QueueInfo `json:"queue"`
}

// QueueInfo AMQP queue info.
type QueueInfo struct {
	Name             string `json:"name"`
	Durable          bool   `json:"durable"`
	DeleteWhenUnused bool   `json:"deleteWhenUnused"`
	Exclusive        bool   `json:"exclusive"`
	NoWait           bool   `json:"noWait"`
}

// InstanceFilter instance filter structure.
type InstanceFilter struct {
	ServiceID *string `json:"serviceId,omitempty"`
	SubjectID *string `json:"subjectId,omitempty"`
	Instance  *uint64 `json:"instance,omitempty"`
}

// LogFilter request log message.
type LogFilter struct {
	From    *time.Time `json:"from"`
	Till    *time.Time `json:"till"`
	NodeIDs []string   `json:"nodeIds,omitempty"`
	InstanceFilter
}

// RequestLog request log message.
type RequestLog struct {
	LogID   string    `json:"logId"`
	LogType string    `json:"logType"`
	Filter  LogFilter `json:"filter"`
}

// StateAcceptance state acceptance message.
type StateAcceptance struct {
	aostypes.InstanceIdent
	Checksum string `json:"checksum"`
	Result   string `json:"result"`
	Reason   string `json:"reason"`
}

// UpdateState state update message.
type UpdateState struct {
	aostypes.InstanceIdent
	Checksum string `json:"stateChecksum"`
	State    string `json:"state"`
}

// NewState new state structure.
type NewState struct {
	aostypes.InstanceIdent
	Checksum string `json:"stateChecksum"`
	State    string `json:"state"`
}

// StateRequest state request structure.
type StateRequest struct {
	aostypes.InstanceIdent
	Default bool `json:"default"`
}

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
type Alerts []AlertItem

// PushLog push service log structure.
type PushLog struct {
	NodeID     string     `json:"nodeId"`
	LogID      string     `json:"logId"`
	PartsCount uint64     `json:"partsCount,omitempty"`
	Part       uint64     `json:"part,omitempty"`
	Content    []byte     `json:"content,omitempty"`
	ErrorInfo  *ErrorInfo `json:"errorInfo,omitempty"`
}

// ErrorInfo error information.
type ErrorInfo struct {
	AosCode  int    `json:"aosCode"`
	ExitCode int    `json:"exitCode"`
	Message  string `json:"message,omitempty"`
}

// RenewCertData renew certificate data.
type RenewCertData struct {
	Type      string    `json:"type"`
	NodeID    string    `json:"nodeId,omitempty"`
	Serial    string    `json:"serial"`
	ValidTill time.Time `json:"validTill"`
}

// RenewCertsNotification renew certificate notification from cloud with pwd.
type RenewCertsNotification struct {
	Certificates []RenewCertData `json:"certificates"`
	UnitSecret   UnitSecret      `json:"unitSecret"`
}

// IssueCertData issue certificate data.
type IssueCertData struct {
	Type   string `json:"type"`
	NodeID string `json:"nodeId,omitempty"`
	Csr    string `json:"csr"`
}

// IssueUnitCerts issue unit certificates request.
type IssueUnitCerts struct {
	Requests []IssueCertData `json:"requests"`
}

// IssuedCertData issued unit certificate data.
type IssuedCertData struct {
	Type             string `json:"type"`
	NodeID           string `json:"nodeId,omitempty"`
	CertificateChain string `json:"certificateChain"`
}

// IssuedUnitCerts issued unit certificates info.
type IssuedUnitCerts struct {
	Certificates []IssuedCertData `json:"certificates"`
}

// InstallCertData install certificate data.
type InstallCertData struct {
	Type        string `json:"type"`
	NodeID      string `json:"nodeId,omitempty"`
	Serial      string `json:"serial"`
	Status      string `json:"status"`
	Description string `json:"description,omitempty"`
}

// InstallUnitCertsConfirmation install unit certificates confirmation.
type InstallUnitCertsConfirmation struct {
	Certificates []InstallCertData `json:"certificates"`
}

// OverrideEnvVars request to override service environment variables.
type OverrideEnvVars struct {
	OverrideEnvVars []EnvVarsInstanceInfo `json:"overrideEnvVars"`
}

// EnvVarsInstanceInfo struct with envs and related service and user.
type EnvVarsInstanceInfo struct {
	InstanceFilter
	EnvVars []EnvVarInfo `json:"envVars"`
}

// EnvVarInfo env info with id and time to live.
type EnvVarInfo struct {
	ID       string     `json:"id"`
	Variable string     `json:"variable"`
	TTL      *time.Time `json:"ttl"`
}

// OverrideEnvVarsStatus override env status.
type OverrideEnvVarsStatus struct {
	OverrideEnvVarsStatus []EnvVarsInstanceStatus `json:"overrideEnvVarsStatus"`
}

// EnvVarsInstanceStatus struct with envs status and related service and user.
type EnvVarsInstanceStatus struct {
	InstanceFilter
	Statuses []EnvVarStatus `json:"statuses"`
}

// EnvVarStatus env status with error message.
type EnvVarStatus struct {
	ID    string `json:"id"`
	Error string `json:"error,omitempty"`
}

// UnitSecret keeps unit secret used to decode secure device password.
type UnitSecret struct {
	Version int `json:"version"`
	Data    struct {
		OwnerPassword string `json:"ownerPassword"`
	} `json:"data"`
}

/***********************************************************************************************************************
 * Public
 **********************************************************************************************************************/

func NewInstanceFilter(serviceID, subjectID string, instance int64) (filter InstanceFilter) {
	if serviceID != "" {
		filter.ServiceID = &serviceID
	}

	if subjectID != "" {
		filter.SubjectID = &subjectID
	}

	if instance != -1 {
		localInstance := (uint64)(instance)

		filter.Instance = &localInstance
	}

	return filter
}
