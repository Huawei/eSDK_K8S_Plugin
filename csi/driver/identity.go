/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2024. All rights reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *       http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

package driver

import (
	"context"

	"github.com/container-storage-interface/spec/lib/go/csi"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

// GetPluginInfo used to get plugin info
func (d *CsiDriver) GetPluginInfo(ctx context.Context,
	req *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	log.AddContext(ctx).Infof("Get plugin info %v", *d)
	return &csi.GetPluginInfoResponse{
		Name:          d.name,
		VendorVersion: d.version,
	}, nil
}

// GetPluginCapabilities used to get plugin capabilities
func (d *CsiDriver) GetPluginCapabilities(ctx context.Context,
	req *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
	log.AddContext(ctx).Infof("Get plugin capabilities of %v", *d)

	return &csi.GetPluginCapabilitiesResponse{
		Capabilities: []*csi.PluginCapability{
			{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{
						Type: csi.PluginCapability_Service_CONTROLLER_SERVICE,
					},
				},
			},
			{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{
						Type: csi.PluginCapability_Service_VOLUME_ACCESSIBILITY_CONSTRAINTS,
					},
				},
			},
		},
	}, nil
}

// Probe used to health check
func (d *CsiDriver) Probe(ctx context.Context, req *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	log.AddContext(ctx).Debugf("Probe plugin %v", *d)
	return &csi.ProbeResponse{}, nil
}
