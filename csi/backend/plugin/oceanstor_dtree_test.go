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

// Package plugin provide storage function
package plugin

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/volume"
)

func Test_OceanstorDTreePlugin_AttachVolume_Scenario(t *testing.T) {
	// arrange
	mockRes := map[string]any{
		"key": "value",
	}
	wantErr := fmt.Errorf("attach error")

	// mock
	patches := gomonkey.ApplyFuncReturn(attachDTreeVolume, mockRes, wantErr)
	defer patches.Reset()

	// action
	gotRes, gotErr := (&OceanstorDTreePlugin{}).AttachVolume(context.Background(), "test-volume", nil)

	// assert
	assert.Nil(t, gotRes)
	assert.EqualError(t, gotErr, wantErr.Error())
}

func Test_OceanstorDTreePlugin_AttachVolume_WithNFSDisabled(t *testing.T) {
	// arrange
	mockRes := map[string]any{
		"key": "value",
	}
	p := &OceanstorDTreePlugin{
		nfsAutoAuthClient: &NfsAutoAuthClient{
			Enabled: false,
		},
	}

	// mock
	patches := gomonkey.ApplyFuncReturn(attachDTreeVolume, mockRes, nil)
	defer patches.Reset()

	// action
	gotRes, gotErr := p.AttachVolume(context.Background(), "test-volume", nil)

	// assert
	assert.Equal(t, mockRes, gotRes)
	assert.NoError(t, gotErr)
}

func Test_OceanstorDTreePlugin_AttachVolume_WithParentNameFromResult(t *testing.T) {
	// arrange
	mockRes := map[string]any{
		constants.DTreeParentKey: "parent-from-result",
	}
	p := &OceanstorDTreePlugin{
		nfsAutoAuthClient: &NfsAutoAuthClient{
			Enabled: true,
		},
	}

	// mock
	patches := gomonkey.ApplyFuncReturn(attachDTreeVolume, mockRes, nil).
		ApplyFuncReturn(getFilteredIPs, []string{"192.168.1.1"}, nil).
		ApplyMethodReturn(&volume.DTree{}, "AutoManageAuthClient", nil)
	defer patches.Reset()

	// action
	gotRes, gotErr := p.AttachVolume(context.Background(), "test-volume", nil)

	// assert
	assert.Equal(t, mockRes, gotRes)
	assert.NoError(t, gotErr)
}

func Test_OceanstorDTreePlugin_AttachVolume_WithParentNameFromPlugin(t *testing.T) {
	// arrange
	mockRes := map[string]any{}
	p := &OceanstorDTreePlugin{
		nfsAutoAuthClient: &NfsAutoAuthClient{
			Enabled: true,
		},
		parentName: "parent-from-plugin",
	}

	// mock
	patches := gomonkey.ApplyFuncReturn(attachDTreeVolume, mockRes, nil).
		ApplyFuncReturn(getFilteredIPs, []string{"192.168.1.1"}, nil).
		ApplyMethodReturn(&volume.DTree{}, "AutoManageAuthClient", nil)
	defer patches.Reset()

	// action
	gotRes, gotErr := p.AttachVolume(context.Background(), "test-volume", nil)

	// assert
	assert.Equal(t, mockRes, gotRes)
	assert.NoError(t, gotErr)
}

func Test_OceanstorDTreePlugin_AttachVolume_WithEmptyParentName(t *testing.T) {
	// arrange
	mockRes := map[string]any{}
	p := &OceanstorDTreePlugin{
		nfsAutoAuthClient: &NfsAutoAuthClient{
			Enabled: true,
		},
		parentName: "",
	}

	// mock
	patches := gomonkey.ApplyFuncReturn(attachDTreeVolume, mockRes, nil)
	defer patches.Reset()

	// action
	gotRes, gotErr := p.AttachVolume(context.Background(), "test-volume", nil)

	// assert
	assert.ErrorContains(t, gotErr, "failed to get parent name")
	assert.Nil(t, gotRes)
}

func Test_OceanstorDTreePlugin_AttachVolume_GetFilteredIPsError(t *testing.T) {
	// arrange
	mockRes := map[string]any{
		constants.DTreeParentKey: "parent-name",
	}
	wantErr := fmt.Errorf("get filtered IPs error")
	p := &OceanstorDTreePlugin{
		nfsAutoAuthClient: &NfsAutoAuthClient{
			Enabled: true,
		},
	}

	// mock
	patches := gomonkey.ApplyFuncReturn(attachDTreeVolume, mockRes, nil)
	patches.ApplyFuncReturn(getFilteredIPs, []string(nil), wantErr)
	defer patches.Reset()

	// action
	gotRes, gotErr := p.AttachVolume(context.Background(), "test-volume", nil)

	// assert
	assert.ErrorContains(t, gotErr, "failed to attach volume")
	assert.Nil(t, gotRes)
}

