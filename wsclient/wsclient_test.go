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

package wsclient_test

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"

	"github.com/aoscloud/aos_common/aoserrors"
	"github.com/aoscloud/aos_common/wsclient"
	"github.com/aoscloud/aos_common/wsserver"
)

/***********************************************************************************************************************
 * Consts
 **********************************************************************************************************************/

const (
	hostURL   = ":8088"
	serverURL = "wss://localhost:8088"
	crtFile   = "../wsserver/data/crt.pem"
	keyFile   = "../wsserver/data/key.pem"
	caCert    = "../wsserver/data/rootCA.pem"
)

/***********************************************************************************************************************
 * Types
 **********************************************************************************************************************/

type processMessage func(client *wsserver.Client, messageType int, data []byte) (response []byte, err error)

type testHandler struct {
	processMessage
}

/***********************************************************************************************************************
 * Vars
 **********************************************************************************************************************/

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

func TestSendRequest(t *testing.T) {
	type Header struct {
		Type      string
		RequestID string
	}

	type Request struct {
		Header Header
		Value  int
	}

	type Response struct {
		Header Header
		Value  float32
		Error  *string `json:"error,omitempty"`
	}

	server, err := wsserver.New("TestServer", hostURL, crtFile, keyFile, newTestHandler(
		func(client *wsserver.Client, messageType int, data []byte) (response []byte, err error) {
			var req Request
			var rsp Response

			if err = json.Unmarshal(data, &req); err != nil {
				return nil, aoserrors.Wrap(err)
			}

			rsp.Header.Type = req.Header.Type
			rsp.Header.RequestID = req.Header.RequestID
			rsp.Value = float32(req.Value) / 10.0

			if response, err = json.Marshal(rsp); err != nil {
				return
			}

			return response, nil
		}))
	if err != nil {
		t.Fatalf("Can't create ws server: %s", err)
	}
	defer server.Close()

	time.Sleep(1 * time.Second)

	client, err := wsclient.New("Test", wsclient.ClientParam{CaCertFile: caCert}, nil)
	if err != nil {
		t.Fatalf("Can't create ws client: %s", err)
	}
	defer client.Close()

	if err = client.Connect(serverURL); err != nil {
		t.Fatalf("Can't connect to ws server: %s", err)
	}

	req := Request{Header: Header{Type: "GET", RequestID: uuid.New().String()}}
	rsp := Response{}

	if err = client.SendRequest("Header.RequestID", req.Header.RequestID, &req, &rsp); err != nil {
		t.Errorf("Can't send request: %s", err)
	}

	if rsp.Header.Type != req.Header.Type {
		t.Errorf("Wrong response type: %s", rsp.Header.Type)
	}
}

func TestMultipleResponses(t *testing.T) {
	type Header struct {
		Type      string
		RequestID string
	}

	type Request struct {
		Header Header
		Value  int
	}

	type Response struct {
		Header Header
		Value  float32
		Error  *string `json:"error,omitempty"`
	}

	server, err := wsserver.New("TestServer", hostURL, crtFile, keyFile, newTestHandler(
		func(client *wsserver.Client, messageType int, data []byte) (response []byte, err error) {
			var req Request
			var rsp Response

			if err = json.Unmarshal(data, &req); err != nil {
				return nil, aoserrors.Wrap(err)
			}

			rsp.Header.Type = req.Header.Type
			rsp.Header.RequestID = req.Header.RequestID
			rsp.Value = float32(req.Value) / 10.0

			if response, err = json.Marshal(rsp); err != nil {
				return nil, aoserrors.Wrap(err)
			}

			if err = client.SendMessage(messageType, response); err != nil {
				return nil, aoserrors.Wrap(err)
			}

			rsp.Header.RequestID = uuid.New().String()

			if response, err = json.Marshal(rsp); err != nil {
				return nil, aoserrors.Wrap(err)
			}

			return response, nil
		}))
	if err != nil {
		t.Fatalf("Can't create ws server: %s", err)
	}
	defer server.Close()

	time.Sleep(1 * time.Second)

	client, err := wsclient.New("Test", wsclient.ClientParam{CaCertFile: caCert}, nil)
	if err != nil {
		t.Fatalf("Can't create ws client: %s", err)
	}
	defer client.Close()

	if err = client.Connect(serverURL); err != nil {
		t.Fatalf("Can't connect to ws server: %s", err)
	}

	req := Request{Header: Header{Type: "GET", RequestID: uuid.New().String()}}
	rsp := Response{}

	if err = client.SendRequest("Header.RequestID", req.Header.RequestID, &req, &rsp); err != nil {
		t.Errorf("Can't send request: %s", err)
	}

	if rsp.Header.Type != req.Header.Type {
		t.Errorf("Wrong response type: %s", rsp.Header.Type)
	}

	if rsp.Header.RequestID != req.Header.RequestID {
		t.Errorf("Wrong request ID: %s", rsp.Header.RequestID)
	}
}

