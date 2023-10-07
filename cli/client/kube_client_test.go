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
	"testing"

	"github.com/smartystreets/goconvey/convey"
	coreV1 "k8s.io/api/core/v1"
)

func TestKubernetesCLI_GetObject_success(t *testing.T) {
	// arrange
	var mockCli *KubernetesCLI = &KubernetesCLI{cli: CLIKubernetes}
	var mockNamespace, mockNodeName, mockObjectName string = "huawei-csi", IgnoreNode, "huawei-csi-node-9lxhm"
	var mockObjectType ObjectType = Pod
	var mockOutputType OutputType = JSON
	var mockData, except coreV1.Pod = coreV1.Pod{}, coreV1.Pod{}
	json.Unmarshal([]byte(returnStr), &except)
	ctx := context.Background()
	// mock
	patches := mockExecReturnStdOut("kubectl get pod huawei-csi-node-9lxhm -n huawei-csi -o=json")
	defer patches.Reset()

	convey.Convey("test get object success", t, func() {
		// action
		err := mockCli.GetObject(ctx, mockObjectType, mockNamespace, mockNodeName, mockOutputType, &mockData,
			mockObjectName)

		//assert
		convey.So(mockData, convey.ShouldResemble, except)
		convey.So(err, convey.ShouldBeNil)
	})
}

func TestKubernetesCLI_GetObject_failed_without_objectType(t *testing.T) {
	// arrange
	var mockCli *KubernetesCLI = &KubernetesCLI{cli: CLIKubernetes}
	var mockNamespace, mockNodeName, mockObjectName string = "huawei-csi", IgnoreNode, "huawei-csi-node-9lxhm"
	var mockObjectType ObjectType = ""
	var mockOutputType OutputType = JSON
	var mockData, except coreV1.Pod = coreV1.Pod{}, coreV1.Pod{}
	ctx := context.Background()
	// mock
	patches := mockExecReturnStdOut("kubectl get pod huawei-csi-node-9lxhm -n huawei-csi -o=json")
	defer patches.Reset()

	convey.Convey("test get object success", t, func() {
		// action
		err := mockCli.GetObject(ctx, mockObjectType, mockNamespace, mockNodeName, mockOutputType, &mockData,
			mockObjectName)

		//assert
		convey.So(mockData, convey.ShouldResemble, except)
		convey.So(err, convey.ShouldBeError)
	})
}

func TestKubernetesCLI_CopyContainerFileToLocal_success(t *testing.T) {
	// arrange
	var mockCli *KubernetesCLI = &KubernetesCLI{cli: CLIKubernetes}
	var mockNamespace, mockContainerName, mockObjectName, mockSrc, mockDst = "huawei-csi", "huawei-csi-driver",
		"huawei-csi-node-9lxhm", "tmp/a.tar", "/tmp/slave1/a.tar"
	mockCtx := context.Background()
	//mock
	patches := mockExecReturnStdOut("kubectl cp huawei-csi/huawei-csi-node-9lxhm:tmp/a.tar " +
		"/tmp/slave1/a.tar -c huawei-csi-driver")
	defer patches.Reset()

	convey.Convey("test copy file from container to local", t, func() {
		// action
		out, err := mockCli.CopyContainerFileToLocal(mockCtx, mockNamespace, mockContainerName, mockSrc, mockDst,
			mockObjectName)

		//assert
		convey.So(out, convey.ShouldResemble, []byte(returnStr))
		convey.So(err, convey.ShouldBeNil)
	})
}

func TestKubernetesCLI_GetConsoleLogs_success(t *testing.T) {
	// arrange
	var mockCli *KubernetesCLI = &KubernetesCLI{cli: CLIKubernetes}
	var mockNamespace, mockContainerName, mockObjectName string = "huawei-csi", "huawei-csi-driver",
		"huawei-csi-node-9lxhm"
	mockCtx := context.Background()
	// mock
	patches := mockExecReturnStdOut("kubectl logs huawei-csi-node-9lxhm -c huawei-csi-driver -n huawei-csi")
	defer patches.Reset()

	convey.Convey("test get container console logs", t, func() {
		// action
		out, err := mockCli.GetConsoleLogs(mockCtx, mockNamespace, mockContainerName, false, mockObjectName)

		//assert
		convey.So(out, convey.ShouldResemble, []byte(returnStr))
		convey.So(err, convey.ShouldBeNil)
	})
}

func TestKubernetesCLI_ExecCmdInSpecifiedContainer_success(t *testing.T) {
	// arrange
	var mockCli *KubernetesCLI = &KubernetesCLI{cli: CLIKubernetes}
	var mockNamespace, mockContainerName, mockCmd, mockObjectName = "huawei-csi", "huawei-csi-driver", "collect.sh",
		"huawei-csi-node-9lxhm"
	mockCtx := context.Background()
	// mock
	patches := mockExecReturnStdOut("kubectl exec huawei-csi-node-9lxhm -c huawei-csi-driver " +
		"-n huawei-csi -- collect.sh")
	defer patches.Reset()

	convey.Convey("test exec script in container", t, func() {
		// action
		out, err := mockCli.ExecCmdInSpecifiedContainer(mockCtx, mockNamespace, mockContainerName, mockCmd,
			mockObjectName)

		//assert
		convey.So(out, convey.ShouldResemble, []byte(returnStr))
		convey.So(err, convey.ShouldBeNil)
	})
}
