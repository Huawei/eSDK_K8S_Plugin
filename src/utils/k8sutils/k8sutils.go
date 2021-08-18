package k8sutils

import (
	"errors"
	"fmt"
	"regexp"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	topologyRegx = "topology.kubernetes.io/*"
)

type Interface interface {
	GetNodeTopology(nodeName string) (map[string]string, error)
}

type KubeClient struct {
	clientSet *kubernetes.Clientset
}

func NewK8SUtils(kubeConfig string) (Interface, error) {
	var clientset *kubernetes.Clientset

	if kubeConfig != "" {
		config, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
		if err != nil {
			return nil, err
		}

		clientset, err = kubernetes.NewForConfig(config)
		if err != nil {
			return nil, err
		}
	} else {
		config, err := rest.InClusterConfig()
		if err != nil {
			return nil, err
		}

		clientset, err = kubernetes.NewForConfig(config)
		if err != nil {
			return nil, err
		}
	}

	return &KubeClient{
		clientSet: clientset,
	}, nil
}

func (k *KubeClient) GetNodeTopology(nodeName string) (map[string]string, error) {
	k8sNode, err := k.getNode(nodeName)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to get node topology with error: %v", err))
	}

	topology := make(map[string]string)
	for key, value := range k8sNode.Labels {
		if match, err := regexp.MatchString(topologyRegx, key); err == nil && match {
			topology[key] = value
		}
	}

	return topology, nil
}

func (k *KubeClient) getNode(nodeName string) (*corev1.Node, error) {
	return k.clientSet.CoreV1().Nodes().Get(nodeName, metav1.GetOptions{})
}
