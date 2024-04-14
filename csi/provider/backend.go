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
	"errors"
	"fmt"

	"huawei-csi-driver/client/apis/xuanwu/v1"
	"huawei-csi-driver/csi/app"
	"huawei-csi-driver/lib/drcsi"
	"huawei-csi-driver/pkg/constants"
	pkgUtils "huawei-csi-driver/pkg/utils"
	"huawei-csi-driver/utils/log"
)

// AddStorageBackend used to add storage backend, and return the backend ID
func (p *Provider) AddStorageBackend(ctx context.Context, req *drcsi.AddStorageBackendRequest) (
	*drcsi.AddStorageBackendResponse, error) {

	log.AddContext(ctx).Infof("Start to add storage backend %s.", req.Name)
	defer log.AddContext(ctx).Infof("Finished to add storage backend %s.", req.Name)

	_, backendName, err := pkgUtils.SplitMetaNamespaceKey(req.Name)
	_, err = p.register.FetchAndRegisterOneBackend(ctx, backendName, false)
	if err != nil {
		log.AddContext(ctx).Errorf("fetch and register backend failed, error: %v", err)
		return nil, err
	}

	log.AddContext(ctx).Infof("Add storage backend: [%s] success.", req.Name)
	return &drcsi.AddStorageBackendResponse{BackendId: req.Name}, nil
}

// RemoveStorageBackend remove the backend id in current provider
func (p *Provider) RemoveStorageBackend(ctx context.Context, req *drcsi.RemoveStorageBackendRequest) (
	*drcsi.RemoveStorageBackendResponse, error) {

	log.AddContext(ctx).Infof("Start to remove storage backend %s.", req.BackendId)
	defer log.AddContext(ctx).Infof("Finish to remove storage backend %s.", req.BackendId)

	_, backendName, err := pkgUtils.SplitMetaNamespaceKey(req.BackendId)
	if err != nil {
		msg := fmt.Sprintf("SplitMetaNamespaceKey [%s] failed, error: [%v]", req.BackendId, err)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	p.register.RemoveRegisteredOneBackend(ctx, backendName)

	return &drcsi.RemoveStorageBackendResponse{}, nil
}

// UpdateStorageBackend update the backend within backend id
func (p *Provider) UpdateStorageBackend(ctx context.Context, req *drcsi.UpdateStorageBackendRequest) (
	*drcsi.UpdateStorageBackendResponse, error) {

	// In the current version, the CSI supports only password change, which is verified through webhook.
	// No other operation is required for the CSI driver.
	log.AddContext(ctx).Infof("Start to update storage backend %s.", req.BackendId)
	defer log.AddContext(ctx).Infof("Finish to update storage backend %s.", req.BackendId)

	_, backendName, err := pkgUtils.SplitMetaNamespaceKey(req.BackendId)
	if err != nil {
		msg := fmt.Sprintf("SplitMetaNamespaceKey [%s] failed, error: [%v]", req.BackendId, err)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	_, err = p.register.FetchAndRegisterOneBackend(ctx, backendName, false)
	if err != nil {
		log.AddContext(ctx).Errorf("fetch and register backend failed, error: %v", err)
		return nil, err
	}

	err = pkgUtils.SetStorageBackendContentOnlineStatus(ctx, req.BackendId, true)
	if err != nil {
		msg := fmt.Sprintf("SetStorageBackendContentOnlineStatus [%s] to online=true failed. error: %v",
			req.BackendId, err)
		return &drcsi.UpdateStorageBackendResponse{}, pkgUtils.Errorln(ctx, msg)
	}

	return &drcsi.UpdateStorageBackendResponse{}, nil
}

// GetBackendStats used to update the storage backend status
func (p *Provider) GetBackendStats(ctx context.Context, req *drcsi.GetBackendStatsRequest) (
	*drcsi.GetBackendStatsResponse, error) {

	log.AddContext(ctx).Debugf("Start to get storage backend %s status.", req.BackendId)
	defer log.AddContext(ctx).Debugf("Finish to get storage backend %s status.", req.BackendId)

	// If the sbct is offline, the status information is not obtained.
	if !pkgUtils.IsSBCTOnline(ctx, req.BackendId) {
		msg := fmt.Sprintf("GetBackendStats backend: [%s] is offline, skip get stats", req.BackendId)
		log.AddContext(ctx).Warningln(msg)
		return nil, errors.New(msg)
	}

	_, backendName, err := pkgUtils.SplitMetaNamespaceKey(req.BackendId)
	if err != nil {
		msg := fmt.Sprintf("SplitMetaNamespaceKey [%s] failed, error: [%v]", req.BackendId, err)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	details, err := p.storageService.GetBackendDetails(ctx, backendName)
	if err != nil {
		log.AddContext(ctx).Errorf("get backend details failed, error: %v", err)
		return nil, err
	}

	response := &drcsi.GetBackendStatsResponse{
		VendorName:      constants.ProviderVendorName,
		ProviderName:    app.GetGlobalConfig().DriverName,
		ProviderVersion: constants.ProviderVersion,
		Capabilities:    details.Capabilities,
		Specifications:  details.Specifications,
		Pools:           details.Pools,
		Online:          true,
	}

	p.registerOrUpdateOneBackend(ctx, backendName, req.BackendId, response)
	return response, nil
}

func (p *Provider) registerOrUpdateOneBackend(ctx context.Context, name, backendId string,
	response *drcsi.GetBackendStatsResponse) {
	var err error
	var sbct *v1.StorageBackendContent
	_, exists := p.cache.Load(name)
	if !exists {
		sbct, err = p.fetcher.FetchBackendByName(ctx, name, false)
		if err != nil {
			log.AddContext(ctx).Errorf("fetch backend %s failed, error: %v", name, err)
			return
		}
	} else {
		sbct = &v1.StorageBackendContent{
			Spec:   v1.StorageBackendContentSpec{BackendClaim: backendId},
			Status: &v1.StorageBackendContentStatus{},
		}
	}
	if sbct == nil || sbct.Status == nil {
		log.AddContext(ctx).Errorf("backend %s status is nil", name)
		return
	}
	sbct.Status.Capabilities = response.Capabilities
	sbct.Status.Pools = convertPool(response.Pools)
	sbct.Status.Online = true

	err = p.register.UpdateOrRegisterOneBackend(ctx, sbct)
	if err != nil {
		log.AddContext(ctx).Errorf("update backend cache failed, error: %v", err)
	}
}

func convertPool(source []*drcsi.Pool) []v1.Pool {
	pools := make([]v1.Pool, 0)
	for _, pool := range source {
		pools = append(pools, v1.Pool{
			Name:       pool.GetName(),
			Capacities: pool.GetCapacities(),
		})
	}
	return pools
}
