/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2022. All rights reserved.
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

package attacher

import (
	"context"

	"huawei-csi-driver/connector"
	_ "huawei-csi-driver/connector/fibrechannel"
	_ "huawei-csi-driver/connector/iscsi"
	_ "huawei-csi-driver/connector/nvme"
	_ "huawei-csi-driver/connector/roce"
	"huawei-csi-driver/utils"
)

func disConnectVolume(ctx context.Context, tgtLunWWN, protocol string) (*connector.DisConnectInfo, error) {
	var conn connector.Connector
	switch protocol {
	case "iscsi":
		conn = connector.GetConnector(ctx, connector.ISCSIDriver)
	case "fc":
		conn = connector.GetConnector(ctx, connector.FCDriver)
	case "roce":
		conn = connector.GetConnector(ctx, connector.RoCEDriver)
	case "fc-nvme":
		conn = connector.GetConnector(ctx, connector.FCNVMeDriver)
	default:
		return nil, utils.Errorf(ctx, "the protocol %s is not valid", protocol)
	}

	return &connector.DisConnectInfo{
		Conn:   conn,
		TgtLun: tgtLunWWN,
	}, nil
}

func connectVolume(ctx context.Context, attacher AttacherPlugin, lunName, protocol string,
	parameters map[string]interface{}) (*connector.ConnectInfo, error) {
	mappingInfo, err := attacher.ControllerAttach(ctx, lunName, parameters)
	if err != nil {
		return nil, err
	}

	var conn connector.Connector
	switch protocol {
	case "iscsi":
		conn = connector.GetConnector(ctx, connector.ISCSIDriver)
	case "fc":
		conn = connector.GetConnector(ctx, connector.FCDriver)
	case "roce":
		conn = connector.GetConnector(ctx, connector.RoCEDriver)
	case "fc-nvme":
		conn = connector.GetConnector(ctx, connector.FCNVMeDriver)
	default:
		return nil, utils.Errorf(ctx, "the protocol %s is not valid", protocol)
	}

	return &connector.ConnectInfo{
		Conn:        conn,
		MappingInfo: mappingInfo,
	}, nil
}
