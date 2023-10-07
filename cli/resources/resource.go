/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2023-2023. All rights reserved.
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

package resources

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"huawei-csi-driver/cli/config"
)

type resourceTuple struct {
	Resource string
	Name     string
}

type Resource struct {
	*ResourceBuilder
}

type ResourceBuilder struct {
	names []string

	resources      []string
	resourceTuples []resourceTuple

	selector  string
	selectAll bool

	namespace        string
	allNamespace     bool
	defaultNamespace bool

	fileName string
	fileType string

	output string

	notValidateName bool

	backend string

	isAllNodes bool
	nodeName   string
}

// NewResourceBuilder initialize a ResourceBuilder instance
func NewResourceBuilder() *ResourceBuilder {
	return &ResourceBuilder{}
}

// Build convert ResourceBuilder to Resource
func (b *ResourceBuilder) Build() *Resource {
	return &Resource{b}
}

// NamespaceParam accepts the namespace that these resources should be
// considered under from - used by DefaultNamespace()
func (b *ResourceBuilder) NamespaceParam(namespace string) *ResourceBuilder {
	if len(namespace) != 0 {
		b.namespace = namespace
	}
	return b
}

// DefaultNamespace instructs the builder to set the namespace value for any object found
// to NamespaceParam() if empty.
func (b *ResourceBuilder) DefaultNamespace() *ResourceBuilder {
	if b.namespace == "" {
		b.defaultNamespace = true
		b.namespace = config.DefaultNamespace
	}
	return b
}

// AllNamespaces instructs the builder to metav1.NamespaceAll as a namespace to request resources
// across all the namespace. This overrides the namespace set by NamespaceParam().
func (b *ResourceBuilder) AllNamespaces(allNamespace bool) *ResourceBuilder {
	if allNamespace {
		b.namespace = metav1.NamespaceAll
	}
	b.allNamespace = allNamespace
	return b
}

// Names instructs the builder to request resources names
func (b *ResourceBuilder) Names(names ...string) *ResourceBuilder {
	b.names = append(b.names, names...)
	return b
}

// ResourceTypes is a list of types of resources to operate on, when listing objects on
// the server or retrieving objects that match a selector.
func (b *ResourceBuilder) ResourceTypes(types ...string) *ResourceBuilder {
	b.resources = append(b.resources, types...)
	return b
}

// Selector accepts a selector directly and will filter the resulting list by that object.
func (b *ResourceBuilder) Selector(selector string) *ResourceBuilder {
	if len(selector) == 0 {
		return b
	}

	b.selector = selector
	return b
}

// SelectAll accepts a bool value and will filter the resulting list by that object.
func (b *ResourceBuilder) SelectAll(selectAll bool) *ResourceBuilder {
	b.selectAll = selectAll
	return b
}

// ResourceNames accepts a default type and one or more names, and creates tuples of resources
func (b *ResourceBuilder) ResourceNames(resource string, names ...string) *ResourceBuilder {
	b.resources = append(b.resources, resource)
	b.names = append(b.names, names...)

	for _, name := range names {
		b.resourceTuples = append(b.resourceTuples, resourceTuple{
			Resource: resource,
			Name:     name,
		})
	}
	return b
}

// Output instructs the builder to request output type
func (b *ResourceBuilder) Output(output string) *ResourceBuilder {
	b.output = output
	return b
}

// FileName instructs the builder to request file name.
func (b *ResourceBuilder) FileName(fileName string) *ResourceBuilder {
	b.fileName = fileName
	return b
}

// FileType instructs the builder to request file type.
func (b *ResourceBuilder) FileType(fileType string) *ResourceBuilder {
	if fileType == "" {
		fileType = config.DefaultInputFormat
	}
	b.fileType = fileType
	return b
}

// BoundBackend instructs the builder to request backend name.
func (b *ResourceBuilder) BoundBackend(backend string) *ResourceBuilder {
	b.backend = backend
	return b
}

// AllNodes instructs the builder to request isAllNodes options.
func (b *ResourceBuilder) AllNodes(isAllNodes bool) *ResourceBuilder {
	b.isAllNodes = isAllNodes
	return b
}

// NodeName instructs the builder to request node name.
func (b *ResourceBuilder) NodeName(nodeName string) *ResourceBuilder {
	b.nodeName = nodeName
	return b
}
