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

package driver

import (
	"context"
	"errors"
	"strings"

	coreV1 "k8s.io/api/core/v1"
	k8sError "k8s.io/apimachinery/pkg/api/errors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xuanwuV1 "huawei-csi-driver/client/apis/xuanwu/v1"
	"huawei-csi-driver/csi/app"
	"huawei-csi-driver/pkg/constants"
	pkgUtils "huawei-csi-driver/pkg/utils"
	labelLock "huawei-csi-driver/pkg/utils/label_lock"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

type deleteTopologiesLabelParam struct {
	topo       *xuanwuV1.ResourceTopology
	pods       []coreV1.Pod
	pvName     string
	targetPath string
	podName    string
	namespace  string
	volumeName string
	topoName   string
}

func nodeAddLabel(ctx context.Context, volumeID, targetPath string) {
	backendName, _ := utils.SplitVolumeId(volumeID)
	backendName = pkgUtils.MakeMetaWithNamespace(app.GetGlobalConfig().Namespace, backendName)

	supportLabel, err := pkgUtils.IsBackendCapabilitySupport(ctx, backendName, constants.SupportLabel)
	if err != nil {
		log.AddContext(ctx).Errorf("IsBackendCapabilitySupport failed, backendName: %v, label: %v, err: %v",
			backendName, supportLabel, err)
		return
	}
	if supportLabel {
		if err := addPodLabel(ctx, volumeID, targetPath); err != nil {
			log.AddContext(ctx).Errorf("nodeAddLabel failed, err: %v", err)
		}
	}
}

func nodeDeleteLabel(ctx context.Context, volumeID, targetPath string) {
	backendName, _ := utils.SplitVolumeId(volumeID)
	backendName = pkgUtils.MakeMetaWithNamespace(app.GetGlobalConfig().Namespace, backendName)

	supportLabel, err := pkgUtils.IsBackendCapabilitySupport(ctx, backendName, constants.SupportLabel)
	if err != nil {
		log.AddContext(ctx).Errorf("IsBackendCapabilitySupport failed, backendName: %v, label: %v, err: %v",
			backendName, supportLabel, err)
		return
	}
	if supportLabel {
		if err := deletePodLabel(ctx, volumeID, targetPath); err != nil {
			log.AddContext(ctx).Errorf("nodeDeleteLabel failed, err: %v", err)
		}
	}
}

func addPodLabel(ctx context.Context, volumeID, targetPath string) error {
	_, volumeName := utils.SplitVolumeId(volumeID)
	topoName := pkgUtils.GetTopoName(volumeName)
	_, podName, namespace, sc, pvName, err := getTargetPathPodRelateInfo(ctx, targetPath)
	if err != nil {
		log.AddContext(ctx).Errorf("get podName failed, pvName: %v targetPath: %v err: %v",
			pvName, targetPath, err)
		return err
	}
	if podName == "" {
		log.AddContext(ctx).Errorf("get podName failed, target pod not exist, targetPath: %v err: %v",
			targetPath, err)
		return err
	}
	if sc == "" {
		log.AddContext(ctx).Infof("addLabel static pv, volumeID: %v, targetPath: %v", volumeID, targetPath)
		return nil
	}

	// lock for rt name
	if err = labelLock.AcquireCmLock(ctx, labelLock.RTLockConfigMap, topoName); err != nil {
		log.AddContext(ctx).Errorf("acquire rt lock failed, key: %s err: %v", topoName, err)
		return err
	}
	defer func(ctx context.Context, lockKey string) {
		if err = labelLock.ReleaseCmlock(ctx, labelLock.RTLockConfigMap, lockKey); err != nil {
			log.AddContext(ctx).Errorf("release rt lock failed, key: %s err: %v", lockKey, err)
		}
	}(ctx, topoName)

	return addTopologiesLabel(ctx, volumeID, targetPath, namespace, podName, pvName)
}

func addTopologiesLabel(ctx context.Context, volumeID, targetPath, namespace, podName, pvName string) error {
	_, volumeName := utils.SplitVolumeId(volumeID)
	topoName := pkgUtils.GetTopoName(volumeName)
	topo, err := app.GetGlobalConfig().BackendUtils.XuanwuV1().ResourceTopologies().Get(ctx,
		topoName, metaV1.GetOptions{})
	log.AddContext(ctx).Debugf("get topo info, topo: %+v, err: %v, notFound: %v", topo,
		err, k8sError.IsNotFound(err))
	if k8sError.IsNotFound(err) {
		_, err = app.GetGlobalConfig().BackendUtils.XuanwuV1().ResourceTopologies().Create(ctx,
			formatTopologies(volumeID, namespace, podName, pvName, topoName), metaV1.CreateOptions{})
		if err != nil {
			log.AddContext(ctx).Errorf("create label failed, data: %+v volumeName: %v targetPath: %v",
				topo, volumeName, targetPath)
			return err
		}
		log.AddContext(ctx).Infof("node create label success, data: %+v volumeName: %v targetPath: %v",
			topo, volumeName, targetPath)
		return nil
	}
	if err != nil {
		log.AddContext(ctx).Errorf("get topo failed, topoName: %v err: %v", topoName, err)
		return err
	}

	log.AddContext(ctx).Debugf("before add rt: %s podName: %s len: %v", topoName, podName, len(topo.Spec.Tags))
	// if pvc label not exist then add & add new pod label
	addPodTopoItem(&topo.Spec.Tags, podName, namespace, volumeName)
	log.AddContext(ctx).Debugf("after add rt: %s podName: %s len: %v", topoName, podName, len(topo.Spec.Tags))

	// update topo
	_, err = app.GetGlobalConfig().BackendUtils.XuanwuV1().ResourceTopologies().Update(ctx,
		topo, metaV1.UpdateOptions{})
	if err != nil {
		log.AddContext(ctx).Errorf("node add label failed, data:%+v, volumeName: %v targetPath: %v err: %v",
			topo, volumeName, targetPath, err)
		return err
	}

	log.AddContext(ctx).Infof("node add label success, data: %+v volumeName: %v targetPath: %v",
		topo, volumeName, targetPath)
	return nil
}

func formatTopologies(volumeID, namespace, podName, pvName, topoName string) *xuanwuV1.ResourceTopology {
	topologySpec := xuanwuV1.ResourceTopologySpec{
		Provisioner:  constants.DefaultTopoDriverName,
		VolumeHandle: volumeID,
		Tags: []xuanwuV1.Tag{
			{
				ResourceInfo: xuanwuV1.ResourceInfo{
					TypeMeta: metaV1.TypeMeta{Kind: constants.PVKind, APIVersion: constants.KubernetesV1},
					Name:     pvName,
				},
			},
			{
				ResourceInfo: xuanwuV1.ResourceInfo{
					TypeMeta:  metaV1.TypeMeta{Kind: constants.PodKind, APIVersion: constants.KubernetesV1},
					Name:      podName,
					Namespace: namespace,
				},
			},
		},
	}
	return &xuanwuV1.ResourceTopology{
		TypeMeta:   metaV1.TypeMeta{Kind: constants.TopologyKind, APIVersion: constants.XuanwuV1},
		ObjectMeta: metaV1.ObjectMeta{Name: topoName},
		Spec:       topologySpec,
	}
}

func addPodTopoItem(tags *[]xuanwuV1.Tag, podName, namespace, volumeName string) {
	// if pvc label not exist then add
	var existPvLabel bool
	currentPodMap := make(map[string]bool)
	for _, tag := range *tags {
		currentPodMap[tag.Name] = true
		if tag.Kind == constants.PVKind {
			existPvLabel = true
		}
	}
	if !existPvLabel {
		*tags = append(*tags, xuanwuV1.Tag{
			ResourceInfo: xuanwuV1.ResourceInfo{
				TypeMeta: metaV1.TypeMeta{Kind: constants.PVKind, APIVersion: constants.KubernetesV1},
				Name:     volumeName,
			},
		})
	}

	// add pod label
	if _, ok := currentPodMap[podName]; !ok {
		*tags = append(*tags, xuanwuV1.Tag{
			ResourceInfo: xuanwuV1.ResourceInfo{
				TypeMeta:  metaV1.TypeMeta{Kind: constants.PodKind, APIVersion: constants.KubernetesV1},
				Name:      podName,
				Namespace: namespace,
			},
		})
	}
}

func deletePodLabel(ctx context.Context, volumeID, targetPath string) error {
	var err error
	_, volumeName := utils.SplitVolumeId(volumeID)
	topoName := pkgUtils.GetTopoName(volumeName)

	// lock for rt name
	if err = labelLock.AcquireCmLock(ctx, labelLock.RTLockConfigMap, topoName); err != nil {
		log.AddContext(ctx).Errorf("acquire rt lock failed, key: %s err: %v", topoName, err)
		return err
	}
	defer func(ctx context.Context, lockKey string) {
		if err = labelLock.ReleaseCmlock(ctx, labelLock.RTLockConfigMap, lockKey); err != nil {
			log.AddContext(ctx).Errorf("release rt lock failed, key: %s err: %v", lockKey, err)
		}
	}(ctx, topoName)

	var topo *xuanwuV1.ResourceTopology
	var flag bool
	if topo, flag, err = checkRTPodDeletedAndGet(ctx, topoName); err != nil {
		log.AddContext(ctx).Infof("check pod tag has been deleted failed, topoName: %s, err: %v, topoName, err")
		return err
	}
	if flag {
		log.AddContext(ctx).Infof("topo pod tag has been deleted, topoName: %s", topoName)
		return nil
	}

	pods, podName, namespace, sc, pvName, err := getTargetPathPodRelateInfo(ctx, targetPath)
	if err != nil {
		log.AddContext(ctx).Errorf("get targetPath pvRelateInfo failed, targetPath: %v, err: %v", targetPath, err)
		return err
	}
	if sc == "" {
		log.AddContext(ctx).Infof("deleteLabel static pv, volumeID: %v, targetPath: %v", volumeID, targetPath)
		return nil
	}

	return deleteTopologiesLabel(ctx, deleteTopologiesLabelParam{
		topo:       topo,
		pods:       pods,
		pvName:     pvName,
		targetPath: targetPath,
		podName:    podName,
		namespace:  namespace,
		volumeName: volumeName,
		topoName:   topoName,
	})
}

func checkRTPodDeletedAndGet(ctx context.Context, topoName string) (*xuanwuV1.ResourceTopology, bool, error) {
	topo, err := app.GetGlobalConfig().BackendUtils.XuanwuV1().ResourceTopologies().Get(ctx, topoName,
		metaV1.GetOptions{})
	if k8sError.IsNotFound(err) {
		log.AddContext(ctx).Infof("node delete label success, topo not found, "+
			"data: %+v topoName: %s", topo, topoName)
		return topo, false, err
	}
	if err != nil {
		log.AddContext(ctx).Errorf("get topo failed, topoName: %v, err: %v", topoName, err)
		return topo, false, err
	}
	if topo == nil {
		log.AddContext(ctx).Errorf("get nil topo, topoName: %v", topoName)
		return topo, false, err
	}

	for _, item := range topo.Spec.Tags {
		if item.Kind == constants.PodKind {
			return topo, false, nil
		}
	}
	return topo, true, nil
}

func deleteTopologiesLabel(ctx context.Context, param deleteTopologiesLabelParam) error {
	// filter pod
	log.AddContext(ctx).Debugf("before delete rt: %s podName: %s len: %v", param.topoName, param.podName,
		len(param.topo.Spec.Tags))
	param.topo.Spec.Tags = filterDeleteTopoTag(param)
	log.AddContext(ctx).Debugf("after delete rt: %s podName: %s len: %v", param.topoName, param.podName,
		len(param.topo.Spec.Tags))

	// update topo
	_, err := app.GetGlobalConfig().BackendUtils.XuanwuV1().ResourceTopologies().Update(ctx,
		param.topo, metaV1.UpdateOptions{})
	if err != nil {
		log.AddContext(ctx).Errorf("node delete label failed, data:%+v, volumeName: %v targetPath: %v err: %v",
			param.topo, param.volumeName, param.targetPath, err)
		return err
	}

	log.AddContext(ctx).Infof("node delete label success, data: %+v volumeName: %v targetPath: %v",
		param.topo, param.volumeName, param.targetPath)
	return nil
}

func filterDeleteTopoTag(param deleteTopologiesLabelParam) []xuanwuV1.Tag {
	var newTags []xuanwuV1.Tag
	var containsPod = func(tag xuanwuV1.Tag, podItems []coreV1.Pod) bool {
		for _, item := range podItems {
			if tag.Name == item.Name && tag.Namespace == item.Namespace {
				return true
			}
		}
		return false
	}
	for _, tag := range param.topo.Spec.Tags {
		if tag.Kind != constants.PodKind {
			newTags = append(newTags, tag)
			continue
		}

		if tag.Kind == constants.PodKind && tag.Name == param.podName && tag.Namespace == param.namespace {
			continue
		}

		if containsPod(tag, param.pods) {
			newTags = append(newTags, tag)
		}
	}

	return newTags
}

// getTargetPathPodRelateInfo get podName nameSpace sc volumeName
func getTargetPathPodRelateInfo(ctx context.Context, targetPath string) ([]coreV1.Pod,
	string, string, string, string, error) {
	k8sAPI := app.GetGlobalConfig().K8sUtils

	targetPathArr := strings.Split(targetPath, "/")
	if len(targetPathArr) < 2 {
		return nil, "", "", "", "", pkgUtils.Errorf(ctx, "targetPath: %s is invalid", targetPath)
	}
	volumeName := targetPathArr[len(targetPathArr)-2]

	log.AddContext(ctx).Debugf("targetPath: %v, volumeName: %v", targetPath, volumeName)

	// get pv info
	pv, err := k8sAPI.GetPVByName(ctx, volumeName)
	if err != nil {
		log.AddContext(ctx).Errorf("get pv failed, pvName: %v, err: %v", volumeName, err)
		return nil, "", "", "", volumeName, err
	}
	if pv == nil {
		log.AddContext(ctx).Errorf("get nil pv, pvName: %v", volumeName)
		return nil, "", "", "", "", errors.New("pv is nil")
	}
	if pv.Spec.ClaimRef == nil {
		log.AddContext(ctx).Errorf("get nil pv.Spec.ClaimRef, pvName: %v", volumeName)
		return nil, "", "", "", "", errors.New("pv.Spec.ClaimRef is nil")
	}

	// get all pod in namespace
	pods, err := k8sAPI.ListPods(ctx, pv.Spec.ClaimRef.Namespace)
	if err != nil {
		log.AddContext(ctx).Errorf("list pods failed, namespace: %v, err: %v", pv.Spec.ClaimRef.Namespace, err)
		return nil, "", "", "", "", err
	}

	// get target pod name
	log.AddContext(ctx).Debugf("getPodInfo podList: %+v, targetPath: %v", pods.Items, targetPath)
	for _, pod := range pods.Items {
		if strings.Contains(targetPath, string(pod.UID)) {
			return pods.Items, pod.Name, pv.Spec.ClaimRef.Namespace, pv.Spec.StorageClassName, volumeName, nil
		}
	}

	return pods.Items, "", pv.Spec.ClaimRef.Namespace, pv.Spec.StorageClassName, volumeName, nil
}
