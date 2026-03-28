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

// Package host defines a set of useful methods, which can help Connector to operate host information
package host

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/proto"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/k8sutils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	// secret name for saving data
	hostInfoSecretName  = "huawei-csi-host-info"
	hostInfoSecretKey   = "secret-provisioner"
	hostInfoSecretValue = "csi.huawei.com"
)

// SaveNodeHostInfoToSecret save the current node host information to secret.
// secret namespace use the namespace of the current pod.
func SaveNodeHostInfoToSecret(ctx context.Context) error {
	info, err := NewNodeHostInfo(ctx, app.GetGlobalConfig().ReportNodeIP)
	if err != nil {
		return err
	}

	data, err := json.Marshal(info)
	if err != nil {
		return err
	}

	storage := NewSecretStorage(app.GetGlobalConfig().K8sUtils, app.GetGlobalConfig().EnablePerNodeSecret)
	return storage.Save(ctx, info.HostName, app.GetGlobalConfig().Namespace, data)
}

// GetNodeHostInfosFromSecret get the specified node host information from secret.
func GetNodeHostInfosFromSecret(ctx context.Context, hostName string) (*NodeHostInfo, error) {
	storage := NewSecretStorage(app.GetGlobalConfig().K8sUtils, app.GetGlobalConfig().EnablePerNodeSecret)
	return storage.Get(ctx, hostName, app.GetGlobalConfig().Namespace)
}

// SecretStorage is an interface for storing and retrieving host information in Kubernetes secrets
type SecretStorage interface {
	// Save stores host information in secret storage
	Save(ctx context.Context, hostname, namespace string, data []byte) error

	// Get retrieves host information from secret storage
	Get(ctx context.Context, hostname, namespace string) (*NodeHostInfo, error)
}

// NewSecretStorage creates a new SecretStorage instance based on isPerNode flag
// isPerNode=true will create PerNodeStorage, otherwise creates UnifiedStorage
func NewSecretStorage(client k8sutils.Interface, isPerNode bool) SecretStorage {
	if isPerNode {
		return NewPerNodeStorage(client)
	}
	return NewUnifiedStorage(client)
}

// UnifiedStorage implements SecretStorage using a single secret for all hosts
type UnifiedStorage struct {
	BaseStorage
}

// NewUnifiedStorage creates a new UnifiedStorage instance
func NewUnifiedStorage(client k8sutils.Interface) *UnifiedStorage {
	return &UnifiedStorage{BaseStorage: BaseStorage{client: client}}
}

// Save stores host information in a unified secret object
// It creates the secret if not exist and updates the data for specified hostname
func (u *UnifiedStorage) Save(ctx context.Context, hostname, namespace string, data []byte) error {
	return u.SaveToSecret(ctx, hostname, hostInfoSecretName, namespace, data)
}

// Get retrieves host information from the unified secret object
func (u *UnifiedStorage) Get(ctx context.Context, hostname, namespace string) (*NodeHostInfo, error) {
	return u.GetFromSecret(ctx, hostname, hostInfoSecretName, namespace)
}

// PerNodeStorage implements SecretStorage using per-host secrets
type PerNodeStorage struct {
	BaseStorage
}

// NewPerNodeStorage creates a new PerNodeStorage instance
func NewPerNodeStorage(client k8sutils.Interface) *PerNodeStorage {
	return &PerNodeStorage{BaseStorage: BaseStorage{client: client}}
}

// Save stores host information in a dedicated secret per host
// It creates the secret if not exist and updates the data for specified hostname
func (p *PerNodeStorage) Save(ctx context.Context, hostname, namespace string, data []byte) error {
	return p.SaveToSecret(ctx, hostname, hostname, namespace, data)
}

// Get retrieves host information from the dedicated host secret
func (p *PerNodeStorage) Get(ctx context.Context, hostname, namespace string) (*NodeHostInfo, error) {
	return p.GetFromSecret(ctx, hostname, hostname, namespace)
}

// BaseStorage define secret-based storage operations
type BaseStorage struct {
	client k8sutils.Interface
}

// SaveToSecret stores host information in a secret object
func (u *BaseStorage) SaveToSecret(ctx context.Context, hostname, name, namespace string, data []byte) error {
	if err := u.createSecretIfNotExist(ctx, name, namespace); err != nil {
		return fmt.Errorf("failed to check and create secret, err: %w", err)
	}
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		hostInfoSecret, err := u.client.GetSecret(ctx, name, namespace)
		if err != nil {
			return err
		}

		addDataToSecret(hostInfoSecret, hostname, data)
		_, err = u.client.UpdateSecret(ctx, hostInfoSecret)
		return err
	})
}

