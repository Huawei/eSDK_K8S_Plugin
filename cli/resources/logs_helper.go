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
	"os"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	coreV1 "k8s.io/api/core/v1"

	"huawei-csi-driver/cli/config"
	"huawei-csi-driver/cli/helper"
)

const (
	csiFlagContainer = "huawei-csi-driver"
	csmFlagContainer = "cmi-controller"

	progressBarLength = 100

	consoleLogDirectory = "console"
)

var (
	identifyPodTypesFunc    = make(map[PodType]func(pod *coreV1.Pod) bool)
	xuanwuPodPrefixNameList = []string{"xuanwu-backup-mngt", "xuanwu-backup-service", "xuanwu-base-mngt",
		"xuanwu-disaster-service", "xuanwu-disaster-mngt", "xuanwu-metadata-service", "xuanwu-volume-service"}
)

// NodeLogCollector Collecting Node Logs
type NodeLogCollector struct {
	podList             []coreV1.Pod
	wg                  sync.WaitGroup
	fileLogsOnce        []helper.Once
	hostInformationOnce helper.Once

	// collect completion status
	completionStatus Status

	local       *helper.LocalGoroutineLimit
	transmitter *helper.TaskHandler

	display *Display
}

// Status Collection status display
type Status struct {
	completed int32
	total     int
}

// Display Collects the progress display function of all nodes to display the progress.
type Display struct {
	displayFunc []func()
	prefixDesc  []string
	totalLines  int
}

// TransmitTask Configuring a Pod Log File Transfer Task
type TransmitTask struct {
	namespace     string
	nodeName      string
	podName       string
	containerName string
	FileLogsCollect
}

func init() {
	RegisterIdentifyPodTypeFunc(CSI, checkCSIPod)
	RegisterIdentifyPodTypeFunc(CSM, checkCSMPod)
	RegisterIdentifyPodTypeFunc(Xuanwu, checkXuanwuPod)
}

// Do copy the compressed log file to the local host.
func (t *TransmitTask) Do() {
	_ = t.CopyToLocal(t.namespace, t.nodeName, t.podName, t.containerName)
}

func newTransmitTask(namespace, nodeName, podName, containerName string, collect FileLogsCollect) *TransmitTask {
	return &TransmitTask{
		namespace:       namespace,
		nodeName:        nodeName,
		podName:         podName,
		containerName:   containerName,
		FileLogsCollect: collect,
	}
}

// NewDisplay initialize a Display instance
func NewDisplay() *Display {
	return &Display{
		displayFunc: make([]func(), 0),
		prefixDesc:  make([]string, 0),
	}
}

// Add the progress function to be displayed.
func (d *Display) Add(prefixDesc string, f func()) {
	d.displayFunc = append(d.displayFunc, f)
	d.prefixDesc = append(d.prefixDesc, prefixDesc)
	d.totalLines++
}

func (d *Display) show() {
	for idx, display := range d.displayFunc {
		fmt.Printf("%s", d.prefixDesc[idx])
		display()
	}
}

func (d *Display) hideCursor() {
	fmt.Printf("\033[?25l")
}

func (d *Display) displayCursor() {
	fmt.Printf("\033[?25h")
}

func (d *Display) resetCursor() {
	fmt.Printf("\033[%dA\r", d.totalLines)
}

