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

package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/kelseyhightower/envconfig"

	"github.com/networkservicemesh/cmd-csi-driver/internal/config"
	"github.com/networkservicemesh/cmd-csi-driver/pkg/driver"
	"github.com/networkservicemesh/cmd-csi-driver/pkg/logkeys"
	"github.com/networkservicemesh/cmd-csi-driver/pkg/server"
	"github.com/networkservicemesh/sdk/pkg/tools/log"
)

var (
	pluginNameFlag    = flag.String("plugin-name", "csi.networkservicemesh.io", "Plugin name to register")
	nsmSocketDirFlag  = flag.String("nsm-socket-dir", "", "Path to the NSM API socket directory")
	csiSocketPathFlag = flag.String("csi-socket-path", "/nsm-csi/csi.sock", "Path to the CSI socket")
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	log.EnableTracing(true)
	logger := log.FromContext(ctx)

	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Provides the NSM API socket directory via ephemeral inline CSI volumes")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintf(os.Stderr, "Usage:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	c := &config.Config{}
	if err := envconfig.Usage("nsm", c); err != nil {
		logger.Fatal(err)
	}
	if err := envconfig.Process("nsm", c); err != nil {
		logger.Fatalf("error processing rootConf from env: %+v", err)
	}

	logger.WithField(logkeys.Version, c.Version).
		WithField(logkeys.NodeID, c.NodeName).
		WithField(logkeys.NSMSocketDir, *nsmSocketDirFlag).
		WithField(logkeys.CSISocketPath, *csiSocketPathFlag).Info("Starting")

	d, err := driver.New(&driver.Config{
		Log:          logger,
		NodeID:       c.NodeName,
		PluginName:   *pluginNameFlag,
		Version:      c.Version,
		NSMSocketDir: *nsmSocketDirFlag,
	})
	if err != nil {
		logger.Fatalf("Failed to create driver: %v", err)
	}

	serverConfig := server.Config{
		Log:           logger,
		CSISocketPath: *csiSocketPathFlag,
		Driver:        d,
	}

	if err := server.Run(serverConfig); err != nil {
		logger.Fatalf("Failed to serve:  %v", err)
	}
	logger.Info("Done")
}
