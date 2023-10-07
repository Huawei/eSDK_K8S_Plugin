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
	. "github.com/smartystreets/goconvey/convey"

	xuanwuv1 "huawei-csi-driver/client/apis/xuanwu/v1"
	"huawei-csi-driver/csi/app"
	cfg "huawei-csi-driver/csi/app/config"
	"huawei-csi-driver/csi/backend"
	"huawei-csi-driver/csi/backend/plugin"
	clientSet "huawei-csi-driver/pkg/client/clientset/versioned"
	pkgUtils "huawei-csi-driver/pkg/utils"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
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
	Convey("Normal", t, func() {
		param := map[string]interface{}{
			"reservedSnapshotSpaceRatio": "50",
		}
		So(checkReservedSnapshotSpaceRatio(context.TODO(), param), ShouldBeNil)
	})

	Convey("Not int", t, func() {
		param := map[string]interface{}{
			"reservedSnapshotSpaceRatio": "20%",
		}
		So(checkReservedSnapshotSpaceRatio(context.TODO(), param), ShouldBeError)
	})

	Convey("Exceed the upper limit", t, func() {
		param := map[string]interface{}{
			"reservedSnapshotSpaceRatio": "60",
		}
		So(checkReservedSnapshotSpaceRatio(context.TODO(), param), ShouldBeError)
	})

	Convey("Below the lower limit", t, func() {
		param := map[string]interface{}{
			"reservedSnapshotSpaceRatio": "-10",
		}
		So(checkReservedSnapshotSpaceRatio(context.TODO(), param), ShouldBeError)
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

func initDriver() *Driver {
	return NewDriver(app.GetGlobalConfig().DriverName,
		"csiVersion",
		app.GetGlobalConfig().K8sUtils,
		app.GetGlobalConfig().NodeName)
}

func initPool(poolName string) *backend.StoragePool {
	return &backend.StoragePool{
		Name:         poolName,
		Storage:      "oceanstor-nas",
		Parent:       "fake-bakcend",
		Capabilities: map[string]interface{}{},
		Plugin:       plugin.GetPlugin("oceanstor-nas"),
	}
}

func TestCreateVolumeWithoutBackend(t *testing.T) {
	driver := initDriver()
	req := mockCreateRequest()

	s := gostub.StubFunc(&pkgUtils.CreatePVLabel)
	defer s.Reset()

	m := gomonkey.ApplyFunc(pkgUtils.ListClaim,
		func(ctx context.Context, client clientSet.Interface, namespace string) (
			*xuanwuv1.StorageBackendClaimList, error) {
			return &xuanwuv1.StorageBackendClaimList{}, errors.New("mock list claim failed")
		})
	defer m.Reset()

	_, err := driver.createVolume(context.TODO(), req)
	if err == nil {
		t.Error("test create without backend failed")
	}
}

func TestCreateVolume(t *testing.T) {
	localPool := initPool("local-pool")

	s := gostub.StubFunc(&backend.SelectStoragePool, localPool, nil, nil)
	defer s.Reset()

	s.StubFunc(&pkgUtils.CreatePVLabel)

	plg := plugin.GetPlugin("oceanstor-nas")
	m := gomonkey.ApplyMethod(reflect.TypeOf(plg), "CreateVolume",
		func(*plugin.OceanstorNasPlugin, context.Context, string, map[string]interface{}) (utils.Volume, error) {
			return utils.NewVolume("fake-nfs"), nil
		})
	defer m.Reset()

	driver := initDriver()
	req := mockCreateRequest()
	_, err := driver.createVolume(context.TODO(), req)
	if err != nil {
		t.Error("test create with storage failed")
	}
}

func TestImportVolumeWithoutBackend(t *testing.T) {
	driver := initDriver()
	req := mockCreateRequest()

	s := gostub.StubFunc(&backend.GetBackendWithFresh, nil)
	defer s.Reset()

	s.StubFunc(&pkgUtils.CreatePVLabel)

	_, err := driver.manageVolume(context.TODO(), req, "fake-nfs", "fake-backend")
	if err == nil {
		t.Error("test import without backend failed")
	}
}

func TestImportVolume(t *testing.T) {
	plg := plugin.GetPlugin("oceanstor-nas")
	localPool := initPool("local-pool")

	s := gostub.StubFunc(&backend.GetBackendWithFresh, &backend.Backend{
		Name:   "fake-backend",
		Plugin: plg,
		Pools:  []*backend.StoragePool{localPool},
	})
	defer s.Reset()

	s.StubFunc(&pkgUtils.CreatePVLabel)

	m := gomonkey.ApplyMethod(reflect.TypeOf(plg), "QueryVolume",
		func(*plugin.OceanstorNasPlugin, context.Context, string, map[string]interface{}) (utils.Volume, error) {
			vol := utils.NewVolume("fake-nfs")
			vol.SetSize(1024 * 1024 * 1024)
			return vol, nil
		})
	defer m.Reset()

	driver := initDriver()
	req := mockCreateRequest()
	_, err := driver.manageVolume(context.TODO(), req, "fake-nfs", "fake-backend")
	if err != nil {
		t.Errorf("test import with storage failed, error %v", err)
	}
}
