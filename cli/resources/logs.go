/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2023-2024. All rights reserved.
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

// Package resources defines the command execution logic.
package resources

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	coreV1 "k8s.io/api/core/v1"

	"huawei-csi-driver/cli/client"
	"huawei-csi-driver/cli/config"
	"huawei-csi-driver/cli/helper"
	"huawei-csi-driver/utils/log"
)

const (
	localCompressedLogsPrefixPath = "/tmp"
	localOceanctlLogPath          = "/var/log/huawei/oceanctl-log"

	logsSeparator       = "-"
	logsSeparatorLength = 35

	maxTransmissionTaskWait = 100
	maxTransmissionsNum     = 10

	maxNodeGoroutineLimit = 1000
)

var (
	checkNamespaceExistFun func(ctx context.Context, ns string, node string, objectName string) (bool, error)
	checkNodeExistFun      func(ctx context.Context, ns string, node string, objectName string) (bool, error)
	getPodListFun          func(ctx context.Context, ns string, node string,
		objectName ...string) (coreV1.PodList, error)

	logsSeparatorStr = strings.Repeat(logsSeparator, logsSeparatorLength)
)

// Logs Records specified conditions and provides a method for collecting logs.
type Logs struct {
	resource *Resource

	nodePodList map[string][]coreV1.Pod
}

// NewLogs initialize a Logs instance
func NewLogs(resource *Resource) *Logs {
	return &Logs{
		resource:    resource,
		nodePodList: make(map[string][]coreV1.Pod),
	}
}

func initFun() {
	checkNamespaceExistFun = client.NewCommonCallHandler[coreV1.Namespace](config.Client).CheckObjectExist
	checkNodeExistFun = client.NewCommonCallHandler[coreV1.Node](config.Client).CheckObjectExist
	getPodListFun = client.NewCommonCallHandler[coreV1.PodList](config.Client).GetObject
}

func (lg *Logs) initLogsType() error {
	if !lg.resource.isAllNodes && lg.resource.nodeName != "" {
		_, err := checkNodeExistFun(context.Background(), client.IgnoreNamespace, client.IgnoreNode,
			lg.resource.nodeName)
		if err != nil {
			return helper.LogErrorf("check node exist failed, error: %v", err)
		}
		return nil
	}
	lg.resource.nodeName = client.IgnoreNode
	return nil
}

func (lg *Logs) initNodePodList() error {
	podList, err := getPodListFun(context.Background(), lg.resource.namespace, lg.resource.nodeName)
	if err != nil {
		return helper.LogErrorf("get pod list failed, error: %v", err)
	}

	if lg.resource.nodeName != client.IgnoreNode && len(podList.Items) != 0 {
		lg.nodePodList[lg.resource.nodeName] = podList.Items
	} else if lg.resource.nodeName == client.IgnoreNode {
		for _, pod := range podList.Items {
			if _, ok := lg.nodePodList[pod.Spec.NodeName]; !ok {
				lg.nodePodList[pod.Spec.NodeName] = []coreV1.Pod{pod}
				continue
			}
			lg.nodePodList[pod.Spec.NodeName] = append(lg.nodePodList[pod.Spec.NodeName], pod)
		}
	}

	return nil
}

func (lg *Logs) checkNamespaceExist() error {
	_, err := checkNamespaceExistFun(context.Background(), client.IgnoreNamespace, client.IgnoreNode,
		lg.resource.namespace)
	if err != nil {
		return helper.LogErrorf("check namespace exist failed, error: %v", err)
	}
	return nil
}

func (lg *Logs) initialize() error {
	if lg.resource == nil {
		return helper.LogErrorf("collect failed, error: %v", errors.New("resource is nil"))
	}

	if lg.resource.maxNodeThreads <= 0 || lg.resource.maxNodeThreads > maxNodeGoroutineLimit {
		return helper.LogErrorf("collect failed, error: %v", errors.New("threads-max must in range [1~1000]"))
	}

	initFun()
	err := lg.checkNamespaceExist()
	if err != nil {
		return err
	}

	err = lg.initLogsType()
	if err != nil {
		return err
	}

	err = lg.initNodePodList()
	return err
}

// Collect logs based on specified conditions
func (lg *Logs) Collect() error {
	log.Infof("%s Start Recording And Collecting Log Information: Namespace: %s Node: %s %s",
		logsSeparatorStr, lg.resource.namespace, lg.getNodeName(), logsSeparatorStr)
	defer log.Infof("%s Stop Recording And Collecting Log Information: Namespace: %s Node: %s %s",
		logsSeparatorStr, lg.resource.namespace, lg.getNodeName(), logsSeparatorStr)
	err := lg.initialize()
	if err != nil {
		return err
	}

	err = createNodeLogsPath(lg.nodePodList)
	if err != nil {
		return err
	}
	defer deleteLocalLogsFile()

	ctx, cancel := context.WithCancel(context.Background())

	display := NewDisplay()

	nodeLimiter := helper.NewGlobalGoroutineLimit(lg.resource.maxNodeThreads)
	nodeLimiter.AddWork(len(lg.nodePodList))

	transmitter := helper.NewTransmitter(maxTransmissionsNum, maxTransmissionTaskWait)
	transmitter.Start()

	lg.collect(ctx, transmitter, display, nodeLimiter)

	go display.Show(ctx)

	nodeLimiter.Wait()
	cancel()
	transmitter.Wait()

	err = compressLocalLogs(lg.nodePodList, lg.getLocalCompressedLogsFileName())
	return err
}

