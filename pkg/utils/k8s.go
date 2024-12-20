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

// Package utils to provide k8s resource utils
package utils

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"

	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	clientV1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"

	xuanwuv1 "github.com/Huawei/eSDK_K8S_Plugin/v4/client/apis/xuanwu/v1"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	clientSet "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/client/clientset/versioned"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

func IsSBCTExist(ctx context.Context, backendID string) bool {
	content, err := GetContentByClaimMeta(ctx, backendID)
	if err != nil {
		log.AddContext(ctx).Infof("IsSBCTExist err: [%v]", err)
		return false
	}

	return content != nil
}

func IsSBCTOnline(ctx context.Context, backendID string) bool {
	online, err := GetSBCTOnlineStatusByClaim(ctx, backendID)
	if err != nil {
		log.AddContext(ctx).Infof("GetSBCTOnlineStatusByClaim failed, err: [%v]", err)
		return false
	}

	return online
}

func GetPasswordFromBackendID(ctx context.Context, backendID string) (string, error) {
	_, secretMeta, err := GetConfigMeta(ctx, backendID)
	if err != nil {
		return "", err
	}

	namespace, secretName, err := SplitMetaNamespaceKey(secretMeta)
	if err != nil {
		return "", fmt.Errorf("split secret secretMeta %s namespace failed, error: %v", secretMeta, err)
	}

	return utils.GetPasswordFromSecret(ctx, secretName, namespace)
}

// GetCertSecretFromBackendID get cert secret meta from backend
func GetCertSecretFromBackendID(ctx context.Context, backendID string) (bool, string, error) {
	useCert, secret, err := GetCertMeta(ctx, backendID)
	if err != nil {
		return false, "", err
	}
	return useCert, secret, nil
}

// GetCertPool get cert pool
func GetCertPool(ctx context.Context, useCert bool, secretMeta string) (bool, *x509.CertPool, error) {
	if !useCert {
		log.AddContext(ctx).Infoln("useCert is false, skip get cert pool")
		return false, nil, nil
	}
	log.AddContext(ctx).Infoln("Start get cert pool")

	namespace, secretName, err := SplitMetaNamespaceKey(secretMeta)
	if err != nil {
		return false, nil, fmt.Errorf("split secret secretMeta %s namespace failed, error: %v", secretMeta, err)
	}

	certMeta, err := utils.GetCertFromSecret(ctx, secretName, namespace)
	if err != nil {
		return false, nil, fmt.Errorf("get cert from secret %s failed, error: %v", secretName, err)
	}

	certBlock, _ := pem.Decode(certMeta)
	if certBlock == nil {
		return false, nil, fmt.Errorf("certificate data decode failed")
	}

	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return false, nil, fmt.Errorf("error parse certificate: %v", err)
	}

	certPool := x509.NewCertPool()
	certPool.AddCert(cert)
	return true, certPool, nil
}

