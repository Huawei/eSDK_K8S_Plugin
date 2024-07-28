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

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// VolumeModifyClaimSpec defines the desired spec of VolumeModifyClaim
type VolumeModifyClaimSpec struct {
	// Source used to config the source resource.
	// +kubebuilder:validation:Required
	Source *VolumeModifySpecSource `json:"source" protobuf:"bytes,1,name=source"`

	// Parameters csi driver specific parameters passed in as opaque key-value pairs. This field is OPTIONAL.
	// The driver is responsible for parsing and validating these parameters.
	// +optional
	// +kubebuilder:validation:Required
	Parameters map[string]string `json:"parameters,omitempty" protobuf:"bytes,2,opt,name=parameters"`
}

// VolumeModifySpecSource defines the desired source of VolumeModifyClaim
type VolumeModifySpecSource struct {
	// Kind is a string value representing the source kind, default StorageClass.
	// +kubebuilder:default=StorageClass
	Kind string `json:"kind,omitempty" protobuf:"bytes,1,opt,name=kind"`

	// Name is the name of the resource
	// +kubebuilder:validation:Required
	Name string `json:"name" protobuf:"bytes,2,name=name"`
	// NameSpace is the namespace of the resource
	// +optional
	Namespace string `json:"namespace,omitempty" protobuf:"bytes,3,opt,name=namespace"`
}

// VolumeModifyClaimStatus defines the desired status of VolumeModifyClaim
type VolumeModifyClaimStatus struct {
	// phase represents the current phase of VolumeModifyClaim.
	// +optional
	Phase VolumeModifyClaimPhase `json:"phase,omitempty" protobuf:"bytes,1,opt,name=phase"`

	// Contents used to save the VolumeModifyContent status detail
	// +optional
	Contents []ModifyContents `json:"contents,omitempty" protobuf:"bytes,2,opt,name=contents"`

	// Progress represents the current progress of VolumeModifyContent. This field is OPTIONAL.
	// +optional
	Ready string `json:"ready,omitempty" protobuf:"bytes,3,opt,name=ready"`

	// Parameters csi driver specific parameters passed in as opaque key-value pairs. This field is OPTIONAL.
	// The driver is responsible for parsing and validating these parameters.
	// +optional
	Parameters map[string]string `json:"parameters,omitempty" protobuf:"bytes,4,opt,name=parameters"`

	// StartedAt is a timestamp representing the server time when this job was created.
	// It is represented in RFC3339 form and is in UTC.
	// Populated by the system.
	// Read-only.
	// +optional
	StartedAt *metav1.Time `json:"startedAt,omitempty" protobuf:"bytes,5,opt,name=startedAt"`

	// CompletedAt is a timestamp representing the server time when this job was completed.
	// It is represented in RFC3339 form and is in UTC.
	// Populated by the system.
	// Read-only.
	// +optional
	CompletedAt *metav1.Time `json:"completedAt,omitempty" protobuf:"bytes,6,opt,name=completedAt"`
}

// ModifyContents defines the desired VolumeModifyContent status detail
type ModifyContents struct {
	// ModifyContentName used to config the VolumeModifyContent name.
	ModifyContentName string `json:"modifyContentName,omitempty" protobuf:"bytes,1,opt,name=modifyContentName"`

	// SourceVolume used to config the source PersistentVolumeClaim, format is <namespace>/<name>.
	SourceVolume string `json:"sourceVolume,omitempty" protobuf:"bytes,2,opt,name=sourceVolume"`

	// phase represents the current phase of VolumeModifyContent.
	// +optional
	Status VolumeModifyContentPhase `json:"status,omitempty" protobuf:"bytes,4,opt,name=status"`
}

// VolumeModifyClaimPhase defines the phase of VolumeModifyContent
type VolumeModifyClaimPhase string

const (
	// VolumeModifyClaimPending means the VolumeModifyClaim has been accepted. but the VolumeModifyContent has not been
	// created yet.
	VolumeModifyClaimPending VolumeModifyClaimPhase = "Pending"

	// VolumeModifyClaimCreating means the VolumeModifyClaim has been accepted, but one or more of VolumeModifyContent
	// has not been Completed.
	VolumeModifyClaimCreating VolumeModifyClaimPhase = "Creating"

	// VolumeModifyClaimCompleted means all associated VolumeModifyContent items have been completed.
	VolumeModifyClaimCompleted VolumeModifyClaimPhase = "Completed"

	// VolumeModifyClaimRollback means the VolumeModifyClaim receives the deletion request and starts rollback.
	VolumeModifyClaimRollback VolumeModifyClaimPhase = "Rollback"

	// VolumeModifyClaimDeleting means the VolumeModifyClaim start deleting.
	VolumeModifyClaimDeleting VolumeModifyClaimPhase = "Deleting"
)

// VolumeModifyClaim is the Schema for the VolumeModifyClaim API
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName="vmc"
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.ready`
// +kubebuilder:printcolumn:name="SourceKind",type=string,priority=1,JSONPath=`.spec.source.kind`
// +kubebuilder:printcolumn:name="SourceName",type=string,priority=1,JSONPath=`.spec.source.name`
// +kubebuilder:printcolumn:name="StartedAt",type=string,priority=1,JSONPath=`.status.startedAt`
// +kubebuilder:printcolumn:name="CompletedAt",type=string,priority=1,JSONPath=`.status.completedAt`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type VolumeModifyClaim struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Spec              VolumeModifyClaimSpec   `json:"spec,omitempty"`
	Status            VolumeModifyClaimStatus `json:"status,omitempty"`
}

// VolumeModifyClaimList contains a list of VolumeModifyClaim
// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type VolumeModifyClaimList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VolumeModifyClaim `json:"items"`
}
