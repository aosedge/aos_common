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
	ServiceDiscoveryType       = "serviceDiscovery"
	RenewCertsNotificationType = "renewCertificatesNotification"
	IssuedUnitCertsType        = "issuedUnitCertificates"
	OverrideEnvVarsType        = "overrideEnvVars"
)

// Device message types.
const (
	IssueUnitCertsType               = "issueUnitCertificates"
	InstallUnitCertsConfirmationType = "installUnitCertificatesConfirmation"
	OverrideEnvVarsStatusType        = "overrideEnvVarsStatus"
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
