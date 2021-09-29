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

// Package k8sutils provides Kubernetes utilities
package k8sutils

import (
	"fmt"
	"regexp"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	// TopologyPrefix supported by CSI plugin
	TopologyPrefix = "topology.kubernetes.io"
	// ProtocolTopologyPrefix supported by CSI plugin
	ProtocolTopologyPrefix = TopologyPrefix + "/protocol."
	topologyRegx           = TopologyPrefix + "/.*"
)

// Interface is a kubernetes utility interface required by CSI plugin to interact with Kubernetes
type Interface interface {
	// GetNodeTopology returns configured kubernetes node's topological labels
	GetNodeTopology(nodeName string) (map[string]string, error)
}

type kubeClient struct {
	clientSet *kubernetes.Clientset
}

// NewK8SUtils returns an object of Kubernetes utility interface
func NewK8SUtils(kubeConfig string) (Interface, error) {
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

	return &kubeClient{
		clientSet: clientset,
	}, nil
}

func (k *kubeClient) GetNodeTopology(nodeName string) (map[string]string, error) {
	k8sNode, err := k.getNode(nodeName)
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

func (k *kubeClient) getNode(nodeName string) (*corev1.Node, error) {
	return k.clientSet.CoreV1().Nodes().Get(nodeName, metav1.GetOptions{})
}
