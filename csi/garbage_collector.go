/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2022. All rights reserved.
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
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"huawei-csi-driver/connector/utils/lock"
	"huawei-csi-driver/csi/backend"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/k8sutils"
	"huawei-csi-driver/utils/log"
)

const (
	// pvDirPath is a relative path inside kubelet root directory
	relativePvPath = "/kubelet/plugins/kubernetes.io/csi/pv/*/vol_data.json"
	// in case of file system,the index of the last occurrence of the specified pv name,
	// For example,the path is "/var/lib/kubelet/plugins/kubernetes.io/csi/pv/pvc-123/vol_data.json", we get pvc-123
	pvLastIndex = 2

	// deviceDirPath is a relative path inside kubelet root directory
	relativeDevicePath = "/kubelet/plugins/kubernetes.io/csi/volumeDevices/*/data/vol_data.json"
	// in case of block,the index of the last occurrence of the specified pv name
	//For example,the path is "/var/lib/kubelet/plugins/kubernetes.io/csi/volumeDevices/pvc-123/data/vol_data.json", we get pvc-123
	deviceLastIndex = 3
)

// PVFileData  represents volume handle and the driver name which created it
type PVFileData struct {
	VolumeHandle string `json:"volumeHandle"` // volume handle
	DriverName   string `json:"driverName"`   // driver name
}

// NodePVData represents volume related info
type NodePVData struct {
	VolumeHandle string
	VolumeName   string
}

// pvPathInfo pv path info
type pvPathInfo struct {
	pvFilePath string
	VolumeName string
}

// nodeStaleDeviceCleanup checks volumes at node and k8s side and triggers cleanup for state devices
func nodeStaleDeviceCleanup(ctx context.Context, k8sUtils k8sutils.Interface, kubeletRootDir string,
	driverName string, nodeName string) error {
	log.AddContext(ctx).Debugf("Enter func nodeStaleDeviceCleanup.")
	allPathInfos, err := getAllPathInfos(kubeletRootDir)
	if err != nil {
		log.AddContext(ctx).Errorln(err)
		return err
	}
	nodeVolumes := getNodeVolumes(ctx, allPathInfos, driverName)
	// If there are any volume files on node, go for stale device cleanup
	if len(nodeVolumes) > 0 {
		// Get all volumes belonging to this node from K8S side
		k8sVolumes, err := k8sUtils.GetVolume(ctx, nodeName, driverName)
		if err != nil {
			log.AddContext(ctx).Errorln(err)
			return err
		}
		checkAndClearStaleDevices(ctx, k8sUtils, k8sVolumes, nodeVolumes)
	}
	return nil
}

func getAllPathInfos(kubeletRootDir string) ([]pvPathInfo, error) {
	var allPathInfos []pvPathInfo

	// get pv path information under the file system
	pvPathInfos, err := getPvPathInfo(path.Join(kubeletRootDir, relativePvPath), pvLastIndex)
	if err != nil {
		return nil, err
	}
	allPathInfos = append(allPathInfos, pvPathInfos...)

	// get pv path information under the block
	devicePathInfos, err := getPvPathInfo(path.Join(kubeletRootDir, relativeDevicePath), deviceLastIndex)
	if err != nil {
		return nil, err
	}
	allPathInfos = append(allPathInfos, devicePathInfos...)
	return allPathInfos, nil
}

// getPvPathInfo Parse the pv information according to the absolute path,
// the lastIndex parameter is used to determine the index of the last occurrence of the specified pv name
func getPvPathInfo(absPvFilePath string, lastIndex int) ([]pvPathInfo, error) {
	var pvPathInfos []pvPathInfo
	pvFilePaths, err := filepath.Glob(absPvFilePath)
	if err != nil {
		return nil, err
	}
	for _, filePath := range pvFilePaths {
		dirNameList := strings.Split(filePath, string(os.PathSeparator))
		info := pvPathInfo{
			pvFilePath: filePath,
			VolumeName: dirNameList[len(dirNameList)-lastIndex],
		}
		pvPathInfos = append(pvPathInfos, info)
	}
	return pvPathInfos, nil
}

// getNodeVolumes extracts all volume handles using node pv file
func getNodeVolumes(ctx context.Context, pvPathInfos []pvPathInfo, driverName string) []NodePVData {
	var nodePVs []NodePVData
	for _, pvPath := range pvPathInfos {
		pvFileData, err := loadPVFileData(ctx, pvPath.pvFilePath)
		if err != nil {
			log.AddContext(ctx).Errorf("Failed to load volume data from %s, %s", pvPath.pvFilePath, err.Error())
			continue
		}
		if pvFileData == nil {
			log.AddContext(ctx).Infof("Missing volume data in [%s], skip processing", pvPath.pvFilePath)
			continue
		}
		// Skip the volumes created by other csi drivers
		if driverName != pvFileData.DriverName {
			log.AddContext(ctx).Infof("Volume belongs to the other driver [%s], skipped", pvFileData.DriverName)
			continue
		}
		nodePV := NodePVData{
			VolumeHandle: pvFileData.VolumeHandle,
			VolumeName:   pvPath.VolumeName,
		}
		nodePVs = append(nodePVs, nodePV)
	}

	log.AddContext(ctx).Infof("PV list from node side for this node:  %v", nodePVs)
	return nodePVs
}

