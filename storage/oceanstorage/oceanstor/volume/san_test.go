/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2025-2025. All rights reserved.
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

package volume

import (
	"context"
	"errors"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/mocks/mock_client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
)

func TestSAN_Query_success(t *testing.T) {
	// arrange
	ctx := context.Background()
	cli, _ := client.NewClient(ctx, &client.NewClientConfig{})
	san := NewSAN(cli, nil, nil, constants.OceanStorDoradoV6)
	param := map[string]interface{}{
		"applicationtype": "testApp",
	}

	lun := map[string]interface{}{
		"WORKLOADTYPEID": "1",
		"CAPACITY":       "1024",
	}

	// mock
	m := gomonkey.NewPatches()
	defer m.Reset()
	m.ApplyMethodReturn(&client.OceanstorClient{}, "GetLunByName", lun, nil).
		ApplyMethodReturn(&client.OceanstorClient{}, "GetApplicationTypeByName", "1", nil)

	// action
	gotVolume, gotErr := san.Query(ctx, "testName", param)

	// assert
	assert.Nil(t, gotErr)
	assert.Equal(t, "testName", gotVolume.GetVolumeName())
}

func TestSAN_Query_WorkLoadTypeUnmatched(t *testing.T) {
	// arrange
	ctx := context.Background()
	cli, _ := client.NewClient(ctx, &client.NewClientConfig{})
	san := NewSAN(cli, nil, nil, constants.OceanStorDoradoV6)
	param := map[string]interface{}{
		"applicationtype": "testApp",
	}

	lun := map[string]interface{}{
		"WORKLOADTYPEID": "1",
		"CAPACITY":       "1024",
	}

	// mock
	m := gomonkey.NewPatches()
	defer m.Reset()
	m.ApplyMethodReturn(&client.OceanstorClient{}, "GetLunByName", lun, nil).
		ApplyMethodReturn(&client.OceanstorClient{}, "GetApplicationTypeByName", "2", nil)

	// action
	gotVolume, gotErr := san.Query(ctx, "testName", param)

	// assert
	assert.Nil(t, gotVolume)
	assert.ErrorContains(t, gotErr, "the workload type is different between")
}

func TestSAN_CreateHyperMetroSnapshot_Succeed(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	remoteCli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	san := NewSAN(cli, remoteCli, nil, constants.OceanStorDoradoV6)
	lunName := "mock-lunName"
	snapshotName := "mock-snapshotName"
	lun := map[string]interface{}{"ID": "mock-lun-ID"}
	pair := map[string]interface{}{"ID": "mock-pair-ID"}
	snap := map[string]interface{}{
		"RUNNINGSTATUS": "43",
		"USERCAPACITY":  "1000",
		"TIMESTAMP":     "123",
		"PARENTID":      "123",
	}

	t.Run("create hyper metro snapshot", func(t *testing.T) {
		// mock
		cli.EXPECT().GetLunByName(ctx, lunName).Return(lun, nil)
		cli.EXPECT().GetLunSnapshotByName(ctx, snapshotName).Return(nil, nil).Times(1)
		cli.EXPECT().GetLunSnapshotByName(ctx, snapshotName).Return(snap, nil).Times(2)
		remoteCli.EXPECT().GetLunSnapshotByName(ctx, snapshotName).Return(nil, nil)
		remoteCli.EXPECT().GetLunSnapshotByName(ctx, snapshotName).Return(snap, nil)
		cli.EXPECT().GetHyperMetroPairByLocalObjID(ctx, "mock-lun-ID").Return(pair, nil)
		cli.EXPECT().CreateHyperMetroSnap(ctx, snapshotName, "mock-pair-ID").Return(
			map[string]interface{}{"localSnapId": "1", "remoteSnapId": "2"}, nil)
		parameters := map[string]interface{}{enableHyperMetroSnap: "true"}

		// action
		_, err := san.CreateSnapshot(ctx, lunName, snapshotName, parameters)
		// assert
		assert.NoError(t, err)
	})

	t.Run("create single site snapshot", func(t *testing.T) {
		// mock
		cli.EXPECT().GetLunByName(ctx, lunName).Return(lun, nil)
		cli.EXPECT().GetLunSnapshotByName(ctx, snapshotName).Return(nil, nil).Times(1)
		cli.EXPECT().GetLunSnapshotByName(ctx, snapshotName).Return(snap, nil).Times(2)
		cli.EXPECT().CreateLunSnapshot(ctx, snapshotName, "mock-lun-ID").Return(
			map[string]interface{}{"ID": "1", "USERCAPACITY": "2"}, nil)
		cli.EXPECT().ActivateLunSnapshot(ctx, "1").Return(nil)
		parameters := map[string]interface{}{}

		// action
		_, err := san.CreateSnapshot(ctx, lunName, snapshotName, parameters)
		// assert
		assert.NoError(t, err)
	})
}

