/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2024-2024. All rights reserved.
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
	"time"

	"huawei-csi-driver/csi/app/config"
)

const (
	defaultVolumeModifyRetryMaxDelay  = 5 * time.Minute
	defaultVolumeModifyRetryBaseDelay = 5 * time.Second
	defaultVolumeModifyReconcileDelay = 1 * time.Second
	defaultVolumeModifyReSyncPeriod   = 0
)

type extenderOptions struct {
	volumeModifyRetryBaseDelay time.Duration
	volumeModifyRetryMaxDelay  time.Duration
	volumeModifyReconcileDelay time.Duration
	volumeModifyReSyncPeriod   time.Duration
}

// NewExtenderOptions returns extender configurations
func NewExtenderOptions() *extenderOptions {
	return &extenderOptions{
		volumeModifyRetryBaseDelay: defaultVolumeModifyRetryBaseDelay,
		volumeModifyRetryMaxDelay:  defaultVolumeModifyRetryMaxDelay,
		volumeModifyReconcileDelay: defaultVolumeModifyReconcileDelay,
		volumeModifyReSyncPeriod:   defaultVolumeModifyReSyncPeriod,
	}
}

// AddFlags add the service flags
func (opt *extenderOptions) AddFlags(ff *flag.FlagSet) {
	ff.DurationVar(&opt.volumeModifyRetryBaseDelay, "volume-modify-retry-base-delay",
		defaultVolumeModifyRetryBaseDelay,
		"Duration, the base delay time when the resource of volume modify enters limit queue")
	ff.DurationVar(&opt.volumeModifyRetryMaxDelay, "volume-modify-retry-max-delay",
		defaultVolumeModifyRetryMaxDelay,
		"Duration, the max delay time when the resource of volume modify enters limit queue")
	ff.DurationVar(&opt.volumeModifyReconcileDelay, "volume-modify-reconcile-delay",
		defaultVolumeModifyReconcileDelay,
		"Duration, volume modify resource reconcile delay time")
	ff.DurationVar(&opt.volumeModifyReSyncPeriod, "volume-modify-re-sync-period",
		defaultVolumeModifyReSyncPeriod,
		"Duration, volume modify resource re-sync period time")
}

// ApplyFlags assign the extender flags
func (opt *extenderOptions) ApplyFlags(cfg *config.AppConfig) {
	cfg.VolumeModifyRetryBaseDelay = opt.volumeModifyRetryBaseDelay
	cfg.VolumeModifyRetryMaxDelay = opt.volumeModifyRetryMaxDelay
	cfg.VolumeModifyReconcileDelay = opt.volumeModifyReconcileDelay
	cfg.VolumeModifyReSyncPeriod = opt.volumeModifyReSyncPeriod
}

// ValidateFlags validate the service flags
func (opt *extenderOptions) ValidateFlags() []error {
	return nil
}
