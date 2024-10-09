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

	"gopkg.in/yaml.v3"

	"huawei-csi-driver/cli/helper"
)

type Filter struct {
	node          string
	namespace     string
	container     string
	allowNotFound bool
	isHistoryLog  bool
}

type KubernetesCLIArgs struct {
	client string

	// Collection of objects to manipulate
	objectName []string

	// Specified condition for objects
	objectType   ObjectType
	outputFormat OutputType
	selector     Filter
}

// ObjectType Define the operation object type.
type ObjectType string

// OutputType Defines the output format type.
type OutputType string

// CopyType Defines the file copy type.
type CopyType byte

const (
	Pod       ObjectType = "pod"       // Operate pod objects.
	Node      ObjectType = "node"      // Operate node objects.
	Namespace ObjectType = "namespace" // Operate namespace objects.
	Unknown   ObjectType = ""          // Unknown object

	JSON OutputType = "-o=json" // Obtains data in JSON format.
	YAML OutputType = "-o=yaml" // Obtains data in YAML format.

	LocalToContainer CopyType = 0 // Copy files from the local host to the container.
	ContainerToLocal CopyType = 1 // Copy files from the container to the local host.

	IgnoreNode      = "" // used to ignore the specified condition of the node when invoking an interface.
	IgnoreContainer = "" // used to ignore the specified condition of the container when invoking an interface.
	IgnoreNamespace = "" // used to ignore the specified condition of the namespace when invoking an interface.

	getStr  = "get"
	execStr = "exec"
	logsStr = "logs"
	copyStr = "cp"

	ignoreNotFound = "--ignore-not-found"
)

var execReturnStdOut = helper.BashExecReturnStdOut

// NewKubernetesCLIArgs get a *KubernetesCLIArgs instance
func NewKubernetesCLIArgs(client string) *KubernetesCLIArgs {
	return &KubernetesCLIArgs{
		client:     client,
		objectName: make([]string, 0),
	}
}

// WithSpecifiedNamespace specifies the namespace of the object to be manipulated.
func (k *KubernetesCLIArgs) WithSpecifiedNamespace(namespace string) *KubernetesCLIArgs {
	k.selector.namespace = namespace
	return k
}

// WithSpecifiedNode specifies the node of the object to be manipulated.
func (k *KubernetesCLIArgs) WithSpecifiedNode(nodeName string) *KubernetesCLIArgs {
	k.selector.node = nodeName
	return k
}

// WithSpecifiedContainer specifies the container of the object to be manipulated.
func (k *KubernetesCLIArgs) WithSpecifiedContainer(containerName string) *KubernetesCLIArgs {
	k.selector.container = containerName
	return k
}

// WithOutPutFormat specifies the output format of the object to be manipulated.
func (k *KubernetesCLIArgs) WithOutPutFormat(outputType OutputType) *KubernetesCLIArgs {
	k.outputFormat = outputType
	return k
}

// WithIgnoreNotFound adds the --ignore-not-found option.
func (k *KubernetesCLIArgs) WithIgnoreNotFound() *KubernetesCLIArgs {
	k.selector.allowNotFound = true
	return k
}

// WithHistoryLogs adds the -p option.
func (k *KubernetesCLIArgs) WithHistoryLogs(isHistoryLog bool) *KubernetesCLIArgs {
	k.selector.isHistoryLog = isHistoryLog
	return k
}

// SelectObject specifies the type and name of the object to be operated.
func (k *KubernetesCLIArgs) SelectObject(objectType ObjectType, objectName ...string) *KubernetesCLIArgs {
	k.objectType = objectType
	k.setObject(objectName)
	return k
}

func (k *KubernetesCLIArgs) setObject(objectName []string) {
	if len(objectName) != 0 {
		k.objectName = append(k.objectName, objectName...)
	}
}

func (k *KubernetesCLIArgs) getObject() ([]string, error) {
	switch k.objectType {
	case Node, Pod, Namespace:
		return k.objectName, nil
	default:
		return nil, errors.New("unknown object type")
	}
}

