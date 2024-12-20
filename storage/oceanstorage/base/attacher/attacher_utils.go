/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2024. All rights reserved.
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

	_ "github.com/Huawei/eSDK_K8S_Plugin/v4/connector/fibrechannel"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/connector/host"
	_ "github.com/Huawei/eSDK_K8S_Plugin/v4/connector/iscsi"
	_ "github.com/Huawei/eSDK_K8S_Plugin/v4/connector/nvme"
	_ "github.com/Huawei/eSDK_K8S_Plugin/v4/connector/roce"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
)

// InitiatorType defines the initiator type
type InitiatorType int

const (
	// ISCSI defines iscsi initiator type
	ISCSI InitiatorType = iota

	// FC defines fc initiator type
	FC

	// ROCE defines roce initiator type
	ROCE
)

// GetMultipleInitiators use this method when the initiator is an array e.g. fc
func GetMultipleInitiators(ctx context.Context,
	protocol InitiatorType, parameters map[string]interface{}) ([]string, error) {
	initiatorData, err := getInitiatorByProtocol(ctx, protocol, parameters)
	if err != nil {
		return nil, err
	}

	if initiators, ok := initiatorData.([]string); ok {
		return initiators, nil
	}

	return nil, utils.Errorf(ctx, "convert %v initiator to string slice error:%v", protocol, initiatorData)
}

// GetSingleInitiator use this method when the initiator is single e.g. iscsi
func GetSingleInitiator(ctx context.Context,
	protocol InitiatorType, parameters map[string]interface{}) (string, error) {
	initiatorData, err := getInitiatorByProtocol(ctx, protocol, parameters)
	if err != nil {
		return "", err
	}

	if iscsiInitiator, ok := initiatorData.(string); ok {
		return iscsiInitiator, nil
	}

	return "", utils.Errorf(ctx, "convert %v initiator to string error:%v", protocol, initiatorData)
}

func getInitiatorByProtocol(ctx context.Context,
	protocol InitiatorType, parameters map[string]interface{}) (interface{}, error) {
	hostName, ok := parameters["HostName"].(string)
	if !ok {
		return nil, utils.Errorf(ctx, "Get node host name error,parameters:%v ", parameters)
	}

	hostInfo, err := host.GetNodeHostInfosFromSecret(ctx, hostName)
	if err != nil {
		return nil, err
	}

	mapping := map[InitiatorType]interface{}{
		ISCSI: hostInfo.IscsiInitiator,
		FC:    hostInfo.FCInitiators,
		ROCE:  hostInfo.RoCEInitiator,
	}

	value, exist := mapping[protocol]
	if !exist {
		return nil, utils.Errorf(ctx, "unsupported protocol: %v", protocol)
	}
	if value == nil || value == "" {
		return nil, utils.Errorf(ctx, "no %v initiator", protocol)
	}

	return value, nil
}
