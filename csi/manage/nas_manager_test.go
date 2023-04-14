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
	"errors"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/container-storage-interface/spec/lib/go/csi"

	"huawei-csi-driver/connector"
)

func mockNasStageVolumeRequest() *csi.NodeStageVolumeRequest {
	return &csi.NodeStageVolumeRequest{
		StagingTargetPath: "/test_staging_target_path",
		VolumeId:          "test_backend.pvc-nas-xxx",
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
}

func mockExpectedConnectInfo() map[string]interface{} {
	return map[string]interface{}{
		"srcType":    connector.MountFSType,
		"sourcePath": "127.0.0.1:/pvc-nas-xxx",
		"targetPath": "/test_staging_target_path",
		"mountFlags": "bound",
		"protocol":   "nfs",
	}
}

func TestNasManagerStageNfsVolume(t *testing.T) {
	manager := &NasManager{
		protocol: "nfs",
		portal:   "127.0.0.1",
		Conn:     connector.GetConnector(context.Background(), connector.NFSDriver),
	}

	mockMountShare := gomonkey.ApplyFunc(Mount, func(ctx context.Context, parameters map[string]interface{}) error {
		if !reflect.DeepEqual(parameters, mockExpectedConnectInfo()) {
			return errors.New("stage nfs volume error")
		}
		return nil
	})
	defer mockMountShare.Reset()

	err := manager.StageVolume(context.Background(), mockNasStageVolumeRequest())
	if err != nil {
		t.Errorf("TestNasManagerStageNfsVolume() want error = nil, got error = %v", err)
	}
}

func TestNasManagerStageDpcVolume(t *testing.T) {
	manager := &NasManager{
		protocol: "dpc",
		Conn:     connector.GetConnector(context.Background(), connector.NFSDriver),
	}

	mockMountShare := gomonkey.ApplyFunc(Mount, func(ctx context.Context, parameters map[string]interface{}) error {
		expectedConnectInfo := mockExpectedConnectInfo()
		expectedConnectInfo["sourcePath"] = "/pvc-nas-xxx"
		expectedConnectInfo["protocol"] = "dpc"

		if !reflect.DeepEqual(parameters, expectedConnectInfo) {
			return errors.New("stage dpc volume error")
		}
		return nil
	})
	defer mockMountShare.Reset()

	err := manager.StageVolume(context.Background(), mockNasStageVolumeRequest())
	if err != nil {
		t.Errorf("TestNasManagerStageDpcVolume() want error = nil, got error = %v", err)
	}
}
