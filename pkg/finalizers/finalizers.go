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

// Package finalizers used add/remove finalizer from object
package finalizers

import (
	"context"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"huawei-csi-driver/utils/log"
)

var storageBackendMutexMap = make(map[string]*sync.Mutex)

// ContainsFinalizer checks if a finalizer already exists.
func ContainsFinalizer(meta metav1.Object, finalizer string) bool {
	if meta == nil {
		return false
	}

	for _, f := range meta.GetFinalizers() {
		if f == finalizer {
			return true
		}
	}
	return false
}

// SetFinalizer adds a finalizer if it doesn't exist.
func SetFinalizer(meta metav1.Object, finalizer string) {
	if meta == nil {
		return
	}

	if !ContainsFinalizer(meta, finalizer) {
		meta.SetFinalizers(append(meta.GetFinalizers(), finalizer))
	}
}

// RemoveFinalizer removes a finalizer if it exists.
func RemoveFinalizer(meta metav1.Object, finalizer string) {
	if meta == nil {
		return
	}
	newObj := make([]string, 0)
	for _, f := range meta.GetFinalizers() {
		if f != finalizer {
			newObj = append(newObj, f)
		}
	}
	meta.SetFinalizers(newObj)
}

// RemoveStorageBackendMutex is used to remove storageBackendMutexMap key and value
func RemoveStorageBackendMutex(ctx context.Context, storageBackendId string) {
	log.AddContext(ctx).Infof("remove storageBackendMutex start, mutexMap: %v", storageBackendMutexMap)
	delete(storageBackendMutexMap, storageBackendId)
	log.AddContext(ctx).Infof("remove storageBackendMutex success, mutexMap: %v", storageBackendMutexMap)
}