func TestSAN_CreateHyperMetroSnapshot_Failed(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	san := NewSAN(cli, nil, nil, constants.OceanStorDoradoV6)
	lunName := "mock-lunName"
	snapshotName := "mock-snapshotName"
	lun := map[string]interface{}{"ID": "mock-lun-ID"}
	pair := map[string]interface{}{"ID": "mock-pair-ID"}

	// mock
	cli.EXPECT().GetLunByName(ctx, lunName).Return(lun, nil)
	cli.EXPECT().GetLunSnapshotByName(ctx, snapshotName).Return(nil, nil).Times(1)
	cli.EXPECT().GetHyperMetroPairByLocalObjID(ctx, "mock-lun-ID").Return(pair, nil)
	cli.EXPECT().CreateHyperMetroSnap(ctx, snapshotName, "mock-pair-ID").Return(
		nil, errors.New("mock-err"))
	parameters := map[string]interface{}{enableHyperMetroSnap: "true"}

	// action
	_, err := san.CreateSnapshot(ctx, lunName, snapshotName, parameters)
	// assert
	assert.Error(t, err)
}

func TestSAN_CreateHyperMetro_Succeed(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	san := NewSAN(cli, nil, nil, constants.OceanStorDoradoV6)
	params := map[string]interface{}{
		"metropairsyncspeed": "3",
	}
	taskResult := map[string]interface{}{
		"metroDomainID": "mock-domain-id",
		"localLunID":    "mock-local-id",
		"remoteLunID":   "mock-remote-id",
	}
	data := map[string]interface{}{
		"DOMAINID":       "mock-domain-id",
		"HCRESOURCETYPE": 1,
		"ISFIRSTSYNC":    false,
		"LOCALOBJID":     "mock-local-id",
		"REMOTEOBJID":    "mock-remote-id",
	}

	// mock
	cli.EXPECT().GetHyperMetroPairByLocalObjID(ctx, "mock-local-id").Return(nil, nil)
	cli.EXPECT().CreateHyperMetroPair(ctx, data).Return(map[string]interface{}{
		"ID": "mock-pair-ID",
	}, nil)
	cli.EXPECT().GetHyperMetroPair(ctx, "mock-pair-ID").Return(map[string]interface{}{
		"HEALTHSTATUS":  "1",
		"RUNNINGSTATUS": hyperMetroPairRunningStatusPause,
	}, nil)
	cli.EXPECT().GetHyperMetroPair(ctx, "mock-pair-ID").Return(map[string]interface{}{
		"HEALTHSTATUS":  "1",
		"RUNNINGSTATUS": hyperMetroPairRunningStatusNormal,
	}, nil)

	// action
	result, err := san.createHyperMetro(ctx, params, taskResult)

	// assert
	assert.NoError(t, err)
	hyperMetroPairID, _ := utils.GetValue[string](result, "hyperMetroPairID")
	if hyperMetroPairID != "mock-pair-ID" {
		t.Fatalf("want hyperMetroPairID is mock-pair-ID, but got %s", hyperMetroPairID)
	}
}

