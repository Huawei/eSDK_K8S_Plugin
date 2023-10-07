/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2023. All rights reserved.
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
	"os"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/container-storage-interface/spec/lib/go/csi"

	"huawei-csi-driver/connector"
	"huawei-csi-driver/connector/fibrechannel"
	"huawei-csi-driver/connector/iscsi"
	"huawei-csi-driver/connector/nvme"
	"huawei-csi-driver/connector/roce"
	"huawei-csi-driver/csi/app"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

func TestSanManagerStageFileSystemVolume(t *testing.T) {
	tests := []struct {
		name              string
		manager           *SanManager
		connectVolumeFunc func(patch *gomonkey.Patches, conn connector.Connector)
		wantErr           bool
	}{
		{
			name: "TestSanManagerStageIscsiFileSystemVolume",
			manager: &SanManager{
				protocol: "iscsi",
				Conn:     connector.GetConnector(context.Background(), connector.ISCSIDriver),
			},
			connectVolumeFunc: mockConnectIscsiVolume,
		},
		{
			name: "TestSanManagerStageFcFileSystemVolume",
			manager: &SanManager{
				protocol: "fc",
				Conn:     connector.GetConnector(context.Background(), connector.FCDriver),
			},
			connectVolumeFunc: mockConnectFcVolume,
		},
		{
			name: "TestSanManagerStageRoceFileSystemVolume",
			manager: &SanManager{
				protocol: "roce",
				Conn:     connector.GetConnector(context.Background(), connector.RoCEDriver),
			},
			connectVolumeFunc: mockConnectRoceVolume,
		},
		{
			name: "TestSanManagerStageFcNvmeFileSystemVolume",
			manager: &SanManager{
				protocol: "fc-nvme",
				Conn:     connector.GetConnector(context.Background(), connector.FCNVMeDriver),
			},
			connectVolumeFunc: mockConnectFcNvmeVolume,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patches := gomonkey.NewPatches()
			mockClearResidualPath(patches, tt.manager.protocol)
			tt.connectVolumeFunc(patches, tt.manager.Conn)
			mockMountShare(patches)
			mockChmodFsPermission(patches, t)
			request := mockSanStageVolumeRequest(t, "filesystem")

			err := tt.manager.StageVolume(context.Background(), request)
			if err != nil {
				t.Errorf("%s want error = nil, got error = %v", tt.name, err)
			}

			t.Cleanup(func() {
				patches.Reset()
				clearWwnFileGeneratedByTest()
			})
		})
	}
}

func TestSanManagerStageBlockVolume(t *testing.T) {
	manager := &SanManager{
		protocol: "iscsi",
		Conn:     connector.GetConnector(context.Background(), connector.ISCSIDriver),
	}
	patches := gomonkey.NewPatches()
	mockClearResidualPath(patches, manager.protocol)
	mockConnectIscsiVolume(patches, manager.Conn)
	mockCreateSymlink(patches)
	request := mockSanStageVolumeRequest(t, "Block")

	err := manager.StageVolume(context.Background(), request)
	if err != nil {
		t.Errorf("TestSanManagerStageBlockVolume() want error = nil, got error = %v", err)
	}

	t.Cleanup(func() {
		patches.Reset()
		clearWwnFileGeneratedByTest()
	})

}

func mockClearResidualPath(patch *gomonkey.Patches, protocol string) {
	patch.ApplyFunc(connector.ClearResidualPath, func(ctx context.Context,
		lunWWN string, volumeMode interface{}) error {

		wwn := "mock_tgt_lun_wwn_1"
		if protocol == "roce" || protocol == "fc-nvme" {
			wwn = "mock_lun_guid_1"
		}

		if lunWWN == wwn {
			return nil
		}

		return errors.New("clear residual path error")
	})
}

func mockConnectIscsiVolume(patch *gomonkey.Patches, conn connector.Connector) {
	patch.ApplyMethod(reflect.TypeOf(conn), "ConnectVolume",
		func(_ *iscsi.ISCSI, ctx context.Context, params map[string]interface{}) (string, error) {
			want := map[string]interface{}{
				"tgtPortals":         []string{"mock_tgt_portal_1"},
				"tgtIQNs":            []string{"mock_tgt_iqn_1"},
				"tgtHostLUNs":        []string{"mock_host_lun_1"},
				"tgtLunWWN":          "mock_tgt_lun_wwn_1",
				"volumeUseMultiPath": app.GetGlobalConfig().VolumeUseMultiPath,
				"multiPathType":      app.GetGlobalConfig().ScsiMultiPathType,
			}

			if checkTargetMapContainsSourceMap(want, params) {
				return "test_dev_path", nil

			}

			return "", errors.New("connect iscsi volume error")
		})
}

