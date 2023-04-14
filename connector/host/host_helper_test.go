/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2022. All rights reserved.
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
	. "github.com/smartystreets/goconvey/convey"
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

	Convey("TestNewNodeHostInfoSuccessful", t, func() {
		execShellCmd := gostub.StubFunc(&utils.ExecShellCmd, want.HostName, nil)
		defer execShellCmd.Reset()
		nodeHostInfo, err := NewNodeHostInfo(context.Background())
		if !reflect.DeepEqual(nodeHostInfo, want) {
			t.Errorf("NewNodeHostInfo() got = %v, want %v", nodeHostInfo, want)
		}
		So(err, ShouldBeNil)
	})

	Convey("TestNewNodeHostInfoWithQueryHostNameFail", t, func() {
		execShellCmd := gostub.StubFunc(&utils.ExecShellCmd, nil, errors.New("timeout"))
		defer execShellCmd.Reset()

		nodeHostInfo, err := NewNodeHostInfo(context.Background())
		So(nodeHostInfo, ShouldBeNil)
		So(err, ShouldBeError)
	})
}

func TestSaveNodeHostInfoToSecretWithGetSecretError(t *testing.T) {
	getSecret := gomonkey.ApplyMethod(reflect.TypeOf(testK8sUtils), "GetSecret",
		func(_ *k8sutils.KubeClient, ctx context.Context, secretName, namespace string) (*corev1.Secret, error) {
			return nil, errors.New("error")
		})
	defer getSecret.Reset()

	Convey("TestSaveNodeHostInfoToSecretWithGetSecretError", t, func() {
		err := SaveNodeHostInfoToSecret(context.Background())
		So(err, ShouldBeError)
	})

	isNotFound := gomonkey.ApplyFunc(apiErrors.IsNotFound, func(err error) bool {
		return true
	})
	defer isNotFound.Reset()

	Convey("TestSaveNodeHostInfoToSecretWithIsNotFoundError", t, func() {
		createSecret := gomonkey.ApplyMethod(reflect.TypeOf(testK8sUtils), "CreateSecret",
			func(_ *k8sutils.KubeClient, ctx context.Context, secret *corev1.Secret) (*corev1.Secret, error) {
				return secret, nil
			})
		defer createSecret.Reset()
		err := SaveNodeHostInfoToSecret(context.Background())
		So(err, ShouldBeError)
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

	Convey("TestSaveNodeHostInfoToSecretWithSecretNotExistAndCreateFail", t, func() {
		err := SaveNodeHostInfoToSecret(context.Background())
		So(err, ShouldBeError)
	})

	Convey("TestSaveNodeHostInfoToSecretWithSecretNotExistAndCreateReturnExists", t, func() {
		err := SaveNodeHostInfoToSecret(context.Background())
		So(err, ShouldBeError)
	})
}

func TestSaveNodeHostInfoToSecretWithSecretNotExist(t *testing.T) {
	Convey("TestSaveNodeHostInfoToSecretWithGetSecretNotExist", t, func() {
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
		So(err, ShouldBeNil)
	})
}

func TestSaveNodeHostInfoToSecretWithNewHostInfoError(t *testing.T) {
	Convey("TestSaveNodeHostInfoToSecretWithNewHostInfoError", t, func() {
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
		So(err, ShouldBeError)
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

	Convey("TestSaveNodeHostInfoToSecretWithMarshalError", t, func() {
		errorMsg := "marshal error"
		marshal := gomonkey.ApplyFunc(json.Marshal, func(v any) ([]byte, error) {
			return nil, errors.New(errorMsg)
		})
		defer marshal.Reset()

		err := SaveNodeHostInfoToSecret(context.Background())
		So(err, ShouldBeError)
		So(err, ShouldEqual, errorMsg)
	})

	Convey("TestSaveNodeHostInfoToSecretWithUpdateSecretFail", t, func() {
		updateSecret := gomonkey.ApplyMethod(reflect.TypeOf(testK8sUtils), "UpdateSecret",
			func(_ *k8sutils.KubeClient, ctx context.Context, secret *corev1.Secret) (*corev1.Secret, error) {
				return nil, errors.New("error")
			})
		defer updateSecret.Reset()

		err := SaveNodeHostInfoToSecret(context.Background())
		So(err, ShouldBeError)
	})

	Convey("TestSaveNodeHostInfoToSecretWithUpdateSecretSuccess", t, func() {
		updateSecret := gomonkey.ApplyMethod(reflect.TypeOf(testK8sUtils), "UpdateSecret",
			func(_ *k8sutils.KubeClient, ctx context.Context, secret *corev1.Secret) (*corev1.Secret, error) {
				return secret, nil
			})
		defer updateSecret.Reset()

		err := SaveNodeHostInfoToSecret(context.Background())
		So(err, ShouldBeNil)
	})
}

func TestGetNodeHostInfosFromSecret(t *testing.T) {
	Convey("TestGetNodeHostInfosFromSecretAndGetSecretError", t, func() {
		getSecret := gomonkey.ApplyMethod(reflect.TypeOf(testK8sUtils), "GetSecret",
			func(_ *k8sutils.KubeClient, ctx context.Context, secretName, namespace string) (*corev1.Secret, error) {
				return nil, errors.New(" get secret error")
			})
		defer getSecret.Reset()

		nodeHostInfos, err := GetNodeHostInfosFromSecret(context.Background(), testNodeInfo.HostName)
		So(err, ShouldBeError)
		So(nodeHostInfos, ShouldBeNil)
	})
	Convey("TestGetNodeHostInfosFromSecretAndDataIsNil", t, func() {
		getSecret := gomonkey.ApplyMethod(reflect.TypeOf(testK8sUtils), "GetSecret",
			func(_ *k8sutils.KubeClient, ctx context.Context, secretName, namespace string) (*corev1.Secret, error) {
				return &corev1.Secret{}, nil
			})
		defer getSecret.Reset()

		nodeHostInfos, err := GetNodeHostInfosFromSecret(context.Background(), testNodeInfo.HostName)
		So(err, ShouldBeError)
		So(err.Error(), ShouldEqual, "secret data is empty")
		So(nodeHostInfos, ShouldBeNil)
	})
	Convey("TestGetNodeHostInfosFromSecretSuccess", t, func() {
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
		So(err, ShouldBeNil)
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
