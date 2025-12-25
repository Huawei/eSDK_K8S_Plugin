/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2025-2025. All rights reserved.
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

// Package storage provide base operations for  storage
package storage

import "time"

// Error Code
const (
	// SuccessCode defines error code of success
	SuccessCode = int64(0)

	// UserUnauthorized defines error code of user unauthorized
	UserUnauthorized int64 = -401

	// UserOffline defines error code of user off line
	UserOffline int64 = 1077949069

	// IPLockErrorCode defines error code of ip lock
	IPLockErrorCode int64 = 1077949071

	// ShareNotExist defines error code of share not exist
	ShareNotExist int64 = 1077939717

	// AuthUserNotExist defines error code of auth user not exit
	AuthUserNotExist int64 = 1077939719

	// NFSShareNotExist defines error code of NFS share not exist
	NFSShareNotExist int64 = 1077939726

	// FilesystemNotExist defines error code of filesystem not exist
	FilesystemNotExist int64 = 1073752065

	// SharePathInvalid defines error code of share path invalid
	SharePathInvalid int64 = 1077939729

	// ShareAlreadyExist defines error code of share already exist
	ShareAlreadyExist int64 = 1077939724

	// SharePathAlreadyExist defines error code of share path already exist
	SharePathAlreadyExist int64 = 1077940500

	// SystemBusy defines error code of system busy
	SystemBusy int64 = 1077949006

	// MsgTimeOut defines error code of msg timeout
	MsgTimeOut int64 = 1077949001
)

// Others
const (
	// DefaultVStore defines the default vstore name
	DefaultVStore string = "System_vStore"

	// DefaultVStoreID defines the default vstore ID
	DefaultVStoreID string = "0"

	// QueryCountPerBatch defines query count for each circle of batch operation
	QueryCountPerBatch int = 100

	// Unconnected defines the error msg of unconnected
	Unconnected = "unconnected"

	// LocalUserType defines the user type of local
	LocalUserType = "0"

	// MaxStorageThreads defines max threads of each storage
	MaxStorageThreads = 100

	// UninitializedStorage defines uninitialized storage
	UninitializedStorage = "UninitializedStorage"

	// LocalFilesystemMode normal volume
	LocalFilesystemMode string = "0"

	// HyperMetroFilesystemMode hyper metro volume
	HyperMetroFilesystemMode string = "1"

	// GetInfoWaitInternal defines wait internal of getting info
	GetInfoWaitInternal = 10 * time.Second

	defaultHttpTimeout = 60 * time.Second

	// CharsetUtf8 defines a constant representing the UTF-8 character set
	CharsetUtf8 = "UTF_8"
)
