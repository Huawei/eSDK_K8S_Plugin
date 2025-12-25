/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2022-2023. All rights reserved.
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

package driver

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	cfg "github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app/config"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/handler"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/model"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/plugin"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/k8sutils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	logName = "controller_helper_test.log"
)

func TestMain(m *testing.M) {
	getGlobalConfig := gostub.StubFunc(&app.GetGlobalConfig, cfg.MockCompletedConfig())
	defer getGlobalConfig.Reset()

	log.MockInitLogging(logName)
	defer log.MockStopLogging(logName)

	m.Run()
}

func TestCheckReservedSnapshotSpaceRatio(t *testing.T) {
	t.Run("Normal", func(t *testing.T) {
		param := map[string]interface{}{
			"reservedSnapshotSpaceRatio": "50",
		}
		require.NoError(t, checkReservedSnapshotSpaceRatio(context.TODO(), param))
	})

	t.Run("Not int", func(t *testing.T) {
		param := map[string]interface{}{
			"reservedSnapshotSpaceRatio": "20%",
		}
		require.Error(t, checkReservedSnapshotSpaceRatio(context.TODO(), param))
	})

	t.Run("Exceed the upper limit", func(t *testing.T) {
		param := map[string]interface{}{
			"reservedSnapshotSpaceRatio": "60",
		}
		require.Error(t, checkReservedSnapshotSpaceRatio(context.TODO(), param))
	})

	t.Run("Below the lower limit", func(t *testing.T) {
		param := map[string]interface{}{
			"reservedSnapshotSpaceRatio": "-10",
		}
		require.Error(t, checkReservedSnapshotSpaceRatio(context.TODO(), param))
	})

}

func mockCreateRequest() *csi.CreateVolumeRequest {
	capacity := &csi.CapacityRange{
		RequiredBytes: 1024 * 1024 * 1024,
	}
	parameters := map[string]string{
		"volumeType": "fs",
		"allocType":  "thin",
		"authClient": "*",
	}
	return &csi.CreateVolumeRequest{
		Name:               "fake-pvc-name",
		CapacityRange:      capacity,
		VolumeCapabilities: []*csi.VolumeCapability{},
		Parameters:         parameters,
	}
}

func initDriver() *CsiDriver {
	return NewServer(app.GetGlobalConfig().DriverName,
		"csiVersion",
		app.GetGlobalConfig().K8sUtils,
		app.GetGlobalConfig().NodeName)
}

func initPool(poolName string) *model.StoragePool {
	return &model.StoragePool{
		Name:         poolName,
		Storage:      "oceanstor-nas",
		Parent:       "fake-bakcend",
		Capabilities: make(map[string]bool),
		Plugin:       plugin.GetPlugin("oceanstor-nas"),
	}
}

func TestCreateVolumeWithoutBackend(t *testing.T) {
	driver := initDriver()
	req := mockCreateRequest()

	m := gomonkey.ApplyMethod(reflect.TypeOf(driver.backendSelector), "SelectPoolPair",
		func(hander *handler.BackendSelector, ctx context.Context, requestSize int64,
			parameters map[string]interface{}) (*model.SelectPoolPair, error) {
			return nil, errors.New("backend not exist")
		})
	defer m.Reset()

	_, err := driver.createVolume(context.TODO(), req)
	if err == nil {
		t.Error("test create without backend failed")
	}
}

func TestCreateVolume(t *testing.T) {
	driver := initDriver()
	m := gomonkey.ApplyMethod(reflect.TypeOf(driver.backendSelector), "SelectPoolPair",
		func(hander *handler.BackendSelector, ctx context.Context, requestSize int64,
			parameters map[string]interface{}) (*model.SelectPoolPair, error) {
			return &model.SelectPoolPair{
				Local: &model.StoragePool{
					Name:         "poolName",
					Storage:      "oceanstor-nas",
					Parent:       "fake-bakcend",
					Capabilities: make(map[string]bool),
					Plugin:       plugin.GetPlugin("oceanstor-nas"),
				},
				Remote: nil,
			}, nil
		})
	defer m.Reset()
	plg := plugin.GetPlugin("oceanstor-nas")
	m = gomonkey.ApplyMethod(reflect.TypeOf(plg), "CreateVolume",
		func(*plugin.OceanstorNasPlugin, context.Context, string, map[string]interface{}) (utils.Volume, error) {
			return utils.NewVolume("fake-nfs"), nil
		})
	defer m.Reset()

	req := mockCreateRequest()
	_, err := driver.createVolume(context.TODO(), req)
	if err != nil {
		t.Error("test create with storage failed")
	}
}

func TestImportVolumeWithoutBackend(t *testing.T) {
	driver := initDriver()
	req := mockCreateRequest()

	m := gomonkey.ApplyMethod(reflect.TypeOf(driver.backendSelector), "SelectBackend",
		func(hander *handler.BackendSelector, ctx context.Context, backendName string) (*model.Backend, error) {
			return nil, nil
		})
	defer m.Reset()

	_, err := driver.manageVolume(context.TODO(), req, "fake-nfs", "fake-backend")
	if err == nil {
		t.Error("test import without backend failed")
	}
}

