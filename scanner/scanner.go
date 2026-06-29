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
	"fmt"
	"path/filepath"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/manage"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

// ISCSILunInfo represents iSCSI logical unit information for scanning.
type ISCSILunInfo struct {
	Portal  string
	IQN     string
	HostLUN string
}

// FCLunInfo represents Fibre Channel logical unit information for scanning.
type FCLunInfo struct {
	WWPN    string
	HostLUN string
}

// Interface defines the interface for scanning storage devices.
type Interface interface {
	Scan(ctx context.Context, lunInfo interface{}) error
}

// ISCSIScanner implements Interface for iSCSI devices.
type ISCSIScanner struct{}

// FCScanner implements Interface for Fibre Channel devices.
type FCScanner struct{}

// Factory defines the interface for creating scanners based on protocol type.
type Factory interface {
	Scan(ctx context.Context, publishInfo *manage.ControllerPublishInfo) error
}

type scannerFactoryImpl struct{}

var scannerFactory Factory = &scannerFactoryImpl{}

// GetFactory returns the singleton scanner factory instance.
func GetFactory() Factory {
	return scannerFactory
}

// Scan determines the protocol type from publishInfo and delegates to the appropriate scanner.
func (f *scannerFactoryImpl) Scan(ctx context.Context, publishInfo *manage.ControllerPublishInfo) error {
	var scanner Interface
	var lunInfo interface{}

	exist, err := isDeviceExistsOnHost(publishInfo)
	if err != nil {
		return err
	}

	if !exist {
		log.AddContext(ctx).Infoln("Skip scan options since device is not exist on this host")
		return nil
	}

	if len(publishInfo.TgtIQNs) > 0 {
		scanner = &ISCSIScanner{}
		lunInfo = buildISCSILunInfos(publishInfo)
	} else if len(publishInfo.TgtWWNs) > 0 {
		scanner = &FCScanner{}
		lunInfo = buildFCLunInfos(publishInfo)
	} else {
		return fmt.Errorf("cannot determine protocol from publishInfo")
	}

	return scanner.Scan(ctx, lunInfo)
}

func isDeviceExistsOnHost(publishInfo *manage.ControllerPublishInfo) (bool, error) {
	if publishInfo.TgtLunWWN == "" {
		return false, fmt.Errorf("tgtLunWWN is empty")
	}

	devPath := fmt.Sprintf("/dev/disk/by-id/*%s", publishInfo.TgtLunWWN)
	paths, err := filepath.Glob(devPath)
	if err != nil {
		return false, fmt.Errorf("failed to glob devPath %s: %w", devPath, err)
	}

	return len(paths) > 0, nil
}

// buildISCSILunInfos converts ControllerPublishInfo to ISCSILunInfo slice for iSCSI scanning.
func buildISCSILunInfos(info *manage.ControllerPublishInfo) []ISCSILunInfo {
	var lunInfos []ISCSILunInfo
	for i, iqn := range info.TgtIQNs {
		var portal string
		if i < len(info.TgtPortals) {
			portal = info.TgtPortals[i]
		}

		var hostLUN string
		if i < len(info.TgtHostLUNs) {
			hostLUN = info.TgtHostLUNs[i]
		}

		lunInfos = append(lunInfos, ISCSILunInfo{
			Portal:  portal,
			IQN:     iqn,
			HostLUN: hostLUN,
		})
	}
	return lunInfos
}

// buildFCLunInfos converts ControllerPublishInfo to FCLunInfo slice for FC scanning.
func buildFCLunInfos(info *manage.ControllerPublishInfo) []FCLunInfo {
	var lunInfos []FCLunInfo
	for i, wwn := range info.TgtWWNs {
		var hostLUN string
		if i < len(info.TgtHostLUNs) {
			hostLUN = info.TgtHostLUNs[i]
		}

		lunInfos = append(lunInfos, FCLunInfo{
			WWPN:    wwn,
			HostLUN: hostLUN,
		})
	}
	return lunInfos
}