func TestWrongIDRequest(t *testing.T) {
	type Request struct {
		Type      string
		RequestID string
		Value     int
	}

	type Response struct {
		Type      string
		RequestID string
		Value     float32
		Error     *string `json:"error,omitempty"`
	}

	server, err := wsserver.New("TestServer", hostURL, crtFile, keyFile, newTestHandler(
		func(client *wsserver.Client, messageType int, data []byte) (response []byte, err error) {
			var req Request
			var rsp Response

			if err = json.Unmarshal(data, &req); err != nil {
				return nil, aoserrors.Wrap(err)
			}

			rsp.Type = req.Type
			rsp.RequestID = uuid.New().String()
			rsp.Value = float32(req.Value) / 10.0

			if response, err = json.Marshal(rsp); err != nil {
				return
			}

			return response, aoserrors.Wrap(err)
		}))
	if err != nil {
		t.Fatalf("Can't create ws server: %s", err)
	}
	defer server.Close()

	time.Sleep(1 * time.Second)

	client, err := wsclient.New("Test", wsclient.ClientParam{
		CaCertFile: caCert, WebSocketTimeout: 1 * time.Second,
	}, nil)
	if err != nil {
		t.Fatalf("Can't create ws client: %s", err)
	}
	defer client.Close()

	if err = client.Connect(serverURL); err != nil {
		t.Fatalf("Can't connect to ws server: %s", err)
	}

	req := Request{Type: "GET", RequestID: uuid.New().String()}
	rsp := Response{}

	if err = client.SendRequest("RequestID", req.RequestID, &req, &rsp); err == nil {
		t.Error("Error expected")
	}
}

func TestErrorChannel(t *testing.T) {
	server, err := wsserver.New("TestServer", hostURL, crtFile, keyFile, nil)
	if err != nil {
		t.Fatalf("Can't create ws server: %s", err)
	}
	defer server.Close()

	time.Sleep(1 * time.Second)

	client, err := wsclient.New("Test", wsclient.ClientParam{CaCertFile: caCert}, nil)
	if err != nil {
		t.Fatalf("Can't create ws client: %s", err)
	}
	defer client.Close()

	if err = client.Connect(serverURL); err != nil {
		t.Fatalf("Can't connect to ws server: %s", err)
	}

	server.Close()

	select {
	case <-client.ErrorChannel:

	case <-time.After(5 * time.Second):
		t.Error("Waiting error channel timeout")
	}
}

func TestMessageHandler(t *testing.T) {
	server, err := wsserver.New("TestServer", hostURL, crtFile, keyFile, nil)
	if err != nil {
		t.Fatalf("Can't create ws server: %s", err)
	}
	defer server.Close()

	type Message struct {
		Type  string
		Value int
	}

	messageChannel := make(chan Message)

	time.Sleep(1 * time.Second)

	client, err := wsclient.New("Test", wsclient.ClientParam{CaCertFile: caCert}, func(data []byte) {
		var message Message

		if err := json.Unmarshal(data, &message); err != nil {
			t.Errorf("Parse message error: %s", err)

			return
		}

		messageChannel <- message
	})
	if err != nil {
		t.Fatalf("Can't create ws client: %s", err)
	}
	defer client.Close()

	if err = client.Connect(serverURL); err != nil {
		t.Fatalf("Can't connect to ws server: %s", err)
	}

	clientHandlers := server.GetClients()
	if len(clientHandlers) == 0 {
		t.Fatalf("No connected clients")
	}

	for _, clientHandler := range clientHandlers {
		if err = clientHandler.SendMessage(websocket.TextMessage,
			[]byte(`{"Type":"NOTIFY", "Value": 123}`)); err != nil {
			t.Fatalf("Can't send message: %s", err)
		}
	}

	select {
	case message := <-messageChannel:
		if message.Type != "NOTIFY" || message.Value != 123 {
			t.Error("Wrong message value")
		}

	case <-time.After(5 * time.Second):
		t.Error("Waiting message timeout")
	}
}

func TestSendMessage(t *testing.T) {
	type Message struct {
		Type  string
		Value int
	}

	server, err := wsserver.New("TestServer", hostURL, crtFile, keyFile, newTestHandler(
		func(client *wsserver.Client, messageType int, data []byte) (response []byte, err error) {
			return data, nil
		}))
	if err != nil {
		t.Fatalf("Can't create ws server: %s", err)
	}
	defer server.Close()

	messageChannel := make(chan Message)

	time.Sleep(1 * time.Second)

	client, err := wsclient.New("Test", wsclient.ClientParam{CaCertFile: caCert}, func(data []byte) {
		var message Message

		if err := json.Unmarshal(data, &message); err != nil {
			t.Errorf("Parse message error: %s", err)

			return
		}

		messageChannel <- message
	})
	if err != nil {
		t.Fatalf("Error create a new ws client: %s", err)
	}
	defer client.Close()

	// Send message to server before connect
	if err = client.SendMessage(&Message{Type: "NOTIFY", Value: 123}); err == nil {
		t.Error("Expect error because client is not connected")
	}

	if err = client.Connect(serverURL); err != nil {
		t.Fatalf("Can't connect to ws server: %s", err)
	}

	if err = client.SendMessage(&Message{Type: "NOTIFY", Value: 123}); err != nil {
		t.Errorf("Error sending message form client: %s", err)
	}

	select {
	case message := <-messageChannel:
		if message.Type != "NOTIFY" || message.Value != 123 {
			t.Error("Wrong message value")
		}

	case <-time.After(5 * time.Second):
		t.Error("Waiting message timeout")
	}
}

