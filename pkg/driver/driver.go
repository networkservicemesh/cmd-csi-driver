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

// Package driver contains a CSI driver implementation
package driver

import (
	"context"
	"os"

	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/spiffe/spiffe-csi/pkg/mount"

	"github.com/networkservicemesh/cmd-csi-driver/pkg/logkeys"
	"github.com/networkservicemesh/sdk/pkg/tools/log"
)

type mountFunction func(string, string) error
type unmountFunction func(string) error
type isMountPointFunction func(string) (bool, error)

// Config is the configuration for the driver
type Config struct {
	Log          log.Logger
	NodeID       string
	PluginName   string
	Version      string
	NSMSocketDir string

	customMount   mountFunction
	customUnmount unmountFunction
}

// Driver is the ephemeral-inline CSI driver implementation
type Driver struct {
	csi.UnimplementedIdentityServer
	csi.UnimplementedNodeServer

	logger       log.Logger
	nodeID       string
	pluginName   string
	version      string
	nsmSocketDir string

	mount        mountFunction
	unmount      unmountFunction
	isMountPoint isMountPointFunction
}

// New creates a new driver with the given config
func New(config *Config) (*Driver, error) {
	switch {
	case config.NodeID == "":
		return nil, errors.New("node ID is required")
	case config.NSMSocketDir == "":
		return nil, errors.New("network service API socket directory is required")
	}
	d := &Driver{
		logger:       config.Log,
		nodeID:       config.NodeID,
		pluginName:   config.PluginName,
		version:      config.Version,
		nsmSocketDir: config.NSMSocketDir,
		mount:        mount.BindMountRW,
		unmount:      mount.Unmount,
		isMountPoint: mount.IsMountPoint,
	}
	if config.customMount != nil {
		d.mount = config.customMount
	}
	if config.customUnmount != nil {
		d.unmount = config.customUnmount
	}

	return d, nil
}

/////////////////////////////////////////////////////////////////////////////
// Identity Server
/////////////////////////////////////////////////////////////////////////////

// GetPluginInfo returns plugin info
func (d *Driver) GetPluginInfo(_ context.Context, _ *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	return &csi.GetPluginInfoResponse{
		Name:          d.pluginName,
		VendorVersion: d.version,
	}, nil
}

// GetPluginCapabilities returns plugin capabilities
func (d *Driver) GetPluginCapabilities(_ context.Context, _ *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
	// Only the Node server is implemented. No other capabilities are available.
	return &csi.GetPluginCapabilitiesResponse{}, nil
}

// Probe verifies that the plugin is in a healthy state
func (d *Driver) Probe(_ context.Context, _ *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	return &csi.ProbeResponse{}, nil
}

/////////////////////////////////////////////////////////////////////////////
// Node Server implementation
/////////////////////////////////////////////////////////////////////////////

// NodePublishVolume is called when a workload that wants to use the specified volume is placed (scheduled) on a node
func (d *Driver) NodePublishVolume(_ context.Context, req *csi.NodePublishVolumeRequest) (_ *csi.NodePublishVolumeResponse, err error) {
	ephemeralMode := req.GetVolumeContext()["csi.storage.k8s.io/ephemeral"]

	logger := d.logger.
		WithField(logkeys.VolumeID, req.VolumeId).
		WithField(logkeys.TargetPath, req.TargetPath)

	if req.VolumeCapability != nil && req.VolumeCapability.AccessMode != nil {
		logger = logger.WithField("access_mode", req.VolumeCapability.AccessMode.Mode)
	}

	defer func() {
		if err != nil {
			logger.Error(err, "Failed to publish volume")
		}
	}()

	// Validate request
	switch {
	case req.VolumeId == "":
		return nil, status.Error(codes.InvalidArgument, "request missing required volume id")
	case req.TargetPath == "":
		return nil, status.Error(codes.InvalidArgument, "request missing required target path")
	case req.VolumeCapability == nil:
		return nil, status.Error(codes.InvalidArgument, "request missing required volume capability")
	case req.VolumeCapability.AccessType == nil:
		return nil, status.Error(codes.InvalidArgument, "request missing required volume capability access type")
	case !isVolumeCapabilityPlainMount(req.VolumeCapability):
		return nil, status.Error(codes.InvalidArgument, "request volume capability access type must be a simple mount")
	case req.VolumeCapability.AccessMode == nil:
		return nil, status.Error(codes.InvalidArgument, "request missing required volume capability access mode")
	case isVolumeCapabilityAccessModeReadOnly(req.VolumeCapability.AccessMode):
		return nil, status.Error(codes.InvalidArgument, "request volume capability access mode is not valid")
	case !req.Readonly:
		return nil, status.Error(codes.InvalidArgument, "pod.spec.volumes[].csi.readOnly must be set to 'true'")
	case ephemeralMode != "true":
		return nil, status.Error(codes.InvalidArgument, "only ephemeral volumes are supported")
	}

	// Create the target path (required by CSI interface)
	if err := os.Mkdir(req.TargetPath, 0o750); err != nil && !os.IsExist(err) {
		return nil, status.Errorf(codes.Internal, "unable to create target path %q: %v", req.TargetPath, err)
	}

	// Ideally the volume is writable by the host to enable, for example,
	// manipulation of file attributes by SELinux. However, the volume MUST NOT
	// be writable by workload containers. We enforce that the CSI volume is
	// marked read-only above, instructing the kubelet to mount it read-only
	// into containers, while we mount the volume read-write to the host.
	if err := d.mount(d.nsmSocketDir, req.TargetPath); err != nil {
		return nil, status.Errorf(codes.Internal, "unable to mount %q: %v", req.TargetPath, err)
	}

	logger.Info("Volume published")

	return &csi.NodePublishVolumeResponse{}, nil
}

