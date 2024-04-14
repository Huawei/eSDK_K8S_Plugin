/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2023. All rights reserved.
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

package manage

import (
	"context"
	"errors"

	"github.com/container-storage-interface/spec/lib/go/csi"

	"huawei-csi-driver/connector"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
	"huawei-csi-driver/utils/taskflow"
)

// SanManager implements Manager interface
type SanManager struct {
	Conn     connector.Connector
	protocol string
}

// NewSanManager build a san manager instance according to the protocol
func NewSanManager(ctx context.Context, protocol string) (Manager, error) {
	var conn connector.Connector
	switch protocol {
	case "iscsi":
		conn = connector.GetConnector(ctx, connector.ISCSIDriver)
	case "fc":
		conn = connector.GetConnector(ctx, connector.FCDriver)
	case "roce":
		conn = connector.GetConnector(ctx, connector.RoCEDriver)
	case "fc-nvme":
		conn = connector.GetConnector(ctx, connector.FCNVMeDriver)
	case "scsi":
		conn = connector.GetConnector(ctx, connector.LocalDriver)
	default:
		return nil, utils.Errorf(ctx, "protocol: [%s] is not unsupported under san", protocol)
	}

	return &SanManager{Conn: conn, protocol: protocol}, nil
}

// StageVolume stage volume
func (m *SanManager) StageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) error {
	if err := CheckParam(ctx, req); err != nil {
		log.AddContext(ctx).Errorf("check san parameters filed, error: %v", err)
		return err
	}

	parameters, err := BuildParameters(
		WithProtocol(m.protocol),
		WithConnector(m.Conn),
		WithVolumeCapability(ctx, req),
		WithControllerPublishInfo(ctx, req),
		WithMultiPathType(m.protocol),
	)
	if err != nil {
		log.AddContext(ctx).Errorf("build san parameters filed, error: %v", err)
		return err
	}

	tasks := taskflow.NewTaskFlow(ctx, "StageVolume").
		AddTaskWithOutRevert(clearResidualPathWithWwn).
		AddTaskWithOutRevert(clearResidualPathWithLunId).
		AddTaskWithOutRevert(connectVolume)

	if volMode, exist := parameters["volumeMode"].(string); exist && volMode == "Block" {
		tasks = tasks.AddTaskWithOutRevert(stageForBlock)
	} else {
		tasks = tasks.AddTaskWithOutRevert(stageForMount)
	}

	return tasks.AddTaskWithOutRevert(saveWwnToDisk).
		RunWithOutRevert(parameters)
}

// UnStageVolume for block volumes, unstage needs to remove from the host
func (m *SanManager) UnStageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) error {
	volumeId := req.VolumeId
	targetPath := req.GetStagingTargetPath()

	wwn, err := getDeviceWwn(ctx, volumeId, targetPath, true, true)
	if err != nil || wwn == "" {
		// If the wwn doesn't exist, there is nothing we can do and a retry is unlikely to help, so return success.
		log.AddContext(ctx).Warningf("get device wwn failed while unstage volume, error: %v", err)
		return nil
	}

	if err = Unmount(ctx, targetPath); err != nil {
		log.AddContext(ctx).Errorf("umount target path failed while unstage volume, error: %v", err)
		return err
	}

	return m.UnStageWithWwn(ctx, wwn, volumeId)
}

// ExpandVolume return nil error if specified volume expand success
// If getting device wwn failed, return an error with call getDeviceWwn.
// If the device expand failed according to the specified wwn, return an error with call connector.ResizeBlock.
// If the volume capability is mount, will need to call connector.ResizeMountPath.
func (m *SanManager) ExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) error {
	capacityRange := req.GetCapacityRange()
	if capacityRange == nil || capacityRange.RequiredBytes <= 0 {
		return utils.Errorln(ctx, "NodeExpandVolume CapacityRange must be provided")
	}

	if req.GetVolumePath() == "" {
		return utils.Errorln(ctx, "NodeExpandVolume volumePath must be provided")
	}

	wwn, err := getDeviceWwn(ctx, req.GetVolumeId(), req.GetStagingTargetPath(), false, false)
	if err != nil {
		log.AddContext(ctx).Errorf("get device wwn failed while unstage volume, error: %v", err)
		return err
	}

	err = connector.ResizeBlock(ctx, wwn, capacityRange.RequiredBytes)
	if err != nil {
		log.AddContext(ctx).Errorf("Volume %s resize error: %v", req.GetVolumePath(), err)
		return err
	}

	if req.GetVolumeCapability().GetMount() != nil {
		err = connector.ResizeMountPath(ctx, req.GetVolumePath())
		if err != nil {
			log.AddContext(ctx).Errorf("MountPath %s resize error: %v", req.GetVolumePath(), err)
			return err
		}
	}

	return nil
}

// UnStageWithWwn unstage volume by wwn
func (m *SanManager) UnStageWithWwn(ctx context.Context, wwn, volumeId string) error {
	err := m.Conn.DisConnectVolume(ctx, wwn)
	if err != nil {
		log.AddContext(ctx).Errorf("disconnect volume failed while unstage volume,"+
			" wwn: %s, error: %v", wwn, err)
		return err
	}

	if err := utils.RemoveWwnFile(ctx, volumeId); err != nil {
		log.AddContext(ctx).Errorf("remove wwn file failed while unstage volume, "+
			"volumeId: %s, error: %v", volumeId, err)
	}
	return nil
}

