/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2023-2024. All rights reserved.
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

// Package options control the service configurations, include env and config
package options

import (
	"flag"
	"fmt"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app/config"
)

const (
	dmMultiPath     = "DM-multipath"
	hwUltraPath     = "HW-UltraPath"
	hwUltraPathNVMe = "HW-UltraPath-NVMe"

	defaultCleanupTimeout     = 240
	defaultScanVolumeTimeout  = 3
	defaultConnectorThreads   = 4
	defaultExecCommandTimeout = 30

	minThreads = 1
	maxThreads = 10

	minScanVolumeTimeout = 1
	maxScanVolumeTimeout = 600

	minExecCommandTimeout = 1
	maxExecCommandTimeout = 600
)

type connectorOptions struct {
	volumeUseMultiPath   bool
	scsiMultiPathType    string
	nvmeMultiPathType    string
	deviceCleanupTimeout int
	scanVolumeTimeout    int
	connectorThreads     int
	allPathOnline        bool
	execCommandTimeout   int
}

// NewConnectorOptions returns connector configurations
func NewConnectorOptions() *connectorOptions {
	return &connectorOptions{
		volumeUseMultiPath:   true,
		scsiMultiPathType:    dmMultiPath,
		nvmeMultiPathType:    hwUltraPathNVMe,
		deviceCleanupTimeout: defaultCleanupTimeout,
		scanVolumeTimeout:    defaultScanVolumeTimeout,
		connectorThreads:     defaultConnectorThreads,
		allPathOnline:        false,
	}
}

// AddFlags add the connector flags
func (opt *connectorOptions) AddFlags(ff *flag.FlagSet) {
	ff.BoolVar(&opt.volumeUseMultiPath, "volume-use-multipath",
		true,
		"Whether to use multiPath when attach block volume")
	ff.StringVar(&opt.scsiMultiPathType, "scsi-multipath-type",
		dmMultiPath,
		"Multipath software for fc/iscsi block volumes")
	ff.StringVar(&opt.nvmeMultiPathType, "nvme-multipath-type",
		hwUltraPathNVMe,
		"Multipath software for roce/fc-nvme block volumes")
	ff.IntVar(&opt.deviceCleanupTimeout, "deviceCleanupTimeout",
		defaultCleanupTimeout,
		"Timeout interval in seconds for stale device cleanup")
	ff.IntVar(&opt.scanVolumeTimeout, "scan-volume-timeout",
		defaultScanVolumeTimeout,
		"The timeout for waiting for multipath aggregation when DM-multipath is used on the host")
	ff.IntVar(&opt.connectorThreads, "connector-threads",
		defaultConnectorThreads,
		"The concurrency supported during disk operations.")
	ff.BoolVar(&opt.allPathOnline, "all-path-online",
		false,
		"Whether to check the number of online paths for DM-multipath aggregation, default false")
	ff.IntVar(&opt.execCommandTimeout, "exec-command-timeout",
		defaultExecCommandTimeout,
		"The timeout for running command on host")
}

// ApplyFlags assign the connector flags
func (opt *connectorOptions) ApplyFlags(cfg *config.AppConfig) {
	cfg.VolumeUseMultiPath = opt.volumeUseMultiPath
	cfg.ScsiMultiPathType = opt.scsiMultiPathType
	cfg.NvmeMultiPathType = opt.nvmeMultiPathType
	cfg.DeviceCleanupTimeout = opt.deviceCleanupTimeout
	cfg.ScanVolumeTimeout = opt.scanVolumeTimeout
	cfg.ConnectorThreads = opt.connectorThreads
	cfg.AllPathOnline = opt.allPathOnline
	cfg.ExecCommandTimeout = opt.execCommandTimeout
}

// ValidateFlags validate the connector flags
func (opt *connectorOptions) ValidateFlags() []error {
	errs := make([]error, 0)
	err := opt.validateScsiMultiPathType()
	if err != nil {
		errs = append(errs, err)
	}

	err = opt.validateNvmeMultiPathType()
	if err != nil {
		errs = append(errs, err)
	}

	err = opt.validateScanVolumeTimeout()
	if err != nil {
		errs = append(errs, err)
	}

	return errs
}

func (opt *connectorOptions) validateScsiMultiPathType() error {
	switch opt.scsiMultiPathType {
	case dmMultiPath, hwUltraPath, hwUltraPathNVMe:
		return nil
	default:
		return fmt.Errorf("the scsi-multipath-type=%v configuration is incorrect", opt.scsiMultiPathType)
	}
}

func (opt *connectorOptions) validateNvmeMultiPathType() error {
	switch opt.nvmeMultiPathType {
	case hwUltraPathNVMe:
		return nil
	default:
		return fmt.Errorf("the nvme-multipath-type=%v configuration is incorrect", opt.nvmeMultiPathType)
	}
}

func (opt *connectorOptions) validateScanVolumeTimeout() error {
	if opt.scanVolumeTimeout < minScanVolumeTimeout || opt.scanVolumeTimeout > maxScanVolumeTimeout {
		return fmt.Errorf("the value of scanVolumeTimeout ranges from 1 to 600, current is: %d",
			opt.scanVolumeTimeout)
	}
	return nil
}

func (opt *connectorOptions) validateConnectorThreads() error {
	if opt.connectorThreads < minThreads || opt.connectorThreads > maxThreads {
		return fmt.Errorf("the connector-threads %d should be %d~%d",
			opt.connectorThreads, minThreads, maxThreads)
	}
	return nil
}
func (opt *connectorOptions) validateExecCommandTimeout() error {
	if opt.execCommandTimeout < minExecCommandTimeout || opt.execCommandTimeout > maxExecCommandTimeout {
		return fmt.Errorf("the value of execCommandTimeout ranges from 1 to 600, current is: %d",
			opt.scanVolumeTimeout)
	}
	return nil
}
