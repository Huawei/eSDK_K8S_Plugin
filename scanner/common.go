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

// Package scanner provides options for scan device
package scanner

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
)

const (
	hostIndexOfISCSI  = 26
	hostIndexOfFC     = 30
	scanWildcard      = "-"
	pathWildcard      = "*"
	hctLen            = 3
	scanTimeout       = 30 * time.Second
	scanRetryInterval = 1 * time.Second
)

type hctl struct {
	h, c, t, l string
}

// scanScsiUtilFindDevice scans SCSI device and waits until it becomes available.
func scanScsiUtilFindDevice(ctx context.Context, hctl hctl) error {
	err := utils.WaitUntil(func() (bool, error) {
		err := executeScan(ctx, hctl)
		if err != nil {
			return false, err
		}

		exist, err := checkDeviceExisted(ctx, hctl)
		if err != nil {
			return false, err
		}

		return exist, nil
	}, scanTimeout, scanRetryInterval)

	return err
}

// executeScan writes scan command to sysfs to trigger SCSI device scan.
func executeScan(ctx context.Context, hctl hctl) error {
	if hctl.h == "" {
		return fmt.Errorf("cannot scan device while the host no is empty, htcl: %v", hctl)
	}

	channel, target, lun := hctl.c, hctl.t, hctl.l
	if channel == "" {
		channel = scanWildcard
	}
	if target == "" {
		target = scanWildcard
	}
	if lun == "" {
		lun = scanWildcard
	}

	scanPath := fmt.Sprintf("/sys/class/scsi_host/host%s/scan", hctl.h)
	scanContent := fmt.Sprintf("%s %s %s", channel, target, lun)
	scanCommand := fmt.Sprintf(`echo "%s" > %s`, scanContent, scanPath)
	_, err := utils.ExecShellCmd(ctx, scanCommand)
	if err != nil {
		return fmt.Errorf("failed to scan scsi device hctl: %+v, %w", hctl, err)
	}

	return nil
}

// checkDeviceExisted verifies if the SCSI device exists at the specified hctl address.
func checkDeviceExisted(ctx context.Context, hctl hctl) (bool, error) {
	host, channel, target, lun := hctl.h, hctl.c, hctl.t, hctl.l
	if host == "" {
		host = pathWildcard
	}
	if channel == "" {
		channel = pathWildcard
	}
	if target == "" {
		target = pathWildcard
	}
	if lun == "" {
		lun = pathWildcard
	}

	devicePath := fmt.Sprintf("/sys/class/scsi_device/%s:%s:%s:%s/device/block/", host, channel, target, lun)
	output, err := utils.ExecShellCmd(ctx, "ls %s", devicePath)
	if err != nil {
		return false, fmt.Errorf("failed to verify scsi device hctl: %+v, %w", hctl, err)
	}

	if strings.TrimSpace(output) == "" {
		return false, nil
	}

	return true, nil
}

type scanState struct {
	wg       sync.WaitGroup
	mu       sync.Mutex
	scanErrs []error
}

func doScanDevice(ctx context.Context, hctlInfo hctl, state *scanState) {
	defer state.wg.Done()
	err := scanScsiUtilFindDevice(ctx, hctlInfo)
	if err != nil {
		state.mu.Lock()
		state.scanErrs = append(state.scanErrs, err)
		state.mu.Unlock()
	}
}
