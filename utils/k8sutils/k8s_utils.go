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

// Package k8sutils provides Kubernetes utilities
package k8sutils

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	"huawei-csi-driver/utils/log"
)

const (
	// TopologyPrefix supported by CSI plugin
	TopologyPrefix = "topology.kubernetes.io"
	// ProtocolTopologyPrefix supported by CSI plugin
	ProtocolTopologyPrefix = TopologyPrefix + "/protocol."
	topologyRegx           = TopologyPrefix + "/.*"
	// Interval (in miliseconds) between pod get retry with k8s
	podRetryInterval = 10
)

// Interface is a kubernetes utility interface required by CSI plugin to interact with Kubernetes
type Interface interface {
	// GetNodeTopology returns configured kubernetes node's topological labels
	GetNodeTopology(ctx context.Context, nodeName string) (map[string]string, error)

	// GetVolume returns volumes on the node at K8S side
	GetVolume(ctx context.Context, nodeName string, driverName string) (map[string]struct{}, error)

	// GetPVByName get all pv info
	GetPVByName(ctx context.Context, name string) (*corev1.PersistentVolume, error)

	// ListPods get pods by namespace
	ListPods(ctx context.Context, namespace string) (*corev1.PodList, error)

	// GetPod get pod by name and namespace
	GetPod(ctx context.Context, namespace, podName string) (*corev1.Pod, error)

	// GetVolumeAttributes returns volume attributes of PV
	GetVolumeAttributes(ctx context.Context, pvName string) (map[string]string, error)

	// Activate the k8s helpers when start the service
	Activate()
	// Deactivate the k8s helpers when stop the service
	Deactivate()

	secretOps
	ConfigmapOps
	persistentVolumeClaimOps
}

// KubeClient provides a wrapper for kubernetes client interface.
type KubeClient struct {
	clientSet kubernetes.Interface

	// pvc resources cache
	pvcIndexer            cache.Indexer
	pvcController         cache.SharedIndexInformer
	pvcControllerStopChan chan struct{}
	pvcSource             cache.ListerWatcher

	volumeNamePrefix string
	volumeLabels     map[string]string
}

// NewK8SUtils returns an object of Kubernetes utility interface
func NewK8SUtils(kubeConfig string, volumeNamePrefix string, volumeLabels map[string]string) (Interface, error) {
	var (
		config    *rest.Config
		clientset *kubernetes.Clientset
		err       error
	)

	if kubeConfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", kubeConfig)
		if err != nil {
			return nil, err
		}
	} else {
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
	}

	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	helper := &KubeClient{
		clientSet:             clientset,
		pvcControllerStopChan: make(chan struct{}),
		volumeNamePrefix:      volumeNamePrefix,
		volumeLabels:          volumeLabels,
	}
	initPVCWatcher(context.Background(), helper)
	return helper, nil
}

// GetNodeTopology gets topology belonging to this node by node name
func (k *KubeClient) GetNodeTopology(ctx context.Context, nodeName string) (map[string]string, error) {
	k8sNode, err := k.getNode(ctx, nodeName)
	if err != nil {
		return nil, fmt.Errorf("failed to get node topology with error: %v", err)
	}

	topology := make(map[string]string)
	for key, value := range k8sNode.Labels {
		if match, err := regexp.MatchString(topologyRegx, key); err == nil && match {
			topology[key] = value
		}
	}

	return topology, nil
}

func (k *KubeClient) getNode(ctx context.Context, nodeName string) (*corev1.Node, error) {
	return k.clientSet.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
}

