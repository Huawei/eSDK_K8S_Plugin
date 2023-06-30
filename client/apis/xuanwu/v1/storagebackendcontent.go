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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StorageBackendContentSpec defines the desired state of StorageBackendContent
type StorageBackendContentSpec struct {
	// Provider is required in StorageBackendContent, used to filter the provider to add backend
	Provider string `json:"provider" protobuf:"bytes,1,name=provider"`

	// ConfigmapMeta is current storage configmap namespace and name, format is <namespace>/<name>.
	// such as xuanwu/backup-instance-configmap
	// +optional
	ConfigmapMeta string `json:"configmapMeta" protobuf:"bytes,1,name=configmapMeta"`

	// SecretMeta is current storage secret namespace and name, format is <namespace>/<name>.
	// such as xuanwu/backup-instance-secret
	// +optional
	SecretMeta string `json:"secretMeta" protobuf:"bytes,1,name=secretMeta"`

	// BackendClaim is the bound StorageBackendClaim namespace and name, format is <namespace>/<name>.
	// +optional
	BackendClaim string `json:"backendClaim,omitempty" protobuf:"bytes,1,opt,name=backendClaim"`

	// maxClientThreads is used to limit the number of storage client request connections
	// +optional
	MaxClientThreads string `json:"maxClientThreads,omitempty" protobuf:"bytes,8,opt,name=maxClientThreads"`

	// User defined parameter for extension
	// +optional
	Parameters map[string]string `json:"parameters,omitempty" protobuf:"bytes,8,opt,name=parameters"`
}

// StorageBackendContentStatus defines the observed state of StorageBackendContent
type StorageBackendContentStatus struct {
	// ContentName means the identity of the backend, the format is as follows: provider-name@backend-name#pool-name
	ContentName string `json:"contentName,omitempty" protobuf:"bytes,1,opt,name=contentName"`

	// VendorName means the flag of the storage vendor, such as EMC/IBM/NetApp/Huawei
	VendorName string `json:"vendorName,omitempty" protobuf:"bytes,1,opt,name=vendorName"`

	// ProviderVersion means the version of the provider
	ProviderVersion string `json:"providerVersion,omitempty" protobuf:"bytes,1,opt,name=providerVersion"`

	// Capacity get the storage total capacity, used capacity and free capacity.
	Capacity map[CapacityType]string `json:"capacity,omitempty" protobuf:"bytes,1,opt,name=capacity"`

	// Capabilities get the storage total capacity, used capacity and free capacity.
	Capabilities map[string]bool `json:"capabilities,omitempty" protobuf:"bytes,1,opt,name=capabilities"`

	// Specification get the storage total specification of used capacity and free capacity.
	Specification map[string]string `json:"specification,omitempty" protobuf:"bytes,1,opt,name=specification"`

	// ConfigmapMeta is current storage configmap namespace and name, format is <namespace>/<name>.
	ConfigmapMeta string `json:"configmapMeta" protobuf:"bytes,1,name=configmapMeta"`

	// SecretMeta is current storage secret namespace and name, format is <namespace>/<name>.
	SecretMeta string `json:"secretMeta" protobuf:"bytes,1,name=secretMeta"`

	// Online indicates whether the storage login is successful
	Online bool `json:"online,omitempty" protobuf:"bytes,1,opt,name=online"`

	// maxClientThreads is used to limit the number of storage client request connections
	MaxClientThreads string `json:"maxClientThreads,omitempty" protobuf:"bytes,8,opt,name=maxClientThreads"`

	// SN is the unique identifier of a storage device.
	SN string `json:"sn,omitempty" protobuf:"bytes,1,opt,name=sn"`
}

// CapacityType means the capacity types
type CapacityType string

const (
	// TotalCapacity the total capacity of the storage pool
	TotalCapacity CapacityType = "TotalCapacity"
	// UsedCapacity the total capacity of the storage pool
	UsedCapacity CapacityType = "UsedCapacity"
	// FreeCapacity the total capacity of the storage pool
	FreeCapacity CapacityType = "FreeCapacity"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName="sbct"
// +kubebuilder:printcolumn:name="Claim",type=string,JSONPath=`.spec.backendClaim`
// +kubebuilder:printcolumn:name="SN",type=string,JSONPath=`.status.sn`
// +kubebuilder:printcolumn:name="VendorName",type=string,JSONPath=`.status.vendorName`
// +kubebuilder:printcolumn:name="ProviderVersion",type=string,JSONPath=`.status.providerVersion`
// +kubebuilder:printcolumn:name="Online",type=boolean,JSONPath=`.status.online`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// StorageBackendContent is the Schema for the StorageBackendContents API
type StorageBackendContent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StorageBackendContentSpec    `json:"spec,omitempty"`
	Status *StorageBackendContentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// StorageBackendContentList contains a list of StorageBackendContent
type StorageBackendContentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StorageBackendContent `json:"items"`
}