func getDeviceWwn(ctx context.Context, volumeId, targetPath string,
	checkDevRef, saveToDisk bool) (string, error) {
	wwn, err := utils.ReadWwnFile(ctx, volumeId)
	if err != nil || wwn == "" {
		wwn, err = connector.GetWwnFromTargetPath(ctx, volumeId, targetPath, checkDevRef)
		if err != nil {
			log.AddContext(ctx).Errorf("get wwn form targetPath failed, error: %v",
				targetPath, err)
			return "", err
		}
		if saveToDisk {
			// For a mounted volume without wwn information in the disk. If the first unStage fails,
			// the targetPath may have been unmounted, when k8s retries call unStage. We will not
			// be able to obtain the wwn information from targetPath, so we need to write the wwn
			// information to the disk.
			if err = utils.WriteWWNFileIfNotExist(ctx, wwn, volumeId); err != nil {
				// If write wwn filed, there is nothing we can do and a retry is unlikely to help, because the mapping
				// information doesn't exist in /proc/mount file, so the error with call utils.WriteWWNFileIfNotExist
				// will not return
				log.AddContext(ctx).Warningf("write wwn file failed, wwn: %s, volumeId: %s error: %v",
					wwn, volumeId, err)
			}
		}
	}
	return wwn, nil
}

func saveWwnToDisk(ctx context.Context, parameters map[string]interface{}) error {
	wwn, err := ExtractWwn(parameters)
	if err != nil {
		log.AddContext(ctx).Errorf("extract wwn failed while save wwn to disk, error: %v", err)
		return err
	}

	volumeId, exist := parameters["volumeId"].(string)
	if !exist {
		return errors.New("volumeId doesn't exist while save wwn to disk")
	}

	err = utils.WriteWWNFile(ctx, wwn, volumeId)
	if err != nil {
		log.AddContext(ctx).Errorf("write wwn file failed while save wwn to disk, error: %v", err)
		return err
	}

	return nil
}

func clearResidualPathWithWwn(ctx context.Context, parameters map[string]interface{}) error {
	wwn, err := ExtractWwn(parameters)
	if err != nil {
		log.AddContext(ctx).Errorf("extract wwn failed while clear residual path, error: %v", err)
		return err
	}

	return connector.ClearResidualPath(ctx, wwn, parameters["volumeMode"])
}

func connectVolume(ctx context.Context, parameters map[string]interface{}) error {
	publishInfo, exist := parameters["publishInfo"].(*ControllerPublishInfo)
	if !exist {
		log.AddContext(ctx).Errorf("publishInfo not fount, publishInfo: %v", parameters["publishInfo"])
		return errors.New("publishInfo not fount while connect volume")
	}

	connectionParams := publishInfo.ReflectToMap()
	conn, exist := parameters["connector"].(connector.Connector)
	if !exist {
		return errors.New("connector doesn't exist while connect volume")
	}

	devPath, err := conn.ConnectVolume(ctx, connectionParams)
	if err != nil {
		return err
	}

	parameters["devPath"] = devPath
	return nil
}

// stageForMount when AccessType is csi.VolumeCapability_Mount, this function will be called to mount share path
func stageForMount(ctx context.Context, parameters map[string]interface{}) error {
	log.AddContext(ctx).Infoln("the request to stage filesystem device")

	connectInfo := map[string]interface{}{
		"fsType":     parameters["fsType"],
		"srcType":    connector.MountBlockType,
		"sourcePath": parameters["devPath"],
		"targetPath": parameters["targetPath"],
		"mountFlags": parameters["mountFlags"],
		"accessMode": parameters["accessMode"],
	}
	err := Mount(ctx, connectInfo)
	if err != nil {
		return err
	}

	return chmodFsPermission(ctx, parameters)
}

// stageForBlock when AccessType is csi.VolumeCapability_Block, this function will be called to create system link
func stageForBlock(ctx context.Context, parameters map[string]interface{}) error {
	log.AddContext(ctx).Infoln("the request to stage raw block device")

	mountPoint, exist := parameters["stagingPath"].(string)
	if !exist {
		return errors.New("stagingPath doesn't exist while stage for block")
	}

	devPath, exist := parameters["devPath"].(string)
	if !exist {
		return errors.New("device path doesn't exist while stage for block")
	}

	err := utils.CreateSymlink(ctx, devPath, mountPoint)
	if err != nil {
		log.AddContext(ctx).Errorln("create system link failed, error: %v", err)
		return err
	}

	return nil
}

func chmodFsPermission(ctx context.Context, parameters map[string]interface{}) error {
	fsPermission, exist := parameters["fsPermission"].(string)
	if !exist || fsPermission == "" {
		log.AddContext(ctx).Infoln("global mount directory permission dose not need to be modified")
		return nil
	}

	targetPath, exist := parameters["targetPath"].(string)
	if !exist || targetPath == "" {
		return errors.New("targetPath doesn't exist while chmod filesystem permission")
	}

	utils.ChmodFsPermission(ctx, targetPath, fsPermission)
	return nil
}

func clearResidualPathWithLunId(ctx context.Context, parameters map[string]interface{}) error {
	publishInfo, exist := parameters["publishInfo"].(*ControllerPublishInfo)
	if !exist {
		log.AddContext(ctx).Errorf("publishInfo not fount, publishInfo: %v", parameters["publishInfo"])
		return errors.New("publishInfo not fount while connect volume")
	}

	if !publishInfo.VolumeUseMultiPath || publishInfo.MultiPathType != connector.HWUltraPath {
		return nil
	}

	protocol, ok := parameters["protocol"]
	if !ok || (protocol != "iscsi" && protocol != "fc") {
		return nil
	}

	targets := publishInfo.TgtIQNs
	if protocol != "iscsi" {
		targets = publishInfo.TgtWWNs
	}

	err := connector.CleanDeviceByLunId(ctx, publishInfo.TgtHostLUNs[0], targets)
	if err != nil {
		log.AddContext(ctx).Infof("clean device by id failed,error:%v", err)
	}
	return nil
}
