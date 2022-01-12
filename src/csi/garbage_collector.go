/*
 Copyright (c) Huawei Technologies Co., Ltd. 2021-2021. All rights reserved.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at
      http://www.apache.org/licenses/LICENSE-2.0
 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package main

import (
	"csi/backend"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"utils"
	"utils/k8sutils"
	"utils/log"
)

const (
	// volumeDataFileName refers the volume data file maintained by kubelet on node
	volumeDataFileName = "vol_data.json"

	// pvDirPath is a relative path inside kubelet root directory
	relativePvDirPath = "/kubelet/plugins/kubernetes.io/csi/pv/"
)

// PVFileData represents volume handle and the driver name which created it
type PVFileData struct {
	VolumeHandle string `json:"volumeHandle"` // volume handle
	DriverName   string `json:"driverName"`   // driver name
}

// NodePVData represents volume related info
type NodePVData struct {
	VolumeHandle string
	VolumeName   string
}

// nodeStaleDeviceCleanup checks volumes at node and k8s side and triggers cleanup for state devices
func nodeStaleDeviceCleanup(k8sUtils k8sutils.Interface, kubeletRootDir string,
	driverName string, nodeName string) error {
	nodeVolumes, err := getNodeVolumes(kubeletRootDir, driverName)
	if err != nil {
		log.Errorln(err)
		return err
	}

	// If there are any volume files on node, go for stale device cleanup
	if len(nodeVolumes) > 0 {
		// Get all volumes belonging to this node from K8S side
		k8sVolumes, err := k8sUtils.GetVolume(nodeName, driverName)
		if err != nil {
			log.Errorln(err)
			return err
		}
		checkAndClearStaleDevices(k8sUtils, k8sVolumes, nodeVolumes)
	}

	return nil
}

// dirExists checks if path exists, and it is a directory or not
func dirExists(path string) (bool, bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, false, nil
		}
		return true, false, err
	}
	return true, info.IsDir(), nil
}

// getNodeVolumes extracts all volume handles using node pv files
func getNodeVolumes(kubeletRootDir string, driverName string) ([]NodePVData, error) {
	absPvDirPath := kubeletRootDir + relativePvDirPath

	// Check if pv path exists
	exists, isDir, err := dirExists(absPvDirPath)
	if err != nil {
		return nil, err
	}
	if !exists || !isDir {
		return nil, fmt.Errorf("Path [%s] does not exist or not a directory", absPvDirPath)
	}

	var nodePVs []NodePVData
	err = filepath.Walk(absPvDirPath, func(fileFullPath string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			log.Errorf("Error while processing the path [%s], %s", fileFullPath, walkErr.Error())
			return walkErr
		}
		if info == nil {
			log.Infof("FileInfo is nil, Skipping directory path [%s]", fileFullPath)
			return filepath.SkipDir /* Skip processing current directory and continue processing other directories */
		}

		// Process only 'vol_data.json' files
		if info.IsDir() || info.Name() != volumeDataFileName {
			return nil
		}

		targetDirPath := filepath.Dir(fileFullPath)

		pvFileData, err := loadPVFileDataData(targetDirPath, volumeDataFileName)
		if err != nil {
			log.Errorf("Failed to load volume data from %s, %s", volumeDataFileName, err.Error())
			return nil
		}
		if pvFileData == nil {
			log.Infof("Missing volume data in %s, skip processing", fileFullPath)
			return nil
		}

		// Skip the volumes created by other csi drivers
		if driverName != pvFileData.DriverName {
			log.Infof("Volume belongs to the other driver %s, skipped", pvFileData.DriverName)
			return nil
		}
		pvName := filepath.Base(targetDirPath)
		nodePV := NodePVData{
			VolumeHandle:pvFileData.VolumeHandle,
			VolumeName:pvName,
		}
		nodePVs = append(nodePVs, nodePV)

		return nil
	})

	log.Infof("PV list from node side for this node:  %v", nodePVs)
	return nodePVs, err
}

// loadPVFileDataData loads pv data file from input path
func loadPVFileDataData(dir string, fileName string) (*PVFileData, error) {
	dataFilePath := filepath.Join(dir, fileName)

	// Check if the node pv data directory exists
	exists, err := utils.PathExist(dataFilePath)
	if err != nil {
		return nil, err
	}
	if !exists {
		log.Infof("Volume data file %s does not exist. Returning here", dataFilePath)
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
	log.Infof("Data file [%s] loaded successfully", dataFilePath)
	return &data, nil
}

// isNodePvValid checks if node pv file still has valid usage on node
func isNodePvValid(nodePVVolumeHandle string, k8sVolumes map[string]struct{}) bool {
	_, isPresent := k8sVolumes[nodePVVolumeHandle]
	return isPresent
}

// checkAndClearStaleDevices checks and triggers cleanup if needed
func checkAndClearStaleDevices(k8sUtils k8sutils.Interface, k8sVolumes map[string]struct{}, nodePVs []NodePVData) {
	var staleVolumesCnt int
	staleDeviceCleanupChan := make(chan struct{})

	defer close(staleDeviceCleanupChan)

	for _, nodePV := range nodePVs {
		if isNodePvValid(nodePV.VolumeHandle, k8sVolumes) {
			continue
		}
		lunWWN := ""
		volumeAttr, err := k8sUtils.GetVolumeAttributes(nodePV.VolumeName)
		if nil == err {
			lunWWN = volumeAttr["lunWWN"]
		}
		staleVolumesCnt++
		go func(volumeHandle, lunWWN string) {
			err := cleanStaleDevices(volumeHandle, lunWWN)
			if err != nil {
				log.Warningf("Cleanup failed for the volume %s error: %v", volumeHandle, err)
			}
			staleDeviceCleanupChan <- struct{}{}
		}(nodePV.VolumeHandle, lunWWN)
	}

	for i:=0; i<staleVolumesCnt; i++ {
		<-staleDeviceCleanupChan
	}

	return
}

func cleanStaleDevices(volumeHandle, lunWWN string) error {
	log.Infof("Start to clean stale devices for the volume %s lunWWN %s", volumeHandle, lunWWN)

	var err error
	backendName, volName := utils.SplitVolumeId(volumeHandle)
	backend := backend.GetBackend(backendName)
	if backend == nil {
		msg := fmt.Sprintf("Backend %s doesn't exist", backendName)
		log.Warningln(msg)
		return nil
	}

	// Based on lunWWN availability, perform node side cleanup
	if lunWWN != "" {
		err = backend.Plugin.UnstageVolumeWithWWN(lunWWN)
	} else {
		parameters := map[string]interface{}{
			"targetPath": "",
		}
		err = backend.Plugin.UnstageVolume(volName, parameters)
	}
	if err != nil {
		return err
	}

	log.Infof("Cleanup stale devices completed for the volume %s", volumeHandle)
	return nil
}
