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
	remoteMockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	remoteCli := mock_client.NewMockOceanstorClientInterface(remoteMockCtrl)
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

func TestSAN_DeleteSnapshot_WithHyperMetro_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
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