// NodeUnpublishVolume is a reverse operation of NodePublishVolume
func (d *Driver) NodeUnpublishVolume(_ context.Context, req *csi.NodeUnpublishVolumeRequest) (_ *csi.NodeUnpublishVolumeResponse, err error) {
	logger := d.logger.
		WithField(logkeys.VolumeID, req.VolumeId).
		WithField(logkeys.TargetPath, req.TargetPath)

	defer func() {
		if err != nil {
			logger.Error(err, "Failed to unpublish volume")
		}
	}()

	// Validate request
	switch {
	case req.VolumeId == "":
		return nil, status.Error(codes.InvalidArgument, "request missing required volume id")
	case req.TargetPath == "":
		return nil, status.Error(codes.InvalidArgument, "request missing required target path")
	}

	if err := d.unmount(req.TargetPath); err != nil {
		return nil, status.Errorf(codes.Internal, "unable to unmount %q: %v", req.TargetPath, err)
	}
	if err := os.Remove(req.TargetPath); err != nil {
		return nil, status.Errorf(codes.Internal, "unable to remove target path %q: %v", req.TargetPath, err)
	}

	logger.Info("Volume unpublished")

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

// NodeGetCapabilities allows to check the supported capabilities of node service provided by the Plugin
func (d *Driver) NodeGetCapabilities(_ context.Context, _ *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: []*csi.NodeServiceCapability{
			{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: csi.NodeServiceCapability_RPC_VOLUME_CONDITION,
					},
				},
			},
			{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: csi.NodeServiceCapability_RPC_GET_VOLUME_STATS,
					},
				},
			},
		},
	}, nil
}

// NodeGetInfo returns the node identifier
func (d *Driver) NodeGetInfo(_ context.Context, _ *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	return &csi.NodeGetInfoResponse{
		NodeId:            d.nodeID,
		MaxVolumesPerNode: 0,
	}, nil
}

// NodeGetVolumeStats returns the volume capacity statistics available for the volume.
func (d *Driver) NodeGetVolumeStats(_ context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	logger := d.logger.
		WithField(logkeys.VolumeID, req.VolumeId).
		WithField(logkeys.VolumePath, req.VolumePath)

	volumeConditionAbnormal := false
	volumeConditionMessage := "mounted"
	if err := d.checkNsAPIMount(req.VolumePath); err != nil {
		volumeConditionAbnormal = true
		volumeConditionMessage = err.Error()
		logger.Error(err, "Volume is unhealthy")
	} else {
		logger.Info("Volume is healthy")
	}

	return &csi.NodeGetVolumeStatsResponse{
		VolumeCondition: &csi.VolumeCondition{
			Abnormal: volumeConditionAbnormal,
			Message:  volumeConditionMessage,
		},
	}, nil
}

func (d *Driver) checkNsAPIMount(volumePath string) error {
	// Check whether it is a mount point.
	if ok, err := d.isMountPoint(volumePath); err != nil {
		return errors.Errorf("failed to determine root for volume path mount: %v", err)
	} else if !ok {
		return errors.New("volume path is not mounted")
	}
	// If a mount point, try to list files... this should fail if the mount is
	// broken for whatever reason.
	if _, err := os.ReadDir(volumePath); err != nil {
		return errors.Errorf("unable to list contents of volume path: %v", err)
	}
	return nil
}

func isVolumeCapabilityPlainMount(volumeCapability *csi.VolumeCapability) bool {
	m := volumeCapability.GetMount()
	switch {
	case m == nil:
		return false
	case m.FsType != "":
		return false
	case len(m.MountFlags) != 0:
		return false
	}
	return true
}

func isVolumeCapabilityAccessModeReadOnly(accessMode *csi.VolumeCapability_AccessMode) bool {
	return accessMode.Mode == csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY
}
