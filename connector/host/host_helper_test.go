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

package host

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"huawei-csi-driver/csi/app"
	cfg "huawei-csi-driver/csi/app/config"
	"huawei-csi-driver/proto"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/k8sutils"
	"huawei-csi-driver/utils/log"
)

const (
	logName = "hostHelperTest.log"
)

var (
	testK8sUtils k8sutils.Interface
	testNodeInfo = &NodeHostInfo{
		HostName:       "test_hostname",
		IscsiInitiator: "test_iscsi_initiator",
		RoCEInitiator:  "test_roce_initiator",
		FCInitiators:   []string{"test_fc_initiator_1,test_fc_initiator_2"},
	}
)

func TestMain(m *testing.M) {
	getGlobalConfig := gostub.StubFunc(&app.GetGlobalConfig, cfg.MockCompletedConfig())
	defer getGlobalConfig.Reset()

	testK8sUtils = app.GetGlobalConfig().K8sUtils
	log.MockInitLogging(logName)
	defer log.MockStopLogging(logName)

	m.Run()
}
func TestNewNodeHostInfo(t *testing.T) {
	want := &NodeHostInfo{
		HostName:       "test_hostname",
		IscsiInitiator: "test_iscsi_initiator",
		RoCEInitiator:  "test_roce_initiator",
		FCInitiators:   nil,
	}
	getISCSIInitiator := gomonkey.ApplyFunc(proto.GetISCSIInitiator, func(_ context.Context) (string, error) {
		return want.IscsiInitiator, errors.New("no iscsi initiator")
	})
	defer getISCSIInitiator.Reset()

	getFCInitiator := gomonkey.ApplyFunc(proto.GetFCInitiator, func(_ context.Context) ([]string, error) {
		return nil, errors.New("no fc initiator")
	})
	defer getFCInitiator.Reset()

	getRoCEInitiator := gomonkey.ApplyFunc(proto.GetRoCEInitiator, func(_ context.Context) (string, error) {
		return want.RoCEInitiator, errors.New("no roce initiator")
	})
	defer getRoCEInitiator.Reset()

	t.Run("TestNewNodeHostInfoSuccessful", func(t *testing.T) {
		execShellCmd := gostub.StubFunc(&utils.ExecShellCmd, want.HostName, nil)
		defer execShellCmd.Reset()
		nodeHostInfo, err := NewNodeHostInfo(context.Background())
		if !reflect.DeepEqual(nodeHostInfo, want) {
			t.Errorf("NewNodeHostInfo() got = %v, want %v", nodeHostInfo, want)
		}
		require.NoError(t, err)
	})

	t.Run("TestNewNodeHostInfoWithQueryHostNameFail", func(t *testing.T) {
		execShellCmd := gostub.StubFunc(&utils.ExecShellCmd, nil, errors.New("timeout"))
		defer execShellCmd.Reset()

		nodeHostInfo, err := NewNodeHostInfo(context.Background())
		require.Error(t, err)
		require.Nil(t, nodeHostInfo)
	})
}

func TestSaveNodeHostInfoToSecretWithGetSecretError(t *testing.T) {
	getSecret := gomonkey.ApplyMethod(reflect.TypeOf(testK8sUtils), "GetSecret",
		func(_ *k8sutils.KubeClient, ctx context.Context, secretName, namespace string) (*corev1.Secret, error) {
			return nil, errors.New("error")
		})
	defer getSecret.Reset()

	t.Run("TestSaveNodeHostInfoToSecretWithGetSecretError", func(t *testing.T) {
		err := SaveNodeHostInfoToSecret(context.Background())
		require.Error(t, err)
	})

	isNotFound := gomonkey.ApplyFunc(apiErrors.IsNotFound, func(err error) bool {
		return true
	})
	defer isNotFound.Reset()

	t.Run("TestSaveNodeHostInfoToSecretWithIsNotFoundError", func(t *testing.T) {
		createSecretFunc := func(_ *k8sutils.KubeClient, ctx context.Context, secret *corev1.Secret) (*corev1.Secret,
			error) {
			return secret, nil
		}
		createSecret := gomonkey.ApplyMethod(reflect.TypeOf(testK8sUtils), "CreateSecret", createSecretFunc)
		defer createSecret.Reset()

		newNodeHostInfoFunc := func(ctx context.Context) (*NodeHostInfo, error) { return nil, nil }
		newNodeHostInfo := gomonkey.ApplyFunc(NewNodeHostInfo, newNodeHostInfoFunc)
		defer newNodeHostInfo.Reset()

		err := SaveNodeHostInfoToSecret(context.Background())
		require.Error(t, err)
	})

	createSecret := gomonkey.ApplyMethod(reflect.TypeOf(testK8sUtils), "CreateSecret",
		func(_ *k8sutils.KubeClient, ctx context.Context, secret *corev1.Secret) (*corev1.Secret, error) {
			return nil, errors.New("create  secret exist")
		})
	defer createSecret.Reset()

	isAlreadyExists := gomonkey.ApplyFunc(apiErrors.IsAlreadyExists, func(err error) bool {
		return false
	})
	defer isAlreadyExists.Reset()

	t.Run("TestSaveNodeHostInfoToSecretWithSecretNotExistAndCreateFail", func(t *testing.T) {
		err := SaveNodeHostInfoToSecret(context.Background())
		require.Error(t, err)
	})

	t.Run("TestSaveNodeHostInfoToSecretWithSecretNotExistAndCreateReturnExists", func(t *testing.T) {
		err := SaveNodeHostInfoToSecret(context.Background())
		require.Error(t, err)
	})
}

