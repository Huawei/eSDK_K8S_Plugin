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

// Package utils to provide utils for label lock
package utils

import (
	"context"
	"time"

	"huawei-csi-driver/csi/app"
	pkgUtils "huawei-csi-driver/pkg/utils"
	"huawei-csi-driver/utils/log"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var rtLockWaitTimeInterval = 1 * time.Second
var rtLockWaitTimeOut = 10 * time.Minute

// RTLockConfigMap lock for rt
const RTLockConfigMap = "rt-lock-cm"

// InitCmLock create configmap if not exist
func InitCmLock() {
	ctx := context.Background()
	_, err := app.GetGlobalConfig().K8sUtils.CreateConfigmap(context.Background(), &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      RTLockConfigMap,
			Namespace: app.GetGlobalConfig().Namespace,
		},
		Immutable:  nil,
		Data:       make(map[string]string),
		BinaryData: nil,
	})
	if err != nil {
		log.AddContext(ctx).Warningf("create cm for %s failed, err: %v", RTLockConfigMap, err)
		return
	}
}

// AcquireCmLock acquire lock from configmap
func AcquireCmLock(ctx context.Context, cmName, lockKey string) error {
	start := time.Now()
	for {
		if time.Now().After(start.Add(rtLockWaitTimeOut)) {
			return pkgUtils.Errorf(ctx, "acquire rt lock timeout, cmName: %s key:%s", cmName, lockKey)
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
		_, err = app.GetGlobalConfig().K8sUtils.UpdateConfigmap(context.Background(), configmap)
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
			return pkgUtils.Errorf(ctx, "release rt lock timeout, cmName: %s key:%s", cmName, lockKey)
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
		_, err = app.GetGlobalConfig().K8sUtils.UpdateConfigmap(context.Background(), configmap)
		if err != nil {
			log.AddContext(ctx).Warningf("update release rt lock failed, key:%s err: %v", lockKey, err)
			time.Sleep(rtLockWaitTimeInterval)
			continue
		}

		log.AddContext(ctx).Infof("release rt lock success, key:%s", lockKey)
		return nil
	}
}

// ClearCmLock clear lock
func ClearCmLock(ctx context.Context, cmName string) {
	_, err := app.GetGlobalConfig().K8sUtils.UpdateConfigmap(context.Background(), &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
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
}
