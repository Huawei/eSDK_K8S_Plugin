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

	"huawei-csi-driver/csi/app"
	"huawei-csi-driver/csi/backend"
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

	useCert, certSecret, err := pkgUtils.GetCertMeta(ctx, req.Name)
	if err != nil {
		msg := fmt.Sprintf("GetCertMeta %s failed. error: %v", req.Name, err)
		return nil, pkgUtils.Errorln(ctx, msg)
	}

	// backendId: <namespace>/<backend-name> eg:huawei-csi/nfs-180
	backendId, err := backend.RegisterOneBackend(ctx, req.Name, req.ConfigmapMeta, req.SecretMeta, certSecret, useCert)
	if err != nil {
		msg := fmt.Sprintf("RegisterBackend %s failed, error %v", req.Name, err)
		return nil, pkgUtils.Errorln(ctx, msg)
	}

	log.AddContext(ctx).Infof("Add storage backend: [%s] success.", backendId)
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

	backend.RemoveOneBackend(ctx, backendName)

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

	bk := backend.GetBackendWithFresh(ctx, backendName, false)
	if bk != nil {
		backend.RemoveOneBackend(ctx, bk.Name)
	}

	err = pkgUtils.SetStorageBackendContentOnlineStatus(ctx, req.BackendId, true)
	if err != nil {
		msg := fmt.Sprintf("SetStorageBackendContentOnlineStatus [%s] to online=true failed. error: %v",
			req.BackendId, err)
		return &drcsi.UpdateStorageBackendResponse{}, pkgUtils.Errorln(ctx, msg)
	}

	useCert, certSecret, err := pkgUtils.GetCertMeta(ctx, req.BackendId)
	if err != nil {
		msg := fmt.Sprintf("GetCertMeta [%s] failed. error: %v", req.BackendId, err)
		return &drcsi.UpdateStorageBackendResponse{}, pkgUtils.Errorln(ctx, msg)
	}

	// backendId: <namespace>/<backend-name> eg:huawei-csi/nfs-180
	_, err = backend.RegisterOneBackend(ctx, req.BackendId, req.ConfigmapMeta, req.SecretMeta, certSecret, useCert)
	if err != nil {
		msg := fmt.Sprintf("RegisterBackend %s failed, error %v", req.Name, err)
		return nil, pkgUtils.Errorln(ctx, msg)
	}

	return &drcsi.UpdateStorageBackendResponse{}, nil
}

// GetBackendStats used to update the storage backend status
func (p *Provider) GetBackendStats(ctx context.Context, req *drcsi.GetBackendStatsRequest) (
	*drcsi.GetBackendStatsResponse, error) {

	log.AddContext(ctx).Infof("Start to get storage backend %s status.", req.BackendId)
	defer log.AddContext(ctx).Infof("Finish to get storage backend %s status.", req.BackendId)

	// If the sbct is offline, the status information is not obtained.
	if !pkgUtils.IsSBCTOnline(ctx, req.BackendId) {
		msg := fmt.Sprintf("GetBackendStats backend: [%s] is offline, skip get stats", req.BackendId)
		return nil, pkgUtils.Errorln(ctx, msg)
	}

	_, backendName, err := pkgUtils.SplitMetaNamespaceKey(req.BackendId)
	if err != nil {
		msg := fmt.Sprintf("SplitMetaNamespaceKey [%s] failed, error: [%v]", req.BackendId, err)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	// If the backend does not exist in the memory, the get operation is not performed to prevent users from being
	// locked in the update scenario.
	if !backend.IsBackendRegistered(backendName) {
		msg := fmt.Sprintf("Backend: [%s] is not registered, skip get stats", backendName)
		log.AddContext(ctx).Infoln(msg)
		return nil, errors.New(msg)
	}

	capabilities, specifications, err := backend.GetBackendCapabilities(ctx, backendName)
	if err != nil {
		msg := fmt.Sprintf("GetBackendCapabilities backend:[%s] failed, error: [%v]", backendName, err)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	return &drcsi.GetBackendStatsResponse{
		VendorName:      constants.ProviderVendorName,
		ProviderName:    app.GetGlobalConfig().DriverName,
		ProviderVersion: constants.ProviderVersion,
		Capabilities:    capabilities,
		Specifications:  specifications,
		Online:          true,
	}, nil
}
