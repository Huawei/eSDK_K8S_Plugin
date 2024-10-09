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
	"errors"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetObjectType_get_namespace_type(t *testing.T) {
	// arrange
	var mockObject corev1.Namespace
	var except = Namespace
	t.Run("test get_namespace_type", func(t *testing.T) {
		// action
		objectType := getObjectType(&mockObject)
		// assert
		require.Equal(t, except, objectType)
	})
}

func TestGetObjectType_get_node_type(t *testing.T) {
	// arrange
	var mockObject corev1.NodeList
	var except = Node
	t.Run("test get_node_type", func(t *testing.T) {
		// action
		objectType := getObjectType(&mockObject)
		// assert
		require.Equal(t, except, objectType)
	})
}

func TestGetObjectType_get_pod_type(t *testing.T) {
	// arrange
	var mockObject corev1.Pod
	var except = Pod
	t.Run("test get_pod_type", func(t *testing.T) {
		// action
		objectType := getObjectType(&mockObject)
		// assert
		require.Equal(t, except, objectType)
	})
}

func TestGetObjectType_get_unknown_type(t *testing.T) {
	// arrange
	var mockObject interface{}
	var except = Unknown
	t.Run("test get_unknown_type", func(t *testing.T) {
		// action
		objectType := getObjectType(&mockObject)
		// assert
		require.Equal(t, except, objectType)
	})
}

func TestCommonCallHandler_CheckObjectExist_check_exist_namespace(t *testing.T) {
	// arrange
	var mockNamespace, mockNodeName, mockObjectName = IgnoreNamespace, IgnoreNode, "namespace"
	var mockCli = NewCommonCallHandler[corev1.Namespace](&KubernetesCLI{})
	var except = true
	// mock
	patches := gomonkey.ApplyMethod(reflect.TypeOf(&KubernetesCLI{}), "GetObject",
		func(cli *KubernetesCLI, ctx context.Context, objectType ObjectType, namespace string, nodeName string,
			outputType OutputType, data interface{}, objectName ...string) error {
			if objectType == Namespace && namespace == mockNamespace && nodeName == mockNodeName &&
				outputType == JSON && objectName[0] == mockObjectName {
				if ns, ok := data.(*corev1.Namespace); ok {
					*ns = corev1.Namespace{}
					(*ns).Name = mockObjectName
				}
				return nil
			}
			return errors.New("")
		})
	defer patches.Reset()

	t.Run("test check_exist_namespace", func(t *testing.T) {
		// action
		exist, err := mockCli.CheckObjectExist(context.Background(), mockNamespace, mockNodeName, mockObjectName)
		// assert
		require.NoError(t, err)
		require.Equal(t, except, exist)
	})
}

func TestCommonCallHandler_CheckObjectExist_check_not_exist_node(t *testing.T) {
	// arrange
	var mockNamespace, mockNodeName, mockObjectName = IgnoreNamespace, IgnoreNode, "node"
	var mockCli = NewCommonCallHandler[corev1.Node](&KubernetesCLI{})
	var except = false
	// mock
	patches := gomonkey.ApplyMethod(reflect.TypeOf(&KubernetesCLI{}), "GetObject",
		func(cli *KubernetesCLI, ctx context.Context, objectType ObjectType, namespace string, nodeName string,
			outputType OutputType, data interface{}, objectName ...string) error {
			if objectType == Node && namespace == mockNamespace && nodeName == mockNodeName &&
				outputType == JSON && objectName[0] == mockObjectName {
				return nil
			}
			return errors.New("")
		})
	defer patches.Reset()

	t.Run("test check_not_exist_node", func(t *testing.T) {
		// action
		exist, err := mockCli.CheckObjectExist(context.Background(), mockNamespace, mockNodeName, mockObjectName)
		// assert
		require.NoError(t, err)
		require.Equal(t, except, exist)
	})
}

func TestCommonCallHandler_GetObject_get_podList(t *testing.T) {
	// arrange
	var mockNamespace, mockNodeName = "namespace", IgnoreNode
	var mockCli = NewCommonCallHandler[corev1.PodList](&KubernetesCLI{})
	var except = corev1.PodList{
		Items: []corev1.Pod{
			{
				ObjectMeta: v1.ObjectMeta{
					Name: "pod1",
				},
			},
			{
				ObjectMeta: v1.ObjectMeta{
					Name: "pod2",
				},
			},
		},
	}
	// mock
	patches := gomonkey.ApplyMethod(reflect.TypeOf(&KubernetesCLI{}), "GetObject",
		func(cli *KubernetesCLI, ctx context.Context, objectType ObjectType, namespace string, nodeName string,
			outputType OutputType, data interface{}, objectName ...string) error {
			if objectType == Pod && namespace == mockNamespace && nodeName == mockNodeName &&
				outputType == JSON && objectName == nil {
				if podList, ok := data.(*corev1.PodList); ok {
					*podList = except
				}
				return nil
			}
			return errors.New("")
		})
	defer patches.Reset()

	t.Run("test get pod list", func(t *testing.T) {
		// action
		object, err := mockCli.GetObject(context.Background(), mockNamespace, mockNodeName)
		// assert
		require.NoError(t, err)
		require.Equal(t, except, object)
	})
}
