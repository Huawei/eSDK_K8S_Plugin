/*
 Copyright (c) Huawei Technologies Co., Ltd. 2022-2024. All rights reserved.

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
// Code generated by informer-gen. DO NOT EDIT.

package v1

import (
	internalinterfaces "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/client/informers/externalversions/internalinterfaces"
)

// Interface provides access to all the informers in this group version.
type Interface interface {
	// StorageBackendClaims returns a StorageBackendClaimInformer.
	StorageBackendClaims() StorageBackendClaimInformer
	// StorageBackendContents returns a StorageBackendContentInformer.
	StorageBackendContents() StorageBackendContentInformer
	// VolumeModifyClaims returns a VolumeModifyClaimInformer.
	VolumeModifyClaims() VolumeModifyClaimInformer
	// VolumeModifyContents returns a VolumeModifyContentInformer.
	VolumeModifyContents() VolumeModifyContentInformer
}

type version struct {
	factory          internalinterfaces.SharedInformerFactory
	namespace        string
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// New returns a new Interface.
func New(f internalinterfaces.SharedInformerFactory, namespace string, tweakListOptions internalinterfaces.TweakListOptionsFunc) Interface {
	return &version{factory: f, namespace: namespace, tweakListOptions: tweakListOptions}
}

// StorageBackendClaims returns a StorageBackendClaimInformer.
func (v *version) StorageBackendClaims() StorageBackendClaimInformer {
	return &storageBackendClaimInformer{factory: v.factory, namespace: v.namespace, tweakListOptions: v.tweakListOptions}
}

// StorageBackendContents returns a StorageBackendContentInformer.
func (v *version) StorageBackendContents() StorageBackendContentInformer {
	return &storageBackendContentInformer{factory: v.factory, tweakListOptions: v.tweakListOptions}
}

// VolumeModifyClaims returns a VolumeModifyClaimInformer.
func (v *version) VolumeModifyClaims() VolumeModifyClaimInformer {
	return &volumeModifyClaimInformer{factory: v.factory, tweakListOptions: v.tweakListOptions}
}

// VolumeModifyContents returns a VolumeModifyContentInformer.
func (v *version) VolumeModifyContents() VolumeModifyContentInformer {
	return &volumeModifyContentInformer{factory: v.factory, tweakListOptions: v.tweakListOptions}
}