// Show all progresses
func (d *Display) Show(ctx context.Context) {
	d.hideCursor()
	defer d.displayCursor()
	d.show()
	for {
		select {
		case <-ctx.Done():
			d.resetCursor()
			d.show()
			return
		default:
			d.resetCursor()
			d.show()
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func (n *Status) getPercent() int {
	return int(atomic.LoadInt32(&n.completed)) * 100 / n.total
}

func (n *Status) getCompletedStr() string {
	return fmt.Sprintf("%c[1;40;32m%s%c[0m", 0x1B, strings.Repeat("+", n.getPercent()*progressBarLength/100), 0x1B)
}

func (n *Status) getRemainedStr() string {
	return fmt.Sprintf("%s", strings.Repeat("-", progressBarLength-n.getPercent()*progressBarLength/100))
}

// Display displays the pod log collection status of the current node.
func (n *Status) Display() {
	fmt.Printf("Collection Progress：[%s%s] %d/%d Pods\n",
		n.getCompletedStr(), n.getRemainedStr(), atomic.LoadInt32(&n.completed), n.total)
}

func checkCSIPod(pod *coreV1.Pod) bool {
	for _, container := range pod.Spec.Containers {
		if container.Name == csiFlagContainer {
			return true
		}
	}
	return false
}

func checkCSMPod(pod *coreV1.Pod) bool {
	for _, container := range pod.Spec.Containers {
		if container.Name == csmFlagContainer {
			return true
		}
	}
	return false
}

func checkXuanwuPod(pod *coreV1.Pod) bool {
	for _, prefix := range xuanwuPodPrefixNameList {
		if strings.HasPrefix(pod.Name, prefix) {
			return true
		}
	}
	return false
}

// RegisterIdentifyPodTypeFunc used to register a func into the identifyPodTypeFuncSet
func RegisterIdentifyPodTypeFunc(name PodType, f func(pod *coreV1.Pod) bool) {
	identifyPodTypesFunc[name] = f
}

// NewNodeLogsCollector initialize a NodeLogsCollector instance
func NewNodeLogsCollector(podList []coreV1.Pod, local *helper.LocalGoroutineLimit,
	transmitter *helper.TaskHandler, display *Display) *NodeLogCollector {
	return &NodeLogCollector{
		podList: podList,
		completionStatus: Status{
			total: len(podList),
		},
		local:        local,
		transmitter:  transmitter,
		fileLogsOnce: make([]helper.Once, len(podList)),
		display:      display,
	}
}

// Collect collects container logs of specified conditions on the node.
func (n *NodeLogCollector) Collect() {
	n.wg.Add(n.completionStatus.total)
	for idx := range n.podList {
		pod := &n.podList[idx]
		localIdx := idx
		n.local.Do(func() {
			n.collectPodLogs(pod, localIdx)
			atomic.AddInt32(&n.completionStatus.completed, 1)
			n.wg.Done()
		})
	}
	n.wg.Wait()
}

func (n *NodeLogCollector) collectPodLogs(pod *coreV1.Pod, onceIdx int) {
	ctx := context.WithValue(context.Background(), "tag", pod.Name)
	var isRunning = pod.Status.Phase == coreV1.PodRunning
	fileLogCollector, err := LoadSupportedCollector(getPodType(pod))
	if err != nil {
		_ = helper.LogWarningf(ctx, "unknown pod types, error: %v", err)
		return
	}

	if !isRunning {
		logPath, err := getPodFileLogPaths(pod)
		if err != nil {
			return
		}

		msg := fmt.Sprintf("Failed to collect [%s] file logs on node [%s], please collect logs manually,"+
			" file logs path is [%s]", pod.Name, pod.Spec.NodeName, logPath)
		n.display.Add("", func() {
			fmt.Printf("%c[1;40;31m%s%c[0m\n", 0x1B, msg, 0x1B)
		})
		_ = helper.LogWarningf(ctx, "error: %v", errors.New(msg))
	}

	for idx := range pod.Spec.Containers {
		container := &pod.Spec.Containers[idx]
		getConsoleLogs(ctx, pod.Namespace, container.Name, pod.Name, pod.Spec.NodeName, false)
		getConsoleLogs(ctx, pod.Namespace, container.Name, pod.Name, pod.Spec.NodeName, true)
		if isRunning {
			n.fileLogsOnce[onceIdx].Do(func() error {
				err = fileLogCollector.GetFileLogs(pod.Namespace, pod.Name, container)
				if err == nil {
					n.transmitter.AddTask(newTransmitTask(pod.Namespace, pod.Spec.NodeName, pod.Name, container.Name,
						fileLogCollector))
				}
				return err
			})

			n.hostInformationOnce.Do(func() error {
				return fileLogCollector.GetHostInformation(pod.Namespace, container.Name, pod.Spec.NodeName, pod.Name)
			})
		}
	}
}

func getConsoleLogs(ctx context.Context, namespace, containerName, podName, nodeName string, isHistoryLogs bool) {
	logs, err := config.Client.GetConsoleLogs(ctx, namespace, containerName, isHistoryLogs, podName)
	if err != nil {
		_ = helper.LogWarningf(ctx, "get container console logs failed, error: %v", err)
	} else {
		_ = saveConsoleLog(logs, namespace, podName, containerName, nodeName, isHistoryLogs)
	}
}

func getPodType(pod *coreV1.Pod) PodType {
	for podType, f := range identifyPodTypesFunc {
		if f(pod) {
			return podType
		}
	}
	return UnKnow
}

func saveConsoleLog(logs []byte, namespace, podName, containerName, nodeName string, isHistoryLogs bool) error {
	ctx := context.WithValue(context.Background(), "tag", podName)
	fileName := fmt.Sprintf("%s-%s-%s.log", namespace, podName, containerName)
	if isHistoryLogs {
		fileName = "last-" + fileName
	}
	file, err := os.Create(path.Join(localLogsPrefixPath, nodeName, consoleLogDirectory, fileName))
	if err != nil {
		return helper.LogWarningf(ctx, "create container console log file failed, error: %v", err)
	}
	defer file.Close()

	_, err = file.Write(logs)
	if err != nil {
		return helper.LogWarningf(ctx, "write container console log to file failed, error: %v", err)
	}

	return nil
}

func getPodFileLogPaths(pod *coreV1.Pod) (string, error) {
	for idx := range pod.Spec.Containers {
		logPath, err := getContainerFileLogPaths(&pod.Spec.Containers[idx])
		if err == nil {
			return logPath, err
		}
	}
	return "", helper.LogWarningf(context.Background(), "get pod file log paths failed, error: %v",
		errors.New("not found a available file log directory"))
}

func getContainerFileLogPaths(container *coreV1.Container) (string, error) {
	if container.Args == nil {
		return "", helper.LogWarningf(context.Background(), "get container file log paths failed, error: %v",
			errors.New("args is nil"))
	}
	for _, arg := range container.Args {
		if strings.HasPrefix(arg, "--log-file-dir=") {
			logPath := strings.Split(arg, "=")
			if len(logPath) != 2 {
				return "", helper.LogWarningf(context.Background(), "get container file log paths failed, error: %v",
					errors.New("log-file-dir is not set correctly"))
			}
			return logPath[1], nil
		}
	}

	return "", helper.LogWarningf(context.Background(), "get container file log paths failed, error: %v",
		errors.New("log-file-dir is not set"))
}
