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

// Package handle implements AddStorageBackend/RemoveStorageBackend/UpdateStorageBackend/GetBackendStats
package handle

import (
	"context"
	"errors"

	"google.golang.org/grpc"

	"huawei-csi-driver/lib/drcsi"
	"huawei-csi-driver/lib/drcsi/rpc"
	"huawei-csi-driver/utils/log"
)

// BackendInterfaces includes interfaces that call provider
type BackendInterfaces interface {
	// AddStorageBackend add storageBackend to provider
	AddStorageBackend(ctx context.Context, claimName, configmapMeta, secretMeta string,
		parameters map[string]string) (string, string, error)
	// RemoveStorageBackend remove the storageBackend from provider
	RemoveStorageBackend(ctx context.Context, backendName string) (err error)
	// UpdateStorageBackend update the storageBackend
	UpdateStorageBackend(ctx context.Context, contentName, backendName, configmapMeta, secretMeta string,
		parameters map[string]string) error
	// GetStorageBackendStats get all backend info from the provider
	GetStorageBackendStats(ctx context.Context, contentName, backendName string) (*drcsi.GetBackendStatsResponse, error)
}

type backend struct {
	conn *grpc.ClientConn
}

// NewBackend returns a new BackendInterfaces
func NewBackend(conn *grpc.ClientConn) BackendInterfaces {
	return &backend{
		conn: conn,
	}
}

func addStorageBackend(ctx context.Context, conn *grpc.ClientConn, req *drcsi.AddStorageBackendRequest) (
	*drcsi.AddStorageBackendResponse, error) {
	return drcsi.NewStorageBackendClient(conn).AddStorageBackend(ctx, req)
}

// AddStorageBackend add storageBackend to provider
func (b *backend) AddStorageBackend(ctx context.Context, claimName, configmapMeta, secretMeta string,
	parameters map[string]string) (string, string, error) {
	providerName, err := rpc.GetProviderName(ctx, b.conn)
	if err != nil {
		return "", "", err
	}

	req := drcsi.AddStorageBackendRequest{
		Name:          claimName,
		ConfigmapMeta: configmapMeta,
		SecretMeta:    secretMeta,
		Parameters:    parameters,
	}

	rep, err := addStorageBackend(ctx, b.conn, &req)
	if err != nil {
		return "", "", err
	}
	return providerName, rep.GetBackendId(), nil
}

// RemoveStorageBackend remove the storageBackend from provider
func (b *backend) RemoveStorageBackend(ctx context.Context, backendName string) error {
	client := drcsi.NewStorageBackendClient(b.conn)
	_, err := client.RemoveStorageBackend(ctx, &drcsi.RemoveStorageBackendRequest{
		BackendId: backendName,
	})

	return err
}

func updateStorageBackend(ctx context.Context, conn *grpc.ClientConn, req *drcsi.UpdateStorageBackendRequest) (
	*drcsi.UpdateStorageBackendResponse, error) {
	return drcsi.NewStorageBackendClient(conn).UpdateStorageBackend(ctx, req)
}

// UpdateStorageBackend update the storageBackend
func (b *backend) UpdateStorageBackend(ctx context.Context, contentName, backendName, configmapMeta, secretMeta string,
	parameters map[string]string) error {
	log.AddContext(ctx).Infof("UpdateStorageBackend of backend %s", backendName)
	req := drcsi.UpdateStorageBackendRequest{
		Name:          contentName,
		BackendId:     backendName,
		ConfigmapMeta: configmapMeta,
		SecretMeta:    secretMeta,
		Parameters:    parameters,
	}

	_, err := updateStorageBackend(ctx, b.conn, &req)
	if err != nil {
		return err
	}
	return nil
}

// GetStorageBackendStats get all backend info from the provider
func (b *backend) GetStorageBackendStats(ctx context.Context, contentName, backendName string) (
	*drcsi.GetBackendStatsResponse, error) {
	log.AddContext(ctx).Infof("GetStorageBackendStats of backend %s", backendName)
	if backendName == "" {
		return &drcsi.GetBackendStatsResponse{}, errors.New("backendName can not be empty")
	}

	return drcsi.NewStorageBackendClient(b.conn).GetBackendStats(ctx, &drcsi.GetBackendStatsRequest{
		Name:      contentName,
		BackendId: backendName,
	})
}
