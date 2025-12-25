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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/tools/cache"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
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

// GetVolumeConfiguration return pvc's annotations
func (k *KubeClient) GetVolumeConfiguration(ctx context.Context, pvName string) (map[string]string, error) {
	log.AddContext(ctx).Infof("Start to get volume %s configuration.", pvName)
	// Get the PVC corresponding to the new PV being provisioned
	pvcUID := strings.TrimPrefix(pvName, fmt.Sprintf("%s-", k.volumeNamePrefix))
	pvcs, err := k.pvcAccessor.GetByIndex(uidIndex, pvcUID)
	if err != nil {
		return nil, err
	}

	if len(pvcs) != 1 {
		return nil, fmt.Errorf("get %d number of pvcs in cache by uid %s", len(pvcs), pvcUID)
	}

	return pvcs[0].Annotations, nil
}

func initPVCAccessor(helper *KubeClient) error {
	pvcAccessor, err := NewResourceAccessor(
		helper.informerFactory.Core().V1().PersistentVolumeClaims().Informer(),
		WithTransformer[*corev1.PersistentVolumeClaim](stripUnusedPvcFields),
		WithIndexers[*corev1.PersistentVolumeClaim](cache.Indexers{uidIndex: metaUIDKeyFunc}),
		WithHandler[*corev1.PersistentVolumeClaim](cache.ResourceEventHandlerFuncs{
			AddFunc:    helper.addPVC,
			UpdateFunc: helper.updatePVC,
			DeleteFunc: helper.deletePVC,
		}))
	helper.pvcAccessor = pvcAccessor

	return err
}

func stripUnusedPvcFields(obj any) (any, error) {
	pvc, ok := obj.(*corev1.PersistentVolumeClaim)
	if !ok {
		return obj, nil
	}

	res := &corev1.PersistentVolumeClaim{}
	res.SetUID(pvc.GetUID())
	res.SetName(pvc.Name)
	res.SetNamespace(pvc.Namespace)
	res.SetAnnotations(pvc.GetAnnotations())
	res.Spec.VolumeName = pvc.Spec.VolumeName
	res.Status.Phase = pvc.Status.Phase

	return res, nil
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
	case *corev1.PersistentVolumeClaim:
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
func (k *KubeClient) processPVCEvent(pvc *corev1.PersistentVolumeClaim, eventType string) {
	// Validate the PVC
	size, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
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
