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

// Package labellock to provide utils for label lock
package labellock

import (
	"context"
	"time"

	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"huawei-csi-driver/csi/app"
	"huawei-csi-driver/pkg/utils"
	"huawei-csi-driver/utils/log"
)

var rtLockWaitTimeInterval = 1 * time.Second
var rtLockWaitTimeOut = 20 * time.Minute

// RTLockConfigMap lock for rt
const RTLockConfigMap = "rt-lock-cm"

// InitCmLock if cm not exist then create or refresh
func InitCmLock(ctx context.Context, cmName string) {
	if !app.GetGlobalConfig().EnableLabel {
		return
	}
	configmap, err := app.GetGlobalConfig().K8sUtils.GetConfigmap(ctx, cmName, app.GetGlobalConfig().Namespace)
	if errors.IsNotFound(err) {
		_, err = app.GetGlobalConfig().K8sUtils.CreateConfigmap(ctx, &coreV1.ConfigMap{
			TypeMeta: metaV1.TypeMeta{},
			ObjectMeta: metaV1.ObjectMeta{
				Name:      cmName,
				Namespace: app.GetGlobalConfig().Namespace,
			},
			Immutable:  nil,
			Data:       make(map[string]string),
			BinaryData: nil,
		})
		if err != nil {
			log.AddContext(ctx).Errorf("create cm for %s failed, err: %v", cmName, err)
			return
		}
		log.AddContext(ctx).Infof("create cm for %s success", cmName)
		return
	}
	if len(configmap.Data) != 0 {
		_, err = app.GetGlobalConfig().K8sUtils.UpdateConfigmap(ctx, &coreV1.ConfigMap{
			TypeMeta: metaV1.TypeMeta{},
			ObjectMeta: metaV1.ObjectMeta{
				Name:      cmName,
				Namespace: app.GetGlobalConfig().Namespace,
			},
			Immutable:  nil,
			Data:       make(map[string]string),
			BinaryData: nil,
		})
		if err != nil {
			log.AddContext(ctx).Warningf("clear rt lock configmap failed, key:%s err: %v", cmName, err)
			return
		}
		log.AddContext(ctx).Infof("clear rt lock configmap success, key:%s", cmName)
		return
	}
}

// AcquireCmLock acquire lock from configmap
func AcquireCmLock(ctx context.Context, cmName, lockKey string) error {
	start := time.Now()
	for {
		if time.Now().After(start.Add(rtLockWaitTimeOut)) {
			return utils.Errorf(ctx, "acquire rt lock timeout, cmName: %s key:%s", cmName, lockKey)
		}

		configmap, err := app.GetGlobalConfig().K8sUtils.GetConfigmap(ctx, cmName, app.GetGlobalConfig().Namespace)
		if err != nil {
			log.AddContext(ctx).Warningf("get rt lock configmap failed, key:%s err: %v", lockKey, err)
			time.Sleep(rtLockWaitTimeInterval)
			continue
		}

		if configmap.Data[lockKey] == "true" {
			log.AddContext(ctx).Warningf("acquire rt lock failed, lock is acquired, "+
				" key:%s err: %v", lockKey, err)
			time.Sleep(rtLockWaitTimeInterval)
			continue
		}
		if configmap.Data == nil {
			configmap.Data = make(map[string]string)
		}
		configmap.Data[lockKey] = "true"
		_, err = app.GetGlobalConfig().K8sUtils.UpdateConfigmap(ctx, configmap)
		if err != nil {
			log.AddContext(ctx).Warningf("update rt lock failed, key:%s err: %v", lockKey, err)
			time.Sleep(rtLockWaitTimeInterval)
			continue
		}

		log.AddContext(ctx).Infof("acquire rt lock success, key:%s", lockKey)
		return nil
	}
}

// ReleaseCmlock release lock from configmap
func ReleaseCmlock(ctx context.Context, cmName, lockKey string) error {
	start := time.Now()
	for {
		if time.Now().After(start.Add(rtLockWaitTimeOut)) {
			return utils.Errorf(ctx, "release rt lock timeout, cmName: %s key:%s", cmName, lockKey)
		}

		configmap, err := app.GetGlobalConfig().K8sUtils.GetConfigmap(ctx, cmName, app.GetGlobalConfig().Namespace)
		if err != nil {
			log.AddContext(ctx).Warningf("get release rt lock configmap failed, key:%s err: %v", lockKey, err)
			time.Sleep(rtLockWaitTimeInterval)
			continue
		}

		if configmap.Data == nil {
			configmap.Data = make(map[string]string)
		}
		delete(configmap.Data, lockKey)
		_, err = app.GetGlobalConfig().K8sUtils.UpdateConfigmap(ctx, configmap)
		if err != nil {
			log.AddContext(ctx).Warningf("update release rt lock failed, key:%s err: %v", lockKey, err)
			time.Sleep(rtLockWaitTimeInterval)
			continue
		}

		log.AddContext(ctx).Infof("release rt lock success, key:%s", lockKey)
		return nil
	}
}
