/*
Copyright (c) Huawei Technologies Co., Ltd. 2022-2024. All rights reserved.

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

// Package webhook validate the request
package webhook

import (
	"context"

	"k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
)

// CreateCertSecrets creates k8s secret to store signed cert data
func CreateCertSecrets(ctx context.Context, webHookCfg Config, cert, key []byte, ns string) (*v1.Secret, error) {
	secretData := make(map[string][]byte)
	secretData[webHookCfg.PrivateKey] = key
	secretData[webHookCfg.PrivateCert] = cert
	secret := &v1.Secret{
		ObjectMeta: metaV1.ObjectMeta{
			Name:      webHookCfg.SecretName,
			Namespace: ns,
		},
		Data: secretData,
	}

	certSecret, err := app.GetGlobalConfig().K8sUtils.CreateSecret(ctx, secret)
	if err != nil {
		return nil, err
	}

	return certSecret, nil
}
