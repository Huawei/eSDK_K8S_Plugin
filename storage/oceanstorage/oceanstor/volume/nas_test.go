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

package volume

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/mocks/mock_client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
)

func Test_isHyperMetroFromParams(t *testing.T) {
	tests := []struct {
		name    string
		params  map[string]any
		want    bool
		wantErr error
	}{
		{name: "not exists", params: map[string]any{"hyperMetro": true}, want: false, wantErr: nil},
		{name: "exists true", params: map[string]any{"hypermetro": true}, want: true, wantErr: nil},
		{name: "exists false", params: map[string]any{"hypermetro": false}, want: false, wantErr: nil},
		{name: "exists not bool type", params: map[string]any{"hypermetro": "true"}, want: false,
			wantErr: fmt.Errorf("parameter hyperMetro [%v] in sc must be bool", "true")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := isHyperMetroFromParams(tt.params)
			require.Equal(t, tt.wantErr, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestNAS_Expand_FilesystemNotFound(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	nas := NewNAS(cli, nil, constants.OceanStorDoradoV6, NASHyperMetro{}, false)

	fsName := "non-existent-fs"
	newSize := int64(1073741824)

	// mock - filesystem not found
	cli.EXPECT().GetFileSystemByName(ctx, fsName).Return(nil, nil)

	// action
	err := nas.Expand(ctx, fsName, newSize)

	// assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Filesystem")
	assert.Contains(t, err.Error(), "does not exist")
}

func TestNAS_Expand_GetFileSystemError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	nas := NewNAS(cli, nil, constants.OceanStorDoradoV6, NASHyperMetro{}, false)

	fsName := "test-fs"
	newSize := int64(1073741824)

	// mock - get filesystem error
	cli.EXPECT().GetFileSystemByName(ctx, fsName).Return(nil, errors.New("get fs error"))

	// action
	err := nas.Expand(ctx, fsName, newSize)

	// assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "get fs error")
}

func TestNAS_CreateSnapshot_FilesystemNotFound(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	nas := NewNAS(cli, nil, constants.OceanStorDoradoV6, NASHyperMetro{}, false)

	fsName := "non-existent-fs"
	snapshotName := "test-snapshot"

	// mock - filesystem not found
	cli.EXPECT().GetFileSystemByName(ctx, fsName).Return(nil, nil)

	// action
	_, err := nas.CreateSnapshot(ctx, fsName, snapshotName)

	// assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Filesystem")
	assert.Contains(t, err.Error(), "does not exist")
}

func TestNAS_CreateSnapshot_GetFileSystemError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	nas := NewNAS(cli, nil, constants.OceanStorDoradoV6, NASHyperMetro{}, false)

	fsName := "test-fs"
	snapshotName := "test-snapshot"

	// mock - get filesystem error
	cli.EXPECT().GetFileSystemByName(ctx, fsName).Return(nil, errors.New("get fs error"))

	// action
	_, err := nas.CreateSnapshot(ctx, fsName, snapshotName)

	// assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "get fs error")
}

func TestNAS_selectSnapshotParent_ConvertFirstThenFallback(t *testing.T) {
	ctx := context.Background()
	parentId := "fs-001"

	tests := []struct {
		name         string
		snapshotName string
		mockSetup    func(cli *mock_client.MockOceanstorClientInterface)
		wantErr      bool
		errContains  string
	}{
		{
			name:         "CSI-created snapshot found with converted name directly",
			snapshotName: "snap-with-dash",
			mockSetup: func(cli *mock_client.MockOceanstorClientInterface) {
				cli.EXPECT().GetFSSnapshotByName(ctx, parentId, "snap_with_dash").
					Return(map[string]any{"ID": "snap-001", "NAME": "snap_with_dash"}, nil)
			},
		},
		{
			name:         "pre-provisioned snapshot found with original name after converted name not found",
			snapshotName: "snap-with-dash",
			mockSetup: func(cli *mock_client.MockOceanstorClientInterface) {
				// Converted name not found on storage
				cli.EXPECT().GetFSSnapshotByName(ctx, parentId, "snap_with_dash").Return(nil, nil)
				// Fallback: found with original name
				cli.EXPECT().GetFSSnapshotByName(ctx, parentId, "snap-with-dash").
					Return(map[string]any{"ID": "snap-001", "NAME": "snap-with-dash"}, nil)
			},
		},
		{
			name:         "snapshot not found with both converted and original name",
			snapshotName: "nonexistent-snap",
			mockSetup: func(cli *mock_client.MockOceanstorClientInterface) {
				cli.EXPECT().GetFSSnapshotByName(ctx, parentId, "nonexistent_snap").Return(nil, nil)
				cli.EXPECT().GetFSSnapshotByName(ctx, parentId, "nonexistent-snap").Return(nil, nil)
			},
			wantErr:     true,
			errContains: "not exists",
		},
		{
			name:         "snapshot name without hyphen found directly",
			snapshotName: "snap_no_dash",
			mockSetup: func(cli *mock_client.MockOceanstorClientInterface) {
				cli.EXPECT().GetFSSnapshotByName(ctx, parentId, "snap_no_dash").
					Return(map[string]any{"ID": "snap-001", "NAME": "snap_no_dash"}, nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
			nas := NewNAS(cli, nil, constants.OceanStorDoradoV6, NASHyperMetro{}, false)

			tt.mockSetup(cli)

			params := map[string]interface{}{
				"sourcesnapshotname": tt.snapshotName,
				"snapshotparentid":   parentId,
				"fromSnapshot":       tt.snapshotName,
			}
			err := nas.selectSnapshotParent(ctx, params)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNAS_preCreate_UnconditionalConversion(t *testing.T) {
	tests := []struct {
		name               string
		product            constants.OceanstorVersion
		pvcName            string
		sourceSnapshotName string
		wantFsName         string
		wantFromSnapshot   string
	}{
		{
			name:               "V5 converts both filesystem name and snapshot name",
			product:            constants.OceanStorV5,
			pvcName:            "pvc-with-dash",
			sourceSnapshotName: "snap-with-dash",
			wantFsName:         "pvc_with_dash",
			wantFromSnapshot:   "snap_with_dash",
		},
		{
			name:               "V6 converts both filesystem name and snapshot name",
			product:            constants.OceanStorDoradoV6,
			pvcName:            "pvc-with-dash",
			sourceSnapshotName: "snap-with-dash",
			wantFsName:         "pvc_with_dash",
			wantFromSnapshot:   "snap_with_dash",
		},
		{
			name:               "V7 converts both filesystem name and snapshot name",
			product:            constants.OceanStorDoradoV7,
			pvcName:            "pvc-with-dash",
			sourceSnapshotName: "snap-with-dash",
			wantFsName:         "pvc_with_dash",
			wantFromSnapshot:   "snap_with_dash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// preCreate unconditionally converts - to _ for all versions
			fsName := utils.GetFileSystemName(tt.pvcName)
			assert.Equal(t, tt.wantFsName, fsName)

			fromSnapshot := utils.GetFSSnapshotName(tt.sourceSnapshotName)
			assert.Equal(t, tt.wantFromSnapshot, fromSnapshot)
		})
	}
}

func TestNAS_preCreate_SourceSnapshotName_V5Converts(t *testing.T) {
	// Arrange
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	nas := NewNAS(cli, nil, constants.OceanStorV5, NASHyperMetro{}, false)
	ctx := context.Background()
	params := map[string]any{
		"authclient":         "*",
		"name":               "pvc-with-dash",
		"pvName":             "pvc-with-dash",
		"storagepool":        "pool001",
		"sourcesnapshotname": "snap-with-dash",
	}

	// Mock - getPoolID needs GetPoolByName
	cli.EXPECT().GetPoolByName(ctx, "pool001").Return(map[string]any{"ID": "pool-001"}, nil)

	// Action
	err := nas.preCreate(ctx, params)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "snap_with_dash", params["fromSnapshot"])
}

func TestNAS_preCreate_SourceSnapshotName_V6ConvertsSnapshotName(t *testing.T) {
	// Arrange
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	nas := NewNAS(cli, nil, constants.OceanStorDoradoV6, NASHyperMetro{}, false)
	ctx := context.Background()
	params := map[string]any{
		"authclient":         "*",
		"name":               "pvc-with-dash",
		"pvName":             "other-name",
		"storagepool":        "pool001",
		"sourcesnapshotname": "snap-with-dash",
	}

	// Mock - getPoolID needs GetPoolByName
	cli.EXPECT().GetPoolByName(ctx, "pool001").Return(map[string]any{"ID": "pool-001"}, nil)

	// Action
	err := nas.preCreate(ctx, params)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "snap_with_dash", params["fromSnapshot"])
}

func TestNAS_DeleteSnapshot_OriginalThenConvertedFallback(t *testing.T) {
	ctx := context.Background()
	parentId := "fs-001"

	tests := []struct {
		name         string
		snapshotName string
		mockSetup    func(cli *mock_client.MockOceanstorClientInterface)
		wantErr      bool
	}{
		{
			name:         "found with original name directly",
			snapshotName: "snap-with-dash",
			mockSetup: func(cli *mock_client.MockOceanstorClientInterface) {
				cli.EXPECT().GetFSSnapshotByName(ctx, parentId, "snap-with-dash").
					Return(map[string]any{"ID": "snap-001", "NAME": "snap-with-dash"}, nil)
				cli.EXPECT().DeleteFSSnapshot(ctx, "snap-001").Return(nil)
			},
		},
		{
			name:         "fallback: original name not found, found with converted name",
			snapshotName: "snap-with-dash",
			mockSetup: func(cli *mock_client.MockOceanstorClientInterface) {
				// Original name not found on storage
				cli.EXPECT().GetFSSnapshotByName(ctx, parentId, "snap-with-dash").Return(nil, nil)
				// Fallback: found with converted name
				cli.EXPECT().GetFSSnapshotByName(ctx, parentId, "snap_with_dash").
					Return(map[string]any{"ID": "snap-001", "NAME": "snap_with_dash"}, nil)
				cli.EXPECT().DeleteFSSnapshot(ctx, "snap-001").Return(nil)
			},
		},
		{
			name:         "snapshot not found with both original and converted name",
			snapshotName: "nonexistent-snap",
			mockSetup: func(cli *mock_client.MockOceanstorClientInterface) {
				cli.EXPECT().GetFSSnapshotByName(ctx, parentId, "nonexistent-snap").Return(nil, nil)
				cli.EXPECT().GetFSSnapshotByName(ctx, parentId, "nonexistent_snap").Return(nil, nil)
			},
			wantErr: false, // Idempotent delete: not found returns nil
		},
		{
			name:         "name without hyphen found directly, no fallback needed",
			snapshotName: "snap_no_dash",
			mockSetup: func(cli *mock_client.MockOceanstorClientInterface) {
				cli.EXPECT().GetFSSnapshotByName(ctx, parentId, "snap_no_dash").
					Return(map[string]any{"ID": "snap-001", "NAME": "snap_no_dash"}, nil)
				cli.EXPECT().DeleteFSSnapshot(ctx, "snap-001").Return(nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
			nas := NewNAS(cli, nil, constants.OceanStorDoradoV6, NASHyperMetro{}, false)

			tt.mockSetup(cli)

			err := nas.DeleteSnapshot(ctx, parentId, tt.snapshotName)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
