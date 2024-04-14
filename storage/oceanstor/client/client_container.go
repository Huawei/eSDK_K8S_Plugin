/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2023-2023. All rights reserved.
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

// Package client provide client of storage
package client

import (
	"context"
	"strings"

	"huawei-csi-driver/utils/log"
)

// Container interface of container service
type Container interface {
	IsSupportContainer(ctx context.Context) bool
}

// IsSupportContainer used to determine whether labels are supported.
// If 404 is returned when a PV label is queried, the container service is not supported.
func (cli *BaseClient) IsSupportContainer(ctx context.Context) bool {
	_, err := cli.Get(ctx, "/container_pv", nil)
	if err != nil && strings.Contains(err.Error(), UrlNotFound) {
		log.AddContext(ctx).Debugf("query container pv failed, error: %v", err)
		return false
	}
	return true
}