func (lg *Logs) collect(ctx context.Context, transmitter *helper.TaskHandler, display *Display,
	nodeLimiter *helper.GlobalGoroutineLimit) {
	for nodeName, pods := range lg.nodePodList {
		nodeLogsCollector := NewNodeLogsCollector(pods, transmitter, display)
		display.Add(fmt.Sprintf("node[%s] ", nodeName), nodeLogsCollector.completionStatus.Display)

		nodeLimiter.HandleWork(func() {
			nodeLogsCollector.Collect()
		})

	}
}

func createNodeLogsPath(nodeList map[string][]coreV1.Pod) error {
	for node := range nodeList {
		err := os.MkdirAll(path.Join(localLogsPrefixPath, node, fileLogsDirectory), os.ModePerm)
		if err != nil {
			return helper.LogErrorf("create node logs directory failed, error: %v", err)
		}

		err = os.MkdirAll(path.Join(localLogsPrefixPath, node, consoleLogDirectory), os.ModePerm)
		if err != nil {
			return helper.LogErrorf("create node logs directory failed, error: %v", err)
		}
	}
	return nil
}

func (lg *Logs) getNodeName() string {
	if lg.resource.nodeName == client.IgnoreNode {
		return "all"
	}
	return lg.resource.nodeName
}

func (lg *Logs) getLocalCompressedLogsFileName() string {
	nowTime := time.Now().Format("2006-01-02 15:04:05")
	return fmt.Sprintf("%s-%s-%s.zip", lg.resource.namespace,
		strings.Join(strings.Split(nowTime, " "), "-"),
		lg.getNodeName())
}

func deleteLocalLogsFile() error {
	err := os.RemoveAll(localLogsPrefixPath)
	if err != nil {
		return helper.LogErrorf("delete local logs file failed,error: %v", err)
	}
	return nil
}

func compressLocalLogs(nodeList map[string][]coreV1.Pod, fileName string) error {
	nodeLogsDirList := make([]string, 0)
	for node := range nodeList {
		nodeLogsDirList = append(nodeLogsDirList, path.Join(localLogsPrefixPath, node))
	}
	nodeLogsDirList = append(nodeLogsDirList, localOceanctlLogPath)

	return zipMultiFiles(path.Join(localCompressedLogsPrefixPath, fileName), nodeLogsDirList...)
}

func zipMultiFiles(zipPath string, filePaths ...string) error {
	// Create zip file and it's parent dir.
	if err := os.MkdirAll(filepath.Dir(zipPath), os.ModePerm); err != nil {
		return helper.LogErrorf("create compressed logs directory failed, error: %v", err)
	}
	archive, err := os.OpenFile(zipPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return helper.LogErrorf("create compressed logs file failed, error: %v", err)
	}
	defer archive.Close()

	// New zip writer.
	zipWriter := zip.NewWriter(archive)
	defer zipWriter.Close()

	// Traverse the file or directory.
	for _, rootPath := range filePaths {
		// Remove the trailing path separator if path is a directory.
		rootPath = strings.TrimSuffix(rootPath, string(os.PathSeparator))

		// Visit all the files or directories in the tree.
		err = filepath.Walk(rootPath, walkFunc(rootPath, zipWriter))
		if err != nil {
			return err
		}
	}
	return nil
}

func walkFunc(rootPath string, zipWriter *zip.Writer) filepath.WalkFunc {
	return func(path string, info fs.FileInfo, err error) error {
		// If a file is a symbolic link it will be skipped.
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}

		// Create a local file header.
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return helper.LogErrorf("get compressed file info header failed, error: %v", err)
		}

		// Set compression method.
		// Select store method  because the file is already compressed.
		header.Method = zip.Store

		// Set relative path of a file as the header name.
		header.Name, err = filepath.Rel(filepath.Dir(rootPath), path)
		if err != nil {
			return helper.LogErrorf("get relative directory failed, error: %v", err)
		}
		if info.IsDir() {
			header.Name += string(os.PathSeparator)
		}

		// Create writer for the file header and save content of the file.
		headerWriter, err := zipWriter.CreateHeader(header)
		if err != nil {
			return helper.LogErrorf("create writer for writing file to compressed file failed, error: %v", err)
		}
		if info.IsDir() {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return helper.LogErrorf("open log file failed, error:%v", err)
		}
		defer f.Close()
		_, err = io.Copy(headerWriter, f)
		if err != nil {
			return helper.LogErrorf("write file to compress file failed, error: %v", err)
		}

		return nil
	}
}
