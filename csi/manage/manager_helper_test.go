/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2025. All rights reserved.
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

package manage

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/assert"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/connector"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/connector/nvme"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	cfg "github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app/config"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/plugin"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	logName = "managerHelperTest.log"
)

type testCaseStructForNewManager struct {
	name        string
	protocol    string
	backendName string
	want        VolumeManager
	wantErr     bool
}

func TestMain(m *testing.M) {
	getGlobalConfig := gostub.StubFunc(&app.GetGlobalConfig, cfg.MockCompletedConfig())
	defer getGlobalConfig.Reset()

	log.MockInitLogging(logName)
	defer log.MockStopLogging(logName)

	m.Run()
}

func mockControllerPublishInfo() *ControllerPublishInfo {
	return &ControllerPublishInfo{
		TgtLunWWN:          "mock_tgt_lun_wwn_1",
		TgtPortals:         []string{"mock_tgt_portal_1"},
		TgtIQNs:            []string{"mock_tgt_iqn_1"},
		TgtHostLUNs:        []string{"mock_host_lun_1"},
		TgtLunGuid:         "mock_lun_guid_1",
		TgtWWNs:            []string{"mock_wwn_1"},
		VolumeUseMultiPath: true,
		MultiPathType:      "mock_type_1",
		PortWWNList: []nvme.PortWWNPair{
			{InitiatorPortWWN: "mock_initiator_port_wwn_1", TargetPortWWN: "mock_target_port_wwn_1"},
		},
	}
}

func mockNodeStageVolumeRequest() (*csi.NodeStageVolumeRequest, error) {
	testControllerPublishInfo := mockControllerPublishInfo()

	jsonMockInfo, err := json.Marshal(testControllerPublishInfo)
	if err != nil {
		log.Errorf("mock node stage volume request failed, error: %v", err)
		return nil, err
	}

	return &csi.NodeStageVolumeRequest{
		PublishContext: map[string]string{"publishInfo": string(jsonMockInfo)},
	}, nil
}

func mockParametersWithPublishInfo() map[string]interface{} {
	parameters := map[string]interface{}{}
	parameters["publishInfo"] = mockControllerPublishInfo()
	return parameters
}

func TestWithControllerPublishInfo(t *testing.T) {
	request, err := mockNodeStageVolumeRequest()
	if err != nil {
		t.Errorf("TestWithControllerPublishInfo() mock node stage volume request failed, "+
			"want error = nil, got error = %v", err)
		return
	}

	requestParam := map[string]interface{}{}
	if err = WithControllerPublishInfo(context.Background(), request)(requestParam); err != nil {
		t.Errorf("TestWithControllerPublishInfo() build parameters failed, "+
			"want error = nil, got error = %v", err)
		return
	}

	wantParameters := mockParametersWithPublishInfo()
	equal := reflect.DeepEqual(requestParam, wantParameters)
	if !equal {
		t.Errorf("TestWithControllerPublishInfo() want params = %v, got params = %v",
			wantParameters, requestParam)
	}
}

func TestWithMultiPathType(t *testing.T) {
	tests := []struct {
		name              string
		protocol          string
		wantMultiPathType string
	}{
		{
			name:              "test_multiPath_type_for_fc",
			protocol:          "fc",
			wantMultiPathType: app.GetGlobalConfig().ScsiMultiPathType,
		},
		{
			name:              "test_multiPath_type_for_iscsi",
			protocol:          "iscsi",
			wantMultiPathType: app.GetGlobalConfig().ScsiMultiPathType,
		},
		{
			name:              "test_multiPath_type_for_roce",
			protocol:          "roce",
			wantMultiPathType: app.GetGlobalConfig().NvmeMultiPathType,
		},
		{
			name:              "test_multiPath_type_for_fc_nvme",
			protocol:          "fc-nvme",
			wantMultiPathType: app.GetGlobalConfig().NvmeMultiPathType,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parameters := mockParametersWithPublishInfo()
			if err := WithMultiPathType(tt.protocol)(parameters); err != nil {
				t.Errorf("WithMultiPathType() want error = nil, got error = %v", err)
				return
			}

			publishInfo, exist := parameters["publishInfo"].(*ControllerPublishInfo)
			if !exist {
				t.Errorf("WithMultiPathType() not found publishInfo")
				return
			}

			if publishInfo.MultiPathType != tt.wantMultiPathType {
				t.Errorf("WithMultiPathType() want mutilPathType = %v, got mutilPathType = %v",
					tt.wantMultiPathType, publishInfo.MultiPathType)
			}
		})
	}
}

