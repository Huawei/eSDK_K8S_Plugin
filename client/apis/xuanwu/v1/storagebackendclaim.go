/*
 Copyright (c) Huawei Technologies Co., Ltd. 2022-2022. All rights reserved.

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

// StorageBackendClaimSpec defines the desired state of StorageBackend
type StorageBackendClaimSpec struct {
	// Provider is required in storageBackend, used to filter the provider to add backend
	Provider string `json:"provider" protobuf:"bytes,1,name=provider"`

	// ConfigMapMeta used to config the storage management info, the format is <namespace>/<name>.
	// +optional
	ConfigMapMeta string `json:"configmapMeta,omitempty" protobuf:"bytes,8,opt,name=configmapMeta"`

	// SecretMeta used to config the storage sensitive info, the format is <namespace>/<name>.
	// +optional
	SecretMeta string `json:"secretMeta,omitempty" protobuf:"bytes,8,opt,name=secretMeta"`

	// maxClientThreads is used to limit the number of storage client request connections
	// +optional
	MaxClientThreads string `json:"maxClientThreads,omitempty" protobuf:"bytes,8,opt,name=maxClientThreads"`

	// User defined parameter for extension
	// +optional
	Parameters map[string]string `json:"parameters,omitempty" protobuf:"bytes,8,opt,name=parameters"`
}

// StorageBackendClaimStatus defines the observed state of StorageBackend
type StorageBackendClaimStatus struct {
	// Phase represents the current phase of PersistentVolumeClaim
	// +optional
	Phase StorageBackendPhase `json:"phase,omitempty" protobuf:"bytes,1,opt,name=phase"`

	// StorageBackendId is required in storageBackend, used to filter the provider to add backend
	StorageBackendId string `json:"storageBackendId" protobuf:"bytes,1,name=storageBackendId"`

	// ConfigmapMeta is current storage configmap namespace and name, format is <namespace>/<name>,
	// such as xuanwu/backup-instance-configmap
	ConfigmapMeta string `json:"configmapMeta" protobuf:"bytes,1,name=configmapMeta"`

	// SecretMeta is current storage secret namespace and name, format is <namespace>/<name>,
	// such as xuanwu/backup-instance-secret
	SecretMeta string `json:"secretMeta" protobuf:"bytes,1,name=secretMeta"`

	// maxClientThreads is used to limit the number of storage client request connections
	// +optional
	MaxClientThreads string `json:"maxClientThreads,omitempty" protobuf:"bytes,8,opt,name=maxClientThreads"`

	// BoundContentName is the binding reference
	BoundContentName string `json:"boundContentName,omitempty" protobuf:"bytes,2,opt,name=boundContentName"`

	// StorageType is storage type
	StorageType string `json:"storageType,omitempty" protobuf:"bytes,2,opt,name=storageType"`

	// Protocol is used storage protocol
	Protocol string `json:"protocol,omitempty" protobuf:"bytes,2,opt,name=protocol"`

	// MetroBackend is the backend that form hyperMetro
	MetroBackend string `json:"metroBackend,omitempty" protobuf:"bytes,2,opt,name=metroBackend"`
}

// StorageBackendPhase defines the phase of StorageBackend
type StorageBackendPhase string

const (
	// BackendPending means StorageBackend that are not yet bound
	BackendPending StorageBackendPhase = "Pending"
	// BackendBound means StorageBackend that are already bound
	BackendBound StorageBackendPhase = "Bound"
	// BackendUnavailable means StorageBackend failed to log in due to incorrect configurations(maybe wrong password)
	BackendUnavailable StorageBackendPhase = "Unavailable"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName="sbc"
// +kubebuilder:printcolumn:name="StorageBackendContentName",type=string,JSONPath=`.status.boundContentName`
// +kubebuilder:printcolumn:name="StorageType",type=string,priority=1,JSONPath=`.status.storageType`
// +kubebuilder:printcolumn:name="Protocol",type=string,priority=1,JSONPath=`.status.protocol`
// +kubebuilder:printcolumn:name="MetroBackend",type=string,priority=1,JSONPath=`.status.metroBackend`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// StorageBackendClaim is the Schema for the storageBackends API
type StorageBackendClaim struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   StorageBackendClaimSpec    `json:"spec,omitempty"`
	Status *StorageBackendClaimStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// StorageBackendClaimList contains a list of StorageBackend
type StorageBackendClaimList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StorageBackendClaim `json:"items"`
}