func TestSaveNodeHostInfoToSecretWithSecretNotExist(t *testing.T) {
	t.Run("TestSaveNodeHostInfoToSecretWithGetSecretNotExist", func(t *testing.T) {
		getSecret := gomonkey.ApplyMethod(reflect.TypeOf(testK8sUtils), "GetSecret",
			func(_ *k8sutils.KubeClient, ctx context.Context, secretName, namespace string) (*corev1.Secret, error) {
				return &corev1.Secret{}, nil
			})
		defer getSecret.Reset()

		isNotFound := gomonkey.ApplyFunc(apiErrors.IsNotFound, func(err error) bool {
			return true
		})
		defer isNotFound.Reset()

		createSecret := gomonkey.ApplyMethod(reflect.TypeOf(testK8sUtils), "CreateSecret",
			func(_ *k8sutils.KubeClient, ctx context.Context, secret *corev1.Secret) (*corev1.Secret, error) {
				return secret, nil
			})
		defer createSecret.Reset()

		newNodeHostInfo := gomonkey.ApplyFunc(NewNodeHostInfo, func(ctx context.Context) (*NodeHostInfo, error) {
			return testNodeInfo, nil
		})
		defer newNodeHostInfo.Reset()

		updateSecret := gomonkey.ApplyMethod(reflect.TypeOf(testK8sUtils), "UpdateSecret",
			func(_ *k8sutils.KubeClient, ctx context.Context, secret *corev1.Secret) (*corev1.Secret, error) {
				return secret, nil
			})
		defer updateSecret.Reset()

		err := SaveNodeHostInfoToSecret(context.Background())
		require.NoError(t, err)
	})
}

func TestSaveNodeHostInfoToSecretWithNewHostInfoError(t *testing.T) {
	t.Run("TestSaveNodeHostInfoToSecretWithNewHostInfoError", func(t *testing.T) {
		getSecret := gomonkey.ApplyMethod(reflect.TypeOf(testK8sUtils), "GetSecret",
			func(_ *k8sutils.KubeClient, ctx context.Context, secretName, namespace string) (*corev1.Secret, error) {
				return &corev1.Secret{}, nil
			})
		defer getSecret.Reset()

		newNodeHostInfo := gomonkey.ApplyFunc(NewNodeHostInfo, func(ctx context.Context) (*NodeHostInfo, error) {
			return nil, errors.New("new host info error")
		})
		defer newNodeHostInfo.Reset()

		err := SaveNodeHostInfoToSecret(context.Background())
		require.Error(t, err)
	})
}

