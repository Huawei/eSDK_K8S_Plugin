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
	"strings"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
)

// Scan scans Fibre Channel devices and makes them available to the host.
func (s *FCScanner) Scan(ctx context.Context, lunInfo interface{}) error {
	lunInfos, ok := lunInfo.([]FCLunInfo)
	if !ok {
		return fmt.Errorf("invalid lunInfo type, expected []FCLunInfo")
	}

	var state scanState
	for _, lun := range lunInfos {
		hctlInfos, err := s.getHctlByWWPN(ctx, lun.WWPN, lun.HostLUN)
		if err != nil {
			return fmt.Errorf("get host number for WWPN %s failed: %w", lun.WWPN, err)
		}

		for _, hctlInfo := range hctlInfos {
			state.wg.Add(1)
			go doScanDevice(ctx, hctlInfo, &state)
		}
	}

	state.wg.Wait()
	if len(state.scanErrs) > 0 {
		return errors.Join(state.scanErrs...)
	}

	return nil
}

// getHctlByWWPN retrieves the host, channel, target, lun tuple for a given WWPN.
func (s *FCScanner) getHctlByWWPN(ctx context.Context, wwpn string, lun string) ([]hctl, error) {
	cmd := fmt.Sprintf(`grep -l %s /sys/class/fc_transport/target*/port_name 2>/dev/null`, wwpn)
	output, err := utils.ExecShellCmd(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("grep fc_transport failed: %w", err)
	}

	if strings.TrimSpace(output) == "" {
		return nil, fmt.Errorf("cannot find host number for WWPN %s", wwpn)
	}

	lines := strings.Split(output, "\n")
	hctls := make([]hctl, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		index := strings.Index(line[hostIndexOfFC:], "/")
		hostPart := line[hostIndexOfFC : hostIndexOfFC+index]
		hctParts := strings.Split(strings.TrimPrefix(hostPart, "target"), ":")
		if len(hctParts) < hctLen {
			continue
		}

		hctls = append(hctls, hctl{
			h: hctParts[0],
			c: hctParts[1],
			t: hctParts[2],
			l: lun,
		})
	}

	if len(hctls) == 0 {
		return nil, fmt.Errorf("cannot find host number for WWPN %s", wwpn)
	}

	return hctls, nil
}