// Get obtains object data based on the configured parameters and unmarshal to the data.
func (k *KubernetesCLIArgs) Get(ctx context.Context, data interface{}) error {
	object, err := k.getObject()
	if err != nil {
		return err
	}

	args := []string{getStr}
	args = append(args, string(k.objectType))
	args = append(args, object...)
	args = append(args, k.selector.getAllFilter()...)
	args = append(args, string(k.outputFormat))

	out, err := execReturnStdOut(ctx, k.client, args)
	if err != nil {
		return err
	}

	switch k.outputFormat {
	case JSON:
		err = json.Unmarshal(out, data)
	case YAML:
		err = yaml.Unmarshal(out, data)
	default:
		return errors.New("outputFormat not valid")
	}
	return err
}

// Exec run the command in the specified container based on the configured parameters.
func (k *KubernetesCLIArgs) Exec(ctx context.Context, cmd string) ([]byte, error) {
	objects, err := k.getObject()
	if err != nil {
		return nil, err
	}

	res := make([]byte, 0)
	for _, object := range objects {
		args := []string{execStr, object}
		args = append(args, k.selector.getAllFilter()...)
		args = append(args, []string{"--", cmd}...)
		out, err := execReturnStdOut(ctx, k.client, args)
		if err != nil {
			return nil, err
		}

		res = append(res, out...)
	}

	return res, nil
}

// Copy local files and specified container files based on the configured parameters.
func (k *KubernetesCLIArgs) Copy(ctx context.Context, containerPath, localPath string, cpType CopyType) ([]byte, error) {
	objects, err := k.getObject()
	if err != nil {
		return nil, err
	}

	var isContainerToLocal bool
	switch cpType {
	case ContainerToLocal:
		isContainerToLocal = true
	case LocalToContainer:
	default:
		return nil, errors.New("copyType not valid ")
	}

	res := make([]byte, 0)
	for _, object := range objects {
		args := []string{copyStr}
		containerPath = fmt.Sprintf("%s/%s:%s", k.selector.namespace, object, containerPath)
		if isContainerToLocal {
			args = append(args, []string{containerPath, localPath}...)
		} else {
			args = append(args, []string{localPath, containerPath}...)
		}

		args = append(args, k.selector.getContainerFilter()...)
		out, err := execReturnStdOut(ctx, k.client, args)
		if err != nil {
			return nil, err
		}

		res = append(res, out...)
	}

	return res, nil
}

// Logs obtains the console logs of a specified container.
func (k *KubernetesCLIArgs) Logs(ctx context.Context) ([]byte, error) {
	objects, err := k.getObject()
	if err != nil {
		return nil, err
	}

	res := make([]byte, 0)
	for _, object := range objects {
		args := []string{logsStr, object}
		args = append(args, k.selector.getAllFilter()...)
		out, err := execReturnStdOut(ctx, k.client, args)
		if err != nil {
			return nil, err
		}

		res = append(res, out...)
	}

	return res, nil
}

func (f *Filter) getNamespaceFilter() []string {
	if f.namespace != IgnoreNamespace {
		return []string{"-n", f.namespace}
	}
	return []string{}
}

func (f *Filter) getNodeFilter() []string {
	if f.node != IgnoreNode {
		return []string{fmt.Sprintf("--field-selector spec.nodeName=%s", f.node)}
	}
	return []string{}
}

func (f *Filter) getContainerFilter() []string {
	if f.container != IgnoreContainer {
		return []string{"-c", f.container}
	}
	return []string{}
}

func (f *Filter) getIgnoreFilter() []string {
	if f.allowNotFound {
		return []string{ignoreNotFound}
	}
	return []string{}
}

func (f *Filter) getHistoryFilter() []string {
	if f.isHistoryLog {
		return []string{"-p"}
	}
	return []string{}
}

func (f *Filter) getAllFilter() []string {
	args := append(f.getNodeFilter(), f.getContainerFilter()...)
	args = append(args, f.getNamespaceFilter()...)
	args = append(f.getIgnoreFilter(), args...)
	args = append(f.getHistoryFilter(), args...)
	return args
}
