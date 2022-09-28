/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2022-2022. All rights reserved.
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

package utils

import (
	"context"
	"os"
	"regexp"
	"strconv"

	"huawei-csi-driver/utils/log"
)

// ChmodFsPermission used for change target path permission
func ChmodFsPermission(ctx context.Context, targetPath, fsPermission string) {
	reg := regexp.MustCompile(`^[0-7][0-7][0-7]$`)
	match := reg.FindStringSubmatch(fsPermission)
	if match == nil {
		log.AddContext(ctx).Errorf("fsPermission [%s] in storageClass.yaml format must be [0-7][0-7][0-7]. "+
			"Change directory [%v] fsPermission failed.", fsPermission, targetPath)
		return
	}

	// chmod need convert fsPermission to octal(8)
	mode, err := strconv.ParseInt(fsPermission, 8, 0)
	if err != nil {
		log.AddContext(ctx).Errorf("convert %s to int(octal) failed. error: %v", fsPermission, err)
		return
	}

	err = os.Chmod(targetPath, os.FileMode(mode))
	if err != nil {
		log.AddContext(ctx).Errorf("Failed to modify the directory permission. "+
			"targetPath: [%v], fsPermission: [%s]", targetPath, fsPermission)
		return
	}
	log.AddContext(ctx).Infof("Change directory [%s] to [%s] permission success.", targetPath, fsPermission)
}