func TestSAN_Create_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	san := NewSAN(cli, nil, nil, constants.OceanStorDoradoV6)

	params := map[string]interface{}{
		"name":            "test-lun",
		"storagepool":     "test-pool",
		"capacity":        int64(1073741824),
		"applicationtype": "testApp",
	}

	pool := map[string]interface{}{
		"ID": "pool-123",
	}
	lun := map[string]interface{}{
		"ID":       "lun-123",
		"WWN":      "wwn-123456",
		"CAPACITY": "1073741824",
	}

	// mock - preCreate
	cli.EXPECT().GetPoolByName(ctx, "test-pool").Return(pool, nil)
	cli.EXPECT().MakeLunName("test-lun").Return("k8s_test-lun")
	cli.EXPECT().GetApplicationTypeByName(ctx, "testApp").Return("1", nil)

	// mock - createLocalLun
	cli.EXPECT().GetLunByName(ctx, "k8s_test-lun").Return(nil, nil)
	cli.EXPECT().CreateLun(ctx, gomock.Any()).Return(lun, nil)

	// action
	vol, err := san.Create(ctx, params)

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, vol)
	assert.Equal(t, "k8s_test-lun", vol.GetVolumeName())
}

func TestSAN_Create_WithHyperMetro_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	metroCli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	san := NewSAN(cli, metroCli, nil, constants.OceanStorDoradoV6)

	params := map[string]interface{}{
		"name":              "test-lun",
		"storagepool":       "test-pool",
		"remotestoragepool": "test-remote-pool",
		"capacity":          int64(1073741824),
		"applicationtype":   "testApp",
		"hypermetro":        true,
		"metrodomain":       "test-domain",
	}

	pool := map[string]interface{}{
		"ID": "pool-123",
	}
	remotePool := map[string]interface{}{
		"ID": "remote-pool-456",
	}
	domain := map[string]interface{}{
		"ID":            "domain-789",
		"RUNNINGSTATUS": "1",
	}
	lun := map[string]interface{}{
		"ID":       "lun-123",
		"WWN":      "wwn-123456",
		"CAPACITY": "1073741824",
	}
	remoteLun := map[string]interface{}{
		"ID":        "remote-lun-456",
		"WWN":       "remote-wwn-456789",
		"CAPACITY":  "1073741824",
		"IOCLASSID": "",
	}

	// mock - preCreate for local cli
	cli.EXPECT().GetPoolByName(ctx, "test-pool").Return(pool, nil)
	cli.EXPECT().MakeLunName("test-lun").Return("k8s_test-lun")
	cli.EXPECT().GetApplicationTypeByName(ctx, "testApp").Return("1", nil)

	// mock - getHyperMetroParams
	metroCli.EXPECT().GetPoolByName(ctx, "test-remote-pool").Return(remotePool, nil)
	metroCli.EXPECT().GetHyperMetroDomainByName(ctx, "test-domain").Return(domain, nil)
	metroCli.EXPECT().GetApplicationTypeByName(ctx, "testApp").Return("1", nil)

	// mock - createLocalLun
	cli.EXPECT().GetLunByName(ctx, "k8s_test-lun").Return(nil, nil)
	cli.EXPECT().CreateLun(ctx, gomock.Any()).Return(lun, nil)

	// mock - createRemoteLun
	metroCli.EXPECT().GetLunByName(ctx, "k8s_test-lun").Return(nil, nil)
	metroCli.EXPECT().CreateLun(ctx, gomock.Any()).Return(remoteLun, nil)

	// mock - createHyperMetro
	cli.EXPECT().GetHyperMetroPairByLocalObjID(ctx, "lun-123").Return(nil, nil)
	cli.EXPECT().CreateHyperMetroPair(ctx, gomock.Any()).Return(map[string]interface{}{"ID": "pair-123"}, nil)
	cli.EXPECT().GetHyperMetroPair(ctx, "pair-123").Return(map[string]interface{}{
		"HEALTHSTATUS":  "1",
		"RUNNINGSTATUS": hyperMetroPairRunningStatusNormal,
	}, nil)

	// action
	vol, err := san.Create(ctx, params)

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, vol)
	assert.Equal(t, "k8s_test-lun", vol.GetVolumeName())
}

