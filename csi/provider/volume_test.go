/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2024-2024. All rights reserved.
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

// Package provider used to test volume module
package provider

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/require"

	"huawei-csi-driver/csi/backend/handler"
	"huawei-csi-driver/csi/backend/model"
	"huawei-csi-driver/csi/backend/plugin"
	"huawei-csi-driver/lib/drcsi"
	"huawei-csi-driver/storage/oceanstor/volume"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

const (
	logName = "csi_provider_volume_test.log"
)

func TestMain(m *testing.M) {
	log.MockInitLogging(logName)
	defer log.MockStopLogging(logName)

	m.Run()
}

func TestModifyVolume_EmptyParam(t *testing.T) {
	// arrange
	p := NewProvider("providerForTest", "TestVersion")
	req := &drcsi.ModifyVolumeRequest{
		VolumeId:               "Backend1.PVC1",
		StorageClassParameters: nil,
		MutableParameters:      map[string]string{},
	}

	// act
	_, err := p.ModifyVolume(context.TODO(), req)

	// assert
	require.Equal(t, nil, err)
}

func TestModifyVolume_HyperMetroTrue(t *testing.T) {
	// arrange
	p := NewProvider("providerForTest", "TestVersion")
	nasPlugin := &plugin.OceanstorNasPlugin{}
	req := &drcsi.ModifyVolumeRequest{
		VolumeId:               "Backend1.PVC1",
		StorageClassParameters: nil,
		MutableParameters:      map[string]string{"hyperMetro": "true"},
	}
	nas := volume.NewNAS(nil, nil, "test", volume.NASHyperMetro{}, true)

	// mock
	m := gomonkey.ApplyMethod(reflect.TypeOf(p.backendSelector), "SelectBackend",
		func(b *handler.BackendSelector, ctx context.Context, name string) (*model.Backend, error) {
			return &model.Backend{
				Plugin:       plugin.GetPlugin("oceanstor-nas"),
				MetroBackend: &model.Backend{},
			}, nil
		})
	m.ApplyMethod(reflect.TypeOf(p.backendSelector), "SelectRemotePool",
		func(*handler.BackendSelector, context.Context, int64, string, map[string]interface{}) (*model.StoragePool, error) {
			return &model.StoragePool{Name: "remotePoolName"}, nil
		})
	m.ApplyPrivateMethod(nasPlugin, "canModify", func() error { return nil })
	m.ApplyMethod(reflect.TypeOf(nasPlugin), "GetLocal2HyperMetroParameters",
		func(p *plugin.OceanstorNasPlugin, ctx context.Context, VolumeId string, parameters map[string]string) (
			map[string]interface{}, error) {
			return map[string]interface{}{
				"alloctype":    "thin",
				"authclient":   "*",
				"backend":      "test-backend",
				"capacity":     20971520,
				"description":  "test-description",
				"hypermetro":   true,
				"name":         "pvc-test",
				"storagepool":  "test-pool",
				"vstorepairid": "0",
			}, nil
		})
	m.ApplyMethod(reflect.TypeOf(nas), "Modify",
		func(n *volume.NAS, ctx context.Context, params map[string]interface{}) (utils.Volume, error) {
			return nil, nil
		})
	defer m.Reset()

	// act
	_, err := p.ModifyVolume(context.TODO(), req)

	// assert
	require.Equal(t, nil, err)
}

func TestModifyVolume_HyperMetroFalse(t *testing.T) {
	// arrange
	p := NewProvider("providerForTest", "TestVersion")
	req := &drcsi.ModifyVolumeRequest{
		VolumeId:               "Backend1.PVC1",
		StorageClassParameters: nil,
		MutableParameters:      map[string]string{"hyperMetro": "false"},
	}

	// mock
	m := gomonkey.ApplyMethod(reflect.TypeOf(p.backendSelector), "SelectBackend",
		func(b *handler.BackendSelector, ctx context.Context, name string) (*model.Backend, error) {
			return &model.Backend{
				Plugin:       plugin.GetPlugin("oceanstor-nas"),
				MetroBackend: &model.Backend{},
			}, nil
		})
	defer m.Reset()

	// act
	_, err := p.ModifyVolume(context.TODO(), req)

	// assert
	require.Error(t, err)
}

func TestModifyVolume_HyperMetroInvalid(t *testing.T) {
	// arrange
	p := NewProvider("providerForTest", "TestVersion")
	req := &drcsi.ModifyVolumeRequest{
		VolumeId:               "Backend1.PVC1",
		StorageClassParameters: nil,
		MutableParameters:      map[string]string{"hyperMetro": "False"},
	}

	// mock
	m := gomonkey.ApplyMethod(reflect.TypeOf(p.backendSelector), "SelectBackend",
		func(b *handler.BackendSelector, ctx context.Context, name string) (*model.Backend, error) {
			return &model.Backend{
				Plugin: plugin.GetPlugin("oceanstor-nas"),
			}, nil
		})
	defer m.Reset()

	// act
	_, err := p.ModifyVolume(context.TODO(), req)

	// assert
	require.Error(t, err)
}

func TestModifyVolume_HyperMetroSelectBackendFailed(t *testing.T) {
	// arrange
	p := NewProvider("providerForTest", "TestVersion")
	req := &drcsi.ModifyVolumeRequest{
		VolumeId:               "Backend1.PVC1",
		StorageClassParameters: nil,
		MutableParameters:      map[string]string{"hyperMetro": "False"},
	}

	// mock
	m := gomonkey.ApplyMethod(reflect.TypeOf(p.backendSelector), "SelectBackend",
		func(b *handler.BackendSelector, ctx context.Context, name string) (*model.Backend, error) {
			return nil, errors.New("mock error")
		})
	defer m.Reset()

	// act
	_, err := p.ModifyVolume(context.TODO(), req)

	// assert
	require.Error(t, err)
}

func TestModifyVolume_HyperMetroModifyVolumeFailed(t *testing.T) {
	// arrange
	p := NewProvider("providerForTest", "TestVersion")
	req := &drcsi.ModifyVolumeRequest{
		VolumeId:               "Backend1.PVC1",
		StorageClassParameters: nil,
		MutableParameters:      map[string]string{"hyperMetro": "False"},
	}

	// mock
	m := gomonkey.ApplyMethod(reflect.TypeOf(p.backendSelector), "SelectBackend",
		func(b *handler.BackendSelector, ctx context.Context, name string) (*model.Backend, error) {
			return &model.Backend{
				Plugin: plugin.GetPlugin("oceanstor-san"),
			}, nil
		})
	defer m.Reset()

	// act
	_, err := p.ModifyVolume(context.TODO(), req)

	// assert
	require.Error(t, err)
}
