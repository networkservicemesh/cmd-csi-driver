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

// Package config - contain environment variables used by csi driver
package config

import (
	"github.com/pkg/errors"
)

// Config - configuration for cmd-csi-dirver
type Config struct {
	NodeName      string `default:"" desc:"Envvar from which to obtain the node ID" split_words:"true"`
	PluginName    string `default:"csi.networkservicemesh.io" desc:"Plugin name to register" split_words:"true"`
	SocketDir     string `default:"" desc:"Path to the NSM API socket directory" split_words:"true"`
	CSISocketPath string `default:"/nsm-csi/csi.sock" desc:"Path to the CSI socket" split_words:"true"`
	Version       string `default:"undefined" desc:"Version"`
}

// IsValid - check if configuration is valid
func (c *Config) IsValid() error {
	if c.NodeName == "" {
		return errors.New("node name is required")
	}
	if c.SocketDir == "" {
		return errors.New("socket dir is required")
	}
	return nil
}