func loadPVFileData(ctx context.Context, dataFilePath string) (*PVFileData, error) {
	// Check if the node pv data directory exists
	exists, err := utils.PathExist(dataFilePath)

	if err != nil {
		return nil, err
	}
	if !exists {
		log.AddContext(ctx).Infof("Volume data file %s does not exist. Returning here", dataFilePath)
		return nil, nil
	}
	file, err := os.Open(dataFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open volume data file [%s], %v", dataFilePath, err)
	}
	defer file.Close()
	data := PVFileData{}
	if err := json.NewDecoder(file).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to parse data file [%s], %v", dataFilePath, err)
	}
	log.AddContext(ctx).Infof("Data file [%s] loaded successfully", dataFilePath)
	return &data, nil
}

// isNodePvValid checks if node pv file still has valid usage on node
func isNodePvValid(nodePVVolumeHandle string, k8sVolumes map[string]struct{}) bool {
	_, isPresent := k8sVolumes[nodePVVolumeHandle]
	return isPresent
}

func cleanStaleDevicesWithRetry(ctx context.Context, retry int, volumeHandle, lunWWN string,
	staleDeviceCleanupChan chan struct{}) {
	for i := 0; i < retry; i++ {
		err := cleanStaleDevices(ctx, volumeHandle, lunWWN)
		if err != nil {
			if strings.Contains(err.Error(), lock.GetSemaphoreTimeout) ||
				strings.Contains(err.Error(), lock.GetLockTimeout) {
				log.AddContext(ctx).Warningf("Cleanup volume [%s] timeout, error: %v", volumeHandle, err)
				continue
			}
			log.AddContext(ctx).Warningf("Cleanup failed for the volume [%s], error: %v", volumeHandle, err)
		}
		break
	}
	staleDeviceCleanupChan <- struct{}{}
}

// checkAndClearStaleDevices checks and triggers cleanup if needed
func checkAndClearStaleDevices(ctx context.Context, k8sUtils k8sutils.Interface, k8sVolumes map[string]struct{},
	nodePVs []NodePVData) {
	var staleVolumesCnt int
	staleDeviceCleanupChan := make(chan struct{})
	defer close(staleDeviceCleanupChan)

	retry := *deviceCleanupTimeout / lock.GetLockTimeoutSec
	log.AddContext(ctx).Debugf("Cleanup timeout: [%d], Get lock timeout: [%d], Retry times: [%d].",
		*deviceCleanupTimeout, lock.GetLockTimeoutSec, retry)
	for _, nodePV := range nodePVs {
		if isNodePvValid(nodePV.VolumeHandle, k8sVolumes) {
			continue
		}
		lunWWN := ""
		volumeAttr, err := k8sUtils.GetVolumeAttributes(ctx, nodePV.VolumeName)
		if err == nil {
			lunWWN = volumeAttr["lunWWN"]
		}

		staleVolumesCnt++
		go cleanStaleDevicesWithRetry(ctx, retry, nodePV.VolumeHandle, lunWWN, staleDeviceCleanupChan)
	}

	for i := 0; i < staleVolumesCnt; i++ {
		<-staleDeviceCleanupChan
	}

	log.AddContext(ctx).Infof("Clear stale device count: %d", staleVolumesCnt)
	return
}

func cleanStaleDevices(ctx context.Context, volumeHandle, lunWWN string) error {
	log.AddContext(ctx).Infof("Start to clean stale devices for the volume %s lunWWN %s", volumeHandle, lunWWN)
	var err error
	backendName, volName := utils.SplitVolumeId(volumeHandle)
	backend := backend.GetBackend(backendName)
	if backend == nil {
		log.AddContext(ctx).Warningf("Backend [%s] doesn't exist.", backendName)
		return nil
	}

	// Based on lunWWN availability, perform node side cleanup
	if lunWWN != "" {
		log.AddContext(ctx).Debugf("Unstage volume [%s] with WWN [%s].", volName, lunWWN)
		err = backend.Plugin.UnstageVolumeWithWWN(ctx, lunWWN)
	} else {
		log.AddContext(ctx).Debugf("Unstage volume [%s].", volName)
		parameters := map[string]interface{}{
			"targetPath": "",
		}
		err = backend.Plugin.UnstageVolume(ctx, volName, parameters)
	}

	if err != nil {
		return err
	}
	log.AddContext(ctx).Infof("Cleanup stale devices completed for the volume %s", volumeHandle)
	return nil
}
