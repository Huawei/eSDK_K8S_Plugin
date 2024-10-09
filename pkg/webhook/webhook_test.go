/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2024. All rights reserved.
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

package webhook

import (
	"context"
	"crypto/tls"
	"net/http"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/prashantv/gostub"
	corev1 "k8s.io/api/core/v1"

	xuanwuv1 "huawei-csi-driver/client/apis/xuanwu/v1"
	"huawei-csi-driver/csi/app"
	cfg "huawei-csi-driver/csi/app/config"
	"huawei-csi-driver/utils/k8sutils"
	"huawei-csi-driver/utils/log"
)

var (
	ctx = context.Background()
)

func TestMain(m *testing.M) {
	getGlobalConfig := gostub.StubFunc(&app.GetGlobalConfig, cfg.MockCompletedConfig())
	defer getGlobalConfig.Reset()

	log.MockInitLogging("webhook_test.log")
	defer log.MockStopLogging("webhook_test.log")

	m.Run()
}

// TestControllerStart test start function in normal scenario
func TestControllerStart(t *testing.T) {
	m := gomonkey.ApplyFunc(getNameSpaceFromEnv, func(webHookCfg Config) string {
		return "namespace"
	})
	defer m.Reset()

	m.ApplyMethod(reflect.TypeOf(app.GetGlobalConfig().K8sUtils), "GetSecret",
		func(_ *k8sutils.KubeClient, ctx context.Context, secretName, namespace string) (*corev1.Secret, error) {
			dataMap := make(map[string][]byte, 0)
			dataMap["privateKey"] = []byte{}
			dataMap["privateCert"] = []byte{}
			return &corev1.Secret{Data: dataMap}, nil
		})

	m.ApplyFunc(tls.X509KeyPair, func(cert, priv []byte) (tls.Certificate, error) {
		return tls.Certificate{}, nil
	})

	var srv *http.Server
	m.ApplyMethod(reflect.TypeOf(srv), "ListenAndServeTLS",
		func(_ *http.Server, certFile string, keyFile string) error {
			return nil
		})

	m.ApplyFunc(http.HandleFunc,
		func(pattern string, handler func(http.ResponseWriter, *http.Request)) {
			return
		})

	c := Controller{started: false}
	webHookCfg := Config{
		WebHookType: AdmissionWebHookValidating,
		PrivateKey:  "privateKey",
		PrivateCert: "privateCert",
	}
	if err := c.Start(ctx, webHookCfg, []AdmissionWebHookCFG{}); err != nil {
		t.Errorf("Start() error = %v, want no err", err)
	}
}

func newFakeClaim(providerName, configmapMeta, meta string) *xuanwuv1.StorageBackendClaim {
	return &xuanwuv1.StorageBackendClaim{
		Spec: xuanwuv1.StorageBackendClaimSpec{
			Provider:      providerName,
			ConfigMapMeta: configmapMeta,
			SecretMeta:    meta,
		},
	}
}

func TestValidateUpdate(t *testing.T) {
	if err := validateUpdate(context.TODO(), newFakeClaim(
		"provider-1", "configmap-1", "secret-1"), newFakeClaim(
		"provider-2", "configmap-1", "secret-1")); err == nil {
		t.Error("TestValidateUpdate failed")
	}
}

func TestValidateUpdateConfigmapChanged(t *testing.T) {
	if err := validateUpdate(context.TODO(), newFakeClaim(
		"provider-1", "configmap-1", "secret-1"), newFakeClaim(
		"provider-1", "configmap-2", "secret-1")); err == nil {
		t.Error("TestValidateUpdate failed")
	}
}
