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

// Package proto provides initiators with protocols
package proto

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

// GetISCSIInitiator used to get iscsi initiator
func GetISCSIInitiator(ctx context.Context) (string, error) {
	output, err := utils.ExecShellCmd(ctx,
		"awk 'BEGIN{FS=\"=\";ORS=\"\"}/^InitiatorName=/{print $2}' /etc/iscsi/initiatorname.iscsi")
	if err != nil {
		if strings.Contains(output, "cannot open file") {
			return "", errors.New("no ISCSI initiator exist")
		}

		log.AddContext(ctx).Infof("Get ISCSI initiator error: %v", output)
		return "", err
	}

	return output, nil
}

// GetFCInitiator used to get fc initiator
func GetFCInitiator(ctx context.Context) ([]string, error) {
	output, err := utils.ExecShellCmd(ctx,
		"cat /sys/class/fc_host/host*/port_name | awk 'BEGIN{FS=\"0x\";ORS=\" \"}{print $2}'")
	if err != nil {
		log.AddContext(ctx).Infof("Get FC initiator error: %v", output)
		return nil, err
	}

	if strings.Contains(output, "No such file or directory") {
		return nil, errors.New("no FC initiator exist")
	}

	return strings.Fields(output), nil
}

// GetRoCEInitiator used to get roce initiator
func GetRoCEInitiator(ctx context.Context) (string, error) {
	output, err := utils.ExecShellCmd(ctx, "cat /etc/nvme/hostnqn")
	if err != nil {
		if strings.Contains(output, "No such file or directory") {
			return "", errors.New("no NVME initiator exists")
		}

		log.AddContext(ctx).Infof("Get NVME initiator error: %v", output)
		return "", err
	}

	return strings.TrimRight(output, "\n"), nil
}

// VerifyIscsiPortals used to verify iscsi portals
func VerifyIscsiPortals(ctx context.Context, portals []interface{}) ([]string, error) {
	if len(portals) < 1 {
		return nil, errors.New("at least 1 portal must be provided for iscsi backend")
	}

	var verifiedPortals []string

	for _, i := range portals {
		portal, ok := i.(string)
		if !ok {
			log.AddContext(ctx).Warningf("VerifyIscsiPortals, convert portal to string failed, data: %v", i)
			continue
		}
		ip := net.ParseIP(portal)
		if ip == nil {
			return nil, fmt.Errorf("%s of portals is invalid", portal)
		}

		verifiedPortals = append(verifiedPortals, portal)
	}

	return verifiedPortals, nil
}
