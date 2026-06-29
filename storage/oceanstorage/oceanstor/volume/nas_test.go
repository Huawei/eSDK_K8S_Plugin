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