func mockConnectFcVolume(patch *gomonkey.Patches, conn connector.Connector) {
	patch.ApplyMethod(reflect.TypeOf(conn), "ConnectVolume",
		func(_ *fibrechannel.FibreChannel, ctx context.Context, params map[string]interface{}) (string, error) {
			want := map[string]interface{}{
				"tgtWWNs":            []string{"mock_wwn_1"},
				"tgtHostLUNs":        []string{"mock_host_lun_1"},
				"tgtLunWWN":          "mock_tgt_lun_wwn_1",
				"volumeUseMultiPath": app.GetGlobalConfig().VolumeUseMultiPath,
				"multiPathType":      app.GetGlobalConfig().ScsiMultiPathType,
			}

			if checkTargetMapContainsSourceMap(want, params) {
				return "test_dev_path", nil

			}

			return "", errors.New("connect fc volume error")
		})
}

func mockConnectRoceVolume(patch *gomonkey.Patches, conn connector.Connector) {
	patch.ApplyMethod(reflect.TypeOf(conn), "ConnectVolume",
		func(_ *roce.RoCE, ctx context.Context, params map[string]interface{}) (string, error) {
			want := map[string]interface{}{
				"tgtPortals":         []string{"mock_tgt_portal_1"},
				"tgtLunGuid":         "mock_lun_guid_1",
				"volumeUseMultiPath": app.GetGlobalConfig().VolumeUseMultiPath,
				"multiPathType":      app.GetGlobalConfig().ScsiMultiPathType,
			}

			if checkTargetMapContainsSourceMap(want, params) {
				return "test_dev_path", nil

			}

			return "", errors.New("connect roce volume error")
		})
}

func mockConnectFcNvmeVolume(patch *gomonkey.Patches, conn connector.Connector) {
	patch.ApplyMethod(reflect.TypeOf(conn), "ConnectVolume",
		func(_ *nvme.FCNVMe, ctx context.Context, params map[string]interface{}) (string, error) {
			want := map[string]interface{}{
				"tgtLunGuid":         "mock_lun_guid_1",
				"volumeUseMultiPath": app.GetGlobalConfig().VolumeUseMultiPath,
				"multiPathType":      app.GetGlobalConfig().ScsiMultiPathType,
				"portWWNList": []nvme.PortWWNPair{{InitiatorPortWWN: "mock_initiator_port_wwn_1",
					TargetPortWWN: "mock_target_port_wwn_1"},
				},
			}

			if checkTargetMapContainsSourceMap(want, params) {
				return "test_dev_path", nil

			}

			return "", errors.New("connect fc-nvme volume error")
		})
}

func mockMountShare(patch *gomonkey.Patches) {
	patch.ApplyFunc(Mount, func(ctx context.Context, parameters map[string]interface{}) error {
		want := map[string]interface{}{
			"fsType":     "ext4",
			"srcType":    connector.MountBlockType,
			"sourcePath": "test_dev_path",
			"targetPath": "/test_staging_target_path",
			"mountFlags": "bound",
			"accessMode": csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
		}

		if checkTargetMapContainsSourceMap(want, parameters) {
			return nil
		}

		return errors.New("mount share error")
	})
}

func mockChmodFsPermission(patch *gomonkey.Patches, t *testing.T) {
	patch.ApplyFunc(utils.ChmodFsPermission, func(ctx context.Context, targetPath, fsPermission string) {
		if targetPath == "/test_staging_target_path" && fsPermission == "777" {
			return
		}

		t.Errorf("chmod filesystem permisssion error")
	})
}

func mockCreateSymlink(patch *gomonkey.Patches) {
	patch.ApplyFunc(utils.CreateSymlink, func(ctx context.Context, source string, target string) error {
		if source == "test_dev_path" && target == "/test_staging_target_path/test_backend.pvc-san-xxx" {
			return nil
		}

		return errors.New("create system link error")
	})
}

func mockSanStageVolumeRequest(t *testing.T, volumeType string) *csi.NodeStageVolumeRequest {
	publishInfo := &ControllerPublishInfo{
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

	jsonMockInfo, err := json.Marshal(publishInfo)
	if err != nil {
		t.Errorf("mock node stage volume request failed, error: %v", err)
	}

	volumeCapability := &csi.VolumeCapability{
		AccessType: &csi.VolumeCapability_Mount{
			Mount: &csi.VolumeCapability_MountVolume{
				FsType:     "ext4",
				MountFlags: []string{"bound"},
			},
		}}

	if volumeType == "Block" {
		volumeCapability = &csi.VolumeCapability{
			AccessType: &csi.VolumeCapability_Block{},
		}
	}

	return &csi.NodeStageVolumeRequest{
		StagingTargetPath: "/test_staging_target_path",
		VolumeId:          "test_backend.pvc-san-xxx",
		PublishContext:    map[string]string{"publishInfo": string(jsonMockInfo)},
		VolumeContext:     map[string]string{"fsPermission": "777"},
		VolumeCapability:  volumeCapability,
	}
}

// checkTargetMapContainsSourceMap is a helper function called from multiple test cases
func checkTargetMapContainsSourceMap(source, target map[string]interface{}) bool {
	for key, sourceValue := range source {
		targetValue, exist := target[key]
		if !exist && !reflect.DeepEqual(sourceValue, targetValue) {
			return false
		}
	}
	return true
}

func clearWwnFileGeneratedByTest() {
	err := os.RemoveAll("/csi/disks/test_backend.pvc-san-xxx.wwn")
	if err != nil {
		log.Errorln("clear wwn file generated by test failed")
		return
	}
}
