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

package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"reflect"

	corev1 "k8s.io/api/core/v1"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/cli/helper"
	xuanwuV1 "github.com/Huawei/eSDK_K8S_Plugin/v4/client/apis/xuanwu/v1"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

// CommonCallHandler common call handler
type CommonCallHandler[T any] struct {
	client KubernetesClient
}

// ListResult list query result
type ListResult[T any] struct {
	Items []T `json:"items"`
}

// NewCommonCallHandler init common call handler
func NewCommonCallHandler[T any](client KubernetesClient) *CommonCallHandler[T] {
	return &CommonCallHandler[T]{client: client}
}

// Create resource
func (r *CommonCallHandler[T]) Create(t T) error {
	return r.commonOperateResource(t, Create)
}

// Update resource
func (r *CommonCallHandler[T]) Update(t T) error {
	return r.commonOperateResource(t, Apply)
}

// QueryByName query resource by name
func (r *CommonCallHandler[T]) QueryByName(namespace, name string) (T, error) {
	return commonQuery[T, T](r.client, namespace, name)
}

// QueryList query resource list
func (r *CommonCallHandler[T]) QueryList(namespace string, names ...string) ([]T, error) {
	if len(names) == 1 {
		t, err := r.QueryByName(namespace, names[0])
		if err != nil {
			return []T{}, err
		}
		return safeToArray(t), nil
	}

	result, err := commonQuery[ListResult[T], T](r.client, namespace, names...)
	if err != nil {
		return []T{}, err
	}
	return result.Items, nil
}

// DeleteByNames delete resource by names
func (r *CommonCallHandler[T]) DeleteByNames(namespace string, names ...string) error {
	var qualifiedNames []string
	resourceType, err := GetResourceTypeByT[T]()
	if err != nil {
		return err
	}
	for _, name := range names {
		qualifiedNames = append(qualifiedNames, path.Join(string(resourceType), name))
	}
	_, err = r.client.DeleteResourceByQualifiedNames(qualifiedNames, namespace)
	return err
}

// commonQuery common query resource
// T is return Type
// R is Resource struct
func commonQuery[T any, R any](client KubernetesClient, namespace string, names ...string) (T, error) {
	var t T
	resourceType, err := GetResourceTypeByT[R]()
	if err != nil {
		return t, err
	}
	jsonBytes, err := client.GetResource(names, namespace, "json", resourceType)
	if err != nil || len(jsonBytes) == 0 {
		return t, err
	}

	if err := json.Unmarshal(jsonBytes, &t); err != nil {
		return t, err
	}

	return t, nil
}

// commonQuery common query resource
// T is resource struct
func (r *CommonCallHandler[T]) commonOperateResource(t T, operateType string) error {
	bytes, err := helper.StructToYAML(t)
	if err != nil {
		log.Errorf("%s resource failed, error:  %v", operateType, err)
		return err
	}
	return r.client.OperateResourceByYaml(string(bytes), operateType, false)
}

// GetResourceTypeByT get resource type
func GetResourceTypeByT[T any]() (ResourceType, error) {
	var t T
	resourceType := parseType(t)
	if resourceType == "" {
		return "", errors.New(fmt.Sprintf("Unsupported query type: %s", reflect.TypeOf(t).Name()))
	}
	return resourceType, nil
}

func parseType(target interface{}) ResourceType {
	switch target.(type) {
	case corev1.Secret:
		return Secret
	case corev1.ConfigMap:
		return ConfigMap
	case xuanwuV1.StorageBackendClaim:
		return Storagebackendclaim
	case xuanwuV1.StorageBackendContent:
		return StoragebackendclaimContent
	default:
		return ""
	}
}

func safeToArray[T any](t T) []T {
	var emptyStruct T
	if reflect.DeepEqual(t, emptyStruct) {
		return []T{}
	}
	return []T{t}
}

func getObjectType(object interface{}) ObjectType {
	switch object.(type) {
	case *corev1.Namespace, *corev1.NamespaceList:
		return Namespace
	case *corev1.Node, *corev1.NodeList:
		return Node
	case *corev1.Pod, *corev1.PodList:
		return Pod
	default:
		return Unknown
	}
}

func (r *CommonCallHandler[T]) CheckObjectExist(ctx context.Context, namespace, nodeName,
	objectName string) (bool, error) {
	var object, empty T
	err := r.client.GetObject(ctx, getObjectType(&object), namespace, nodeName, JSON, &object, objectName)
	if err != nil {
		return false, err
	}

	return !reflect.DeepEqual(object, empty), nil
}

func (r *CommonCallHandler[T]) GetObject(ctx context.Context, namespace, nodeName string,
	objectName ...string) (T, error) {
	var object T
	err := r.client.GetObject(ctx, getObjectType(&object), namespace, nodeName, JSON, &object, objectName...)
	return object, err
}
