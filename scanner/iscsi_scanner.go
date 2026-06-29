/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
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

package scanner

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/connector/iscsi"
)

// Scan scans iSCSI devices and makes them available to the host.
func (s *ISCSIScanner) Scan(ctx context.Context, lunInfo interface{}) error {
	lunInfos, ok := lunInfo.([]ISCSILunInfo)
	if !ok {
		return fmt.Errorf("invalid lunInfo type, expected []ISCSILunInfo")
	}

	var state scanState
	for _, lun := range lunInfos {
		sessionId, _ := iscsi.SingleConnectISCSIPortal(ctx, lun.Portal, lun.IQN, iscsi.ChapInfo{})
		if sessionId == "" {
			return fmt.Errorf("build iscsi session failed, portal: %s", lun.Portal)
		}

		hostNo, err := getIscsiHostNobySessionId(sessionId)
		if err != nil {
			return err
		}

		hctlInfo := hctl{h: hostNo, l: lun.HostLUN}

		state.wg.Add(1)
		go doScanDevice(ctx, hctlInfo, &state)
	}

	state.wg.Wait()
	if len(state.scanErrs) > 0 {
		return errors.Join(state.scanErrs...)
	}

	return nil
}

// getIscsiHostNobySessionId retrieves the host number from an iSCSI session ID.
func getIscsiHostNobySessionId(sessionId string) (string, error) {
	globPath := fmt.Sprintf(`/sys/class/iscsi_host/host*/device/session%s`, sessionId)
	paths, err := filepath.Glob(globPath)
	if err != nil {
		return "", fmt.Errorf("get iscsi path failed, err: %w", err)
	}

	if len(paths) == 0 {
		return "", fmt.Errorf("can not find iscsi path by session id %s", sessionId)
	}

	index := strings.Index(paths[0][hostIndexOfISCSI:], "/")
	return paths[0][hostIndexOfISCSI:][:index], nil
}
