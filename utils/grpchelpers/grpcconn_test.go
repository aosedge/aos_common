// SPDX-License-Identifier: Apache-2.0
//
// Copyright (C) 2024 Renesas Electronics Corporation.
// Copyright (C) 2024 EPAM Systems, Inc.
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

package grpchelpers_test

import (
	"context"
	"testing"
	"time"

	"github.com/aosedge/aos_common/aoserrors"
	pb "github.com/aosedge/aos_common/api/iamanager"
	"github.com/aosedge/aos_common/utils/grpchelpers"
	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

/***********************************************************************************************************************
 * Tests
 **********************************************************************************************************************/

func TestStoppedConnectionFails(t *testing.T) {
	// Start grpc server
	testServer := startTestServer(testURL)

	// Create connection & check RPC call succeeds
	grpcConn, err := NewGRPCConn(testURL)
	if err != nil {
		t.Fatalf("Connection failed: err=%v", err)
	}

	conn := grpchelpers.NewGRPCConn()

	conn.Set(grpcConn)
	conn.Start()

	if err := getNodeInfo(conn); err != nil {
		t.Fatalf("Server started, GetNodeInfo call should succeed: err=%v", err)
	}

	// Stop the server & check RPC call fails because of timeout
	testServer.StopServer()
	conn.Stop()

	if err := getNodeInfo(conn); err == nil {
		t.Fatal("Server stopped, GetNodeInfo should fail")
	}
}

func TestRPCBlockedUntilConnectionReset(t *testing.T) {
	// Start grpc server
	testServer := startTestServer(testURL)
	defer testServer.StopServer()

	// Create connection & check RPC call succeeds
	grpcConn, err := NewGRPCConn(testURL)
	if err != nil {
		t.Fatalf("Connection failed: err=%v", err)
	}

	conn := grpchelpers.NewGRPCConn()

	conn.Set(grpcConn)
	conn.Start()

	if err := getNodeInfo(conn); err != nil {
		t.Fatalf("GetNodeInfo call should succeed: err=%v", err)
	}

	// Stop the server & start rpc call
	testServer.StopServer()
	conn.Stop()

	rpcResultChan := make(chan error)

	go func() {
		rpcResultChan <- getNodeInfo(conn)
	}()

	select {
	case err := <-rpcResultChan:
		t.Fatalf("Result received before server started: err=%v", err)

	case <-time.After(testTimeout / 2): // Give some time for goroutine to start.
	}

	// Start the server, restore connection & wait until previous rpc succeed
	if err = testServer.RestartServer([]grpc.ServerOption{grpc.ConnectionTimeout(testTimeout)}); err != nil {
		t.Fatalf("Connection failed: err=%v", err)
	}

	grpcConn, err = NewGRPCConn(testURL)
	if err != nil {
		t.Fatalf("Connection failed: err=%v", err)
	}

	conn.Set(grpcConn)
	conn.Start()

	if err := <-rpcResultChan; err != nil {
		t.Fatalf("GetNodeInfo call should succeed: err=%v", err)
	}
}

/***********************************************************************************************************************
 * Utils
 **********************************************************************************************************************/

func NewGRPCConn(address string) (*grpc.ClientConn, error) {
	cred := grpc.WithTransportCredentials(insecure.NewCredentials())

	grpcConn, err := grpc.NewClient(address, cred)
	if err != nil {
		return nil, aoserrors.Wrap(err)
	}

	return grpcConn, nil
}

func getNodeInfo(connection *grpchelpers.GRPCConn) error {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	service := pb.NewIAMPublicServiceClient(connection)
	_, err := service.GetNodeInfo(ctx, &empty.Empty{})

	return aoserrors.Wrap(err)
}

func startTestServer(address string) *grpchelpers.GRPCServer {
	testServer := grpchelpers.NewGRPCServer(address)

	service := testIAMServer{}
	testServer.RegisterService(&pb.IAMPublicService_ServiceDesc, service)

	_ = testServer.RestartServer([]grpc.ServerOption{grpc.ConnectionTimeout(time.Second)})

	return testServer
}
