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
	k8sError "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"

	xuanwuv1 "huawei-csi-driver/client/apis/xuanwu/v1"
	"huawei-csi-driver/csi/app"
	"huawei-csi-driver/pkg/constants"
	pkgUtils "huawei-csi-driver/pkg/utils"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

func nodeAddLabel(ctx context.Context, volumeID, targetPath string) {
	backendName, _ := utils.SplitVolumeId(volumeID)
	backendName = pkgUtils.MakeMetaWithNamespace(app.GetGlobalConfig().Namespace, backendName)

	supportLabel, err := pkgUtils.IsBackendCapabilitySupport(ctx, backendName, constants.SupportLabel)
	if err != nil {
		log.AddContext(ctx).Errorf("IsBackendCapabilitySupport failed, backendName: %v, label: %v, err: %v",
			backendName, supportLabel, err)
	}
	if supportLabel {
		if err := addLabel(ctx, volumeID, targetPath); err != nil {
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
	}
	if supportLabel {
		if err := deleteLabel(ctx, volumeID, targetPath); err != nil {
			log.AddContext(ctx).Errorf("nodeDeleteLabel failed, err: %v", err)
		}
	}
}

func addLabel(ctx context.Context, volumeID, targetPath string) error {
	_, volumeName := utils.SplitVolumeId(volumeID)
	topoName := pkgUtils.GetTopoName(volumeName)
	podName, namespace, sc, pvName, err := getTargetPathPodRelateInfo(ctx, targetPath)
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

	topo, err := app.GetGlobalConfig().BackendUtils.XuanwuV1().ResourceTopologies().Get(ctx,
		topoName, metav1.GetOptions{})
	log.AddContext(ctx).Debugf("get topo info, topo: %+v, err: %v, notFound: %v", topo,
		err, k8sError.IsNotFound(err))
	if k8sError.IsNotFound(err) {
		_, err = app.GetGlobalConfig().BackendUtils.XuanwuV1().ResourceTopologies().Create(ctx,
			formatTopologies(volumeID, namespace, podName, pvName, topoName),
			metav1.CreateOptions{})
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

	// if pvc label not exist then add
	// add new topo item
	addPodTopoItem(&topo.Spec.Tags, podName, namespace, volumeName)

	// update topo
	_, err = app.GetGlobalConfig().BackendUtils.XuanwuV1().ResourceTopologies().Update(ctx,
		topo, metav1.UpdateOptions{})
	if err != nil {
		log.AddContext(ctx).Errorf("node add label failed, data:%+v, volumeName: %v targetPath: %v err: %v",
			topo, volumeName, targetPath, err)
		return err
	}

	log.AddContext(ctx).Infof("node add label success, data: %+v volumeName: %v targetPath: %v",
		topo, volumeName, targetPath)
	return nil
}

func formatTopologies(volumeID, namespace, podName, pvName, topoName string) *xuanwuv1.ResourceTopology {
	topologySpec := xuanwuv1.ResourceTopologySpec{
		Provisioner:  constants.DefaultTopoDriverName,
		VolumeHandle: volumeID,
		Tags: []xuanwuv1.Tag{
			{
				ResourceInfo: xuanwuv1.ResourceInfo{
					TypeMeta: metav1.TypeMeta{Kind: constants.PVKind, APIVersion: constants.KubernetesV1},
					Name:     pvName,
				},
			},
			{
				ResourceInfo: xuanwuv1.ResourceInfo{
					TypeMeta:  metav1.TypeMeta{Kind: constants.PodKind, APIVersion: constants.KubernetesV1},
					Name:      podName,
					Namespace: namespace,
				},
			},
		},
	}
	return &xuanwuv1.ResourceTopology{
		TypeMeta:   metav1.TypeMeta{Kind: constants.TopologyKind, APIVersion: constants.XuanwuV1},
		ObjectMeta: metav1.ObjectMeta{Name: topoName},
		Spec:       topologySpec,
	}
}

func addPodTopoItem(tags *[]xuanwuv1.Tag, podName, namespace, volumeName string) {
	// if pvc label not exist then add
	var existPvLabel bool
	for _, tag := range *tags {
		if tag.Kind == constants.PVKind {
			existPvLabel = true
			break
		}
	}
	if !existPvLabel {
		*tags = append(*tags, xuanwuv1.Tag{
			ResourceInfo: xuanwuv1.ResourceInfo{
				TypeMeta: metav1.TypeMeta{Kind: constants.PVKind, APIVersion: constants.KubernetesV1},
				Name:     volumeName,
			},
		})
	}

	// add pod label
	*tags = append(*tags, xuanwuv1.Tag{
		ResourceInfo: xuanwuv1.ResourceInfo{
			TypeMeta:  metav1.TypeMeta{Kind: constants.PodKind, APIVersion: constants.KubernetesV1},
			Name:      podName,
			Namespace: namespace,
		},
	})
}

func deleteLabel(ctx context.Context, volumeID, targetPath string) error {
	_, volumeName := utils.SplitVolumeId(volumeID)
	topoName := pkgUtils.GetTopoName(volumeName)
	podName, namespace, sc, pvName, err := getTargetPathPodRelateInfo(ctx, targetPath)
	if err != nil {
		log.AddContext(ctx).Errorf("get targetPath pvRelateInfo failed, targetPath: %v, err: %v", targetPath, err)
		return err
	}

	if sc == "" {
		log.AddContext(ctx).Infof("deleteLabel static pv, volumeID: %v, targetPath: %v", volumeID, targetPath)
		return nil
	}

	// get topo label
	topo, err := app.GetGlobalConfig().BackendUtils.XuanwuV1().ResourceTopologies().Get(ctx, topoName,
		metav1.GetOptions{})
	if k8sError.IsNotFound(err) {
		log.AddContext(ctx).Infof("node delete label success, topo not found, "+
			"data: %+v pvName: %v targetPath: %v", topo, pvName, targetPath)
		return nil
	}
	if err != nil {
		log.AddContext(ctx).Errorf("get topo failed, topoName: %v, err: %v", topoName, err)
		return err
	}
	if topo == nil {
		log.AddContext(ctx).Errorf("get nil topo, topoName: %v", topoName)
		return errors.New("topo is nil")
	}

	// filter pod
	topo.Spec.Tags = filterDeleteTopoTag(ctx, topo.Spec.Tags, podName, namespace)

	// update topo
	_, err = app.GetGlobalConfig().BackendUtils.XuanwuV1().ResourceTopologies().Update(ctx,
		topo, metav1.UpdateOptions{})
	if err != nil {
		log.AddContext(ctx).Errorf("node add label failed, data:%+v, volumeName: %v targetPath: %v err: %v",
			topo, volumeName, targetPath, err)
		return err
	}

	log.AddContext(ctx).Infof("node delete label success, data: %+v volumeName: %v targetPath: %v",
		topo, volumeName, targetPath)
	return nil
}

func filterDeleteTopoTag(ctx context.Context, tags []xuanwuv1.Tag, currentPodName, namespace string) []xuanwuv1.Tag {
	var newTags []xuanwuv1.Tag
	for _, tag := range tags {
		if tag.Kind != constants.PodKind {
			newTags = append(newTags, tag)
			continue
		}

		if tag.Kind == constants.PodKind && tag.Name == currentPodName && tag.Namespace == namespace {
			continue
		}

		_, err := app.GetGlobalConfig().K8sUtils.GetPod(ctx, tag.Namespace, tag.Name)
		if k8sError.IsNotFound(err) {
			continue
		}
		if err != nil {
			log.AddContext(ctx).Errorf("get pod failed, podInfo: %v, err: %v", tag, err)
		}

		newTags = append(newTags, tag)
	}
	return newTags
}

// getTargetPathPodRelateInfo get podName nameSpace sc volumeName
func getTargetPathPodRelateInfo(ctx context.Context, targetPath string) (string, string, string, string, error) {
	k8sAPI := app.GetGlobalConfig().K8sUtils

	targetPathArr := strings.Split(targetPath, "/")
	if len(targetPathArr) < 2 {
		return "", "", "", "", pkgUtils.Errorf(ctx, "targetPath: %s is invalid", targetPath)
	}
	volumeName := targetPathArr[len(targetPathArr)-2]

	log.AddContext(ctx).Debugf("targetPath: %v, volumeName: %v", targetPath, volumeName)

	// get pv info
	pv, err := k8sAPI.GetPVByName(ctx, volumeName)
	if err != nil {
		log.AddContext(ctx).Errorf("get pv failed, pvName: %v, err: %v", volumeName, err)
		return "", "", "", volumeName, err
	}
	if pv == nil {
		log.AddContext(ctx).Errorf("get nil pv, pvName: %v", volumeName)
		return "", "", "", "", errors.New("pv is nil")
	}
	if pv.Spec.ClaimRef == nil {
		log.AddContext(ctx).Errorf("get nil pv.Spec.ClaimRef, pvName: %v", volumeName)
		return "", "", "", "", errors.New("pv.Spec.ClaimRef is nil")
	}

	// get all pod in namespace
	pods, err := k8sAPI.ListPods(ctx, pv.Spec.ClaimRef.Namespace)
	if err != nil {
		log.AddContext(ctx).Errorf("list pods failed, namespace: %v, err: %v", pv.Spec.ClaimRef.Namespace, err)
		return "", "", "", "", err
	}

	// get target pod name
	log.AddContext(ctx).Debugf("getPodInfo podList: %+v, targetPath: %v", pods.Items, targetPath)
	for _, pod := range pods.Items {
		if strings.Contains(targetPath, string(pod.UID)) {
			return pod.Name, pv.Spec.ClaimRef.Namespace, pv.Spec.StorageClassName, volumeName, nil
		}
	}

	return "", pv.Spec.ClaimRef.Namespace, pv.Spec.StorageClassName, volumeName, nil
}