func (u *BaseStorage) createSecretIfNotExist(ctx context.Context, name, namespace string) error {
	hostInfoSecret, err := u.client.GetSecret(ctx, name, namespace)
	if err == nil {
		return nil
	}

	if !apiErrors.IsNotFound(err) {
		return fmt.Errorf("get secret err: %w", err)
	}

	hostInfoSecret = makeNodeHostInfoSecret(name, namespace)
	hostInfoSecret, err = u.client.CreateSecret(ctx, hostInfoSecret)
	if err == nil || apiErrors.IsAlreadyExists(err) {
		return nil
	}
	return fmt.Errorf("create host secret err: %w", err)
}

// GetFromSecret retrieves host information from the secret object
func (u *BaseStorage) GetFromSecret(ctx context.Context, hostname, name, namespace string) (*NodeHostInfo, error) {
	secret, err := u.client.GetSecret(ctx, name, namespace)
	if err != nil {
		return nil, fmt.Errorf("get host secret data failed, hostname:%s, error: %w", hostname, err)
	}

	return unmarshalSecret(hostname, secret)
}

func unmarshalSecret(hostname string, secret *corev1.Secret) (*NodeHostInfo, error) {
	if secret == nil || secret.Data == nil {
		return nil, errors.New("secret data is empty")
	}

	secretData, ok := secret.Data[hostname]
	if !ok {
		return nil, fmt.Errorf("secret data does not contain hostname %s", hostname)
	}

	hostNodeInfo := &NodeHostInfo{}
	if err := json.Unmarshal([]byte(secretData), hostNodeInfo); err != nil {
		return nil, fmt.Errorf("unmarshal secret data error: %w", err)
	}
	return hostNodeInfo, nil
}

func addDataToSecret(secret *corev1.Secret, hostname string, data []byte) {
	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}
	secret.Data[hostname] = data

	if len(secret.Labels) == 0 {
		secret.Labels = map[string]string{hostInfoSecretKey: hostInfoSecretValue}
	} else {
		secret.Labels[hostInfoSecretKey] = hostInfoSecretValue
	}
}

// NodeHostInfo defines the base information of node host
type NodeHostInfo struct {
	// HostName the name of host
	HostName string `json:"-"`
	// HostIPs the IP addresses of host
	HostIPs []string `json:"hostIPs,omitempty"`
	// IscsiInitiator the initiator of ISCSI protocol
	IscsiInitiator string `json:"iscsiInitiator,omitempty"`
	// FCInitiators the initiator of FC protocol
	FCInitiators []string `json:"fCInitiators,omitempty"`
	// NVMeInitiator the initiator of NVMe protocol
	NVMeInitiator string `json:"nvmeInitiator,omitempty"`
}

// NewNodeHostInfo instantiates this node host info.
// If the initiator query fails, the error will not be returned directly,
// because the current host may not have an initiator, which should be judged by the caller and handled accordingly
func NewNodeHostInfo(ctx context.Context, reportIps bool) (*NodeHostInfo, error) {
	hostName, err := utils.GetHostName(ctx)
	if err != nil {
		log.AddContext(ctx).Errorf("failed to get host name: [%v]", err)
		return nil, err
	}

	iscsiInitiator, err := proto.GetISCSIInitiator(ctx)
	if err != nil {
		log.AddContext(ctx).Warningf("failed to get ISCSI initiator: [%v]", err)
	}

	fcInitiators, err := proto.GetFCInitiator(ctx)
	if err != nil {
		log.AddContext(ctx).Warningf("failed to get FC initiator: [%v]", err)
	}

	nvmeInitiator, err := proto.GetNVMeInitiator(ctx)
	if err != nil {
		log.AddContext(ctx).Warningf("failed to get RoCE initiator: [%v]", err)
	}

	hostInfo := &NodeHostInfo{
		HostName:       strings.Trim(hostName, " "),
		IscsiInitiator: iscsiInitiator,
		FCInitiators:   fcInitiators,
		NVMeInitiator:  nvmeInitiator,
	}

	if reportIps {
		if hostIPs, err := utils.GetHostIPs(ctx); err != nil {
			log.AddContext(ctx).Warningf("failed to get host ips: [%v]", err)
		} else {
			hostInfo.HostIPs = hostIPs
		}
	}

	return hostInfo, nil
}

// makeNodeHostInfoSecret make node host info secret
func makeNodeHostInfoSecret(name, namespace string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{hostInfoSecretKey: hostInfoSecretValue},
		},
		StringData: map[string]string{},
		Type:       corev1.SecretTypeOpaque,
	}
}
