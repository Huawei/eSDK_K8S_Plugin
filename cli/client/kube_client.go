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
	"fmt"

	"k8s.io/api/core/v1"

	"huawei-csi-driver/cli/helper"
)

const (
	CLIKubernetes = "kubectl"
	CLIOpenShift  = "oc"

	ConfigMap                  ResourceType = "configmap"
	Secret                     ResourceType = "secret"
	Storagebackendclaim        ResourceType = "storagebackendclaim"
	StoragebackendclaimContent ResourceType = "storagebackendcontent"

	Create = "create" // used to create resource
	Delete = "delete" // used to delete resource
	Apply  = "apply"  // used to update resource
)

const (
	namespaceFlag      = "--namespace"
	ignoreNotFoundFlag = "--ignore-not-found"
)

type KubernetesCLI struct {
	cli string
}

func init() {
	RegisterClient(CLIKubernetes, &KubernetesCLI{cli: CLIKubernetes})
	RegisterClient(CLIOpenShift, &KubernetesCLI{cli: CLIOpenShift})
}

// CLI return current cli command
func (k *KubernetesCLI) CLI() string {
	return k.cli
}

// OperateResourceByYaml operate resource by yaml
// operate supported: Create, Delete, Apply
func (k *KubernetesCLI) OperateResourceByYaml(yaml, operate string, ignoreNotfound bool) error {
	args := []string{operate, "-f", "-"}
	if ignoreNotfound {
		args = append(args, ignoreNotFoundFlag)
	}
	return helper.ExecWithStdin(k.cli, []byte(yaml), args)
}

// DeleteResourceByQualifiedNames delete resource based on the specified qualified names
func (k *KubernetesCLI) DeleteResourceByQualifiedNames(qualifiedNames []string, namespace string) (string, error) {
	args := []string{"delete"}
	if len(qualifiedNames) != 0 {
		args = append(args, qualifiedNames...)
	}
	args = append(args, namespaceFlag, namespace, ignoreNotFoundFlag)
	return k.deleteResource(args)
}

// DeleteFinalizersInResourceByQualifiedNames delete finalizers in resource based on the specified qualified names
func (k *KubernetesCLI) DeleteFinalizersInResourceByQualifiedNames(qualifiedNames []string,
	namespace string) error {
	if len(qualifiedNames) == 0 {
		return nil
	}

	for _, name := range qualifiedNames {
		err := k.deleteFinalizersInResourceByQualifiedName(name, namespace)
		if err != nil {
			return err
		}
	}

	return nil
}

func (k *KubernetesCLI) deleteFinalizersInResourceByQualifiedName(qualifiedName, namespace string) error {
	args := []string{"get"}
	args = append(args, qualifiedName)
	args = append(args, namespaceFlag, namespace, "-o=yaml", ignoreNotFoundFlag)
	out, err := helper.ExecReturnStdOut(k.cli, args)
	if err != nil {
		return err
	}
	if len(out) == 0 {
		return nil
	}

	args = []string{"patch"}
	args = append(args, qualifiedName)
	args = append(args, namespaceFlag, namespace, "--type", "merge", "--patch", "{\"metadata\":{\"finalizers\":[]}}")
	out, err = helper.ExecReturnStdOut(k.cli, args)
	return err
}

// GetResource get resources based on the specified resourceType, name and outputType
func (k *KubernetesCLI) GetResource(name []string, namespace, outputType string, resourceType ResourceType) ([]byte, error) {
	args := []string{"get", string(resourceType)}
	if len(name) != 0 {
		args = append(args, name...)
	}

	args = append(args, namespaceFlag, namespace)
	if outputType != "" {
		output := fmt.Sprintf("-o=%s", outputType)
		args = append(args, output)
	}
	args = append(args, ignoreNotFoundFlag)
	return helper.ExecReturnStdOut(k.cli, args)
}

// CheckResourceExist check whether resource exists based on the specified args.
func (k *KubernetesCLI) CheckResourceExist(name, namespace string, resourceType ResourceType) (bool, error) {
	args := []string{"get", string(resourceType), name, namespaceFlag, namespace, ignoreNotFoundFlag}
	return k.checkResourceExist(args)
}

func (k *KubernetesCLI) checkResourceExist(args []string) (bool, error) {
	out, err := helper.ExecReturnStdOut(k.cli, args)
	if err != nil {
		return false, fmt.Errorf("output: %s; error: %v", string(out), err)
	}
	return len(out) > 0, nil
}

func (k *KubernetesCLI) deleteResource(args []string) (string, error) {
	out, err := helper.ExecReturnStdOut(k.cli, args)
	return string(out), err
}

// GetNameSpace used to get namespace from service account info.
func (k *KubernetesCLI) GetNameSpace() (string, error) {
	args := []string{"get", "serviceaccount", "default", "-o=json"}
	out, err := helper.ExecReturnStdOut(k.cli, args)
	if err != nil {
		return "", err
	}

	var serviceAccount v1.ServiceAccount
	err = json.Unmarshal(out, &serviceAccount)
	if err != nil {
		return "", err
	}

	namespace := serviceAccount.ObjectMeta.Namespace
	return namespace, nil
}

// GetObject used to get the specified format data of the object with specified conditions and unmarshal to the data.
func (k *KubernetesCLI) GetObject(ctx context.Context, objectType ObjectType, namespace, nodeName string,
	outputType OutputType, data interface{}, objectName ...string) error {
	return NewKubernetesCLIArgs(k.CLI()).
		SelectObject(objectType, objectName...).
		WithSpecifiedNamespace(namespace).
		WithSpecifiedNode(nodeName).
		WithOutPutFormat(outputType).
		Get(ctx, &data)
}

// ExecCmdInSpecifiedContainer used to executes the specified command in the container with specified conditions.
func (k *KubernetesCLI) ExecCmdInSpecifiedContainer(ctx context.Context, namespace, containerName, cmd string,
	podName ...string) ([]byte, error) {
	return NewKubernetesCLIArgs(k.CLI()).
		SelectObject(Pod, podName...).
		WithSpecifiedNamespace(namespace).
		WithSpecifiedContainer(containerName).
		Exec(ctx, cmd)
}

// CopyContainerFileToLocal used to copying a Local File to a Container with Specified Conditions
func (k *KubernetesCLI) CopyContainerFileToLocal(ctx context.Context, namespace, containerName, src, dst string,
	podName ...string) ([]byte, error) {
	return NewKubernetesCLIArgs(k.CLI()).
		SelectObject(Pod, podName...).
		WithSpecifiedNamespace(namespace).
		WithSpecifiedContainer(containerName).
		Copy(ctx, src, dst, ContainerToLocal)
}

// GetConsoleLogs used to get the console logs of a specified container.
func (k *KubernetesCLI) GetConsoleLogs(ctx context.Context, namespace, containerName string, isHistoryLogs bool,
	podName ...string) ([]byte, error) {
	return NewKubernetesCLIArgs(k.CLI()).
		SelectObject(Pod, podName...).
		WithSpecifiedNamespace(namespace).
		WithSpecifiedContainer(containerName).
		WithHistoryLogs(isHistoryLogs).
		Logs(ctx)
}