func TestSaveNodeHostInfoToSecretWithUpdateSecret(t *testing.T) {
	getSecret := gomonkey.ApplyMethod(reflect.TypeOf(testK8sUtils), "GetSecret",
		func(_ *k8sutils.KubeClient, ctx context.Context, secretName, namespace string) (*corev1.Secret, error) {
			return &corev1.Secret{}, nil
		})
	defer getSecret.Reset()

	newNodeHostInfo := gomonkey.ApplyFunc(NewNodeHostInfo, func(ctx context.Context) (*NodeHostInfo, error) {
		return testNodeInfo, nil
	})
	defer newNodeHostInfo.Reset()

	t.Run("TestSaveNodeHostInfoToSecretWithMarshalError", func(t *testing.T) {
		errorMsg := "marshal error"
		marshal := gomonkey.ApplyFunc(json.Marshal, func(v any) ([]byte, error) {
			return nil, errors.New(errorMsg)
		})
		defer marshal.Reset()

		err := SaveNodeHostInfoToSecret(context.Background())
		require.Error(t, err)
		require.ErrorContains(t, err, errorMsg)
	})

	t.Run("TestSaveNodeHostInfoToSecretWithUpdateSecretFail", func(t *testing.T) {
		updateSecret := gomonkey.ApplyMethod(reflect.TypeOf(testK8sUtils), "UpdateSecret",
			func(_ *k8sutils.KubeClient, ctx context.Context, secret *corev1.Secret) (*corev1.Secret, error) {
				return nil, errors.New("error")
			})
		defer updateSecret.Reset()

		err := SaveNodeHostInfoToSecret(context.Background())
		require.Error(t, err)
	})

	t.Run("TestSaveNodeHostInfoToSecretWithUpdateSecretSuccess", func(t *testing.T) {
		updateSecret := gomonkey.ApplyMethod(reflect.TypeOf(testK8sUtils), "UpdateSecret",
			func(_ *k8sutils.KubeClient, ctx context.Context, secret *corev1.Secret) (*corev1.Secret, error) {
				return secret, nil
			})
		defer updateSecret.Reset()

		err := SaveNodeHostInfoToSecret(context.Background())
		require.NoError(t, err)
	})
}

func TestGetNodeHostInfosFromSecret(t *testing.T) {
	t.Run("TestGetNodeHostInfosFromSecretAndGetSecretError", func(t *testing.T) {
		getSecret := gomonkey.ApplyMethod(reflect.TypeOf(testK8sUtils), "GetSecret",
			func(_ *k8sutils.KubeClient, ctx context.Context, secretName, namespace string) (*corev1.Secret, error) {
				return nil, errors.New(" get secret error")
			})
		defer getSecret.Reset()

		nodeHostInfos, err := GetNodeHostInfosFromSecret(context.Background(), testNodeInfo.HostName)
		require.Error(t, err)
		require.Nil(t, nodeHostInfos)
	})
	t.Run("TestGetNodeHostInfosFromSecretAndDataIsNil", func(t *testing.T) {
		getSecret := gomonkey.ApplyMethod(reflect.TypeOf(testK8sUtils), "GetSecret",
			func(_ *k8sutils.KubeClient, ctx context.Context, secretName, namespace string) (*corev1.Secret, error) {
				return &corev1.Secret{}, nil
			})
		defer getSecret.Reset()

		nodeHostInfos, err := GetNodeHostInfosFromSecret(context.Background(), testNodeInfo.HostName)
		require.Error(t, err)
		require.EqualError(t, err, "secret data is empty")
		require.Nil(t, nodeHostInfos)
	})
	t.Run("TestGetNodeHostInfosFromSecretSuccess", func(t *testing.T) {
		secretData, err := json.Marshal(testNodeInfo)
		if err != nil {
			t.Errorf("TestGetNodeHostInfosFromSecretSuccess() json marshal %v", err)
		}
		testSecretData := map[string][]byte{
			testNodeInfo.HostName: secretData,
		}

		getSecret := gomonkey.ApplyMethod(reflect.TypeOf(testK8sUtils), "GetSecret",
			func(_ *k8sutils.KubeClient, ctx context.Context, secretName, namespace string) (*corev1.Secret, error) {
				return &corev1.Secret{
					Data: testSecretData,
				}, nil
			})
		defer getSecret.Reset()

		testReturnData, err := GetNodeHostInfosFromSecret(context.Background(), testNodeInfo.HostName)
		require.NoError(t, err)
		if !reflect.DeepEqual(testReturnData, testNodeInfo) {
			t.Errorf("TestGetNodeHostInfosFromSecretSuccess() got = %v, want %v", testReturnData, testNodeInfo)
		}
	})
}

func TestMakeNodeHostInfoSecret(t *testing.T) {
	want := &corev1.Secret{
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

	hostInfoSecret := makeNodeHostInfoSecret()
	if !reflect.DeepEqual(hostInfoSecret, want) {
		t.Errorf("TestMakeNodeHostInfoSecret() got = %v, want %v", hostInfoSecret, want)
	}
}
