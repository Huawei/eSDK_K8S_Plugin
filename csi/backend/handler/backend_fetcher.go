/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2023-2023. All rights reserved.
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

package handler

import (
	"context"
	"errors"
	"fmt"

	"huawei-csi-driver/client/apis/xuanwu/v1"
	"huawei-csi-driver/csi/app"
	pkgUtils "huawei-csi-driver/pkg/utils"
	"huawei-csi-driver/utils/log"
)

// BackendFetchInterface fetch backend operation set
type BackendFetchInterface interface {
	FetchAllBackends(ctx context.Context) ([]v1.StorageBackendContent, error)
	FetchBackendByName(ctx context.Context, name string, online bool) (*v1.StorageBackendContent, error)
}

// BackendFetcher fetch resources of StorageBackendClaim and StorageBackendContent
type BackendFetcher struct{}

// NewBackendFetcher init instance of BackendFetcher
func NewBackendFetcher() *BackendFetcher {
	return &BackendFetcher{}
}

// FetchAllBackends fetch all backends
func (b *BackendFetcher) FetchAllBackends(ctx context.Context) ([]v1.StorageBackendContent, error) {
	contents, err := pkgUtils.ListContent(ctx, app.GetGlobalConfig().BackendUtils)
	if err != nil {
		log.AddContext(ctx).Errorf("list storageBackendContent failed, error: %v", err)
		return []v1.StorageBackendContent{}, err
	}

	if contents == nil || len(contents.Items) == 0 {
		log.AddContext(ctx).Debugln("not found any storageBackendContents")
		return []v1.StorageBackendContent{}, nil
	}

	var result []v1.StorageBackendContent
	for _, content := range contents.Items {
		if contentCanSync(ctx, content) {
			result = append(result, content)
		}
	}
	return result, nil
}

// FetchBackendByName fetch storage tuple from kube-api by name
func (b *BackendFetcher) FetchBackendByName(ctx context.Context, name string,
	checkOnline bool) (*v1.StorageBackendContent, error) {
	claimNameMeta := pkgUtils.MakeMetaWithNamespace(app.GetGlobalConfig().Namespace, name)
	content, err := pkgUtils.GetContentByClaimMeta(ctx, claimNameMeta)
	if err != nil {
		log.AddContext(ctx).Errorf("get storageBackendContent failed, name: [%s] error: [%v]", name, err)
		return nil, err
	}

	if content.Status == nil || (checkOnline && !content.Status.Online) {
		msg := fmt.Sprintf("storageBackendContent is offline, name: [%s] ", name)
		return nil, errors.New(msg)
	}
	return content, nil
}

func contentCanSync(ctx context.Context, content v1.StorageBackendContent) bool {
	if content.Status == nil {
		log.AddContext(ctx).Debugf("content %s status is nil, skipping", content.Name)
		return false
	}

	if len(content.Status.Capabilities) == 0 {
		log.AddContext(ctx).Debugf("content %s capabilities is empty, skipping", content.Name)
		return false
	}
	return true
}
