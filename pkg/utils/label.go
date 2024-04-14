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

// Package utils is label-related function and method.
package utils

import (
	"context"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"

	xuanwuv1 "huawei-csi-driver/client/apis/xuanwu/v1"
	"huawei-csi-driver/csi/app"
	"huawei-csi-driver/pkg/constants"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

// GetTopoName get topo name
func GetTopoName(pvName string) string {
	return fmt.Sprintf("topo-%s", strings.ReplaceAll(pvName, "_", "-"))
}

// CreatePVLabel create label when create pvc
var CreatePVLabel = func(pvName, volumeId string) {
	ctx := utils.NewContextWithRequestID()
	backendName, _ := utils.SplitVolumeId(volumeId)
	backendName = MakeMetaWithNamespace(app.GetGlobalConfig().Namespace, backendName)

	supportLabel, err := IsBackendCapabilitySupport(ctx, backendName, constants.SupportLabel)
	if err != nil {
		log.AddContext(ctx).Errorf("CreatePVLabel get backend capability support failed,"+
			" backendName: %v, label: %v, err: %v", backendName, supportLabel, err)
	}
	if supportLabel {
		CreateLabel(pvName, volumeId)
	}
}

// CreateLabel used to create ResourceTopology resource
var CreateLabel = func(pvName, volumeId string) {
	ctx := utils.NewContextWithRequestID()
	var err error
	_, volumeName := utils.SplitVolumeId(volumeId)
	topologyName := GetTopoName(volumeName)

	topologySpec := xuanwuv1.ResourceTopologySpec{
		Provisioner:  constants.DefaultTopoDriverName,
		VolumeHandle: volumeId,
		Tags: []xuanwuv1.Tag{
			{
				ResourceInfo: xuanwuv1.ResourceInfo{
					TypeMeta: metav1.TypeMeta{Kind: constants.PVKind, APIVersion: constants.KubernetesV1},
					Name:     pvName,
				},
			},
		},
	}

	_, err = app.GetGlobalConfig().BackendUtils.XuanwuV1().ResourceTopologies().Create(ctx,
		&xuanwuv1.ResourceTopology{
			TypeMeta:   metav1.TypeMeta{Kind: constants.TopologyKind, APIVersion: constants.XuanwuV1},
			ObjectMeta: metav1.ObjectMeta{Name: topologyName},
			Spec:       topologySpec,
		},
		metav1.CreateOptions{})

	if err != nil {
		log.AddContext(ctx).Errorf("Create topologies for pv: %s failed. error: %v", volumeName, err)
	} else {
		log.AddContext(ctx).Infof("Create topologies: %s success.", topologyName)
	}
}

// DeletePVLabel delete label when delete pvc
var DeletePVLabel = func(volumeId string) {
	ctx := utils.NewContextWithRequestID()
	backendName, volName := utils.SplitVolumeId(volumeId)

	backendName = MakeMetaWithNamespace(app.GetGlobalConfig().Namespace, backendName)
	supportLabel, err := IsBackendCapabilitySupport(ctx, backendName, constants.SupportLabel)
	if err != nil {
		log.AddContext(ctx).Errorf("DeletePVLabel get backend capability support failed,"+
			" backendName: %v, label: %v, err: %v", backendName, supportLabel, err)
	}
	if supportLabel {
		DeleteLabel(volName)
	}
}

// DeleteLabel used to delete label resource
var DeleteLabel = func(volName string) {
	err := app.GetGlobalConfig().BackendUtils.XuanwuV1().ResourceTopologies().Delete(context.TODO(),
		GetTopoName(volName), metav1.DeleteOptions{})
	if err != nil {
		log.Errorf("Delete topologies: %s failed. error: %v", GetTopoName(volName), err)
	} else {
		log.Infof("Delete topologies: %s success.", GetTopoName(volName))
	}
}
