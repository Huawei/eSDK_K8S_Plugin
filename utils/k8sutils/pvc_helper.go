/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2023-2024. All rights reserved.
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

// Package k8sutils provides Kubernetes utilities
package k8sutils

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"

	"huawei-csi-driver/utils/log"
)

const (
	uidIndex = "uid"

	eventAdd    = "add"
	eventUpdate = "update"
	eventDelete = "delete"

	cacheSyncPeriod = 60 * time.Second
)

var uidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type persistentVolumeClaimOps interface {
	// GetVolumeConfiguration returns PVC's volume info
	GetVolumeConfiguration(ctx context.Context, pvName string) (map[string]string, error)
}

func initPVCWatcher(ctx context.Context, helper *KubeClient) {
	// Set up a watch for PVCs
	helper.pvcSource = &cache.ListWatch{
		ListFunc: func(options metaV1.ListOptions) (runtime.Object, error) {
			return helper.clientSet.CoreV1().PersistentVolumeClaims(v1.NamespaceAll).List(ctx, options)
		},
		WatchFunc: func(options metaV1.ListOptions) (watch.Interface, error) {
			return helper.clientSet.CoreV1().PersistentVolumeClaims(v1.NamespaceAll).Watch(ctx, options)
		},
	}

	// Set up the PVC indexing controller
	helper.pvcController = cache.NewSharedIndexInformer(
		helper.pvcSource,
		&v1.PersistentVolumeClaim{},
		cacheSyncPeriod,
		cache.Indexers{uidIndex: metaUIDKeyFunc},
	)

	helper.pvcIndexer = helper.pvcController.GetIndexer()
	_, err := helper.pvcController.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    helper.addPVC,
			UpdateFunc: helper.updatePVC,
			DeleteFunc: helper.deletePVC,
		})
	if err != nil {
		log.Errorf("Add event handler failed, error %v", err)
	}
}

// metaUIDKeyFunc is a default index function that indexes based on an object's uid
func metaUIDKeyFunc(obj interface{}) ([]string, error) {
	if key, ok := obj.(string); ok && uidRegex.MatchString(key) {
		return []string{key}, nil
	}

	objMeta, err := meta.Accessor(obj)
	if err != nil {
		return []string{""}, fmt.Errorf("object has no meta: %v", err)
	}

	if len(objMeta.GetUID()) == 0 {
		return []string{""}, fmt.Errorf("object has no UID: %v", err)
	}
	return []string{string(objMeta.GetUID())}, nil
}

func (k *KubeClient) processPVC(obj interface{}, eventType string) {
	switch pvc := obj.(type) {
	case *v1.PersistentVolumeClaim:
		k.processPVCEvent(pvc, eventType)
	default:
		log.Errorf("K8S helper expected PVC; got %v", obj)
	}
}

func (k *KubeClient) addPVC(obj interface{}) {
	k.processPVC(obj, eventAdd)
}

func (k *KubeClient) updatePVC(_, obj interface{}) {
	k.processPVC(obj, eventUpdate)
}

func (k *KubeClient) deletePVC(obj interface{}) {
	k.processPVC(obj, eventDelete)
}

// processPVC logs the add/update/delete PVC events.
func (k *KubeClient) processPVCEvent(pvc *v1.PersistentVolumeClaim, eventType string) {
	// Validate the PVC
	size, ok := pvc.Spec.Resources.Requests[v1.ResourceStorage]
	if !ok {
		log.Debugf("Rejecting PVC %s/%s, no size specified.", pvc.Namespace, pvc.Name)
		return
	}

	msg := fmt.Sprintf("name: %s/%s, phase: %s, size: %s, uid: %s",
		pvc.Namespace, pvc.Name, pvc.Status.Phase, size.String(), pvc.UID)
	switch eventType {
	case eventAdd:
		log.Debugf("PVC [%s] added to the cache", msg)
	case eventUpdate:
		log.Debugf("PVC [%s] updated in cache", msg)
	case eventDelete:
		log.Debugf("PVC [%s] deleted in cache", msg)
	default:
		log.Warningf("UnSupport event type %s", eventType)
	}
}

// GetVolumeConfiguration return pvc's annotations
func (k *KubeClient) GetVolumeConfiguration(ctx context.Context, pvName string) (map[string]string, error) {
	log.AddContext(ctx).Infof("Start to get volume %s configuration.", pvName)
	// Get the PVC corresponding to the new PV being provisioned
	pvc, err := k.getPVC(ctx, pvName)
	if err != nil {
		return nil, err
	}

	return pvc.Annotations, nil
}

func (k *KubeClient) getPVC(ctx context.Context, pvName string) (*v1.PersistentVolumeClaim, error) {
	pvcUID := strings.TrimPrefix(pvName, fmt.Sprintf("%s-", k.volumeNamePrefix))
	pvc, err := k.getCachedPVCByUID(pvcUID)
	if err != nil {
		log.AddContext(ctx).Debugf("PVC %s not found in local cache: %v", pvName, err)

		// Not found immediately, so re-sync and try again
		if err = k.pvcIndexer.Resync(); err != nil {
			return nil, fmt.Errorf("could not refresh local PVC cache: %v", err)
		}

		if pvc, err = k.getCachedPVCByUID(pvcUID); err != nil {
			log.AddContext(ctx).Debugf("PVC %s not found in local cache after reSync: %v", pvName, err)
			return nil, fmt.Errorf("PVCNotFound %s: %v", pvcUID, err)
		}
	}

	return pvc, nil
}

func (k *KubeClient) getCachedPVCByUID(uid string) (*v1.PersistentVolumeClaim, error) {
	items, err := k.pvcIndexer.ByIndex(uidIndex, uid)
	if err != nil {
		return nil, fmt.Errorf("could not search cache for PVC by UID %s", uid)
	} else if len(items) == 0 {
		return nil, fmt.Errorf("PVC object not found in cache by UID %s", uid)
	} else if len(items) > 1 {
		return nil, fmt.Errorf("multiple cached PVC objects found by UID %s", uid)
	}

	if pvc, ok := items[0].(*v1.PersistentVolumeClaim); !ok {
		return nil, fmt.Errorf("non-PVC cached object found by UID %s", uid)
	} else {
		return pvc, nil
	}
}