func TestSAN_Create_PreCreateFailed(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	san := NewSAN(cli, nil, nil, constants.OceanStorDoradoV6)

	params := map[string]interface{}{
		"name":        "test-lun",
		"capacity":    int64(1073741824),
		"storagepool": "",
	}

	// action
	vol, err := san.Create(ctx, params)

	// assert
	assert.Nil(t, vol)
	assert.ErrorContains(t, err, "must specify storage pool")
}

func TestSAN_Create_LunAlreadyExistsWithInsufficientCapacity(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	san := NewSAN(cli, nil, nil, constants.OceanStorDoradoV6)

	params := map[string]interface{}{
		"name":        "test-lun",
		"storagepool": "test-pool",
		"capacity":    int64(1073741824),
	}

	pool := map[string]interface{}{
		"ID": "pool-123",
	}
	existingLun := map[string]interface{}{
		"ID":       "lun-123",
		"WWN":      "wwn-123456",
		"CAPACITY": "536870912",
	}

	// mock - preCreate
	cli.EXPECT().GetPoolByName(ctx, "test-pool").Return(pool, nil)
	cli.EXPECT().MakeLunName("test-lun").Return("k8s_test-lun")

	// mock - createLocalLun: lun already exists
	cli.EXPECT().GetLunByName(ctx, "k8s_test-lun").Return(existingLun, nil)

	// action
	vol, err := san.Create(ctx, params)

	// assert
	assert.Nil(t, vol)
	assert.ErrorContains(t, err, "actual capacity is less than requested capacity")
}

func TestSAN_Create_LunAlreadyExists(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	san := NewSAN(cli, nil, nil, constants.OceanStorDoradoV6)

	params := map[string]interface{}{
		"name":        "test-lun",
		"storagepool": "test-pool",
		"capacity":    int64(1073741824),
	}

	pool := map[string]interface{}{
		"ID": "pool-123",
	}
	existingLun := map[string]interface{}{
		"ID":       "lun-123",
		"WWN":      "wwn-123456",
		"CAPACITY": "1073741824",
	}

	// mock - preCreate
	cli.EXPECT().GetPoolByName(ctx, "test-pool").Return(pool, nil)
	cli.EXPECT().MakeLunName("test-lun").Return("k8s_test-lun")

	// mock - createLocalLun: lun already exists
	cli.EXPECT().GetLunByName(ctx, "k8s_test-lun").Return(existingLun, nil)
	cli.EXPECT().GetClonePairInfo(ctx, "lun-123").Return(nil, nil)
	cli.EXPECT().DeleteClonePair(ctx, "lun-123").Return(nil)

	// action
	vol, err := san.Create(ctx, params)

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, vol)
	assert.Equal(t, "k8s_test-lun", vol.GetVolumeName())
}

func TestSAN_DeleteSnapshot_WithHyperMetro_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	metroCli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	san := NewSAN(cli, metroCli, nil, constants.OceanStorDoradoV6)

	snapshotName := "mock-snapshotName"
	snapshotId := "mock-snapshot-ID"
	snapshot := map[string]interface{}{"ID": snapshotId}
	remoteSnapshotId := "mock-remote-snapshot-ID"
	remoteSnapshot := map[string]interface{}{"ID": remoteSnapshotId}

	// mock
	cli.EXPECT().GetLunSnapshotByName(ctx, snapshotName).Return(snapshot, nil)
	metroCli.EXPECT().GetLunSnapshotByName(ctx, snapshotName).Return(remoteSnapshot, nil)
	metroCli.EXPECT().DeactivateLunSnapshot(ctx, remoteSnapshotId).Return(nil)
	metroCli.EXPECT().DeleteLunSnapshot(ctx, remoteSnapshotId).Return(nil)
	cli.EXPECT().DeactivateLunSnapshot(ctx, snapshotId).Return(nil)
	cli.EXPECT().DeleteLunSnapshot(ctx, snapshotId).Return(nil)

	// action
	err := san.DeleteSnapshot(ctx, snapshotName)
	// assert
	assert.NoError(t, err)
}
