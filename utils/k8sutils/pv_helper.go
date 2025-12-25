/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2024-2024. All rights reserved.
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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const volumeIdIndex = "volumeId"

// GetVolumeAttrsByVolumeId returns volume attributes of PV by volume id
func (k *KubeClient) GetVolumeAttrsByVolumeId(volumeId string) ([]map[string]string, error) {
	volumes, err := k.pvAccessor.GetByIndex(volumeIdIndex, volumeId)
	if err != nil {
		return nil, fmt.Errorf("get pv %s by index failed: %v", volumeId, err)
	}

	res := make([]map[string]string, 0, len(volumes))
	for _, volume := range volumes {
		if volume.Spec.CSI == nil || volume.Spec.CSI.VolumeAttributes == nil {
			log.Warningf("CSI volume attribute missing from PV with volume ID %s", volumeId)
			continue
		}

		res = append(res, volume.Spec.CSI.VolumeAttributes)
	}

	return res, nil
}

// GetDTreeParentNameByVolumeId returns dDTreeParentname field of PV by volume id
func (k *KubeClient) GetDTreeParentNameByVolumeId(volumeId string) (string, error) {
	volumeAttrs, err := k.GetVolumeAttrsByVolumeId(volumeId)
	if err != nil {
		return "", fmt.Errorf("failed to get volume atrributes by volume id %q: %v", volumeId, err)
	}

	if len(volumeAttrs) == 0 {
		return "", fmt.Errorf("volume attrs %q does not exist", volumeId)
	}

	value, ok := volumeAttrs[0][constants.DTreeParentKey]
	if !ok {
		return "", fmt.Errorf("dTreeParentName field of volume %q is not exist", volumeId)
	}

	for _, volumeAttr := range volumeAttrs {
		if value != volumeAttr[constants.DTreeParentKey] {
			return "", fmt.Errorf("attrs %s in pvs with same volume ID %s is confict",
				constants.DTreeParentKey, volumeId)
		}
	}

	return value, nil
}

// volumeIdKeyFunc is a default index function that indexes based on volume id
func volumeIdKeyFunc(obj any) ([]string, error) {
	volume, ok := obj.(*corev1.PersistentVolume)
	if !ok {
		return []string{}, fmt.Errorf("convert obj to v1.PersistentVolume failed")
	}
	if volume.Spec.CSI == nil {
		return []string{}, nil
	}

	return []string{volume.Spec.CSI.VolumeHandle}, nil
}

func initPVAccessor(helper *KubeClient) error {
	pvAccessor, err := NewResourceAccessor(
		helper.informerFactory.Core().V1().PersistentVolumes().Informer(),
		WithTransformer[*corev1.PersistentVolume](stripUnusedPvFields),
		WithIndexers[*corev1.PersistentVolume](cache.Indexers{volumeIdIndex: volumeIdKeyFunc}),
	)
	helper.pvAccessor = pvAccessor

	return err
}

func stripUnusedPvFields(obj any) (any, error) {
	pv, ok := obj.(*corev1.PersistentVolume)
	if !ok {
		return obj, nil
	}

	res := &corev1.PersistentVolume{}
	res.SetUID(pv.GetUID())
	res.SetName(pv.Name)
	res.Spec.CSI = pv.Spec.CSI

	return res, nil
}
