/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2023-2025. All rights reserved.
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
	"fmt"
	"strconv"
	"testing"

	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	cfg "github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app/config"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/mocks/mock_client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	logName = "volume_dTree_test.log"
)

func TestMain(m *testing.M) {
	getGlobalConfig := gostub.StubFunc(&app.GetGlobalConfig, cfg.MockCompletedConfig())
	defer getGlobalConfig.Reset()

	log.MockInitLogging(logName)
	defer log.MockStopLogging(logName)

	m.Run()
}

func TestFormatKerberosParam(t *testing.T) {
	type TestFormatKerberosParam struct {
		target   interface{}
		expected int
	}
	var testCases = []TestFormatKerberosParam{
		{
			target:   "read_only",
			expected: 0,
		},
		{
			target:   "read_write",
			expected: 1,
		},
		{
			target:   "none",
			expected: 5,
		},
		{
			target:   "",
			expected: -1,
		},
		{
			target:   nil,
			expected: -1,
		},
	}

	for _, c := range testCases {
		assert.Equal(t, c.expected, formatKerberosParam(c.target))
	}
}

func Test_generateCreateDTreeDataFromParams(t *testing.T) {
	// arrange
	tests := []struct {
		name            string
		params          map[string]any
		want            map[string]any
		wantErrContains string
	}{
		{name: "full parameters",
			params: map[string]any{"fspermission": "777", "name": "test-dtree-name", "parentname": "test-parent-name",
				"vstoreid": "0", constants.AdvancedOptionsKey: "{\"key\": \"value\"}"},
			want: map[string]any{"unixPermissions": "777", "NAME": "test-dtree-name", "PARENTNAME": "test-parent-name",
				"vstoreId": "0", "PARENTTYPE": client.ParentTypeFS, "securityStyle": client.SecurityStyleUnix,
				"key": "value"}, wantErrContains: ""},
		{name: "unixPermissions override",
			params: map[string]any{"fspermission": "777", "name": "test-dtree-name", "parentname": "test-parent-name",
				"vstoreid": "0", constants.AdvancedOptionsKey: "{\"unixPermissions\": \"755\"}"},
			want: map[string]any{"unixPermissions": "755", "NAME": "test-dtree-name", "PARENTNAME": "test-parent-name",
				"vstoreId": "0", "PARENTTYPE": client.ParentTypeFS, "securityStyle": client.SecurityStyleUnix},
			wantErrContains: ""},
		{name: "unmarshal failed", params: map[string]any{constants.AdvancedOptionsKey: `{`}, want: nil,
			wantErrContains: "failed to unmarshal advancedOptions"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// action
			got, err := generateCreateDTreeDataFromParams(tt.params)

			// assert
			if tt.wantErrContains != "" {
				require.ErrorContains(t, err, tt.wantErrContains)
				return
			}
			require.Equal(t, tt.want, got)
		})
	}
}

var (
	fakeDTreeName              = "fake-dtree-name"
	fakeParentName             = "fake-parent-name"
	fakeDTreeID                = "fake-dtree-id"
	fakeVStoreID               = "fake-vstore-id"
	fakeDtreeQuotaSpaceHard    = "1024"
	fakeDtreeQuotaSpaceHardInt = int64(1024)
)

func Test_QueryVolume_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	dtree := NewDTree(cli)

	// mock
	cli.EXPECT().GetDTreeByName(ctx, "", fakeParentName, fakeVStoreID, fakeDTreeName).
		Return(
			map[string]interface{}{
				"ID": fakeDTreeID,
			},
			nil,
		)
	fakeQuotaResult := map[string]interface{}{"SPACEHARDQUOTA": fakeDtreeQuotaSpaceHard}
	fakeQuotaResultList := []interface{}{fakeQuotaResult}
	batchQueryParam := map[string]interface{}{
		"PARENTTYPE":    client.ParentTypeDTree,
		"PARENTID":      fakeDTreeID,
		"range":         "[0-100]",
		"vstoreId":      fakeVStoreID,
		"QUERYTYPE":     "2",
		"SPACEUNITTYPE": client.SpaceUnitTypeBytes,
	}
	cli.EXPECT().BatchGetQuota(ctx, batchQueryParam).
		Return(fakeQuotaResultList, nil)

	// action
	volume, err := dtree.Query(ctx, fakeDTreeName, fakeParentName, fakeVStoreID)

	// assert
	require.NoError(t, err)
	require.NotNil(t, volume)
	require.Equal(t, fakeDTreeName, volume.GetVolumeName())
	require.Equal(t, fakeDtreeQuotaSpaceHard, strconv.FormatInt(volume.GetSize(), 10))
}

