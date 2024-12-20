/*
 Copyright (c) Huawei Technologies Co., Ltd. 2022-2023. All rights reserved.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at
      http://www.apache.org/licenses/LICENSE-2.0
 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

// Package controller used deal with the backend backend content resources
package controller

import (
	"context"
	"time"

	xuanwuv1 "github.com/Huawei/eSDK_K8S_Plugin/v4/client/apis/xuanwu/v1"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/lib/drcsi"
	storageBackend "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/storage-backend/handle"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

// Handler includes the interface of storage backend side
type Handler interface {
	// CreateStorageBackend add storageBackend to provider
	CreateStorageBackend(ctx context.Context, content *xuanwuv1.StorageBackendContent) (string, string, error)
	// DeleteStorageBackend remove the storageBackend from provider
	DeleteStorageBackend(ctx context.Context, backendName string) error
	// UpdateStorageBackend update the storageBackend
	UpdateStorageBackend(ctx context.Context, content *xuanwuv1.StorageBackendContent) error
	// GetStorageBackendStats get all backend info from the provider
	GetStorageBackendStats(ctx context.Context, contentName, backendName string) (*drcsi.GetBackendStatsResponse, error)
}

type drCSIHandler struct {
	backend storageBackend.BackendInterfaces
	timeout time.Duration
}

// NewCDRHandler returns a new Handler
func NewCDRHandler(backend storageBackend.BackendInterfaces,
	timeout time.Duration) Handler {
	return &drCSIHandler{
		backend: backend,
		timeout: timeout,
	}
}

// CreateStorageBackend add storageBackend to provider
func (cdr *drCSIHandler) CreateStorageBackend(ctx context.Context, content *xuanwuv1.StorageBackendContent) (
	string, string, error) {
	return cdr.backend.AddStorageBackend(ctx, content.Spec.BackendClaim,
		content.Spec.ConfigmapMeta, content.Spec.SecretMeta, content.Spec.Parameters)
}

// DeleteStorageBackend remove the storageBackend from provider
func (cdr *drCSIHandler) DeleteStorageBackend(ctx context.Context, backendName string) error {
	return cdr.backend.RemoveStorageBackend(ctx, backendName)
}

// UpdateStorageBackend update the storageBackend
func (cdr *drCSIHandler) UpdateStorageBackend(ctx context.Context, content *xuanwuv1.StorageBackendContent) error {
	return cdr.backend.UpdateStorageBackend(ctx, content)
}

// GetStorageBackendStats get all backend info from the provider
func (cdr *drCSIHandler) GetStorageBackendStats(ctx context.Context, contentName, backendName string) (
	*drcsi.GetBackendStatsResponse, error) {
	status, err := cdr.backend.GetStorageBackendStats(ctx, contentName, backendName)
	if err != nil {
		return nil, err
	}
	log.AddContext(ctx).Debugf("GetStorageBackendStats: get backend [%s] status [%v] within backend handler",
		backendName, status)
	return status, nil
}
