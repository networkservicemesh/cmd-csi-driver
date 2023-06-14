// Copyright (c) 2023 Cisco and/or its affiliates.
//
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package server is used to run grpc server
package server

import (
	"context"
	"net"
	"os"

	"github.com/pkg/errors"
	"google.golang.org/grpc"

	"github.com/container-storage-interface/spec/lib/go/csi"

	"github.com/networkservicemesh/cmd-csi-driver/pkg/logkeys"

	"github.com/networkservicemesh/sdk/pkg/tools/log"
)

// Config is used to run grpc server
type Config struct {
	Log           log.Logger
	CSISocketPath string
	Driver        Driver
}

// Driver is a CSI driver interface
type Driver interface {
	csi.IdentityServer
	csi.NodeServer
}

// Run starts grpc server
func Run(config Config) error {
	if config.CSISocketPath == "" {
		return errors.New("CSI socket path is required")
	}

	if err := os.Remove(config.CSISocketPath); err != nil && !os.IsNotExist(err) {
		config.Log.Error(err, "Unable to remove CSI socket")
	}

	listener, err := net.Listen("unix", config.CSISocketPath)
	if err != nil {
		return errors.Errorf("unable to create CSI socket listener: %v", err)
	}

	rpcLogger := rpcLogger{Log: config.Log}

	server := grpc.NewServer(
		grpc.UnaryInterceptor(rpcLogger.UnaryRPCLogger),
		grpc.StreamInterceptor(rpcLogger.StreamRPCLogger),
	)
	csi.RegisterIdentityServer(server, config.Driver)
	csi.RegisterNodeServer(server, config.Driver)

	config.Log.Info("Listening...")
	return server.Serve(listener)
}

type rpcLogger struct {
	Log log.Logger
}

// UnaryRPCLogger is used for logging
func (l rpcLogger) UnaryRPCLogger(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	logger := l.Log.WithField(logkeys.FullMethod, info.FullMethod)
	resp, err := handler(ctx, req)
	if err != nil {
		logger.Error(err, "RPC failed")
	} else {
		logger.Info("RPC succeeded")
	}
	return resp, err
}

// StreamRPCLogger is used for logging
func (l rpcLogger) StreamRPCLogger(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	logger := l.Log.WithField(logkeys.FullMethod, info.FullMethod)
	err := handler(srv, ss)
	if err != nil {
		logger.Error(err, "RPC failed")
	} else {
		logger.Info("RPC succeeded")
	}
	return err
}
