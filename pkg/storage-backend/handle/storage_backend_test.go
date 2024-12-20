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

package handle

import (
	"context"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"google.golang.org/grpc"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/lib/drcsi"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/lib/drcsi/rpc"
)

func initBackend() *backend {
	return &backend{}
}

func TestNewBackend(t *testing.T) {
	NewBackend(&grpc.ClientConn{})
}

func TestAddStorageBackend(t *testing.T) {
	fakeBackend := initBackend()
	clientPatch := gomonkey.ApplyFunc(drcsi.NewStorageBackendClient,
		func(cc grpc.ClientConnInterface) drcsi.StorageBackendClient {
			return drcsi.NewStorageBackendClient(fakeBackend.conn)
		})
	defer clientPatch.Reset()

	providerPatch := gomonkey.ApplyFunc(rpc.GetProviderName,
		func(ctx context.Context, conn *grpc.ClientConn) (string, error) {
			return "fake-provider", nil
		})
	defer providerPatch.Reset()

	addPatch := gomonkey.ApplyFunc(addStorageBackend,
		func(ctx context.Context, conn *grpc.ClientConn, req *drcsi.AddStorageBackendRequest) (
			*drcsi.AddStorageBackendResponse, error) {
			return &drcsi.AddStorageBackendResponse{BackendId: "fake-backend-id"}, nil
		})
	defer addPatch.Reset()

	name, backendId, err := fakeBackend.AddStorageBackend(context.TODO(),
		"fake-content", "fake-configmapMeta", "fake-secretMeta", nil)
	if name != "fake-provider" || backendId != "fake-backend-id" || err != nil {
		t.Errorf("TestAddStorageBackend failed, name: %s, backendId: %s, error: %v",
			name, backendId, err)
	}
}