// GetVolume gets all volumes belonging to this node from K8S side
func (k *KubeClient) GetVolume(ctx context.Context, nodeName string, driverName string) (map[string]struct{}, error) {
	podList, err := k.getPods(ctx, nodeName)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve pod list. %s", err)
	}
	// get PVC list
	pvcList := make(map[string]struct{}, 0)

	for _, pod := range podList.Items {
		// If a pod is being terminating, skip the pod.
		log.AddContext(ctx).Infof("Get pod [%s], pod.DeletionTimestamp: [%v].", pod.Name, pod.DeletionTimestamp)
		if pod.DeletionTimestamp != nil {
			log.AddContext(ctx).Infof("Pod [%s] is terminating, skip it.", pod.Name)
			continue
		}
		for _, volume := range pod.Spec.Volumes {
			if volume.PersistentVolumeClaim != nil {
				pvcList[volume.PersistentVolumeClaim.ClaimName+"@"+pod.Namespace] = struct{}{}
			}
		}
	}
	k8sVolumeHandles := make(map[string]struct{})
	errChan := make(chan error)
	pvChan := make(chan *corev1.PersistentVolume)

	defer func() {
		close(errChan)
		close(pvChan)
	}()
	// aggregate all volume information
	for claimName := range pvcList {
		pvcInfo := strings.Split(claimName, "@")
		if len(pvcInfo) <= 1 {
			log.AddContext(ctx).Errorf("The length of pvcInfo: [%d] is less than 2", len(pvcInfo))
			continue
		}
		go func(claimName string, namespace string,
			volChan chan<- *corev1.PersistentVolume,
			errorChan chan<- error) {
			vol, err := k.getPVByPVCName(ctx, namespace, claimName)
			if err != nil {
				errorChan <- err
				return
			}
			volChan <- vol
		}(pvcInfo[0], pvcInfo[1], pvChan, errChan)
	}
	var volumeError error
	for i := 0; i < len(pvcList); i++ {
		select {
		case err := <-errChan:
			volumeError = err
		case volume := <-pvChan:
			if volume.Spec.PersistentVolumeSource.CSI != nil &&
				driverName == volume.Spec.PersistentVolumeSource.CSI.Driver {
				k8sVolumeHandles[volume.Spec.PersistentVolumeSource.CSI.VolumeHandle] = struct{}{}
			}
		}
	}
	if volumeError != nil {
		return nil, volumeError
	}
	log.AddContext(ctx).Infof("PV list from k8s side for the node %s:  %v", nodeName, k8sVolumeHandles)
	return k8sVolumeHandles, nil
}

func (k *KubeClient) getPods(ctx context.Context, nodeName string) (*corev1.PodList, error) {
	var (
		podList *corev1.PodList
		err     error
	)
	// get pods with retry
	for i := 0; i < 5; i++ {
		podList, err = k.clientSet.CoreV1().Pods("").
			List(ctx, metav1.ListOptions{FieldSelector: "spec.nodeName=" + nodeName})
		if err == nil {
			break
		}
		time.Sleep(podRetryInterval * time.Millisecond)
	}
	return podList, err
}

func (k *KubeClient) getPVByPVCName(ctx context.Context, namespace string,
	claimName string) (*corev1.PersistentVolume, error) {
	pvc, err := k.clientSet.CoreV1().
		PersistentVolumeClaims(namespace).
		Get(ctx, claimName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve pvc. %s", err)
	}
	pv, err := k.clientSet.CoreV1().
		PersistentVolumes().
		Get(ctx, pvc.Spec.VolumeName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve volume. %s", err)
	}
	return pv, nil
}

// GetPVByName gets the pv by pv name
func (k *KubeClient) GetPVByName(ctx context.Context, name string) (*corev1.PersistentVolume, error) {
	return k.clientSet.CoreV1().
		PersistentVolumes().
		Get(ctx, name, metav1.GetOptions{})
}

// ListPods lists all pods from this namespace
func (k *KubeClient) ListPods(ctx context.Context, namespace string) (*corev1.PodList, error) {
	return k.clientSet.CoreV1().
		Pods(namespace).List(ctx, metav1.ListOptions{})
}

// GetPod gets a pod by pod name from this namespace
func (k *KubeClient) GetPod(ctx context.Context, namespace, podName string) (*corev1.Pod, error) {
	return k.clientSet.CoreV1().
		Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
}

// GetVolumeAttributes returns volume attributes of PV
func (k *KubeClient) GetVolumeAttributes(ctx context.Context, pvName string) (map[string]string, error) {
	pv, err := k.GetPVByName(ctx, pvName)
	if err != nil {
		return nil, err
	}

	if pv.Spec.CSI == nil {
		return nil, errors.New("CSI volume attribute missing from PV")
	}

	return pv.Spec.CSI.VolumeAttributes, nil
}

// Activate activate k8s helpers
func (k *KubeClient) Activate() {
	log.Infoln("Activate k8S helpers.")
	go k.pvcController.Run(k.pvcControllerStopChan)
}

// Deactivate deactivate k8s helpers
func (k *KubeClient) Deactivate() {
	log.Infoln("Deactivate k8S helpers.")
	close(k.pvcControllerStopChan)
}
