/*
 Copyright (c) Huawei Technologies Co., Ltd. 2022-2023. All rights reserved.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at
      http://www.apache.org/licenses/LICENSE-2.0
 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

// Package controller used deal with the backend claim and backend content resources
package controller

import (
	"context"
	"errors"
	"fmt"

	xuanwuv1 "github.com/Huawei/eSDK_K8S_Plugin/v4/client/apis/xuanwu/v1"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

func (ctrl *BackendController) deleteStorageBackendContent(ctx context.Context,
	content *xuanwuv1.StorageBackendContent) error {

	err := ctrl.contentStore.Delete(content)
	if err != nil {
		msg := fmt.Sprintf("delete content from store failed, error: %v", err)
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}
	log.AddContext(ctx).Infof("Delete storageBackendContent %s finished.", content.Name)

	if content.Spec.BackendClaim == "" {
		log.AddContext(ctx).Infof("deleteStorageBackendContent %s, there is no claim bound.",
			content.Name)
		return nil
	}

	ctrl.claimQueue.Add(content.Spec.BackendClaim)
	return nil
}