func TestExtractWwn(t *testing.T) {
	tests := []struct {
		name     string
		protocol string
		want     string
	}{
		{
			name:     "test_extract_wwn_for_fc",
			protocol: "fc",
			want:     "mock_tgt_lun_wwn_1",
		},
		{
			name:     "test_extract_wwn_for_iscsi",
			protocol: "iscsi",
			want:     "mock_tgt_lun_wwn_1",
		},
		{
			name:     "test_extract_wwn_for_roce",
			protocol: "roce",
			want:     "mock_lun_guid_1",
		},
		{
			name:     "test_extract_wwn_for_fc_nvme",
			protocol: "fc-nvme",
			want:     "mock_lun_guid_1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parameters := mockParametersWithPublishInfo()
			parameters["protocol"] = tt.protocol

			got, err := ExtractWwn(parameters)
			if err != nil {
				t.Errorf("ExtractWwn() want error = nil, got error = %v", err)
				return
			}

			if got != tt.want {
				t.Errorf("ExtractWwn() want wwn = %v, got wwn = %s", tt.want, got)
			}
		})
	}
}

func TestControllerPublishInfoReflectToMap(t *testing.T) {
	want := map[string]interface{}{
		"tgtLunWWN":          "mock_tgt_lun_wwn_1",
		"tgtPortals":         []string{"mock_tgt_portal_1"},
		"tgtIQNs":            []string{"mock_tgt_iqn_1"},
		"tgtHostLUNs":        []string{"mock_host_lun_1"},
		"tgtLunGuid":         "mock_lun_guid_1",
		"tgtWWNs":            []string{"mock_wwn_1"},
		"volumeUseMultiPath": true,
		"multiPathType":      "mock_type_1",
		"portWWNList": []nvme.PortWWNPair{
			{InitiatorPortWWN: "mock_initiator_port_wwn_1", TargetPortWWN: "mock_target_port_wwn_1"},
		},
	}

	if got := mockControllerPublishInfo().ReflectToMap(); !reflect.DeepEqual(got, want) {
		t.Errorf("ReflectToMap() want map = %v, got map = %v", want, got)
	}
}

func TestWithBlockVolumeCapability(t *testing.T) {
	request := &csi.NodeStageVolumeRequest{
		StagingTargetPath: "test_staging_target_path",
		VolumeId:          "test_volume_id",
		VolumeCapability: &csi.VolumeCapability{
			AccessType: &csi.VolumeCapability_Block{},
		},
	}

	requestParams := map[string]interface{}{}
	if err := WithVolumeCapability(context.Background(), request)(requestParams); err != nil {
		t.Errorf("TestWithBlockVolumeCapability() want error = nil, got error = %v", err)
		return
	}

	wantParams := map[string]interface{}{
		"stagingPath": "test_staging_target_path/test_volume_id",
		"volumeMode":  "Block",
	}

	if reflect.DeepEqual(requestParams, wantParams) {
		t.Errorf("TestWithBlockVolumeCapability()  want params = %v, got params = %v",
			wantParams, requestParams)
	}
}

func TestWithMountVolumeCapability(t *testing.T) {
	request := &csi.NodeStageVolumeRequest{
		StagingTargetPath: "test_staging_target_path",
		VolumeId:          "test_volume_id",
		VolumeContext:     map[string]string{"fsPermission": "777"},
		VolumeCapability: &csi.VolumeCapability{
			AccessType: &csi.VolumeCapability_Mount{
				Mount: &csi.VolumeCapability_MountVolume{
					FsType:     "ext4",
					MountFlags: []string{"bound"},
				},
			},
			AccessMode: &csi.VolumeCapability_AccessMode{
				Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			},
		},
	}

	requestParams := map[string]interface{}{}
	if err := WithVolumeCapability(context.Background(), request)(requestParams); err != nil {
		t.Errorf("TestWithBlockVolumeCapability() want error = nil, got error = %v", err)
		return
	}

	wantParams := map[string]interface{}{
		"targetPath":   request.GetStagingTargetPath(),
		"fsType":       request.GetVolumeCapability().GetMount().GetFsType(),
		"accessMode":   request.GetVolumeCapability().GetAccessMode().GetMode(),
		"fsPermission": "777",
		"mountFlags":   strings.Join(request.GetVolumeCapability().GetMount().GetMountFlags(), ","),
	}

	if reflect.DeepEqual(requestParams, wantParams) {
		t.Errorf("TestWithMountVolumeCapability()  want params = %v, got params = %v", wantParams, requestParams)
	}
}