func Test_QueryVolume_Fail(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	dtree := NewDTree(cli)
	batchQueryParam := map[string]interface{}{
		"PARENTTYPE":    client.ParentTypeDTree,
		"PARENTID":      fakeDTreeID,
		"range":         "[0-100]",
		"vstoreId":      fakeVStoreID,
		"QUERYTYPE":     "2",
		"SPACEUNITTYPE": client.SpaceUnitTypeBytes,
	}

	t.Run("get dtree by name failed", func(t *testing.T) {
		// mock
		fakeErr := fmt.Errorf("get dtree by name failed")
		cli.EXPECT().GetDTreeByName(ctx, "", fakeParentName, fakeVStoreID, fakeDTreeName).
			Return(nil, fakeErr)

		// action
		volume, err := dtree.Query(ctx, fakeDTreeName, fakeParentName, fakeVStoreID)

		// assert
		require.ErrorIs(t, err, fakeErr)
		require.Nil(t, volume)
	})

	t.Run("get nil dtree", func(t *testing.T) {
		// mock
		cli.EXPECT().GetDTreeByName(ctx, "", fakeParentName, fakeVStoreID, fakeDTreeName).
			Return(nil, nil)

		// action
		volume, err := dtree.Query(ctx, fakeDTreeName, fakeParentName, fakeVStoreID)

		// assert
		require.EqualError(t, err, fmt.Sprintf("dtree to query does not exist, dtree:%q parentName:%q vstoreID:%q ",
			fakeDTreeName, fakeParentName, fakeVStoreID))
		require.Nil(t, volume)
	})

	t.Run("batch get quota failed", func(t *testing.T) {
		// mock
		fakeErr := fmt.Errorf("batch get quota failed")
		cli.EXPECT().GetDTreeByName(ctx, "", fakeParentName, fakeVStoreID, fakeDTreeName).
			Return(map[string]interface{}{"ID": fakeDTreeID}, nil)
		cli.EXPECT().BatchGetQuota(ctx, batchQueryParam).
			Return(nil, fakeErr)

		// action
		volume, err := dtree.Query(ctx, fakeDTreeName, fakeParentName, fakeVStoreID)

		// assert
		require.ErrorIs(t, err, fakeErr)
		require.Nil(t, volume)
	})

	t.Run("batch get quota empty result", func(t *testing.T) {
		// mock
		cli.EXPECT().GetDTreeByName(ctx, "", fakeParentName, fakeVStoreID, fakeDTreeName).
			Return(map[string]interface{}{"ID": fakeDTreeID}, nil)
		cli.EXPECT().BatchGetQuota(ctx, batchQueryParam).
			Return(nil, nil)

		// action
		volume, err := dtree.Query(ctx, fakeDTreeName, fakeParentName, fakeVStoreID)

		// assert
		require.Nil(t, volume)
		require.EqualError(t, err, fmt.Sprintf("quota to query does not exist, dtree:%q parentName:%q vstoreID:%q",
			fakeDTreeName, fakeParentName, fakeVStoreID))
	})

	t.Run("has no quota hard space", func(t *testing.T) {
		// mock
		cli.EXPECT().GetDTreeByName(ctx, "", fakeParentName, fakeVStoreID, fakeDTreeName).
			Return(map[string]interface{}{"ID": fakeDTreeID}, nil)

		fakeQuotaResult := map[string]interface{}{"SPACEHARDQUOTA-wrong": fakeDtreeQuotaSpaceHard}
		fakeQuotaResultList := []interface{}{fakeQuotaResult}
		cli.EXPECT().BatchGetQuota(ctx, batchQueryParam).
			Return(fakeQuotaResultList, nil)

		// action
		volume, err := dtree.Query(ctx, fakeDTreeName, fakeParentName, fakeVStoreID)

		// assert
		require.Nil(t, volume)
		require.EqualError(t, err, fmt.Sprintf("quota to query does not contain hard quota, "+
			"dtree:%q parentName:%q vstoreID:%q",
			fakeDTreeName, fakeParentName, fakeVStoreID))
	})
}

func Test_ExpandVolume(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	dtree := NewDTree(cli)

	batchQueryParam := map[string]interface{}{
		"PARENTTYPE":    client.ParentTypeDTree,
		"PARENTID":      fakeDTreeID,
		"range":         "[0-100]",
		"vstoreId":      fakeVStoreID,
		"QUERYTYPE":     "2",
		"SPACEUNITTYPE": client.SpaceUnitTypeBytes,
	}

	createQuotaParam := map[string]any{"PARENTTYPE": client.ParentTypeDTree, "PARENTID": fakeDTreeID,
		"QUOTATYPE": client.QuotaTypeDir, "SPACEUNITTYPE": client.SpaceUnitTypeBytes,
		"SPACEHARDQUOTA": fakeDtreeQuotaSpaceHardInt, "vstoreId": fakeVStoreID}

	t.Run("expand volume success when quota is not exist", func(t *testing.T) {
		// mock
		cli.EXPECT().GetDTreeByName(ctx, "", fakeParentName, fakeVStoreID, fakeDTreeName).
			Return(map[string]interface{}{"ID": fakeDTreeID}, nil)
		cli.EXPECT().BatchGetQuota(ctx, batchQueryParam).
			Return([]interface{}{}, nil)
		cli.EXPECT().CreateQuota(ctx, createQuotaParam).Return(nil, nil)

		// action
		err := dtree.Expand(ctx, fakeParentName, fakeDTreeName, fakeVStoreID, fakeDtreeQuotaSpaceHardInt)

		// assert
		require.NoError(t, err)
	})
}
