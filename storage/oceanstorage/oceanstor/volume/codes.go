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

// Package volume defines status code of oceanstor storage
package volume

const (
	filesystemHealthStatusNormal = "1"

	filesystemSplitStatusNotStart  = "1"
	filesystemSplitStatusSplitting = "2"
	filesystemSplitStatusQueuing   = "3"
	filesystemSplitStatusAbnormal  = "4"

	remoteDeviceHealthStatus        = "1"
	remoteDeviceRunningStatusLinkUp = "10"

	replicationPairRunningStatusNormal = "1"
	replicationPairRunningStatusSync   = "23"

	replicationVStorePairRunningStatusNormal = "1"
	replicationVStorePairRunningStatusSync   = "23"

	replicationRolePrimary = "0"

	systemVStore = "0"

	hyperMetroPairHealthStatusFault = "2"

	hyperMetroPairRunningStatusUnknown = "0"
	hyperMetroPairRunningStatusNormal  = "1"
	hyperMetroPairRunningStatusSyncing = "23"
	hyperMetroPairRunningStatusInvalid = "35"
	hyperMetroPairRunningStatusPause   = "41"
	hyperMetroPairRunningStatusError   = "94"
	hyperMetroPairRunningStatusToSync  = "100"

	hyperMetroDomainRunningStatusNormal = "1"

	lunCopyHealthStatusFault    = "2"
	lunCopyRunningStatusQueuing = "37"
	lunCopyRunningStatusCopying = "39"
	lunCopyRunningStatusStop    = "38"
	lunCopyRunningStatusPaused  = "41"

	clonePairHealthStatusFault         = "1"
	clonePairRunningStatusUnsyncing    = "0"
	clonePairRunningStatusSyncing      = "1"
	clonePairRunningStatusNormal       = "2"
	clonePairRunningStatusInitializing = "3"

	snapshotRunningStatusActive   = "43"
	snapshotRunningStatusInactive = "45"
)
