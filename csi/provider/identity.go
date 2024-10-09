/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2023. All rights reserved.
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

// Package provider is related with storage provider
package provider

import (
	"context"

	"huawei-csi-driver/lib/drcsi"
	"huawei-csi-driver/utils/log"
)

// GetProviderInfo is used to get provider info
func (p *StorageProvider) GetProviderInfo(ctx context.Context, req *drcsi.GetProviderInfoRequest) (
	*drcsi.GetProviderInfoResponse, error) {

	log.AddContext(ctx).Infof("Get provider info %v", *p)
	return &drcsi.GetProviderInfoResponse{
		Provider: p.name,
		Version:  p.version,
	}, nil
}

// GetProviderCapabilities is used to get provider capabilities
func (p *StorageProvider) GetProviderCapabilities(ctx context.Context, req *drcsi.GetProviderCapabilitiesRequest) (
	*drcsi.GetProviderCapabilitiesResponse, error) {

	log.AddContext(ctx).Infof("Get plugin capabilities of %v", *p)
	return &drcsi.GetProviderCapabilitiesResponse{
		Capabilities: []*drcsi.ProviderCapability{
			{
				Type: &drcsi.ProviderCapability_Service_{
					Service: &drcsi.ProviderCapability_Service{
						Type: drcsi.ProviderCapability_Service_STORAGE_BACKEND_SERVICE,
					},
				},
			},
		},
	}, nil
}

// Probe is used to probe provider
func (p *StorageProvider) Probe(ctx context.Context, in *drcsi.ProbeRequest) (*drcsi.ProbeResponse, error) {
	log.AddContext(ctx).Infof("Probe invoked of %v, request: %v", *p, in)
	return &drcsi.ProbeResponse{}, nil
}