func TestNewManagerForNfs(t *testing.T) {
	testCase := testCaseStructForNewManager{
		name:        "test_new_manager_for_nfs",
		protocol:    "nfs",
		backendName: "test_backend_name",
		want: &NasManager{
			protocol:     "nfs",
			portals:      []string{"127.0.0.1"},
			metroPortals: []string{},
			Conn:         connector.GetConnector(context.Background(), connector.NFSDriver),
		},
		wantErr: false,
	}
	newManagerTest(t, testCase)
}

func TestNewManagerForDpc(t *testing.T) {
	testCase := testCaseStructForNewManager{
		name:        "test_new_manager_for_dpc",
		protocol:    "dpc",
		backendName: "test_backend_name",
		want: &NasManager{
			protocol:     "dpc",
			Conn:         connector.GetConnector(context.Background(), connector.NFSDriver),
			portals:      []string{},
			metroPortals: []string{},
		},
		wantErr: false,
	}
	newManagerTest(t, testCase)
}

func TestNewManagerForIscsi(t *testing.T) {
	testCase := testCaseStructForNewManager{
		name:        "test_new_manager_for_iscsi",
		protocol:    "iscsi",
		backendName: "test_backend_name",
		want: &SanManager{
			protocol: "iscsi",
			Conn:     connector.GetConnector(context.Background(), connector.ISCSIDriver),
		},
		wantErr: false,
	}
	newManagerTest(t, testCase)
}

func TestNewManagerForFc(t *testing.T) {
	testCase := testCaseStructForNewManager{
		name:        "test_new_manager_for_fc",
		protocol:    "fc",
		backendName: "test_backend_name",
		want: &SanManager{
			protocol: "fc",
			Conn:     connector.GetConnector(context.Background(), connector.FCDriver),
		},
		wantErr: false,
	}
	newManagerTest(t, testCase)
}

func TestNewManagerForRoce(t *testing.T) {
	testCase := testCaseStructForNewManager{
		name:        "test_new_manager_for_roce",
		protocol:    "roce",
		backendName: "test_backend_name",
		want: &SanManager{
			protocol: "roce",
			Conn:     connector.GetConnector(context.Background(), connector.RoCEDriver),
		},
		wantErr: false,
	}
	newManagerTest(t, testCase)
}

func TestNewManagerForFcNvme(t *testing.T) {
	testCase := testCaseStructForNewManager{
		name:        "test_new_manager_for_fc_nvme",
		protocol:    "fc-nvme",
		backendName: "test_backend_name",
		want: &SanManager{
			protocol: "fc-nvme",
			Conn:     connector.GetConnector(context.Background(), connector.FCNVMeDriver),
		},
		wantErr: false,
	}
	newManagerTest(t, testCase)
}

func TestNewManagerForScsi(t *testing.T) {
	testCase := testCaseStructForNewManager{
		name:        "test_new_manager_for_scsi",
		protocol:    "scsi",
		backendName: "test_backend_name",
		want: &SanManager{
			protocol: "scsi",
			Conn:     connector.GetConnector(context.Background(), connector.LocalDriver),
		},
		wantErr: false,
	}
	newManagerTest(t, testCase)
}

func TestNewManagerAndBackendNotExist(t *testing.T) {
	testCase := testCaseStructForNewManager{
		name:        "test_new_manager_and_backend_not_exist",
		backendName: "test_backend_name1",
		want:        nil,
		wantErr:     true,
	}
	newManagerTest(t, testCase)
}

func TestNewManagerAndProtocolNotExist(t *testing.T) {
	testCase := testCaseStructForNewManager{
		name:        "test_new_manager_and_protocol_not_exist",
		protocol:    "not_exist_protocol",
		backendName: "test_backend_name",
		want:        nil,
		wantErr:     true,
	}
	newManagerTest(t, testCase)
}

