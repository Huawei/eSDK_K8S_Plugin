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

// Package client provides oceanstor A-series storage client
package client

import "github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/base"

// ASeriesVStore defines interfaces for vstore operations
type ASeriesVStore interface {
	base.VStore
}

// GetvStoreName used for get vstore name in oceanstor client
func (cli *OceanASeriesClient) GetvStoreName() string {
	return cli.VStoreName
}

// GetvStoreID used for get vstore ID in oceanstor client
func (cli *OceanASeriesClient) GetvStoreID() string {
	return cli.VStoreID
}
