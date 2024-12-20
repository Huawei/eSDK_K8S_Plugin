/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2024-2024. All rights reserved.
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

// Package provider is related with volume
package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/lib/drcsi"
	pkgUtils "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/volume"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	thinVolumeRequestSize = 0
)

// ModifyVolume is used to modify volume attribute
func (p *StorageProvider) ModifyVolume(ctx context.Context, req *drcsi.ModifyVolumeRequest) (
	*drcsi.ModifyVolumeResponse, error) {

	defer utils.RecoverPanic(ctx)
	log.AddContext(ctx).Infof("Modify volume: %s, MutableParameters: %v", req.VolumeId, req.MutableParameters)

	// Other modification operations are extended in a similar way.
	ret, err := p.modifyHyperMetro(ctx, req)
	if err != nil {
		return ret, err
	}

	return &drcsi.ModifyVolumeResponse{}, nil
}

func (p *StorageProvider) modifyHyperMetro(ctx context.Context, req *drcsi.ModifyVolumeRequest) (
	*drcsi.ModifyVolumeResponse, error) {

	// Determine whether the operation is a conversion between a local volume and a HyperMetro volume.
	val, exist := req.MutableParameters["hyperMetro"]
	if !exist {
		return &drcsi.ModifyVolumeResponse{}, nil
	}

	// Local2HyperMetro or HyperMetro2Local
	var modifyType volume.ModifyVolumeType
	if val == "true" {
		modifyType = volume.Local2HyperMetro
	} else if val == "false" {
		modifyType = volume.HyperMetro2Local
	} else {
		errMsg := fmt.Sprintf("hyperMetro value must be \"true\" or \"false\", \"%v\" is invalid.", val)
		log.AddContext(ctx).Errorln(errMsg)
		return nil, errors.New(errMsg)
	}

	// Invoke the corresponding plug-in to modify the volume.
	backendName, volumeName := utils.SplitVolumeId(req.VolumeId)
	bk, err := p.backendSelector.SelectBackend(ctx, backendName)
	if err != nil {
		errMsg := fmt.Sprintf("select backend failed, backend name: %s, error: %v", backendName, err)
		log.AddContext(ctx).Errorln(errMsg)
		return nil, errors.New(errMsg)
	}
	if bk.MetroBackend == nil {
		errMsg := "have not configured hyper metro backend"
		log.AddContext(ctx).Errorln(errMsg)
		return nil, errors.New(errMsg)
	}

	params := pkgUtils.CombineMap(req.MutableParameters, req.StorageClassParameters)
	remotePool, err := p.backendSelector.SelectRemotePool(ctx, thinVolumeRequestSize, backendName,
		pkgUtils.ConvertMapString2MapInterface(params))
	if err != nil {
		errMsg := fmt.Sprintf("select remote pool failed, backend name: %s, error: %v", backendName, err)
		log.AddContext(ctx).Errorln(errMsg)
		return nil, errors.New(errMsg)
	}

	if remotePool != nil {
		params["remoteStoragePool"] = remotePool.Name
	}
	err = bk.Plugin.ModifyVolume(ctx, req.VolumeId, modifyType, params)
	if err != nil {
		errMsg := fmt.Sprintf("modify volume failed, volume name: %s, modify type: %v, error: %v",
			volumeName, modifyType, err)
		log.AddContext(ctx).Errorln(errMsg)
		return nil, errors.New(errMsg)
	}

	return &drcsi.ModifyVolumeResponse{}, nil
}
