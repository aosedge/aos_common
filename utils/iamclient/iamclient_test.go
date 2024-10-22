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

package iamclient_test

import (
	"context"
	"crypto/x509"
	"fmt"
	"math/rand"
	"net"
	"net/url"
	"os"
	"path"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/aosedge/aos_common/aoserrors"
	"github.com/aosedge/aos_common/aostypes"
	"github.com/aosedge/aos_common/api/cloudprotocol"
	pb "github.com/aosedge/aos_common/api/iamanager"
	"github.com/aosedge/aos_common/utils/cryptutils"
	"github.com/aosedge/aos_common/utils/iamclient"
	"github.com/aosedge/aos_common/utils/pbconvert"
	"github.com/golang/protobuf/ptypes/empty"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

/***********************************************************************************************************************
 * Consts
 **********************************************************************************************************************/

const (
	protectedServerURL = "localhost:8089"
	publicServerURL    = "localhost:8090"
	iamCertStorage     = "iam"
	secretLength       = 8
	secretSymbols      = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

/***********************************************************************************************************************
 * Types
 **********************************************************************************************************************/

type testIAMPublicIdentityServiceServer struct {
	pb.UnimplementedIAMPublicIdentityServiceServer
	subjects chan *pb.Subjects

	currentSubjects []string
	systemID        string
}

type testIAMPublicNodesServiceServer struct {
	pb.UnimplementedIAMPublicNodesServiceServer
	nodeInfo chan *pb.NodeInfo

	currentID       string
	currentNodeName string
}

type testIAMPermissionsServer struct {
	pb.UnimplementedIAMPermissionsServiceServer
	pb.UnimplementedIAMPublicPermissionsServiceServer

	permissionsCache map[string]servicePermissions
}

type servicePermissions struct {
	instanceIdent aostypes.InstanceIdent
	permissions   map[string]map[string]string
}

type testPublicServer struct {
	pb.UnimplementedIAMPublicServiceServer
	testIAMPublicIdentityServiceServer
	testIAMPublicNodesServiceServer

	grpcServer   *grpc.Server
	certInfoChan chan *pb.CertInfo

	systemID string
	certURL  map[string]string
	keyURL   map[string]string
}

type testProtectedServer struct {
	pb.UnimplementedIAMCertificateServiceServer
	pb.UnimplementedIAMProvisioningServiceServer
	pb.UnimplementedIAMNodesServiceServer

	grpcServer *grpc.Server
	csr        map[string]string
	certURL    map[string]string
}

type testServer struct {
	testPublicServer
	testProtectedServer
	testIAMPermissionsServer
}

type testSender struct {
	csr                        map[string]string
	currentConfirmations       []cloudprotocol.InstallCertData
	startProvisioningResponse  *cloudprotocol.StartProvisioningResponse
	finishProvisioningResponse *cloudprotocol.FinishProvisioningResponse
	deprovisioningResponse     *cloudprotocol.DeprovisioningResponse
}

type testCertProvider struct{}

/***********************************************************************************************************************
 * Vars
 **********************************************************************************************************************/

var tmpDir string

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
	var err error

	tmpDir, err = os.MkdirTemp("", "iam_")
	if err != nil {
		log.Fatalf("Error create temporary dir: %v", err)
	}

	ret := m.Run()

	if err := os.RemoveAll(tmpDir); err != nil {
		log.Fatalf("Error removing tmp dir: %v", err)
	}

	os.Exit(ret)
}

/***********************************************************************************************************************
 * Tests
 **********************************************************************************************************************/

func TestGetSystemID(t *testing.T) {
	server, err := newTestServer(publicServerURL, protectedServerURL)
	if err != nil {
		t.Fatalf("Can't create test server: %v", err)
	}

	defer server.close()

	server.testPublicServer.systemID = "testID"

	client, err := iamclient.New(publicServerURL, protectedServerURL, iamCertStorage, &testSender{}, nil, true)
	if err != nil {
		t.Fatalf("Can't create IAM client: %v", err)
	}
	defer client.Close()

	if client.GetSystemID() != server.testPublicServer.systemID {
		t.Errorf("Invalid system ID: %s", client.GetSystemID())
	}
}

func TestRenewCertificatesNotification(t *testing.T) {
	sender := &testSender{}

	server, err := newTestServer(publicServerURL, protectedServerURL)
	if err != nil {
		t.Fatalf("Can't create test server: %v", err)
	}

	defer server.close()

	server.testProtectedServer.csr = map[string]string{"online": "onlineCSR", "offline": "offlineCSR"}

	client, err := iamclient.New(publicServerURL, protectedServerURL, iamCertStorage, sender, nil, true)
	if err != nil {
		t.Fatalf("Can't create IAM client: %v", err)
	}
	defer client.Close()

	certInfo := []cloudprotocol.RenewCertData{
		{NodeID: "node0", Type: "online", Serial: "serial1", ValidTill: time.Now()},
		{NodeID: "node0", Type: "offline", Serial: "serial2", ValidTill: time.Now()},
	}
	secrets := cloudprotocol.UnitSecrets{Nodes: map[string]string{"node0": "pwd"}}

	if err = client.RenewCertificatesNotification(secrets, certInfo); err != nil {
		t.Fatalf("Can't process renew certificate notification: %v", err)
	}

	if !reflect.DeepEqual(server.testProtectedServer.csr, sender.csr) {
		t.Errorf("Wrong sender CSR: %v", sender.csr)
	}
}

func TestInstallCertificates(t *testing.T) {
	sender := &testSender{}

	server, err := newTestServer(publicServerURL, protectedServerURL)
	if err != nil {
		t.Fatalf("Can't create test server: %v", err)
	}

	defer server.close()

	// openssl req -newkey rsa:2048 -nodes -keyout online_key.pem -x509 -days 365 -out online_cert.pem -set_serial 1
	onlineCert := `
-----BEGIN CERTIFICATE-----
MIIDTjCCAjagAwIBAgIBATANBgkqhkiG9w0BAQsFADBAMQswCQYDVQQGEwJVQTET
MBEGA1UECAwKU29tZS1TdGF0ZTENMAsGA1UEBwwES3lpdjENMAsGA1UECgwERVBB
TTAeFw0yMDA5MTAxNDE1MzNaFw0yMTA5MTAxNDE1MzNaMEAxCzAJBgNVBAYTAlVB
MRMwEQYDVQQIDApTb21lLVN0YXRlMQ0wCwYDVQQHDARLeWl2MQ0wCwYDVQQKDARF
UEFNMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAvc1utrAEKlJ5rB9W
envNiEjGDW36NKmEc69nITCmI7wedg00oRSIOZTfZVdKp/Nsh0uuFnOYKt9unXso
fYGzCk4KxWL+t9HDsWMbpL7S/QNB1UF7P+rQFKp7gsj8wQmy+rpvRxwuTC7BfpJ9
az9WoreF4W43m3zDF4lIetnFDilx1NBYBfvW7/KW3e/iJcjs8WPSQCDVU3rOhFRd
8Y/qt4EOiJY5xTya0YFxgF37fKH/asy+ija54Wy7DhbLlkyE2JUqxp/SaomzUAQX
uf/kWYPh/s7VewjfW9xouT9aDLZEsLNQMEk6HRPY9DW2pCdYFjE8qdVYAs/f0V5y
QhPUaQIDAQABo1MwUTAdBgNVHQ4EFgQUCwq4ojzTld4lTka0POLVqgkyMJ4wHwYD
VR0jBBgwFoAUCwq4ojzTld4lTka0POLVqgkyMJ4wDwYDVR0TAQH/BAUwAwEB/zAN
BgkqhkiG9w0BAQsFAAOCAQEAIDdNqOeK4JyZtETgfwj5gqneZ9EH+euJ6HaOict8
7PZzyEt2p0QHZOWBm0s07kd+7SiS+UCLIYjJPCMWDAEN7V6zZqqp9oQL5bR1hSSe
7tDrOgVkEf8T1x4u4F2bpLJsz+GDX7t8H5lcvPGq0YkPDHluSASFJc//GN3TrSDV
yGuI5R8+7XalVAaCteo/Y2zhERMDsVm0KTGeTP2sblaBVFux7GTcWA1DHqVDYFnw
ExQScaNHy16y+a/nlDSpTraFIQdSG3twwlfyjR/Ua5SbkzzTEVG+Kll/3VvOagwV
8xTUZedoJTC7G5C2DGs+syl/B8WVrn7QPK+VSVU2QEG50Q==
-----END CERTIFICATE-----
`

	// openssl req -newkey rsa:2048 -nodes -keyout offline_key.pem -x509 -days 365 -out offline_cert.pem -set_serial 2
	offlineCert := `
-----BEGIN CERTIFICATE-----
MIIDTjCCAjagAwIBAgIBAjANBgkqhkiG9w0BAQsFADBAMQswCQYDVQQGEwJVQTET
MBEGA1UECAwKU29tZS1TdGF0ZTENMAsGA1UEBwwES3lpdjENMAsGA1UECgwERVBB
TTAeFw0yMDA5MTAxNDE1NTdaFw0yMTA5MTAxNDE1NTdaMEAxCzAJBgNVBAYTAlVB
MRMwEQYDVQQIDApTb21lLVN0YXRlMQ0wCwYDVQQHDARLeWl2MQ0wCwYDVQQKDARF
UEFNMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAoDW6XO7bHeTRnF10
5FZ6wFKnyp0v6aACJxuWrLdOGlOZeaO/7q1I1qD5ogxV2Fh6IHm6+4e+wo4lLAUX
F7Fi/ab/FZjHU6a2XNxNi6VnE7C7wp/UDt8/KfHV6ZzkyubHoraMl5Xi8g+3IN1N
kTc90OCZPL6aZHpMXaP+5dOXad/Pv2/6xHKB/Qx+VIQlo1c7oGrYxFc29LsUtlrP
C0tcDVAfMBBlRIhkw/dXoGcBSfqBIAnORYq/kGcbEewWDLdBiExr+TtJh837NgiE
q1+NIvWF4AS3vL+tm/rvXfz71v3Gy4JqL/8Eqn489VE2Vw/XJfLjte8fPpKc5NVN
Ah7bsQIDAQABo1MwUTAdBgNVHQ4EFgQUdjLMDiQH1r6a4ra3GX9hwdVzlWkwHwYD
VR0jBBgwFoAUdjLMDiQH1r6a4ra3GX9hwdVzlWkwDwYDVR0TAQH/BAUwAwEB/zAN
BgkqhkiG9w0BAQsFAAOCAQEAYCm82d7bMk3bROnNvIp/IcZdO6SItuhTy6iPQRd8
ZqIIPsOG/8uMTKimCvJZIhsb+P9rRj3Ubb4EAAHpftZJx6Y1yrGlFUVYhsDNSsQR
RqT4gg71B7R3Mgu0T9tV96nNa7P0w32wvNtc7B/it8DsFMz1wOcq/PPh3ufoR9Lm
onsMCZ7ep/quYwXutmvMrE3SLGApTc7lnqPqBtVq4ju29VS6wDsvLgEyuZQO9HlJ
KbjNq7kDX6ZbgJTVgVwEVY16464lTJ3j6/Osi3R3bUs5cg4onCFAS5KUTsfkbZ+G
KzpDMr/kcScwzmmNcN8aLp31TSRVee64QrK7yF3YJxL+rA==
-----END CERTIFICATE-----	
`

	if err = os.WriteFile(path.Join(tmpDir, "online_cert.pem"), []byte(onlineCert), 0o600); err != nil {
		t.Fatalf("Error create online cert: %v", err)
	}

	if err = os.WriteFile(path.Join(tmpDir, "offline_cert.pem"), []byte(offlineCert), 0o600); err != nil {
		t.Fatalf("Error create offline cert: %v", err)
	}

	onlineURL := url.URL{Scheme: "file", Path: path.Join(tmpDir, "online_cert.pem")}
	offlineURL := url.URL{Scheme: "file", Path: path.Join(tmpDir, "offline_cert.pem")}

	server.testProtectedServer.certURL = map[string]string{
		"online":  onlineURL.String(),
		"offline": offlineURL.String(),
	}

	client, err := iamclient.New(publicServerURL, protectedServerURL, iamCertStorage, sender, nil, true)
	if err != nil {
		t.Fatalf("Can't create IAM client: %v", err)
	}
	defer client.Close()

	expectedConfimation := []cloudprotocol.InstallCertData{
		{Type: "online", Serial: "", Status: "installed"},
		{Type: "offline", Serial: "", Status: "installed"},
		{Type: "invalid", Serial: "", Status: "not installed", Description: "error"},
	}

	certInfo := []cloudprotocol.IssuedCertData{
		{Type: "online", CertificateChain: "onlineCert"},
		{Type: "offline", CertificateChain: "offlineCert"},
		{Type: "invalid", CertificateChain: "invalid"},
	}

	if err = client.InstallCertificates(certInfo, &testCertProvider{}); err != nil {
		t.Fatalf("Can't process install certificates request: %v", err)
	}

	if !reflect.DeepEqual(expectedConfimation, sender.currentConfirmations) {
		log.Debug(expectedConfimation)
		log.Debug(sender.currentConfirmations)
		t.Error("Wrong install confirmation")
	}
}

func TestGetCertificates(t *testing.T) {
	server, err := newTestServer(publicServerURL, protectedServerURL)
	if err != nil {
		t.Fatalf("Can't create test server: %v", err)
	}

	defer server.close()

	server.testPublicServer.certURL = map[string]string{"online": "onlineCertURL", "offline": "offlineCertURL"}
	server.testPublicServer.keyURL = map[string]string{"online": "onlineKeyURL", "offline": "offlineKeyURL"}

	client, err := iamclient.New(publicServerURL, protectedServerURL, iamCertStorage, &testSender{}, nil, true)
	if err != nil {
		t.Fatalf("Can't create IAM client: %v", err)
	}
	defer client.Close()

	for serial, certType := range []string{"online", "offline"} {
		certURL, keyURL, err := client.GetCertificate(certType, nil, strconv.Itoa(serial))
		if err != nil {
			t.Errorf("Can't get %s certificate: %s", certType, err)
			continue
		}

		if certURL != server.testPublicServer.certURL[certType] {
			t.Errorf("Wrong %s cert URL: %s", certType, certURL)
		}

		if keyURL != server.testPublicServer.keyURL[certType] {
			t.Errorf("Wrong %s key URL: %s", certType, keyURL)
		}
	}
}

func TestSubscribeCert(t *testing.T) {
	server, err := newTestServer(publicServerURL, protectedServerURL)
	if err != nil {
		t.Fatalf("Can't create test server: %v", err)
	}

	defer server.close()

	client, err := iamclient.New(publicServerURL, protectedServerURL, iamCertStorage, &testSender{}, nil, true)
	if err != nil {
		t.Fatalf("Can't create IAM client: %v", err)
	}
	defer client.Close()

	iamCert := pb.CertInfo{
		Type:    "iam",
		CertUrl: "iam cert url",
		KeyUrl:  "iam key url",
	}

	ch, err := client.SubscribeCertChanged("iam")
	if err != nil {
		t.Fatalf("Can't subscribe cert change err=%v", err)
	}

	server.testPublicServer.certInfoChan <- &iamCert

	newCertInfo := <-ch

	if newCertInfo.GetType() != iamCert.GetType() || newCertInfo.GetCertUrl() != iamCert.GetCertUrl() ||
		newCertInfo.GetCertUrl() != iamCert.GetCertUrl() {
		t.Fatalf("Can't subscribe cert change err=%v", err)
	}
}

func TestDisconnected(t *testing.T) {
	server, err := newTestServer(publicServerURL, protectedServerURL)
	if err != nil {
		t.Fatalf("Can't create test server: %v", err)
	}

	client, err := iamclient.New(publicServerURL, protectedServerURL, iamCertStorage, &testSender{}, nil, true)
	if err != nil {
		t.Fatalf("Can't create IAM client: %v", err)
	}
	defer client.Close()

	server.testPublicServer.certURL = map[string]string{"online": "onlineCertURL"}
	server.testPublicServer.keyURL = map[string]string{"online": "onlineKeyURL"}

	_, err = client.SubscribeCertChanged("online")
	if err != nil {
		t.Fatalf("Can't subscribe on CertChanged: %v", err)
	}

	_, err = client.SubscribeCertChanged("offline")
	if err != nil {
		t.Fatalf("Can't subscribe on CertChanged: %v", err)
	}

	// reconnect
	server.close()

	server, err = newTestServer(publicServerURL, protectedServerURL)
	if err != nil {
		t.Fatalf("Can't create test server: %v", err)
	}

	server.testPublicServer.certURL = map[string]string{"online": "onlineCertURL"}
	server.testPublicServer.keyURL = map[string]string{"online": "onlineKeyURL"}

	defer server.close()

	time.Sleep(1 * time.Second)

	_, _, err = client.GetCertificate("online", nil, "1")
	if err != nil {
		t.Fatalf("Can't get certificate: %v", err)
	}
}

func TestGetNodeInfo(t *testing.T) {
	server, err := newTestServer(publicServerURL, protectedServerURL)
	if err != nil {
		t.Fatalf("Can't create test server: %v", err)
	}

	defer server.close()

	client, err := iamclient.New(publicServerURL, protectedServerURL, iamCertStorage, &testSender{}, nil, true)
	if err != nil {
		t.Fatalf("Can't create IAM client: %v", err)
	}
	defer client.Close()

	nodeInfo, err := client.GetNodeInfo(server.testPublicServer.currentID)
	if err != nil {
		t.Errorf("Error is not expected: %v", err)
	}

	if nodeInfo.NodeID != server.testPublicServer.currentID {
		t.Errorf("Not expected NodeId: %s", nodeInfo.NodeID)
	}

	if nodeInfo.Name != server.testPublicServer.currentNodeName {
		t.Errorf("Not expected NodeId: %s", nodeInfo.Name)
	}
}

func TestGetCurrentNodeInfo(t *testing.T) {
	server, err := newTestServer(publicServerURL, protectedServerURL)
	if err != nil {
		t.Fatalf("Can't create test server: %v", err)
	}

	defer server.close()

	client, err := iamclient.New(publicServerURL, protectedServerURL, iamCertStorage, &testSender{}, nil, true)
	if err != nil {
		t.Fatalf("Can't create IAM client: %v", err)
	}
	defer client.Close()

	nodeInfo, err := client.GetCurrentNodeInfo()
	if err != nil {
		t.Errorf("Error is not expected: %v", err)
	}

	if nodeInfo.NodeID != server.testPublicServer.currentID {
		t.Errorf("Not expected NodeId: %s", nodeInfo.NodeID)
	}

	if nodeInfo.Name != server.testPublicServer.currentNodeName {
		t.Errorf("Not expected NodeId: %s", nodeInfo.Name)
	}
}

func TestSubscribeNodeInfoChange(t *testing.T) {
	server, err := newTestServer(publicServerURL, protectedServerURL)
	if err != nil {
		t.Fatalf("Can't create test server: %v", err)
	}

	defer server.close()

	client, err := iamclient.New(publicServerURL, protectedServerURL, iamCertStorage, &testSender{}, nil, true)
	if err != nil {
		t.Fatalf("Can't create IAM client: %v", err)
	}

	stream := client.SubscribeNodeInfoChange()

	secondaryNodeInfo := pb.NodeInfo{NodeId: "secondary", NodeType: "secondary"}
	server.testPublicServer.nodeInfo <- &secondaryNodeInfo

	receivedNodeInfo := <-stream

	defer client.Close()

	if secondaryNodeInfo.GetNodeId() != receivedNodeInfo.NodeID {
		t.Errorf("NodeInfo with not expected Id: %s", receivedNodeInfo.NodeID)
	}

	if secondaryNodeInfo.GetName() != receivedNodeInfo.Name {
		t.Errorf("NodeInfo with not expected Name: %s", receivedNodeInfo.Name)
	}
}

func TestStartProvisioning(t *testing.T) {
	sender := &testSender{}

	server, err := newTestServer(publicServerURL, protectedServerURL)
	if err != nil {
		t.Fatalf("Can't create test server: %v", err)
	}

	defer server.close()

	server.testProtectedServer.csr = map[string]string{"online": "onlineCSR", "offline": "offlineCSR"}

	client, err := iamclient.New(publicServerURL, protectedServerURL, iamCertStorage, sender, nil, true)
	if err != nil {
		t.Fatalf("Can't create IAM client: %v", err)
	}
	defer client.Close()

	err = client.StartProvisioning("node1", "password")
	if err != nil {
		t.Errorf("Error is not expected: %v", err)
	}

	expectedResponse := cloudprotocol.StartProvisioningResponse{
		MessageType: cloudprotocol.StartProvisioningResponseMessageType,
		NodeID:      "node1",
		CSRs: []cloudprotocol.IssueCertData{
			{NodeID: "node1", Type: "online", Csr: "onlineCSR"},
			{NodeID: "node1", Type: "offline", Csr: "offlineCSR"},
		},
	}

	if sender.startProvisioningResponse == nil {
		t.Error("Sender didn't receive start provisioning response")
	}

	if !reflect.DeepEqual(expectedResponse, *sender.startProvisioningResponse) {
		log.Debug(expectedResponse)
		log.Debug(*sender.startProvisioningResponse)
		t.Error("Wrong start provisioning response")
	}
}

func TestFinishProvisioning(t *testing.T) {
	sender := &testSender{}

	server, err := newTestServer(publicServerURL, protectedServerURL)
	if err != nil {
		t.Fatalf("Can't create test server: %v", err)
	}

	defer server.close()

	server.testProtectedServer.certURL = map[string]string{"online": "onlineCSR", "offline": "offlineCSR"}

	client, err := iamclient.New(publicServerURL, protectedServerURL, iamCertStorage, sender, nil, true)
	if err != nil {
		t.Fatalf("Can't create IAM client: %v", err)
	}
	defer client.Close()

	err = client.FinishProvisioning("node2", "password",
		[]cloudprotocol.IssuedCertData{
			{NodeID: "node1", Type: "online", CertificateChain: "onlineCSR"},
			{NodeID: "node1", Type: "offline", CertificateChain: "offlineCSR"},
		},
	)
	if err != nil {
		t.Errorf("Error is not expected: %v", err)
	}

	if sender.finishProvisioningResponse == nil {
		t.Error("Sender didn't receive finish provisioning response")
	}

	if sender.finishProvisioningResponse.ErrorInfo != nil {
		t.Errorf("Error is not expected: %v", sender.finishProvisioningResponse.ErrorInfo)
	}
}

func TestDeprovisioning(t *testing.T) {
	sender := &testSender{}

	server, err := newTestServer(publicServerURL, protectedServerURL)
	if err != nil {
		t.Fatalf("Can't create test server: %v", err)
	}

	defer server.close()

	client, err := iamclient.New(publicServerURL, protectedServerURL, iamCertStorage, sender, nil, true)
	if err != nil {
		t.Fatalf("Can't create IAM client: %v", err)
	}
	defer client.Close()

	err = client.Deprovision("test-node-id", "password")
	if err != nil {
		t.Errorf("Error is not expected: %v", err)
	}

	if sender.deprovisioningResponse == nil {
		t.Error("Sender didn't receive deprovisioning response")
	}

	if sender.deprovisioningResponse.ErrorInfo != nil {
		t.Errorf("Error is not expected: %v", sender.deprovisioningResponse.ErrorInfo)
	}
}

func TestFailDeprovisionMainNode(t *testing.T) {
	sender := &testSender{}

	server, err := newTestServer(publicServerURL, protectedServerURL)
	if err != nil {
		t.Fatalf("Can't create test server: %v", err)
	}

	defer server.close()

	client, err := iamclient.New(publicServerURL, protectedServerURL, iamCertStorage, sender, nil, true)
	if err != nil {
		t.Fatalf("Can't create IAM client: %v", err)
	}
	defer client.Close()

	err = client.Deprovision(server.testPublicServer.currentID, "password")
	if err == nil {
		t.Errorf("Deprovisioning main node should fail")
	}
}

func TestPauseNode(t *testing.T) {
	server, err := newTestServer(publicServerURL, protectedServerURL)
	if err != nil {
		t.Fatalf("Can't create test server: %v", err)
	}

	defer server.close()

	client, err := iamclient.New(publicServerURL, protectedServerURL, iamCertStorage, &testSender{}, nil, true)
	if err != nil {
		t.Fatalf("Can't create IAM client: %v", err)
	}
	defer client.Close()

	err = client.PauseNode("test-node-id")
	if err != nil {
		t.Errorf("Error is not expected: %v", err)
	}
}

func TestResumeNode(t *testing.T) {
	server, err := newTestServer(publicServerURL, protectedServerURL)
	if err != nil {
		t.Fatalf("Can't create test server: %v", err)
	}

	defer server.close()

	client, err := iamclient.New(publicServerURL, protectedServerURL, iamCertStorage, &testSender{}, nil, true)
	if err != nil {
		t.Fatalf("Can't create IAM client: %v", err)
	}
	defer client.Close()

	err = client.ResumeNode("test-node-id")
	if err != nil {
		t.Errorf("Error is not expected: %v", err)
	}
}

func TestGetUnitSubjects(t *testing.T) {
	server, err := newTestServer(publicServerURL, protectedServerURL)
	if err != nil {
		t.Fatalf("Can't create test server: %v", err)
	}

	defer server.close()

	server.testPublicServer.systemID = "testID"

	client, err := iamclient.New(publicServerURL, protectedServerURL, iamCertStorage, &testSender{}, nil, true)
	if err != nil {
		t.Fatalf("Can't create IAM client: %v", err)
	}
	defer client.Close()

	subjects, err := client.GetUnitSubjects()
	if err != nil {
		t.Errorf("Can't get subjects: %v", err)
	}

	if !reflect.DeepEqual(subjects, server.testPublicServer.currentSubjects) {
		t.Errorf("Subjects mismatch: expected = %v, got = %v", server.testPublicServer.currentSubjects, subjects)
	}
}

func TestSubscribeUnitSubjectsChanged(t *testing.T) {
	server, err := newTestServer(publicServerURL, protectedServerURL)
	if err != nil {
		t.Fatalf("Can't create test server: %v", err)
	}

	defer server.close()

	client, err := iamclient.New(publicServerURL, protectedServerURL, iamCertStorage, &testSender{}, nil, true)
	if err != nil {
		t.Fatalf("Can't create IAM client: %v", err)
	}
	defer client.Close()

	stream := client.SubscribeUnitSubjectsChanged()

	newSubjects := []string{"new1", "new2"}

	server.testPublicServer.subjects <- &pb.Subjects{Subjects: newSubjects}

	if receivedSubjects := <-stream; !reflect.DeepEqual(newSubjects, receivedSubjects) {
		t.Errorf("Subjects mismatch: expected = %v, got = %v", newSubjects, receivedSubjects)
	}
}

func TestRegisterService(t *testing.T) {
	server, err := newTestServer(publicServerURL, protectedServerURL)
	if err != nil {
		t.Fatalf("Can't create test server: %v", err)
	}

	defer server.close()

	client, err := iamclient.New(publicServerURL, protectedServerURL, iamCertStorage, &testSender{}, nil, true)
	if err != nil {
		t.Fatalf("Can't create IAM client: %v", err)
	}

	defer client.Close()

	var (
		permissions      = map[string]map[string]string{"vis": {"*": "rw", "test": "r"}}
		registerInstance = aostypes.InstanceIdent{ServiceID: "serviceID1", SubjectID: "s1", Instance: 1}
	)

	secret, err := client.RegisterInstance(registerInstance, permissions)
	if err != nil || secret == "" {
		t.Errorf("Can't send a request: %v", err)
	}

	secret, err = client.RegisterInstance(registerInstance, permissions)
	if err == nil || secret != "" {
		t.Error("Re-registration of the service is prohibited")
	}

	err = client.UnregisterInstance(registerInstance)
	if err != nil {
		t.Errorf("Can't send a request: %v", err)
	}

	secret, err = client.RegisterInstance(registerInstance, permissions)
	if err != nil || secret == "" {
		t.Errorf("Can't send a request: %v", err)
	}
}

func TestGetPermissions(t *testing.T) {
	server, err := newTestServer(publicServerURL, protectedServerURL)
	if err != nil {
		t.Fatalf("Can't create test server: %v", err)
	}

	defer server.close()

	client, err := iamclient.New(publicServerURL, protectedServerURL, iamCertStorage, &testSender{}, nil, true)
	if err != nil {
		t.Fatalf("Can't create IAM client: %v", err)
	}

	defer client.Close()

	var (
		permissions      = map[string]map[string]string{"vis": {"*": "rw", "test": "r"}}
		registerInstance = aostypes.InstanceIdent{ServiceID: "serviceID1", SubjectID: "s1", Instance: 1}
	)

	secret, err := client.RegisterInstance(registerInstance, permissions)
	if err != nil || secret == "" {
		t.Errorf("Can't send a request: %v", err)
	}

	receivedInstance, respPermissions, err := client.GetPermissions(secret, "vis")
	if err != nil {
		t.Errorf("Can't send a request: %v", err)
	}

	if !reflect.DeepEqual(respPermissions, permissions["vis"]) {
		t.Errorf("Wrong permissions: %v", respPermissions)
	}

	if receivedInstance != registerInstance {
		t.Error("Incorrect received instance")
	}

	err = client.UnregisterInstance(registerInstance)
	if err != nil {
		t.Errorf("Can't send a request: %v", err)
	}

	_, _, err = client.GetPermissions(secret, "vis")
	if err == nil {
		t.Error("Getting permissions after removing a service")
	}
}

/*******************************************************************************
 * Private
 ******************************************************************************/

func newTestServer(
	publicServerURL, protectedServerURL string,
) (server *testServer, err error) {
	server = &testServer{
		testIAMPermissionsServer: testIAMPermissionsServer{
			permissionsCache: make(map[string]servicePermissions),
		},
	}

	publicListener, err := net.Listen("tcp", publicServerURL)
	if err != nil {
		return nil, aoserrors.Wrap(err)
	}

	server.testPublicServer.grpcServer = grpc.NewServer()
	server.testPublicServer.nodeInfo = make(chan *pb.NodeInfo)
	server.testPublicServer.certInfoChan = make(chan *pb.CertInfo, 1)
	server.testPublicServer.currentSubjects = []string{"initial1", "initial2"}
	server.testPublicServer.subjects = make(chan *pb.Subjects)

	pb.RegisterIAMPublicServiceServer(server.testPublicServer.grpcServer, &server.testPublicServer)
	pb.RegisterIAMPublicIdentityServiceServer(server.testPublicServer.grpcServer, &server.testPublicServer)
	pb.RegisterIAMPublicNodesServiceServer(server.testPublicServer.grpcServer,
		&server.testPublicServer.testIAMPublicNodesServiceServer)
	pb.RegisterIAMPublicPermissionsServiceServer(server.testPublicServer.grpcServer,
		&server.testIAMPermissionsServer)

	go func() {
		if err := server.testPublicServer.grpcServer.Serve(publicListener); err != nil {
			log.Errorf("Can't serve grpc server: %v", err)
		}
	}()

	protectedListener, err := net.Listen("tcp", protectedServerURL)
	if err != nil {
		return nil, aoserrors.Wrap(err)
	}

	server.testProtectedServer.grpcServer = grpc.NewServer()

	pb.RegisterIAMCertificateServiceServer(server.testProtectedServer.grpcServer, &server.testProtectedServer)
	pb.RegisterIAMProvisioningServiceServer(server.testProtectedServer.grpcServer, &server.testProtectedServer)
	pb.RegisterIAMNodesServiceServer(server.testProtectedServer.grpcServer, &server.testProtectedServer)
	pb.RegisterIAMPermissionsServiceServer(server.testProtectedServer.grpcServer, &server.testIAMPermissionsServer)

	go func() {
		if err := server.testProtectedServer.grpcServer.Serve(protectedListener); err != nil {
			log.Errorf("Can't serve grpc server: %v", err)
		}
	}()

	return server, nil
}

func (server *testPublicServer) close() (err error) {
	log.Debug("Closing public IAM server...")

	server.testIAMPublicNodesServiceServer.Close()

	if server.grpcServer != nil {
		server.grpcServer.Stop()
	}

	return nil
}

func (server *testProtectedServer) close() (err error) {
	log.Debug("Closing protected IAM server...")

	if server.grpcServer != nil {
		server.grpcServer.Stop()
	}

	return nil
}

func (server *testServer) close() {
	server.testProtectedServer.close()
	server.testPublicServer.close()
}

func (server *testProtectedServer) CreateKey(
	context context.Context, req *pb.CreateKeyRequest,
) (rsp *pb.CreateKeyResponse, err error) {
	rsp = &pb.CreateKeyResponse{Type: req.GetType()}

	csr, ok := server.csr[req.GetType()]
	if !ok {
		return rsp, aoserrors.New("not found")
	}

	rsp.Csr = csr

	return rsp, nil
}

func (server *testProtectedServer) ApplyCert(
	context context.Context, req *pb.ApplyCertRequest,
) (rsp *pb.ApplyCertResponse, err error) {
	rsp = &pb.ApplyCertResponse{Type: req.GetType()}

	certURL, ok := server.certURL[req.GetType()]
	if !ok {
		return rsp, aoserrors.New("not found")
	}

	rsp.CertUrl = certURL

	return rsp, nil
}

func (server *testProtectedServer) GetCertTypes(
	context context.Context, req *pb.GetCertTypesRequest,
) (rsp *pb.CertTypes, err error) {
	rsp = &pb.CertTypes{Types: []string{"online", "offline"}}

	return rsp, nil
}

func (server *testProtectedServer) StartProvisioning(
	context context.Context, req *pb.StartProvisioningRequest,
) (rsp *pb.StartProvisioningResponse, err error) {
	rsp = &pb.StartProvisioningResponse{}

	return rsp, nil
}

func (server *testProtectedServer) FinishProvisioning(
	context context.Context, req *pb.FinishProvisioningRequest,
) (rsp *pb.FinishProvisioningResponse, err error) {
	rsp = &pb.FinishProvisioningResponse{}

	return rsp, nil
}

func (server *testProtectedServer) Deprovision(
	context context.Context, req *pb.DeprovisionRequest,
) (rsp *pb.DeprovisionResponse, err error) {
	rsp = &pb.DeprovisionResponse{}

	return rsp, nil
}

func (server *testProtectedServer) PauseNode(
	context context.Context, req *pb.PauseNodeRequest,
) (rsp *pb.PauseNodeResponse, err error) {
	rsp = &pb.PauseNodeResponse{}

	return rsp, nil
}

func (server *testProtectedServer) ResumeNode(
	context context.Context, req *pb.ResumeNodeRequest,
) (rsp *pb.ResumeNodeResponse, err error) {
	rsp = &pb.ResumeNodeResponse{}

	return rsp, nil
}

func (server *testPublicServer) GetCert(
	context context.Context, req *pb.GetCertRequest,
) (rsp *pb.CertInfo, err error) {
	rsp = &pb.CertInfo{Type: req.GetType()}

	certURL, ok := server.certURL[req.GetType()]
	if !ok {
		return rsp, aoserrors.New("not found")
	}

	keyURL, ok := server.keyURL[req.GetType()]
	if !ok {
		return rsp, aoserrors.New("not found")
	}

	rsp.CertUrl = certURL
	rsp.KeyUrl = keyURL

	return rsp, nil
}

func (server *testPublicServer) SubscribeCertChanged(
	req *pb.SubscribeCertChangedRequest, stream pb.IAMPublicService_SubscribeCertChangedServer,
) error {
	certInfo := <-server.certInfoChan

	return aoserrors.Wrap(stream.SendMsg(certInfo))
}

func (server *testPublicServer) GetCertTypes(context context.Context, req *empty.Empty) (rsp *pb.CertTypes, err error) {
	return rsp, nil
}

func (server *testPublicServer) GetSystemInfo(
	context context.Context, req *empty.Empty,
) (rsp *pb.SystemInfo, err error) {
	rsp = &pb.SystemInfo{SystemId: server.systemID}

	return rsp, nil
}

func (server *testPublicServer) GetNodeInfo(context context.Context, req *empty.Empty) (*pb.NodeInfo, error) {
	return &pb.NodeInfo{}, nil
}

func (server *testIAMPublicIdentityServiceServer) GetSystemInfo(
	context context.Context, req *empty.Empty,
) (rsp *pb.SystemInfo, err error) {
	rsp = &pb.SystemInfo{SystemId: server.systemID}

	return rsp, nil
}

func (server *testIAMPublicIdentityServiceServer) GetSubjects(
	context.Context, *emptypb.Empty,
) (rsp *pb.Subjects, err error) {
	rsp = &pb.Subjects{Subjects: server.currentSubjects}

	return rsp, nil
}

func (server *testIAMPublicIdentityServiceServer) SubscribeSubjectsChanged(
	empty *emptypb.Empty, stream pb.IAMPublicIdentityService_SubscribeSubjectsChangedServer,
) error {
	log.Error("testIAMPublicIdentityServiceServer SubscribeSubjectsChanged")

	subjects := <-server.subjects

	return aoserrors.Wrap(stream.Send(subjects))
}

func (server *testIAMPublicNodesServiceServer) GetAllNodeIDs(context context.Context, req *emptypb.Empty) (
	*pb.NodesID, error,
) {
	return &pb.NodesID{}, nil
}

func (server *testIAMPublicNodesServiceServer) GetNodeInfo(context context.Context, req *pb.GetNodeInfoRequest) (
	*pb.NodeInfo, error,
) {
	return &pb.NodeInfo{NodeId: server.currentID, Name: server.currentNodeName}, nil
}

func (server *testIAMPublicNodesServiceServer) SubscribeNodeChanged(
	empty *emptypb.Empty, stream pb.IAMPublicNodesService_SubscribeNodeChangedServer,
) error {
	log.Error("testIAMPublicNodesServiceServer SubscribeNodeChanged")

	nodeInfo := <-server.nodeInfo

	return aoserrors.Wrap(stream.Send(nodeInfo))
}

func (server *testIAMPublicNodesServiceServer) RegisterNode(pb.IAMPublicNodesService_RegisterNodeServer) error {
	return nil
}

func (server *testIAMPublicNodesServiceServer) Close() {
	close(server.nodeInfo)
}

func (sender *testSender) SendIssueUnitCerts(requests []cloudprotocol.IssueCertData) (err error) {
	sender.csr = make(map[string]string)

	for _, request := range requests {
		sender.csr[request.Type] = request.Csr
	}

	return nil
}

func (sender *testSender) SendInstallCertsConfirmation(confirmations []cloudprotocol.InstallCertData) error {
	sender.currentConfirmations = confirmations

	for i := range sender.currentConfirmations {
		if sender.currentConfirmations[i].Description != "" {
			sender.currentConfirmations[i].Description = "error"
		}
	}

	return nil
}

func (sender *testSender) SendStartProvisioningResponse(
	response cloudprotocol.StartProvisioningResponse,
) (err error) {
	sender.startProvisioningResponse = &response

	return nil
}

func (sender *testSender) SendFinishProvisioningResponse(
	response cloudprotocol.FinishProvisioningResponse,
) (err error) {
	sender.finishProvisioningResponse = &response

	return nil
}

func (sender *testSender) SendDeprovisioningResponse(
	response cloudprotocol.DeprovisioningResponse,
) (err error) {
	sender.deprovisioningResponse = &response

	return nil
}

func (provider *testCertProvider) GetCertSerial(certURLStr string) (serial string, err error) {
	certURL, err := url.Parse(certURLStr)
	if err != nil {
		return "", aoserrors.Wrap(err)
	}

	var certs []*x509.Certificate

	switch certURL.Scheme {
	case cryptutils.SchemeFile:
		if certs, err = cryptutils.LoadCertificateFromFile(certURL.Path); err != nil {
			return "", aoserrors.Wrap(err)
		}

	default:
		return "", aoserrors.Errorf("unsupported schema %s for certificate", certURL.Scheme)
	}

	return fmt.Sprintf("%X", certs[0].SerialNumber), nil
}

func (server *testIAMPermissionsServer) RegisterInstance(
	context context.Context, req *pb.RegisterInstanceRequest,
) (rsp *pb.RegisterInstanceResponse, err error) {
	rsp = &pb.RegisterInstanceResponse{}

	instanceIdent := pbconvert.InstanceIdentFromPB(req.GetInstance())

	if secret := server.findSecret(instanceIdent); secret != "" {
		return rsp, aoserrors.New("service is already registered")
	}

	secret := randomString()
	rsp.Secret = secret

	permissions := make(map[string]map[string]string)
	for key, value := range req.GetPermissions() {
		permissions[key] = value.GetPermissions()
	}

	server.permissionsCache[secret] = servicePermissions{instanceIdent: instanceIdent, permissions: permissions}

	return rsp, nil
}

func (server *testIAMPermissionsServer) UnregisterInstance(
	ctx context.Context, req *pb.UnregisterInstanceRequest,
) (rsp *empty.Empty, err error) {
	rsp = &empty.Empty{}

	secret := server.findSecret(pbconvert.InstanceIdentFromPB(req.GetInstance()))
	if secret == "" {
		return rsp, aoserrors.New("service is not registered ")
	}

	delete(server.permissionsCache, secret)

	return rsp, nil
}

func (server *testIAMPermissionsServer) GetPermissions(
	ctx context.Context, req *pb.PermissionsRequest,
) (rsp *pb.PermissionsResponse, err error) {
	rsp = &pb.PermissionsResponse{}

	funcServersPermissions, ok := server.permissionsCache[req.GetSecret()]
	if !ok {
		return rsp, aoserrors.New("secret not found")
	}

	permissions, ok := funcServersPermissions.permissions[req.GetFunctionalServerId()]
	if !ok {
		return rsp, aoserrors.New("permissions for functional server not found")
	}

	rsp.Permissions = &pb.Permissions{Permissions: permissions}
	rsp.Instance = pbconvert.InstanceIdentToPB(funcServersPermissions.instanceIdent)

	return rsp, nil
}

func (server *testIAMPermissionsServer) findSecret(instance aostypes.InstanceIdent) (secret string) {
	for key, value := range server.permissionsCache {
		if value.instanceIdent == instance {
			return key
		}
	}

	return ""
}

func randomString() string {
	secret := make([]byte, secretLength)

	for i := range secret {
		//nolint:gosec
		secret[i] = secretSymbols[rand.New(rand.NewSource(time.Now().UnixNano())).Intn(len(secretSymbols))]
	}

	return string(secret)
}