func GetBackendConfigmapByClaimName(ctx context.Context, claimNameMeta string) (*coreV1.ConfigMap, error) {
	log.AddContext(ctx).Infof("Get configmap meta data by claim meta: [%s]", claimNameMeta)
	configmapMeta, _, err := GetConfigMeta(ctx, claimNameMeta)
	if err != nil {
		msg := fmt.Sprintf("GetConfigMeta: [%s] failed, error: [%v].", claimNameMeta, err)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	ns, configmapName, err := SplitMetaNamespaceKey(configmapMeta)
	if err != nil {
		msg := fmt.Sprintf("SplitMetaNamespaceKey ConfigmapMeta: [%s] failed, error: [%v].",
			configmapMeta, err)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	return app.GetGlobalConfig().K8sUtils.GetConfigmap(ctx, configmapName, ns)
}

func GetClaimByMeta(ctx context.Context, claimNameMeta string) (*xuanwuv1.StorageBackendClaim, error) {
	ns, claimName, err := SplitMetaNamespaceKey(claimNameMeta)
	if err != nil {
		msg := fmt.Sprintf("SplitMetaNamespaceKey: [%s] failed, error: [%v].", claimNameMeta, err)
		return nil, Errorln(ctx, msg)
	}

	claim, err := GetClaim(ctx, app.GetGlobalConfig().BackendUtils,
		&xuanwuv1.StorageBackendClaim{
			ObjectMeta: v1.ObjectMeta{
				Namespace: ns,
				Name:      claimName,
			},
		})
	if err != nil {
		msg := fmt.Sprintf("Get storageBackendClaim: [%s] failed, error: [%v].", claimNameMeta, err)
		return nil, Errorln(ctx, msg)
	}

	if claim == nil {
		msg := fmt.Sprintf("StorageBackendClaim: [%s] is nil, get claim failed.", claimName)
		return nil, Errorln(ctx, msg)
	}
	return claim, nil
}

func GetConfigMeta(ctx context.Context, claimNameMeta string) (string, string, error) {
	log.AddContext(ctx).Infof("Get claim: [%s] config meta.", claimNameMeta)

	claim, err := GetClaimByMeta(ctx, claimNameMeta)
	if err != nil {
		return "", "", err
	}

	if claim == nil {
		msg := fmt.Sprintf("Get claim failed, claim: [%s] is nil", claimNameMeta)
		return "", "", Errorln(ctx, msg)
	}

	return claim.Spec.ConfigMapMeta, claim.Spec.SecretMeta, nil
}

// GetCertMeta get cert meta from backend claim
func GetCertMeta(ctx context.Context, claimNameMeta string) (bool, string, error) {
	log.AddContext(ctx).Infof("Get claim: [%s] config meta.", claimNameMeta)

	claim, err := GetClaimByMeta(ctx, claimNameMeta)
	if err != nil {
		return false, "", err
	}

	if claim == nil {
		msg := fmt.Sprintf("Get claim failed, claim: [%s] is nil", claimNameMeta)
		return false, "", Errorln(ctx, msg)
	}

	return claim.Spec.UseCert, claim.Spec.CertSecret, nil
}

func GetContentByClaimMeta(ctx context.Context, claimNameMeta string) (*xuanwuv1.StorageBackendContent, error) {
	log.AddContext(ctx).Debugf("Start to get storageBackendContent by claimMeta: [%s].", claimNameMeta)

	claim, err := GetClaimByMeta(ctx, claimNameMeta)
	if err != nil {
		return nil, err
	}

	if claim.Status == nil {
		msg := fmt.Sprintf("StorageBackendClaim: [%s] status is nil, can not get content name.", claimNameMeta)
		return nil, Errorln(ctx, msg)
	}

	return GetContent(ctx, app.GetGlobalConfig().BackendUtils, claim.Status.BoundContentName)
}

func GetBackendSecret(ctx context.Context, secretMeta string) (*coreV1.Secret, error) {
	namespace, name, err := SplitMetaNamespaceKey(secretMeta)
	if err != nil {
		return nil, fmt.Errorf("split secret secretMeta %s namespace failed, error: %v", secretMeta, err)
	}

	secret, err := app.GetGlobalConfig().K8sUtils.GetSecret(ctx, name, namespace)
	if err != nil {
		return nil, fmt.Errorf("get secret with name %s and namespace %s failed, error: %v",
			name, namespace, err)
	}

	return secret, nil
}

func GetSBCTOnlineStatusByClaim(ctx context.Context, backendID string) (bool, error) {
	content, err := GetContentByClaimMeta(ctx, backendID)
	if err != nil {
		msg := fmt.Sprintf("GetContentByClaimMeta: [%s] failed, err: [%v]", backendID, err)
		return false, Errorln(ctx, msg)
	}

	if content == nil {
		msg := fmt.Sprintf("StorageBackendContent: [%s] content is nil, GetSBCTOnlineStatusByClaim failed.",
			content.Name)
		return false, Errorln(ctx, msg)
	}
	if content.Status == nil {
		msg := fmt.Sprintf("StorageBackendContent: [%s] content.status is nil, GetSBCTOnlineStatusByClaim failed.",
			content.Name)
		return false, Errorln(ctx, msg)
	}

	return content.Status.Online, nil
}

// GetSBCTOnlineStatusByContent get backend capabilities by SBCT
func GetSBCTOnlineStatusByContent(ctx context.Context, content *xuanwuv1.StorageBackendContent) (bool, error) {
	if content == nil {
		msg := fmt.Sprintf("StorageBackendContent: [%s] content is nil, GetSBCTOnlineStatusByContent failed.",
			content.Name)
		return false, Errorln(ctx, msg)
	}
	if content.Status == nil {
		msg := fmt.Sprintf("StorageBackendContent: [%s] content.status is nil, GetSBCTOnlineStatusByContent failed.",
			content.Name)
		return false, Errorln(ctx, msg)
	}

	return content.Status.Online, nil
}

// GetSBCTCapabilitiesByClaim get backend capabilities
func GetSBCTCapabilitiesByClaim(ctx context.Context, backendID string) (map[string]bool, error) {
	content, err := GetContentByClaimMeta(ctx, backendID)
	if err != nil {
		msg := fmt.Sprintf("GetContentByClaimMeta: [%s] failed, err: [%v]", backendID, err)
		return nil, Errorln(ctx, msg)
	}

	if content == nil {
		msg := fmt.Sprintf("StorageBackendContent: [%s] content is nil, GetSBCTOnlineStatusByClaim failed.",
			content.Name)
		return nil, Errorln(ctx, msg)
	}
	if content.Status == nil {
		msg := fmt.Sprintf("StorageBackendContent: [%s] content.status is nil, "+
			"GetSBCTOnlineStatusByClaim failed.", content.Name)
		return nil, Errorln(ctx, msg)
	}

	return content.Status.Capabilities, nil
}

// IsBackendCapabilitySupport valid backend capability
func IsBackendCapabilitySupport(ctx context.Context, backendID string,
	capability constants.BackendCapability) (bool, error) {
	capabilities, err := GetSBCTCapabilitiesByClaim(ctx, backendID)
	if err != nil {
		log.AddContext(ctx).Errorf("GetSBCTCapabilitiesByClaim failed, backendID: %v, err: %v", backendID, err)
		return false, err
	}
	return capabilities[string(capability)], nil
}

func SetSBCTOnlineStatus(ctx context.Context, content *xuanwuv1.StorageBackendContent, status bool) error {
	content.Status.Online = status

	_, err := app.GetGlobalConfig().BackendUtils.XuanwuV1().StorageBackendContents().UpdateStatus(ctx,
		content, v1.UpdateOptions{})
	if err != nil {
		msg := fmt.Sprintf("Update storageBackendContent Status: [%s] failed， err: [%v]", content.Name, err)
		return Errorln(ctx, msg)
	}

	return nil
}

func SetStorageBackendContentOnlineStatus(ctx context.Context, backendID string, online bool) error {
	content, err := GetContentByClaimMeta(ctx, backendID)
	if err != nil {
		msg := fmt.Sprintf("GetContentByClaimMeta: [%s] failed, err: [%v]", backendID, err)
		return Errorln(ctx, msg)
	}

	if content.Status == nil {
		msg := fmt.Sprintf("StorageBackendContent: [%s] status is nil, SetStorageBackendContentOnlineStatus failed.",
			content.Name)
		return Errorln(ctx, msg)
	}

	err = SetSBCTOnlineStatus(ctx, content, online)
	if err != nil {
		return err
	}

	_, backendName, err := SplitMetaNamespaceKey(content.Spec.BackendClaim)
	if err != nil {
		log.AddContext(ctx).Errorf("get backend name failed, error: %v", err)
		return err
	}

	// notify backend status is change
	Publish(ctx, BackendStatus, ctx, backendName, online)
	log.AddContext(ctx).Infof("SetStorageBackendContentOnlineStatus [%s] to [%v] succeeded.",
		backendID, online)
	return nil
}

// GetK8SAndCrdClient return k8sClient, crdClient
func GetK8SAndCrdClient(ctx context.Context) (*kubernetes.Clientset, *clientSet.Clientset, error) {
	var config *rest.Config
	var err error
	if app.GetGlobalConfig().KubeConfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", app.GetGlobalConfig().KubeConfig)
	} else {
		config, err = rest.InClusterConfig()
	}

	if err != nil {
		log.AddContext(ctx).Errorf("Error getting cluster config, kube config: %s, error %v",
			app.GetGlobalConfig().KubeConfig, err)
		return nil, nil, err
	}

	k8sClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.AddContext(ctx).Errorf("Error getting kubernetes client error %v", err)
		return nil, nil, err
	}

	crdClient, err := clientSet.NewForConfig(config)
	if err != nil {
		log.AddContext(ctx).Errorf("Error getting crd client error %v", err)
		return nil, nil, err
	}

	return k8sClient, crdClient, nil
}

// InitRecorder used to init event recorder
func InitRecorder(client kubernetes.Interface, componentName string) record.EventRecorder {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&clientV1.EventSinkImpl{Interface: client.CoreV1().Events(v1.NamespaceAll)})
	return eventBroadcaster.NewRecorder(scheme.Scheme, coreV1.EventSource{Component: componentName})
}
