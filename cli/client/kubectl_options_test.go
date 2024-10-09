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
	"strings"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/require"
	coreV1 "k8s.io/api/core/v1"
)

var returnStr = `{
    "apiVersion": "v1",
    "kind": "Pod",
    "metadata": {
        "creationTimestamp": "2023-07-28T03:22:21Z",
        "generateName": "huawei-csi-node-",
        "labels": {
            "app": "huawei-csi-node",
            "controller-revision-hash": "5d99df786c",
            "pod-template-generation": "1",
            "provisioner": "csi.huawei.com"
        },
        "name": "huawei-csi-node-9lxhm",
        "namespace": "huawei-csi",
        "resourceVersion": "3047239",
        "selfLink": "/api/v1/namespaces/huawei-csi/pods/huawei-csi-node-9lxhm",
        "uid": "3227c5e1-3cce-49ee-8d08-9e12e3ca3877"
    }
}`

func mockExecReturnStdOut(exceptCMD string) *gomonkey.Patches {
	return gomonkey.ApplyFunc(execReturnStdOut, func(ctx context.Context, cli string, args []string) ([]byte, error) {
		cmd := fmt.Sprintf("%s %s", cli, strings.Join(args, " "))
		fmt.Println(cmd)
		if cmd != exceptCMD {
			return nil, errors.New("error")
		}
		return []byte(returnStr), nil
	})
}

func TestKubernetesCLIArgs_Get_failed_without_options(t *testing.T) {
	// arrange
	var mockArgs = NewKubernetesCLIArgs("kubectl")
	var mockData, except = coreV1.Pod{}, coreV1.Pod{}
	mockCtx := context.Background()
	// mock
	patches := mockExecReturnStdOut("kubectl get pod huawei-csi-node-9lxhm -n huawei-csi -o=json")
	defer patches.Reset()

	t.Run("test get object failed without options", func(t *testing.T) {
		// action
		err := mockArgs.Get(mockCtx, &mockData)

		//assert
		require.Error(t, err)
		require.Equal(t, except, mockData)
	})
}

func TestKubernetesCLIArgs_Get_success(t *testing.T) {
	// arrange
	var mockArgs = NewKubernetesCLIArgs("kubectl").SelectObject(Pod, "huawei-csi-node-9lxhm").
		WithSpecifiedNamespace("huawei-csi").
		WithSpecifiedNode(IgnoreNode).
		WithOutPutFormat(JSON)
	var mockData, except = coreV1.Pod{}, coreV1.Pod{}
	err := json.Unmarshal([]byte(returnStr), &except)
	if err != nil {
		t.Errorf("mock return string error, error: %v", err)
		return
	}
	mockCtx := context.Background()
	// mock
	patches := mockExecReturnStdOut("kubectl get pod huawei-csi-node-9lxhm -n huawei-csi -o=json")
	defer patches.Reset()

	t.Run("test get object success", func(t *testing.T) {
		// action
		err := mockArgs.Get(mockCtx, &mockData)

		//assert
		require.NoError(t, err)
		require.Equal(t, except, mockData)
	})
}

func TestKubernetesCLIArgs_Exec_success(t *testing.T) {
	// arrange
	var mockArgs = NewKubernetesCLIArgs("kubectl").SelectObject(Pod, "huawei-csi-node-9lxhm").
		WithSpecifiedNamespace("huawei-csi").
		WithSpecifiedContainer("huawei-csi-driver").
		WithSpecifiedNode(IgnoreNode)
	var mockCmd = "collect.sh"
	var except = []byte(returnStr)
	mockCtx := context.Background()
	// mock
	patches := mockExecReturnStdOut("kubectl exec huawei-csi-node-9lxhm -c huawei-csi-driver " +
		"-n huawei-csi -- collect.sh")
	defer patches.Reset()

	t.Run("test exec cmd in container success", func(t *testing.T) {
		// action
		out, err := mockArgs.Exec(mockCtx, mockCmd)

		// assert
		require.NoError(t, err)
		require.Equal(t, except, out)
	})
}

func TestKubernetesCLIArgs_Copy_success(t *testing.T) {
	// arrange
	var mockArgs = NewKubernetesCLIArgs("kubectl").SelectObject(Pod, "huawei-csi-node-9lxhm").
		WithSpecifiedNamespace("huawei-csi").
		WithSpecifiedContainer("huawei-csi-driver").
		WithSpecifiedNode(IgnoreNode)
	var mockContainerPath, mockLocalPath = "tmp/a.tar", "/tmp/slave1/a.tar"
	var mockCopyType = ContainerToLocal
	var except = []byte(returnStr)
	mockCtx := context.Background()
	// mock
	patches := mockExecReturnStdOut("kubectl cp huawei-csi/huawei-csi-node-9lxhm:tmp/a.tar " +
		"/tmp/slave1/a.tar -c huawei-csi-driver")
	defer patches.Reset()

	t.Run("test copy file from container to local success", func(t *testing.T) {
		// action
		out, err := mockArgs.Copy(mockCtx, mockContainerPath, mockLocalPath, mockCopyType)

		// assert
		require.NoError(t, err)
		require.Equal(t, except, out)
	})
}

func TestKubernetesCLIArgs_Logs(t *testing.T) {
	// arrange
	var mockArgs = NewKubernetesCLIArgs("kubectl").SelectObject(Pod, "huawei-csi-node-9lxhm").
		WithSpecifiedNamespace("huawei-csi").
		WithSpecifiedContainer("huawei-csi-driver").
		WithSpecifiedNode(IgnoreNode)
	var except = []byte(returnStr)
	mockCtx := context.Background()
	// mock
	patches := mockExecReturnStdOut("kubectl logs huawei-csi-node-9lxhm -c huawei-csi-driver -n huawei-csi")
	defer patches.Reset()

	t.Run("test get container console logs success", func(t *testing.T) {
		// action
		out, err := mockArgs.Logs(mockCtx)

		// assert
		require.NoError(t, err)
		require.Equal(t, except, out)
	})
}
