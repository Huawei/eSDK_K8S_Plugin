/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
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
	"encoding/json"
	"fmt"

	storagev1 "k8s.io/api/storage/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
)

const pvNameIndex = "pvName"

// pvNameKeyFunc is a VA index function that indexes based on pv Name.
// Returns empty slice when VA has no associated PV (not an error condition).
func pvNameKeyFunc(obj any) ([]string, error) {
	va, ok := obj.(*storagev1.VolumeAttachment)
	if !ok {
		return []string{}, fmt.Errorf("convert obj to storagev1.VolumeAttachment failed")
	}
	if va.Spec.Source.PersistentVolumeName == nil {
		return []string{}, nil
	}

	return []string{*va.Spec.Source.PersistentVolumeName}, nil
}

// initVAAccessor initializes the VolumeAttachment accessor with informer and indexers
func initVAAccessor(helper *KubeClient) error {
	vaAccessor, err := NewResourceAccessor(
		helper.informerFactory.Storage().V1().VolumeAttachments().Informer(),
		WithIndexers[*storagev1.VolumeAttachment](cache.Indexers{pvNameIndex: pvNameKeyFunc}),
	)
	helper.vaAccessor = vaAccessor

	return err
}

// UpdateVAsWithHostMap updates VAs with the given host map
func (k *KubeClient) UpdateVAsWithHostMap(ctx context.Context, volumeId string,
	hostMap map[string]map[string]interface{}) error {
	if volumeId == "" {
		return fmt.Errorf("volumeId cannot be empty")
	}
	if hostMap == nil {
		return fmt.Errorf("hostMap cannot be nil")
	}

	pvList, err := k.pvAccessor.GetByIndex(volumeIdIndex, volumeId)
	if err != nil {
		return fmt.Errorf("update VAs failed while getting pvList, err: %w", err)
	}

	for _, pv := range pvList {
		vaList, err := k.GetVAsByPVName(pv.Name)
		if err != nil {
			return fmt.Errorf("get vaList by pv name %s failed, err: %w", pv.Name, err)
		}

		vaMap := convertVAListToMap(vaList)
		for host, mappingInfo := range hostMap {
			va, exist := vaMap[host]
			if !exist {
				return fmt.Errorf("can not find the VA with pvName %s, nodeName %s", pv.Name, host)
			}

			mappingBytes, err := json.Marshal(mappingInfo)
			if err != nil {
				return fmt.Errorf("marshal mappingInfo for host %s failed, err: %w", host, err)
			}

			err = k.updateVAPublishInfo(ctx, va, string(mappingBytes))
			if err != nil {
				return fmt.Errorf("update VA publish info %s failed, err: %w", va.Name, err)
			}

			err = k.updateVAScanLabel(ctx, va)
			if err != nil {
				return fmt.Errorf("update VA label %s failed, err: %w", va.Name, err)
			}
		}
	}
	return nil
}

func (k *KubeClient) updateVAScanLabel(ctx context.Context, va *storagev1.VolumeAttachment) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		vaNew, err := k.GetVA(ctx, va.Name)
		if err != nil {
			return fmt.Errorf("get VA %s failed, err: %w", va.Name, err)
		}

		if vaNew.Labels == nil {
			vaNew.Labels = map[string]string{}
		}

		vaNew.Labels[constants.RescanLabelKey] = "true"
		_, err = k.UpdateVA(ctx, vaNew)
		return err
	})
	return err
}

func (k *KubeClient) updateVAPublishInfo(ctx context.Context,
	va *storagev1.VolumeAttachment, publishInfo string) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		vaNew, err := k.GetVA(ctx, va.Name)
		if err != nil {
			return fmt.Errorf("get VA %s failed, err: %w", va.Name, err)
		}

		vaNew.Status.AttachmentMetadata["publishInfo"] = publishInfo
		_, err = k.UpdateVAStatus(ctx, vaNew)
		return err
	})
	return err
}

// GetVA gets the VA object given its name
func (k *KubeClient) GetVA(ctx context.Context, name string) (*storagev1.VolumeAttachment, error) {
	return k.clientSet.StorageV1().VolumeAttachments().Get(ctx, name, metaV1.GetOptions{})
}

// UpdateVA update the VA
func (k *KubeClient) UpdateVA(ctx context.Context,
	va *storagev1.VolumeAttachment) (*storagev1.VolumeAttachment, error) {
	return k.clientSet.StorageV1().VolumeAttachments().Update(ctx, va, metaV1.UpdateOptions{})
}

// UpdateVAStatus update the VA status
func (k *KubeClient) UpdateVAStatus(ctx context.Context,
	va *storagev1.VolumeAttachment) (*storagev1.VolumeAttachment, error) {
	return k.clientSet.StorageV1().VolumeAttachments().UpdateStatus(ctx, va, metaV1.UpdateOptions{})
}

// GetVAsByPVName returns VA resources by pv name
func (k *KubeClient) GetVAsByPVName(pvName string) ([]*storagev1.VolumeAttachment, error) {
	if k.vaAccessor == nil {
		return nil, fmt.Errorf("VolumeAttachment accessor is not initialized")
	}

	vaList, err := k.vaAccessor.GetByIndex(pvNameIndex, pvName)
	if err != nil {
		return nil, fmt.Errorf("get VolumeAttachments by index %s failed, err: %w", pvName, err)
	}

	return vaList, nil
}

func convertVAListToMap(list []*storagev1.VolumeAttachment) map[string]*storagev1.VolumeAttachment {
	vaMap := map[string]*storagev1.VolumeAttachment{}
	for _, va := range list {
		vaMap[va.Spec.NodeName] = va
	}

	return vaMap
}
