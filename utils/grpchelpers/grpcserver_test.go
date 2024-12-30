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
	"google.golang.org/protobuf/types/known/emptypb"
)

/***********************************************************************************************************************
 * Const
 **********************************************************************************************************************/

const (
	testTimeout = 1 * time.Second
	testURL     = ":7891"
)

/***********************************************************************************************************************
 * Type
 **********************************************************************************************************************/

type testIAMServer struct {
	pb.UnimplementedIAMPublicServiceServer
}

/***********************************************************************************************************************
 * Tests
 **********************************************************************************************************************/

func TestStartServer(t *testing.T) {
	testServer := grpchelpers.NewGRPCServer(testURL)

	clientOpt := grpc.WithTransportCredentials(insecure.NewCredentials())

	// Start the server
	defer testServer.StopServer()

	service := testIAMServer{}
	testServer.RegisterService(&pb.IAMPublicService_ServiceDesc, service)

	if err := callGetNodeInfo(clientOpt); err == nil {
		t.Fatalf("Server not started, GetNodeInfo call should return error")
	}

	// Send initial options
	if err := testServer.RestartServer([]grpc.ServerOption{grpc.ConnectionTimeout(time.Second)}); err != nil {
		t.Fatalf("Server not started: err=%v", err)
	}

	if err := callGetNodeInfo(clientOpt); err != nil {
		t.Fatalf("Server started, GetNodeInfo call should succeed: err=%v", err)
	}
}

func TestUpdateOptions(t *testing.T) {
	// Start server
	testServer := grpchelpers.NewGRPCServer(testURL)

	defer testServer.StopServer()

	service := testIAMServer{}
	testServer.RegisterService(&pb.IAMPublicService_ServiceDesc, service)

	// Initial server options fail every rpc call
	failingServerOption := grpc.UnaryInterceptor(func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp interface{}, err error) {
		return nil, aoserrors.New("forced RPC call to fail")
	})

	initialServerOptions := []grpc.ServerOption{failingServerOption}

	if err := testServer.RestartServer(initialServerOptions); err != nil {
		t.Fatalf("Server not started: err=%v", err)
	}

	// Client connection fails
	if err := callGetNodeInfo(grpc.WithTransportCredentials(insecure.NewCredentials())); err == nil {
		t.Fatalf("Expected failed RPC call")
	}

	// Update server options
	if err := testServer.RestartServer([]grpc.ServerOption{grpc.ConnectionTimeout(time.Second)}); err != nil {
		t.Fatalf("Server not started: err=%v", err)
	}

	// Now client connection should succeed
	if err := callGetNodeInfo(grpc.WithTransportCredentials(insecure.NewCredentials())); err != nil {
		t.Fatalf("Expected successful RPC call: err=%v", err)
	}
}

/***********************************************************************************************************************
 * Utils
 **********************************************************************************************************************/

func callGetNodeInfo(dialOpts ...grpc.DialOption) error {
	connection, err := grpc.NewClient(testURL, dialOpts...)
	if err != nil {
		return aoserrors.Wrap(err)
	}
	defer connection.Close()

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	service := pb.NewIAMPublicServiceClient(connection)
	_, err = service.GetNodeInfo(ctx, &empty.Empty{})

	return aoserrors.Wrap(err)
}

func (testIAMServer) GetNodeInfo(context.Context, *emptypb.Empty) (*pb.NodeInfo, error) {
	return &pb.NodeInfo{}, nil
}