func TestConnectDisconnect(t *testing.T) {
	server, err := wsserver.New("TestServer", hostURL, crtFile, keyFile, nil)
	if err != nil {
		t.Fatalf("Can't create ws server: %s", err)
	}
	defer server.Close()

	time.Sleep(1 * time.Second)

	client, err := wsclient.New("Test", wsclient.ClientParam{CaCertFile: caCert}, nil)
	if err != nil {
		t.Fatalf("Can't create ws client: %s", err)
	}
	defer client.Close()

	if err = client.Connect(serverURL); err != nil {
		t.Errorf("Can't connect to ws server: %s", err)
	}

	if err = client.Connect(serverURL); err == nil {
		t.Error("Expect error because client is connected")
	}

	if err = client.Disconnect(); err != nil {
		t.Errorf("Can't disconnect client: %s", err)
	}

	if client.IsConnected() == true {
		t.Error("Client should not be connected")
	}

	if err = client.Connect(serverURL); err != nil {
		t.Errorf("Can't connect to ws server: %s", err)
	}

	if len(server.GetClients()) == 0 {
		t.Error("No connected clients")
	}

	if client.IsConnected() != true {
		t.Error("Client should be connected")
	}
}

func TestWrongCaCert(t *testing.T) {
	server, err := wsserver.New("TestServer", hostURL, crtFile, keyFile, newTestHandler(
		func(client *wsserver.Client, messageType int, data []byte) (response []byte, err error) {
			return data, nil
		}))
	if err != nil {
		t.Fatalf("Can't create ws server: %s", err)
	}
	defer server.Close()

	time.Sleep(1 * time.Second)

	client, err := wsclient.New("Test", wsclient.ClientParam{CaCertFile: ""}, nil)
	if err != nil {
		t.Fatalf("Can't create client: %s", err)
	}

	if err = client.Connect(serverURL); err == nil {
		t.Error("Expecting an error due to unset custom CA cert")
	}

	client.Close()

	if _, err = wsclient.New("Test", wsclient.ClientParam{CaCertFile: keyFile}, nil); err == nil {
		t.Error("Expecting an error due to invalid CA cert file")

		client.Close()
	}

	if client, err = wsclient.New("Test", wsclient.ClientParam{CaCertFile: "123123"}, nil); err == nil {
		t.Error("Expecting an error due to absence of cert file")

		client.Close()
	}
}

func TestWSTimeout(t *testing.T) {
	type Request struct {
		Type      string
		RequestID string
		Value     int
	}

	wsTimeoutData := []struct {
		timeout    int
		minTimeout int
		maxTimeout int
	}{
		{1, 1, 2},
		{3, 3, 4},
		{5, 5, 6},
	}

	server, err := wsserver.New("TestServer", hostURL, crtFile, keyFile, nil)
	if err != nil {
		t.Fatalf("Can't create ws server: %s", err)
	}
	defer server.Close()

	time.Sleep(1 * time.Second)

	for _, value := range wsTimeoutData {
		client, err := wsclient.New("Test", wsclient.ClientParam{
			CaCertFile: caCert, WebSocketTimeout: time.Duration(value.timeout) * time.Second,
		}, nil)
		if err != nil {
			t.Fatalf("Can't create ws client: %s", err)
		}
		defer client.Close()

		if err = client.Connect(serverURL); err != nil {
			t.Fatalf("Can't connect to ws server: %s", err)
		}

		timeReqStart := time.Now()

		req := Request{Type: "GET", RequestID: uuid.New().String()}
		if err = client.SendRequest("RequestID", req.RequestID, &req, nil); err == nil {
			t.Error("Error expected")
		}

		timeReqFinish := time.Now()

		if timeReqFinish.Sub(timeReqStart) > time.Duration(value.maxTimeout)*time.Second ||
			timeReqFinish.Sub(timeReqStart) < time.Duration(value.minTimeout)*time.Second {
			t.Errorf("Timeout differs a lot from the expected one")
		}
	}
}

/*******************************************************************************
 * Private
 ******************************************************************************/

func newTestHandler(p processMessage) (handler *testHandler) {
	return &testHandler{p}
}

func (handler *testHandler) ClientConnected(client *wsserver.Client) {
}

func (handler *testHandler) ProcessMessage(
	client *wsserver.Client, messageType int, message []byte) (response []byte, err error) {
	response, err = handler.processMessage(client, messageType, message)

	return response, aoserrors.Wrap(err)
}

func (handler *testHandler) ClientDisconnected(client *wsserver.Client) {
}
