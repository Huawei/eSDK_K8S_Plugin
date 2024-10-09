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

// Package model for storage pool model
package model

import (
	"context"

	xuanwuV1 "huawei-csi-driver/client/apis/xuanwu/v1"
	"huawei-csi-driver/csi/backend/plugin"
	"huawei-csi-driver/utils/log"
)

// StoragePool field and method of storage pool
type StoragePool struct {
	Name         string
	Storage      string
	Parent       string
	Capabilities map[string]bool
	Capacities   map[string]string
	Plugin       plugin.StoragePlugin
}

func (p *StoragePool) setCapacity(k string, v string) {
	if p.Capacities == nil {
		p.Capacities = make(map[string]string)
	}
	p.Capacities[k] = v
}

// GetCapacities used to get capacities
func (p *StoragePool) GetCapacities() map[string]string {
	return p.Capacities
}

func (p *StoragePool) setCapability(k string, v bool) {
	if p.Capabilities == nil {
		p.Capabilities = make(map[string]bool)
	}
	p.Capabilities[k] = v
}

// GetCapabilities used to get capabilities
func (p *StoragePool) GetCapabilities() map[string]bool {
	return p.Capabilities
}

// GetName used to get pool name
func (p *StoragePool) GetName() string {
	return p.Name
}

// GetStorage used to get storage
func (p *StoragePool) GetStorage() string {
	return p.Storage
}

// GetParent used to get parent
func (p *StoragePool) GetParent() string {
	return p.Parent
}

// UpdatePoolBySBCT update capabilities and capacities by sbct
// step 1: update pool Capabilities
// step 2: update pool Capacities
func (p *StoragePool) UpdatePoolBySBCT(ctx context.Context, content *xuanwuV1.StorageBackendContent) {
	if content.Status == nil || len(content.Status.Pools) == 0 {
		log.AddContext(ctx).Infof("the status or pools field of the specified backend %s is empty, "+
			"so updating it is skipped", p.Parent)
		return
	}
	p.UpdateCapabilities(ctx, content.Status.Capabilities)

	for _, pool := range content.Status.Pools {
		if p.Name == pool.Name {
			p.UpdateCapacities(ctx, pool.Capacities)
		}
	}
}

// UpdateCapacities update pool capacities
func (p *StoragePool) UpdateCapacities(ctx context.Context, capacities map[string]string) {
	// The storage p capability does not need to be updated in the DTree scenario.
	if p.Storage == plugin.DTreeStorage {
		return
	}

	for key, val := range capacities {
		curVal, exist := p.Capacities[key]
		if !exist {
			p.setCapacity(key, val)
			log.AddContext(ctx).Debugf("backend %s add new capacity to %s, %s is set to %val",
				p.Parent, p.Name, key, val)
			continue
		}

		if curVal != val {
			p.setCapacity(key, val)
			log.AddContext(ctx).Debugf("backend %s update capacity to %s, %s from %v to %v",
				p.Parent, p.Name, key, curVal, val)
		}
	}
}

// UpdateCapabilities update pool capabilities
func (p *StoragePool) UpdateCapabilities(ctx context.Context, capabilities map[string]bool) {
	for key, val := range capabilities {
		curVal, exist := p.Capabilities[key]
		if !exist {
			p.setCapability(key, val)
			log.AddContext(ctx).Infof("backend %s add new capability to %s, %s is set to %v",
				p.Parent, p.Name, key, val)
			continue
		}

		if curVal != val {
			p.setCapability(key, val)
			log.AddContext(ctx).Infof("backend %s update capability to %s, %s from %v to %v",
				p.Parent, p.Name, key, curVal, val)
		}
	}
}
