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
	"fmt"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/container-storage-interface/spec/lib/go/csi"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/connector"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
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
		portals:  []string{"127.0.0.1"},
		Conn:     connector.GetConnector(context.Background(), connector.NFSDriver),
	}

	mockMountShare := gomonkey.ApplyFunc(Mount, func(ctx context.Context, parameters map[string]interface{}) error {
		expectedConnectInfo := mockExpectedConnectInfo()
		expectedConnectInfo["portals"] = []string{"127.0.0.1"}
		if !reflect.DeepEqual(parameters, expectedConnectInfo) {
			return fmt.Errorf("stage nfs volume error parameter: %+v expectConnectInfo: %+v", parameters,
				expectedConnectInfo)
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
		portals:  []string{},
	}

	mockMountShare := gomonkey.ApplyFunc(Mount, func(ctx context.Context, parameters map[string]interface{}) error {
		expectedConnectInfo := mockExpectedConnectInfo()
		expectedConnectInfo["sourcePath"] = "/pvc-nas-xxx"
		expectedConnectInfo["protocol"] = "dpc"
		expectedConnectInfo["portals"] = []string{}

		if !reflect.DeepEqual(parameters, expectedConnectInfo) {
			return fmt.Errorf("stage dpc volume error, parameter: %+v, expectConnectInfo: %+v", parameters,
				expectedConnectInfo)
		}
		return nil
	})
	defer mockMountShare.Reset()

	err := manager.StageVolume(context.Background(), mockNasStageVolumeRequest())
	if err != nil {
		t.Errorf("TestNasManagerStageDpcVolume() want error = nil, got error = %v", err)
	}
}

func TestNasManagerStageDtfsVolume(t *testing.T) {
	// arrange
	manager := &NasManager{
		protocol:  constants.ProtocolDtfs,
		Conn:      connector.GetConnector(context.Background(), connector.NFSDriver),
		deviceWWN: "wwn001",
		portals:   []string{},
	}

	// mock
	mockMountShare := gomonkey.ApplyFunc(Mount, func(ctx context.Context, parameters map[string]interface{}) error {
		expectedConnectInfo := mockExpectedConnectInfo()
		expectedConnectInfo["sourcePath"] = "/pvc-nas-xxx"
		expectedConnectInfo["protocol"] = constants.ProtocolDtfs
		expectedConnectInfo["mountFlags"] = "bound,cid=wwn001"
		expectedConnectInfo["portals"] = []string{}

		if !reflect.DeepEqual(parameters, expectedConnectInfo) {
			return fmt.Errorf("stage dtfs volume error, parameter: %+v, expectConnectInfo: %+v", parameters,
				expectedConnectInfo)
		}
		return nil
	})
	defer mockMountShare.Reset()

	// act
	err := manager.StageVolume(context.Background(), mockNasStageVolumeRequest())

	// assert
	if err != nil {
		t.Errorf("TestNasManagerStageDtfsVolume() want error = nil, got error = %v", err)
	}
}
