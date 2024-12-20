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

// Package api provides oceandisk restful urls definition
package api

// Oceandisk storage interface urls
const (
	// QueryAssociateNamespaceGroup is the query path for associating a namespace group.
	QueryAssociateNamespaceGroup = "/namespacegroup/associate?ASSOCIATEOBJTYPE=%d&ASSOCIATEOBJID=%s"
	// GetNamespaceByName is the query path for getting a namespace by its name.
	GetNamespaceByName = "/namespace?filter=NAME::%s&range=[0-100]"
	// GetNamespaceByID is the query path for getting a namespace by its ID.
	GetNamespaceByID = "/namespace/%s"
	// AddNamespaceToGroup is the path for adding a namespace to a group.
	AddNamespaceToGroup = "/namespacegroup/associate"
	// RemoveNamespaceFromGroup is the path for removing a namespace from a group.
	RemoveNamespaceFromGroup = "/namespacegroup/associate"
	// GetNamespaceGroupByName is the query path for getting a namespace group by its name.
	GetNamespaceGroupByName = "/namespacegroup?filter=NAME::%s"
	// CreateNamespaceGroup is the path for creating a namespace group.
	CreateNamespaceGroup = "/namespacegroup"
	// DeleteNamespaceGroup is the path for deleting a namespace group.
	DeleteNamespaceGroup = "/namespacegroup/%s"
	// CreateNamespace is the path for creating a namespace.
	CreateNamespace = "/namespace"
	// DeleteNamespace is the path for deleting a namespace.
	DeleteNamespace = "/namespace/%s"
	// ExtendNamespace is the path for expanding a namespace.
	ExtendNamespace = "/namespace/expand"
	// GetNamespaceCountOfMapping is the query path for getting the number of namespaces in a mapping relationship.
	GetNamespaceCountOfMapping = "/namespace/count?ASSOCIATEOBJTYPE=245&ASSOCIATEOBJID=%s"
	// GetNamespaceCountOfHost is the query path for getting the number of namespaces on a host.
	GetNamespaceCountOfHost = "/namespace/count?ASSOCIATEOBJTYPE=21&ASSOCIATEOBJID=%s"
	// GetHostNamespaceId is the query path for getting the ID of a host namespace.
	GetHostNamespaceId = "/namespace/associate?TYPE=11&ASSOCIATEOBJTYPE=21&ASSOCIATEOBJID=%s"
	// UpdateNamespace is the path for updating a namespace.
	UpdateNamespace = "/namespace/%s"
)
