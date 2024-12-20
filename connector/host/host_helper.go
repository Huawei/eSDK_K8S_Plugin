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
	"time"

	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/proto"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (

	// secret name for saving data
	hostInfoSecretName = "huawei-csi-host-info"
)

// NodeHostInfo defines the base information of node host
type NodeHostInfo struct {
	// HostName the name of host
	HostName string `json:"hostName"`
	// IscsiInitiator the initiator of ISCSI protocol
	IscsiInitiator string `json:"iscsiInitiator"`
	// FCInitiators the initiator of FC protocol
	FCInitiators []string `json:"fCInitiators"`
	// RoCEInitiator the initiator of RoCE protocol
	RoCEInitiator string `json:"roCEInitiator"`
}

// NewNodeHostInfo instantiates this node host info.
// If the initiator query fails, the error will not be returned directly,
// because the current host may not have an initiator, which should be judged by the caller and handled accordingly
func NewNodeHostInfo(ctx context.Context) (*NodeHostInfo, error) {
	hostName, err := utils.GetHostName(ctx)
	if err != nil {
		log.AddContext(ctx).Errorf("get host name error: [%v]", err)
		return nil, err
	}

	iscsiInitiator, err := proto.GetISCSIInitiator(ctx)
	if err != nil {
		log.AddContext(ctx).Infof("get ISCSI initiator error: [%v]", err)
	}

	fcInitiators, err := proto.GetFCInitiator(ctx)
	if err != nil {
		log.AddContext(ctx).Infof("get FC initiator error: [%v]", err)
	}

	roCEInitiator, err := proto.GetRoCEInitiator(ctx)
	if err != nil {
		log.AddContext(ctx).Infof("get RoCE initiator error: [%v]", err)
	}

	return &NodeHostInfo{
		HostName:       strings.Trim(hostName, " "),
		IscsiInitiator: iscsiInitiator,
		FCInitiators:   fcInitiators,
		RoCEInitiator:  roCEInitiator,
	}, nil
}

// SaveNodeHostInfoToSecret save the current node host information to secret.
// secret namespace use the namespace of the current pod.
func SaveNodeHostInfoToSecret(ctx context.Context) error {
	k8sUtils := app.GetGlobalConfig().K8sUtils
	hostInfoSecret, err := k8sUtils.GetSecret(ctx, hostInfoSecretName, app.GetGlobalConfig().Namespace)
	if apiErrors.IsNotFound(err) {
		hostInfoSecret = makeNodeHostInfoSecret()
		hostInfoSecret, err = k8sUtils.CreateSecret(ctx, hostInfoSecret)
		if err != nil && !apiErrors.IsAlreadyExists(err) {
			errMsg := fmt.Sprintf("create host secret err: %s", err)
			return errors.New(errMsg)
		}
	} else if err != nil {
		errMsg := fmt.Sprintf("get secret err: %s", err)
		return errors.New(errMsg)
	}

	hostInfo, err := NewNodeHostInfo(ctx)
	if err != nil {
		log.Errorf("create node host info fail ,error: [%v]", err)
		return err
	}

	newSecretData, err := json.Marshal(hostInfo)
	if err != nil {
		log.Errorf("json marshal error, hosname:[%v],err: [%v]", hostInfo.HostName, err)
		return err
	}

	for true {
		hostInfoSecret, err = k8sUtils.GetSecret(ctx, hostInfoSecretName, app.GetGlobalConfig().Namespace)
		if err != nil {
			errMsg := fmt.Sprintf("get secret err: %s", err)
			return errors.New(errMsg)
		}

		if hostInfoSecret.StringData == nil {
			hostInfoSecret.StringData = make(map[string]string)
		}
		hostInfoSecret.StringData[hostInfo.HostName] = string(newSecretData)

		hostInfoSecret, err = k8sUtils.UpdateSecret(ctx, hostInfoSecret)
		if err != nil && apiErrors.IsConflict(err) {
			time.Sleep(time.Second)
			log.Infof("update secret is conflict,retrying")
			continue
		} else if err != nil {
			errMsg := fmt.Sprintf("update secret err: %s", err)
			return errors.New(errMsg)
		}
		break
	}
	return nil
}

// GetNodeHostInfosFromSecret get the specified node host information from secret.
func GetNodeHostInfosFromSecret(ctx context.Context, hostName string) (*NodeHostInfo, error) {
	k8sUtils := app.GetGlobalConfig().K8sUtils
	secret, err := k8sUtils.GetSecret(ctx, hostInfoSecretName, app.GetGlobalConfig().Namespace)
	if err != nil {
		log.AddContext(ctx).Errorf("get secret data error: [%v]", err)
		return nil, err
	}
	if secret.Data == nil {
		return nil, errors.New("secret data is empty")
	}
	if secretData, ok := secret.Data[hostName]; ok {
		hostNodeInfo := &NodeHostInfo{}
		if err = json.Unmarshal(secretData, hostNodeInfo); err != nil {
			errMsg := fmt.Sprintf("json unmarshal  data error: %s", err)
			return nil, errors.New(errMsg)
		}
		return hostNodeInfo, nil
	}
	return nil, nil
}

// makeNodeHostInfoSecret make node host info secret
func makeNodeHostInfoSecret() *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      hostInfoSecretName,
			Namespace: app.GetGlobalConfig().Namespace,
		},
		StringData: map[string]string{},
		Type:       corev1.SecretTypeOpaque,
	}
}
