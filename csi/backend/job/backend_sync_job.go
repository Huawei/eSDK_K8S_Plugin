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

// Package job contains all scheduled task
package job

import (
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/handler"
	pkgUtils "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

var backendSyncInterface = handler.BackendRegisterInterface(nil)

// RunSyncBackendTaskInBackground start a scheduled task to sync backend
func RunSyncBackendTaskInBackground() {
	ctx := utils.NewContextWithRequestID()
	log.AddContext(ctx).Infof("start backend status subscribe")
	pkgUtils.Subscribe(pkgUtils.BackendStatus, handler.NewCacheWrapper().UpdateCacheBackendStatus)

	log.AddContext(ctx).Infoln("Start to sync Backend")
	backendSyncInterface = handler.NewBackendRegister()
	backendSyncInterface.FetchAndRegisterAllBackend(ctx)
	log.AddContext(ctx).Infoln("End to sync Backend")
}
