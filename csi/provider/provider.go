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

import "github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/handler"

// StorageProvider is for storage provider
type StorageProvider struct {
	name            string
	version         string
	storageService  handler.StorageServiceInterface
	register        handler.BackendRegisterInterface
	fetcher         handler.BackendFetchInterface
	cache           handler.BackendCacheWrapperInterface
	backendSelector handler.BackendSelectInterface
}

// NewProvider is used to create storage provider
func NewProvider(name, version string) *StorageProvider {
	return &StorageProvider{
		name:            name,
		version:         version,
		storageService:  handler.NewStorageHandler(),
		register:        handler.NewBackendRegister(),
		fetcher:         handler.NewBackendFetcher(),
		cache:           handler.NewCacheWrapper(),
		backendSelector: handler.NewBackendSelector(),
	}
}