// newManagerTest is a helper function called from multiple test cases
func newManagerTest(t *testing.T, testCase testCaseStructForNewManager) {
	getBackendConfig := gomonkey.ApplyFunc(GetBackendConfig,
		func(ctx context.Context, backendName string) (*BackendConfig, error) {
			if backendName != "test_backend_name" {
				return nil, errors.New("not found backend")
			}

			var portals []string
			if testCase.protocol == "nfs" {
				portals = []string{"127.0.0.1"}
			}
			return &BackendConfig{protocol: testCase.protocol, portals: portals, metroPortals: []string{}}, nil
		})
	defer getBackendConfig.Reset()

	got, err := NewManager(context.Background(), testCase.backendName)
	if (err != nil) != testCase.wantErr {
		t.Errorf("NewManager() want error status = %v, got error = %v", testCase.wantErr, err)
		return
	}

	if !reflect.DeepEqual(got, testCase.want) {
		t.Errorf("NewManager() want manager = %+v, got manager = %+v", testCase.want, got)
	}
}

func Test_generatePathPrefixByProtocol(t *testing.T) {
	// arrange
	portals := []struct {
		name           string
		protocol       string
		portals        []string
		wantPathPrefix string
	}{
		{
			name:           "NFS IPv4 test",
			protocol:       plugin.ProtocolNfs,
			portals:        []string{"127.0.0.1"},
			wantPathPrefix: "127.0.0.1:/",
		},
		{
			name:           "NFS IPV6 test",
			protocol:       plugin.ProtocolNfsPlus,
			portals:        []string{"127::1"},
			wantPathPrefix: "[127::1]:/",
		},
		{
			name:           "NFS domain name test",
			protocol:       plugin.ProtocolNfs,
			portals:        []string{"domain_name"},
			wantPathPrefix: "domain_name:/",
		},
		{
			name:           "DPC test",
			protocol:       plugin.ProtocolDpc,
			portals:        nil,
			wantPathPrefix: "/",
		},
	}

	for _, tt := range portals {
		t.Run(tt.name, func(t *testing.T) {
			// action
			gotPathPrefix, gotErr := generatePathPrefixByProtocol(tt.protocol, tt.portals)
			assert.Equal(t, gotErr, nil, "failed to generate path prefix, error: %v", gotErr)
			// assert
			if gotPathPrefix != tt.wantPathPrefix {
				t.Errorf("Test_generatePathPrefixByProtocol() failed, "+
					"gotPathPrefix = %v, wantPathPrefix = %v", gotPathPrefix, tt.wantPathPrefix)
			}
		})
	}
}

func Test_generatePathPrefixByProtocol_WithErrors(t *testing.T) {
	// arrange
	portals := []struct {
		name     string
		protocol string
		portals  []string
		wantErr  string
	}{
		{
			name:     "nil portal test",
			protocol: plugin.ProtocolNfs,
			portals:  nil,
			wantErr:  "no portal provided for NFS or NFS+ protocol",
		},
		{
			name:     "invalid ip test",
			protocol: plugin.ProtocolNfsPlus,
			portals:  []string{""},
			wantErr:  "is invalid",
		},
		{
			name:     "nil protocol test",
			protocol: "",
			portals:  []string{"127::1"},
			wantErr:  "protocol [] is not supported",
		},
	}

	for _, tt := range portals {
		t.Run(tt.name, func(t *testing.T) {
			// action
			_, gotErr := generatePathPrefixByProtocol(tt.protocol, tt.portals)
			// assert
			assert.ErrorContains(t, gotErr, tt.wantErr, "Test_generatePathPrefixByProtocol_WithErrors() "+
				"failed, gotErr = %v, wantErr = %v", gotErr, tt.wantErr)
		})
	}
}

