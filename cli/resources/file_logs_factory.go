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
	"context"
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"

	"huawei-csi-driver/cli/config"
	"huawei-csi-driver/cli/helper"
)

type FileLogsCollect interface {
	GetFileLogs(namespace, podName string, container *corev1.Container) (err error)
	GetHostInformation(namespace, containerName, nodeName, podName string) error
	CopyToLocal(namespace, nodeName, podName, containerName string) error
}

type BaseFileLogsCollect struct{}

type FileLogsCollector struct {
	BaseFileLogsCollect
	fileLogPath string
}

type PodType byte

const (
	CSI    PodType = 0
	CSM    PodType = 1
	Xuanwu PodType = 2
	UnKnow PodType = 3

	temporaryDirectoryNamePrefix = "huawei"

	scriptPath = "/tmp/collect.sh"

	hostInformationFilePath = "/tmp/host_information"

	fileLogsDirectory = "file"
)

var (
	fileLogCollectSet = map[PodType]FileLogsCollect{}

	containerLogsPath = fmt.Sprintf("/tmp/%s-%s", temporaryDirectoryNamePrefix,
		strconv.Itoa(int(time.Now().UnixNano())))
	localLogsPrefixPath = fmt.Sprintf("/tmp/%s-%s", temporaryDirectoryNamePrefix,
		strconv.Itoa(int(time.Now().UnixNano())))
)

func init() {
	RegisterCollector(CSI, &FileLogsCollector{})
	RegisterCollector(CSM, &FileLogsCollector{})
	RegisterCollector(Xuanwu, &FileLogsCollector{})
}

func (b *BaseFileLogsCollect) getContainerFileLogs(namespace, podName, containerName string,
	fileLogsPaths ...string) error {
	ctx := context.WithValue(context.Background(), "tag", podName)
	cmd := fmt.Sprintf("%s %s", "mkdir", containerLogsPath)
	_, err := config.Client.ExecCmdInSpecifiedContainer(ctx, namespace, containerName, cmd, podName)
	if err != nil {
		return helper.LogWarningf(ctx, "create container file logs path failed, error: %v", err)
	}

	cmd = fmt.Sprintf("cp -a %s %s 2>/dev/null || :", strings.Join(fileLogsPaths, " "), containerLogsPath)
	_, err = config.Client.ExecCmdInSpecifiedContainer(ctx, namespace, containerName, cmd, podName)
	if err != nil {
		return helper.LogWarningf(ctx, "get container file logs failed, error: %v", err)
	}

	return nil
}

func (b *BaseFileLogsCollect) deleteFileLogsInContainer(namespace, podName, containerName string,
	filePaths ...string) error {
	ctx := context.WithValue(context.Background(), "tag", podName)
	cmd := "rm -rf " + strings.Join(filePaths, " ")
	_, err := config.Client.ExecCmdInSpecifiedContainer(ctx, namespace, containerName, cmd, podName)
	if err != nil {
		return helper.LogWarningf(ctx, "delete file logs in container failed, error: %v", err)
	}

	return nil
}

func (b *BaseFileLogsCollect) compressLogsInContainer(namespace, podName, containerName string) error {
	ctx := context.WithValue(context.Background(), "tag", podName)
	compressedLogsName := fmt.Sprintf("%s-%s.tar", namespace, podName)
	cmd := fmt.Sprintf("%s %s -C %s .", "tar -czvf", path.Join(containerLogsPath, compressedLogsName),
		containerLogsPath)
	_, err := config.Client.ExecCmdInSpecifiedContainer(ctx, namespace, containerName, cmd, podName)
	if err != nil {
		return helper.LogWarningf(ctx, "compress logs in container failed, error: %v", err)
	}

	return nil
}

// CopyToLocal copy the compressed log file to the local host.
func (b *BaseFileLogsCollect) CopyToLocal(namespace, nodeName, podName, containerName string) error {
	defer b.deleteFileLogsInContainer(namespace, podName, containerName, containerLogsPath)
	ctx := context.WithValue(context.Background(), "tag", podName)
	compressedLogsName := fmt.Sprintf("%s-%s.tar", namespace, podName)
	_, err := config.Client.CopyContainerFileToLocal(ctx, namespace, containerName,
		path.Join(containerLogsPath[1:], compressedLogsName),
		path.Join(localLogsPrefixPath, nodeName, fileLogsDirectory, compressedLogsName), podName)
	if err != nil {
		return helper.LogWarningf(ctx, "copy container compressed logs to local failed, error: %v", err)
	}

	return nil
}

// GetHostInformation get the host information of a specified node.
func (b *BaseFileLogsCollect) GetHostInformation(namespace, containerName, nodeName, podName string) error {
	ctx := context.WithValue(context.Background(), "tag", podName)
	_, err := config.Client.ExecCmdInSpecifiedContainer(ctx, namespace, containerName, scriptPath, podName)
	if err != nil {
		return helper.LogWarningf(ctx, "get node host information failed, error: %v", err)
	}
	defer b.deleteFileLogsInContainer(namespace, podName, containerName, hostInformationFilePath)

	_, fileName := path.Split(hostInformationFilePath)
	_, err = config.Client.CopyContainerFileToLocal(ctx, namespace, containerName,
		hostInformationFilePath,
		path.Join(localLogsPrefixPath, nodeName, fileName), podName)
	if err != nil {
		return helper.LogWarningf(ctx, "copy node host information to local failed, error: %v", err)
	}
	return nil
}

// GetFileLogs get the file log of a specified node.
func (c *FileLogsCollector) GetFileLogs(namespace, podName string, container *corev1.Container) (err error) {
	c.fileLogPath, err = getContainerFileLogPaths(container)
	if err != nil {
		return
	}

	if err = c.getContainerFileLogs(namespace, podName, container.Name, c.fileLogPath); err != nil {
		return
	}

	err = c.compressLogsInContainer(namespace, podName, container.Name)

	return
}

// RegisterCollector used to register a collector into the collectorSet
func RegisterCollector(name PodType, collector FileLogsCollect) {
	fileLogCollectSet[name] = collector
}

// LoadSupportedCollector used to load supported collector. Return a collector of type FileLogsCollect and nil error
// if a client with the specified testName exists. If not exists, return an error with not supported.
func LoadSupportedCollector(name PodType) (FileLogsCollect, error) {
	if client, ok := fileLogCollectSet[name]; ok {
		return client, nil
	}
	return nil, errors.New("not valid collector")
}