func Test_OceanstorDTreePlugin_DetachVolume_Scenario(t *testing.T) {
	// arrange
	p := &OceanstorDTreePlugin{
		nfsAutoAuthClient: &NfsAutoAuthClient{
			Enabled: false,
		},
	}

	// action
	gotErr := p.DetachVolume(context.Background(), "test-volume", nil)

	// assert
	assert.NoError(t, gotErr)
}

func Test_OceanstorDTreePlugin_DetachVolume_MissingParentName(t *testing.T) {
	// arrange
	p := &OceanstorDTreePlugin{
		nfsAutoAuthClient: &NfsAutoAuthClient{
			Enabled: true,
		},
	}

	// action
	gotErr := p.DetachVolume(context.Background(), "test-volume", nil)

	// assert
	assert.ErrorContains(t, gotErr, "failed to get parent name")
}

func Test_OceanstorDTreePlugin_DetachVolume_GetFilteredIPsError(t *testing.T) {
	// arrange
	p := &OceanstorDTreePlugin{
		nfsAutoAuthClient: &NfsAutoAuthClient{
			Enabled: true,
		},
	}
	parameters := map[string]interface{}{
		constants.DTreeParentKey: "parent-name",
	}
	wantErr := fmt.Errorf("get filtered IPs error")

	// mock
	patches := gomonkey.ApplyFuncReturn(getFilteredIPs, []string(nil), wantErr)
	defer patches.Reset()

	// action
	gotErr := p.DetachVolume(context.Background(), "test-volume", parameters)

	// assert
	assert.ErrorContains(t, gotErr, "failed to detach volume")
}

func Test_OceanstorDTreePlugin_DetachVolume_IOIsolation(t *testing.T) {
	// arrange
	p := &OceanstorDTreePlugin{
		nfsAutoAuthClient: &NfsAutoAuthClient{
			Enabled: true,
		},
	}
	parameters := map[string]interface{}{
		"IOIsolation":            true,
		constants.DTreeParentKey: "parent-name",
	}

	// mock
	patches := gomonkey.ApplyFuncReturn(getFilteredIPs, []string{"192.168.1.1"}, nil).
		ApplyMethodReturn(&volume.DTree{}, "AutoManageAuthClient", nil).
		ApplyMethodReturn(&volume.DTree{}, "CheckAllClientsStatus", nil)
	defer patches.Reset()

	// action
	gotErr := p.DetachVolume(context.Background(), "test-volume", parameters)

	// assert
	assert.NoError(t, gotErr)
}

func Test_OceanstorDTreePlugin_DetachVolume_AutoManageAuthClientError(t *testing.T) {
	// arrange
	p := &OceanstorDTreePlugin{
		nfsAutoAuthClient: &NfsAutoAuthClient{
			Enabled: true,
		},
	}
	parameters := map[string]interface{}{
		constants.DTreeParentKey: "parent-name",
	}
	wantErr := errors.New("fake error")

	// mock
	patches := gomonkey.ApplyFuncReturn(getFilteredIPs, []string{"192.168.1.1"}, nil).
		ApplyMethodReturn(&volume.DTree{}, "AutoManageAuthClient", wantErr)
	defer patches.Reset()

	// action
	gotErr := p.DetachVolume(context.Background(), "test-volume", parameters)

	// assert
	assert.ErrorIs(t, gotErr, wantErr)
}

func Test_OceanstorDTreePlugin_DetachVolume_CheckAllClientsStatusError(t *testing.T) {
	// arrange
	p := &OceanstorDTreePlugin{
		nfsAutoAuthClient: &NfsAutoAuthClient{
			Enabled: true,
		},
	}
	parameters := map[string]interface{}{
		"IOIsolation":            true,
		constants.DTreeParentKey: "parent-name",
	}
	wantErr := errors.New("fake error")

	// mock
	patches := gomonkey.ApplyFuncReturn(getFilteredIPs, []string{"192.168.1.1"}, nil).
		ApplyMethodReturn(&volume.DTree{}, "AutoManageAuthClient", nil).
		ApplyMethodReturn(&volume.DTree{}, "CheckAllClientsStatus", wantErr)
	defer patches.Reset()

	// action
	gotErr := p.DetachVolume(context.Background(), "test-volume", parameters)

	// assert
	assert.ErrorIs(t, gotErr, wantErr)
}
