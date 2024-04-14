/*
Copyright (c) Huawei Technologies Co., Ltd. 2023-2023. All rights reserved.

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

import metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ResourceTopologyStatusPhase defines the ResourceTopologyStatusPhase type
type ResourceTopologyStatusPhase string

const (
	// ResourceTopologyStatusNormal indicates that the resource is normal
	ResourceTopologyStatusNormal ResourceTopologyStatusPhase = "Normal"
	// ResourceTopologyStatusPending indicates that the resource is pending
	ResourceTopologyStatusPending ResourceTopologyStatusPhase = "Pending"
	// ResourceTopologyStatusDeleting indicates that the resource is deleting
	ResourceTopologyStatusDeleting ResourceTopologyStatusPhase = "Deleting"
)

// ResourceTopologySpec defines the fields in Spec
type ResourceTopologySpec struct {
	// Provisioner is the volume provisioner name
	// +kubebuilder:validation:Required
	Provisioner string `json:"provisioner" protobuf:"bytes,2,name=provisioner"`

	// VolumeHandle is the backend name and identity of the volume, format as <backend>.<identity>
	// +kubebuilder:validation:Required
	VolumeHandle string `json:"volumeHandle" protobuf:"bytes,2,name=volumeHandle"`

	// Tags defines pv and other relationships and ownership
	// +kubebuilder:validation:Required
	Tags []Tag `json:"tags" protobuf:"bytes,2,name=tags"`
}

// ResourceTopologyStatus status of resource topology
type ResourceTopologyStatus struct {
	// Status is the status of the ResourceTopology
	Status ResourceTopologyStatusPhase `json:"status,omitempty" protobuf:"bytes,2,opt,name=status"`

	// Tags defines pv and other relationships and ownership
	Tags []Tag `json:"tags,omitempty" protobuf:"bytes,3,opt,name=tags"`
}

// Tag defines pv and other relationships and ownership
type Tag struct {
	ResourceInfo `json:",inline"`

	// Owner defines who does the resource belongs to
	// +kubebuilder:validation:Optional
	Owner ResourceInfo `json:"owner" protobuf:"bytes,2,name=owner"`
}

// ResourceInfo define resource information
type ResourceInfo struct {
	metaV1.TypeMeta `json:",inline"`
	// NameSpace is the namespace of the resource
	Namespace string `json:"namespace,omitempty" protobuf:"bytes,2,opt,name=target"`
	// Name is the name of the resource
	Name string `json:"name,omitempty" protobuf:"bytes,2,opt,name=name"`
}

// ResourceTopology is the Schema for the ResourceTopologys API
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName="rt"
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Provisioner",type=string,JSONPath=`.spec.provisioner`
// +kubebuilder:printcolumn:name="VolumeHandle",type=string,JSONPath=`.spec.volumeHandle`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type ResourceTopology struct {
	metaV1.TypeMeta   `json:",inline"`
	metaV1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ResourceTopologySpec   `json:"spec,omitempty"`
	Status            ResourceTopologyStatus `json:"status,omitempty"`
}

// ResourceTopologyList contains a list of ResourceTopology
// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ResourceTopologyList struct {
	metaV1.TypeMeta `json:",inline"`
	metaV1.ListMeta `json:"metadata,omitempty"`
	Items           []ResourceTopology `json:"items"`
}
