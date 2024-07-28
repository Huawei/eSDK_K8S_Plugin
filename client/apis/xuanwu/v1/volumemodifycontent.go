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

// Package v1 contains API Schema definitions for the xuanwu v1 API group
package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VolumeModifyContentSpec defines the desired spec of VolumeModifyContent
type VolumeModifyContentSpec struct {
	// VolumeModifyClaimName used to config the VolumeModifyClaim name.
	// +kubebuilder:validation:Required
	VolumeModifyClaimName string `json:"volumeModifyClaimName" protobuf:"bytes,1,name=volumeModifyClaimName"`

	// SourceVolume used to config the source PersistentVolumeClaim, format is <namespace>/<name>.
	// +kubebuilder:validation:Required
	SourceVolume string `json:"sourceVolume" protobuf:"bytes,2,name=sourceVolume"`

	// VolumeHandle used to config the source PersistentVolumeClaim volumeHandle.
	// +kubebuilder:validation:Required
	VolumeHandle string `json:"volumeHandle" protobuf:"bytes,2,name=volumeHandle"`

	// Parameters csi driver specific parameters passed in as opaque key-value pairs. This field is OPTIONAL.
	// The driver is responsible for parsing and validating these parameters.
	// +optional
	// +kubebuilder:validation:Optional
	Parameters map[string]string `json:"parameters,omitempty" protobuf:"bytes,3,opt,name=parameters"`

	// StorageClassParameters storageClass parameters
	// +optional
	// +kubebuilder:validation:Optional
	StorageClassParameters map[string]string `json:"storageClassParameters,omitempty" protobuf:"bytes,3,opt,name=storageClassParameters"`
}

// VolumeModifyContentStatus defines the desired status of VolumeModifyContent
type VolumeModifyContentStatus struct {
	// phase represents the current phase of VolumeModifyContent.
	// +optional
	Phase VolumeModifyContentPhase `json:"phase,omitempty" protobuf:"bytes,1,opt,name=phase"`

	// StartedAt is a timestamp representing the server time when this job was created.
	// It is represented in RFC3339 form and is in UTC.
	// Populated by the system.
	// Read-only.
	// +optional
	StartedAt *metav1.Time `json:"startedAt,omitempty" protobuf:"bytes,2,opt,name=startedAt"`

	// CompletedAt is a timestamp representing the server time when this job was completed.
	// It is represented in RFC3339 form and is in UTC.
	// Populated by the system.
	// Read-only.
	// +optional
	CompletedAt *metav1.Time `json:"completedAt,omitempty" protobuf:"bytes,3,opt,name=completedAt"`
}

// VolumeModifyContentPhase defines the phase of VolumeModifyContent
type VolumeModifyContentPhase string

const (
	// VolumeModifyContentPending means the VolumeModifyContent has been accepted.
	// but modify didn't start.
	VolumeModifyContentPending VolumeModifyContentPhase = "Pending"

	// VolumeModifyContentCreating means the VolumeModifyContent has been accepted,
	// but it is in the process of being modified.
	VolumeModifyContentCreating VolumeModifyContentPhase = "Creating"

	// VolumeModifyContentCompleted means the VolumeModifyContent have been completed.
	VolumeModifyContentCompleted VolumeModifyContentPhase = "Completed"

	// VolumeModifyContentRollback means the VolumeModifyContent receives the deletion request and starts rollback.
	VolumeModifyContentRollback VolumeModifyContentPhase = "Rollback"

	// VolumeModifyContentDeleting means the VolumeModifyContent start deleting.
	VolumeModifyContentDeleting VolumeModifyContentPhase = "Deleting"
)

// VolumeModifyContent is the Schema for the VolumeModifyContent API
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName="vmct"
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="ModifyClaimName",type=string,JSONPath=`.spec.volumeModifyClaimName`
// +kubebuilder:printcolumn:name="SourceVolume",type=string,JSONPath=`.spec.sourceVolume`
// +kubebuilder:printcolumn:name="StartedAt",type=string,priority=1,JSONPath=`.status.startedAt`
// +kubebuilder:printcolumn:name="CompletedAt",type=string,priority=1,JSONPath=`.status.completedAt`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type VolumeModifyContent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   VolumeModifyContentSpec   `json:"spec,omitempty"`
	Status VolumeModifyContentStatus `json:"status,omitempty"`
}

// VolumeModifyContentList contains a list of VolumeModifyContent
// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type VolumeModifyContentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VolumeModifyContent `json:"items"`
}
