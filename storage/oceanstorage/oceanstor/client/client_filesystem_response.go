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

package client

// FilesystemResponse is the response of get filesystem request
type FilesystemResponse struct {
	// ID is the id of the filesystem
	ID string `json:"ID,omitempty"`
	// NAME is the name of the filesystem
	NAME string `json:"NAME,omitempty"`
	// CAPACITY is the capacity of the filesystem
	CAPACITY string `json:"CAPACITY,omitempty"`
	// FileSystemMode is the mode of the filesystem, "0" means local filesystem, and "1" means hyper metro filesystem
	FileSystemMode string `json:"fileSystemMode,omitempty"`
	// HyperMetroPairIds is the hyper metro pair ids of filesystem
	HyperMetroPairIds []string `json:"-"`
}
