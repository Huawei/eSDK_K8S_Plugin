/*
 Copyright (c) Huawei Technologies Co., Ltd. 2022-2023. All rights reserved.

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

// Package webhook provide configuration for volume provider
package webhook

import (
	"fmt"

	admissionV1 "k8s.io/api/admissionregistration/v1"

	"huawei-csi-driver/csi/app"
	"huawei-csi-driver/pkg/constants"
)

const (
	serviceName   = "huawei-csi-controller"
	containerName = "storage-backend-controller"
	privateKey    = "privateKey"
	privateCert   = "privateCert"

	claimWebhookPath = "/storagebackendclaim"
	claimAPIGroups   = "xuanwu.huawei.io"
	claimAPIVersions = "v1"
	claimResources   = "storagebackendclaims"
)

// GetStorageWebHookCfg used to get storage webhook configuration
func GetStorageWebHookCfg() (Config, []AdmissionWebHookCFG) {
	var handleFuncPair []HandleFuncPair
	handleFuncPair = append(handleFuncPair,
		HandleFuncPair{WebhookPath: claimWebhookPath,
			WebHookFunc: admitStorageBackendClaim})

	webHookCfg := Config{
		NamespaceEnv:     constants.NamespaceEnv,
		DefaultNamespace: app.GetGlobalConfig().Namespace,
		ServiceName:      serviceName,
		SecretName:       containerName,
		WebHookPort:      app.GetGlobalConfig().WebHookPort,
		WebHookAddress:   app.GetGlobalConfig().WebHookAddress,
		WebHookType:      AdmissionWebHookValidating,
		PrivateKey:       privateKey,
		PrivateCert:      privateCert,
		HandleFuncPair:   handleFuncPair,
	}

	admissionWebhook := AdmissionWebHookCFG{
		WebhookName: fmt.Sprintf("%s.xuanwu.huawei.io", containerName),
		ServiceName: serviceName,
		WebhookPath: claimWebhookPath,
		WebhookPort: int32(app.GetGlobalConfig().WebHookPort),
		AdmissionOps: []admissionV1.OperationType{
			admissionV1.Create,
			admissionV1.Update,
			admissionV1.Delete},
		AdmissionRule: AdmissionRule{
			APIGroups:   []string{claimAPIGroups},
			APIVersions: []string{claimAPIVersions},
			Resources:   []string{claimResources},
		},
	}

	var admissionWebhooks []AdmissionWebHookCFG
	admissionWebhooks = append(admissionWebhooks, admissionWebhook)

	return webHookCfg, admissionWebhooks
}