func Test_getDTreeSourcePath(t *testing.T) {
	// arrange
	backendConfigs := []struct {
		name            string
		bk              BackendConfig
		wantErrContains string
		wantSourcePath  string
	}{
		{
			name: "success test",
			bk: BackendConfig{
				protocol:        plugin.ProtocolNfs,
				portals:         []string{"127.0.0.1"},
				dTreeParentName: "parentName",
			},
			wantErrContains: "",
			wantSourcePath:  "127.0.0.1:/parentName/volume",
		},
		{
			name: "error test",
			bk: BackendConfig{
				protocol:        plugin.ProtocolNfs,
				portals:         nil,
				dTreeParentName: "parentName",
			},
			wantErrContains: "generate dtree path prefix failed",
			wantSourcePath:  "",
		},
	}
	req := &csi.NodePublishVolumeRequest{}

	// mock
	mock := gomonkey.ApplyMethodReturn(req, "GetPublishContext", map[string]string{})

	for _, tt := range backendConfigs {
		t.Run(tt.name, func(t *testing.T) {
			// action
			gotSourcePath, gotErr := getDTreeSourcePath(&tt.bk, req, "volume")
			// assert
			if tt.wantErrContains == "" {
				assert.NoError(t, gotErr)
				assert.Equal(t, tt.wantSourcePath, gotSourcePath)
			} else {
				assert.ErrorContains(t, gotErr, tt.wantErrContains)
			}
		})
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func Test_GetBackendConfig_BackendConfigMapError(t *testing.T) {
	// arrange
	ctx := context.Background()
	backendName := "test_backend"
	wantErr := "mock config error"

	// mock
	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyFunc(getBackendConfigMap, func(_ context.Context, _ string) (map[string]interface{}, error) {
		return nil, errors.New(wantErr)
	})

	// act
	gotCfg, gotErr := GetBackendConfig(ctx, backendName)

	// assert
	assert.ErrorContains(t, gotErr, wantErr)
	assert.Nil(t, gotCfg)
}

func Test_GetBackendConfig_InvalidParameters(t *testing.T) {
	// arrange
	ctx := context.Background()
	backendName := "invalid_params_backend"
	wantErr := "get backend info"

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFunc(getBackendConfigMap, func(_ context.Context, _ string) (map[string]interface{}, error) {
		return map[string]interface{}{"invalid": "data"}, nil
	})

	// act
	gotCfg, gotErr := GetBackendConfig(ctx, backendName)

	// assert
	assert.ErrorContains(t, gotErr, wantErr)
	assert.Nil(t, gotCfg)
}

func Test_GetBackendConfig_MissingProtocol(t *testing.T) {
	// arrange
	ctx := context.Background()
	backendName := "missing_protocol_backend"
	wantErr := "protocol in parameters"

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFunc(getBackendConfigMap, func(_ context.Context, _ string) (map[string]interface{}, error) {
		return map[string]interface{}{"parameters": map[string]interface{}{}}, nil
	})

	// act
	gotCfg, gotErr := GetBackendConfig(ctx, backendName)

	// assert
	assert.ErrorContains(t, gotErr, wantErr)
	assert.Nil(t, gotCfg)
}

func Test_GetBackendConfig_DtfsMissingDeviceWWN(t *testing.T) {
	// arrange
	ctx := context.Background()
	backendName := "dtfs_backend"
	wantErr := "get empty DeviceWWN"

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFunc(getBackendConfigMap, func(_ context.Context, _ string) (map[string]interface{}, error) {
		return map[string]interface{}{
			"parameters": map[string]interface{}{
				"protocol": constants.ProtocolDtfs,
			},
		}, nil
	}).ApplyFuncReturn(utils.GetSBCTSpecificationByClaim, map[string]string{}, nil)

	// act
	gotCfg, gotErr := GetBackendConfig(ctx, backendName)

	// assert
	assert.ErrorContains(t, gotErr, wantErr)
	assert.Nil(t, gotCfg)
}

func Test_GetBackendConfig_NfsInvalidPortalCount(t *testing.T) {
	// arrange
	ctx := context.Background()
	backendName := "nfs_backend"
	wantErr := "just support one portal"

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFunc(getBackendConfigMap, func(_ context.Context, _ string) (map[string]interface{}, error) {
		return map[string]interface{}{
			"parameters": map[string]interface{}{
				"protocol": constants.ProtocolNfs,
				"portals":  []interface{}{"portal1", "portal2"},
			},
		}, nil
	})

	// act
	gotCfg, gotErr := GetBackendConfig(ctx, backendName)

	// assert
	assert.ErrorContains(t, gotErr, wantErr)
	assert.Nil(t, gotCfg)
}

func Test_GetBackendConfig_MetroBackendError(t *testing.T) {
	// arrange
	ctx := context.Background()
	backendName := "metro_backend"
	wantErr := "metro backend error"

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFuncSeq(getBackendConfigMap, []gomonkey.OutputCell{
		{Values: gomonkey.Params{
			map[string]interface{}{
				"parameters": map[string]interface{}{
					"protocol": constants.ProtocolNfsPlus,
					"portals":  []interface{}{"portal1"},
				},
				"metroBackend": "invalid_metro",
			}, nil,
		}},
		{Values: gomonkey.Params{nil, errors.New("metro backend error")}},
	})

	// act
	gotCfg, gotErr := GetBackendConfig(ctx, backendName)

	// assert
	assert.ErrorContains(t, gotErr, wantErr)
	assert.Nil(t, gotCfg)
}

func Test_GetBackendConfig_InvalidStorage(t *testing.T) {
	// arrange
	ctx := context.Background()
	backendName := "invalid_storage_backend"
	wantErr := "storage in parameters"

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFunc(getBackendConfigMap, func(_ context.Context, _ string) (map[string]interface{}, error) {
		return map[string]interface{}{
			"parameters": map[string]interface{}{
				"protocol": constants.ProtocolNfs,
				"portals":  []interface{}{"portal1"},
			},
		}, nil
	})

	// act
	gotCfg, gotErr := GetBackendConfig(ctx, backendName)

	// assert
	assert.ErrorContains(t, gotErr, wantErr)
	assert.Nil(t, gotCfg)
}

func Test_GetBackendConfig_SuccessNfsProtocol(t *testing.T) {
	// arrange
	ctx := context.Background()
	backendName := "success_nfs_backend"
	expectedStorage := "nfs_storage1"
	expectedPortal := []string{"nfs-portal.example.com"}
	wantCfg := &BackendConfig{
		storage:  expectedStorage,
		protocol: constants.ProtocolNfs,
		portals:  expectedPortal,
	}

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFunc(getBackendConfigMap, func(_ context.Context, _ string) (map[string]interface{}, error) {
		return map[string]interface{}{
			"storage": expectedStorage,
			"parameters": map[string]interface{}{
				"protocol": constants.ProtocolNfs,
				"portals":  []interface{}{expectedPortal[0]},
			},
		}, nil
	})

	// act
	gotCfg, gotErr := GetBackendConfig(ctx, backendName)

	// assert
	assert.NoError(t, gotErr)
	assert.Equal(t, wantCfg.storage, gotCfg.storage)
	assert.Equal(t, wantCfg.protocol, gotCfg.protocol)
	assert.Equal(t, wantCfg.portals, gotCfg.portals)
}

func Test_GetBackendConfig_SuccessNfsPlusWithMetro(t *testing.T) {
	// arrange
	ctx := context.Background()
	backendName := "success_metro_backend"
	expectedStorage := "metro_storage"
	expectedPortals := []string{"primary-portal"}
	expectedMetroPortals := []string{"metro-portal"}
	wantCfg := &BackendConfig{
		storage:      expectedStorage,
		protocol:     constants.ProtocolNfsPlus,
		portals:      expectedPortals,
		metroPortals: expectedMetroPortals,
	}

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFuncSeq(getBackendConfigMap, []gomonkey.OutputCell{
		{ // Main backend
			Values: gomonkey.Params{
				map[string]interface{}{
					"storage": expectedStorage,
					"parameters": map[string]interface{}{
						"protocol": constants.ProtocolNfsPlus,
						"portals":  []interface{}{expectedPortals[0]},
					},
					"metroBackend": "metro_backend",
				}, nil,
			},
		},
		{ // Metro backend
			Values: gomonkey.Params{
				map[string]interface{}{
					"parameters": map[string]interface{}{
						"portals": []interface{}{expectedMetroPortals[0]},
					},
				}, nil,
			},
		},
	})

	// act
	gotCfg, gotErr := GetBackendConfig(ctx, backendName)

	// assert
	assert.NoError(t, gotErr)
	assert.Equal(t, wantCfg.storage, gotCfg.storage)
	assert.Equal(t, wantCfg.portals, gotCfg.portals)
	assert.Equal(t, wantCfg.metroPortals, gotCfg.metroPortals)
}

func Test_GetBackendConfig_SuccessDtfsProtocol(t *testing.T) {
	// arrange
	ctx := context.Background()
	backendName := "success_dtfs_backend"
	expectedStorage := "dtfs_storage"
	expectedWWN := "wwn-123456"
	wantCfg := &BackendConfig{
		storage:   expectedStorage,
		protocol:  constants.ProtocolDtfs,
		deviceWWN: expectedWWN,
	}

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFunc(getBackendConfigMap, func(_ context.Context, _ string) (map[string]interface{}, error) {
		return map[string]interface{}{
			"storage": expectedStorage,
			"parameters": map[string]interface{}{
				"protocol": constants.ProtocolDtfs,
			},
		}, nil
	}).ApplyFuncReturn(utils.GetSBCTSpecificationByClaim, map[string]string{"DeviceWWN": expectedWWN}, nil)

	// act
	gotCfg, gotErr := GetBackendConfig(ctx, backendName)

	// assert
	assert.NoError(t, gotErr)
	assert.Equal(t, wantCfg.protocol, gotCfg.protocol)
	assert.Equal(t, wantCfg.deviceWWN, gotCfg.deviceWWN)
}