func TestImportVolume(t *testing.T) {
	plg := plugin.GetPlugin("oceanstor-nas")
	localPool := initPool("local-pool")

	driver := initDriver()
	m := gomonkey.ApplyMethod(reflect.TypeOf(driver.backendSelector), "SelectBackend",
		func(hander *handler.BackendSelector, ctx context.Context, backendName string) (*model.Backend, error) {
			return &model.Backend{
				Name:   "fake-backend",
				Plugin: plg,
				Pools:  []*model.StoragePool{localPool},
			}, nil
		})
	defer m.Reset()
	m = gomonkey.ApplyMethod(reflect.TypeOf(plg), "QueryVolume",
		func(*plugin.OceanstorNasPlugin, context.Context, string, map[string]interface{}) (utils.Volume, error) {
			vol := utils.NewVolume("fake-nfs")
			vol.SetSize(1024 * 1024 * 1024)
			return vol, nil
		})
	defer m.Reset()

	req := mockCreateRequest()
	_, err := driver.manageVolume(context.TODO(), req, "fake-nfs", "fake-backend")
	if err != nil {
		t.Errorf("test import with storage failed, error %v", err)
	}
}

// Test_processAnnotations test fun
func Test_processAnnotations(t *testing.T) {
	// arrange mock
	fileSystemKey := app.GetGlobalConfig().DriverName + annFileSystemMode
	volumeNameKey := app.GetGlobalConfig().DriverName + annVolumeName
	annotations := map[string]string{
		fileSystemKey: "HyperMetro",
		volumeNameKey: "test",
	}
	req := &csi.CreateVolumeRequest{Parameters: map[string]string{}}

	// action
	err := processAnnotations(annotations, req)
	// assert
	if err != nil {
		t.Errorf("Test_processAnnotations() failed, error = %v", err)
	}

	if mode, exist := req.Parameters["fileSystemMode"]; !exist || mode != "HyperMetro" {
		t.Errorf("Test_processAnnotations() failed, anno: %+v, want fileSystemMode exist and equal HyperMetro, "+
			"but got = %v", annotations, mode)
	}
	if volume, exist := req.Parameters["annVolumeName"]; !exist || volume != "test" {
		t.Errorf("Test_processAnnotations() failed, anno: %+v, want annVolumeName exist and equal HyperMetro, "+
			"but got = %v", annotations, volume)
	}
}

func Test_VerifyExpandArguments_Success(t *testing.T) {
	// arrange
	req := &csi.ControllerExpandVolumeRequest{CapacityRange: &csi.CapacityRange{RequiredBytes: 1073741824}}
	backend := &model.Backend{}

	// mock
	patches := gomonkey.ApplyFuncReturn(isSupportExpandVolume, true, nil).
		ApplyFuncReturn(verifySectorSize, nil)
	defer patches.Reset()

	// action
	err := verifyExpandArguments(context.Background(), req, backend)

	// assert
	assert.NoError(t, err)
}

func Test_VerifySectorSize_Success(t *testing.T) {
	// arrange
	req := &csi.ControllerExpandVolumeRequest{VolumeId: "backend.pvc_test_volume_id"}
	backend := &model.Backend{
		Plugin: &plugin.OceanstorNasPlugin{},
	}
	minSize := int64(1024 * 1024 * 1024)
	app.GetGlobalConfig().K8sUtils = &k8sutils.KubeClient{}

	// mock
	patches := gomonkey.ApplyMethodReturn(backend.Plugin,
		"GetSectorSize", int64(constants.AllocationUnitBytes)).
		ApplyMethodReturn(app.GetGlobalConfig().K8sUtils, "GetVolumeAttrsByVolumeId", nil, nil).
		ApplyFuncReturn(utils.GetValueOrFallback[string], "true")
	defer patches.Reset()

	// action
	err := verifySectorSize(context.Background(), req.GetVolumeId(), backend, minSize)

	// assert
	assert.NoError(t, err)
}

func Test_VerifySectorSize_CapacityNotMultiple(t *testing.T) {
	// arrange
	req := &csi.ControllerExpandVolumeRequest{VolumeId: "backend.pvc_test_volume_id"}
	backend := &model.Backend{
		Plugin: &plugin.OceanstorNasPlugin{},
	}
	minSize := int64(1024*1024*1024) + 511
	app.GetGlobalConfig().K8sUtils = &k8sutils.KubeClient{}

	// mock
	patches := gomonkey.ApplyMethodReturn(backend.Plugin,
		"GetSectorSize", int64(constants.AllocationUnitBytes)).
		ApplyMethodReturn(app.GetGlobalConfig().K8sUtils, "GetVolumeAttrsByVolumeId", []map[string]string{{
			constants.DisableVerifyCapacityKey: "false"}}, nil).
		ApplyFuncReturn(utils.GetValueOrFallback[string], "true")
	defer patches.Reset()

	// action
	err := verifySectorSize(context.Background(), req.GetVolumeId(), backend, minSize)

	// assert
	assert.Error(t, err)
	assert.ErrorContains(t, err, "is not an integer or not multiple of")
}

func Test_isSupportExpandVolume_NasSuccess(t *testing.T) {
	// arrange
	req := &csi.ControllerExpandVolumeRequest{VolumeId: "backend.pvc_test_volume_id"}
	backend := &model.Backend{Storage: constants.OceanStorASeriesNas}

	// action
	res, err := isSupportExpandVolume(context.Background(), req, backend)

	if err != nil {
		t.Errorf("Test_isSupportExpandVolume_NasSuccess failed, err is %v", err)
	}

	if !res {
		t.Errorf("Test_isSupportExpandVolume_NasSuccess failed, wantRes = %v, gotRes = %v", true, res)
	}
}
