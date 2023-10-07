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

package utils

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"

	"huawei-csi-driver/pkg/constants"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/prashantv/gostub"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"

	"huawei-csi-driver/csi/app"
	cfg "huawei-csi-driver/csi/app/config"
	"huawei-csi-driver/utils/k8sutils"
	"huawei-csi-driver/utils/log"
)

const (
	logName = "utilsTest.log"
)

func TestPathExist(t *testing.T) {
	type pathCase struct {
		name     string
		Path     string
		expected bool
	}
	var testCases = []pathCase{
		{
			"notExist",
			"wrongDir",
			false,
		},
		{
			"exist",
			"/rightDir",
			true,
		},
	}

	err := os.Mkdir("/rightDir", os.ModePerm)
	if err != nil {
		return
	}
	defer func() {
		err := os.Remove("/rightDir")
		if err != nil {

		}
	}()

	for _, c := range testCases {
		exist, err := PathExist(c.Path)
		assert.Equal(t, c.expected, exist, "case name is %s, result: %v, %v", c.name, exist, err)
	}
}

func TestMaskSensitiveInfo(t *testing.T) {
	type maskInfoCase struct {
		name       string
		info       interface{}
		expectInfo string
	}

	var testCases = []maskInfoCase{
		{
			"stringMaskInfo",
			"iscsiadm -m node -T iqn.2003-01.io.k8s:e2e.volume -p 192.168.0.2 --interface default --op new",
			"iscsiadm -m node -T ***-p 192.168.0.2 --interface default --op new",
		},
	}
	for _, c := range testCases {
		maskInfo := MaskSensitiveInfo(c.info)
		assert.Equal(t, c.expectInfo, maskInfo, "case name is %s, result: %v", c.name, maskInfo)
	}
}

func TestGetSnapshotName(t *testing.T) {
	shortName := GetSnapshotName("TestShortName")
	assert.Equal(t, "TestShortName", shortName)

	longName := GetSnapshotName("snapshot-f311b342-a4b4-4235-98b3-5a1c289849c0")
	assert.Equal(t, "snapshot-f311b342-a4b4-4235-98b", longName)
}

func TestGetFusionStorageLunName(t *testing.T) {
	shortName := GetFusionStorageLunName("pvc-331a3fcd-6380-4de5-9bc0-be95c801edeb")
	assert.Equal(t, "pvc-331a3fcd-6380-4de5-9bc0-be95c801edeb", shortName)

	longName := GetFusionStorageLunName("pvc-331a3fcd-6380-4de5-9bc0-be95c801edeb-pvc-331a3fcd-6380-4de5-" +
		"9bc0-be95c801edeb-pvc-331a3fcd-6380-4de5-9bc0-be95c801edeb")
	assert.Equal(t, "pvc-331a3fcd-6380-4de5-9bc0-be95c801edeb-pvc-331a3fcd-6380-4de5-9bc0-"+
		"be95c801edeb-pvc-331a3fcd-", longName)
}

func TestGetFusionStorageSnapshotName(t *testing.T) {
	shortName := GetFusionStorageSnapshotName("TestShortName")
	assert.Equal(t, "TestShortName", shortName)

	longName := GetFusionStorageSnapshotName("snapshot-331a3fcd-6380-4de5-9bc0-be95c801edeb-331a3fcd-6380-4de5-" +
		"9bc0-be95c801edeb-331a3fcd-6380-4de5-9bc0-be95c801edeb")
	assert.Equal(t, "snapshot-331a3fcd-6380-4de5-9bc0-be95c801edeb-331a3fcd-6380-4de5-9bc0-"+
		"be95c801edeb-331a3fcd-638", longName)
}

func TestGetFileSystemName(t *testing.T) {
	replaceName := GetFileSystemName("pvc-331a3fcd-6380-4de5-9bc0-be95c801edeb")
	assert.Equal(t, "pvc_331a3fcd_6380_4de5_9bc0_be95c801edeb", replaceName)
}

func TestGetFSSnapshotName(t *testing.T) {
	replaceName := GetFSSnapshotName("snapshot-331a3fcd-6380-4de5-9bc0-be95c801edeb")
	assert.Equal(t, "snapshot_331a3fcd_6380_4de5_9bc0_be95c801edeb", replaceName)
}

func TestGetSharePath(t *testing.T) {
	replaceName := GetSharePath("pvc-331a3fcd-6380-4de5-9bc0-be95c801edeb")
	assert.Equal(t, "/pvc_331a3fcd_6380_4de5_9bc0_be95c801edeb/", replaceName)
}

func TestGetFSSharePath(t *testing.T) {
	replaceName := GetFSSharePath("pvc-331a3fcd-6380-4de5-9bc0-be95c801edeb")
	assert.Equal(t, "/pvc_331a3fcd_6380_4de5_9bc0_be95c801edeb/", replaceName)
}

func TestGetHostName(t *testing.T) {
	temp := ExecShellCmd
	defer func() { ExecShellCmd = temp }()

	ExecShellCmd = func(_ context.Context, _ string, _ ...interface{}) (string, error) {
		return "worker-node1", nil
	}

	expectedHost, err := GetHostName(context.Background())
	assert.Equal(t, "worker-node1", expectedHost,
		"case name is testGetHostName, result: %v, error: %v", expectedHost, err)
}

func mockGetSecret(data map[string][]byte, err error) *gomonkey.Patches {
	return gomonkey.ApplyMethod(reflect.TypeOf(app.GetGlobalConfig().K8sUtils),
		"GetSecret",
		func(_ *k8sutils.KubeClient, ctx context.Context, secretName, namespace string) (*corev1.Secret, error) {
			return &corev1.Secret{Data: data}, err
		})
}

func TestGetPasswordFromSecret(t *testing.T) {
	Convey("TestGetPasswordFromSecret secret is nil case", t, func() {
		m := mockGetSecret(nil, nil)
		defer m.Reset()

		_, err := GetPasswordFromSecret(context.TODO(), "sec-name", "sec-namespace")
		So(err, ShouldBeError)
	})

	Convey("TestGetPasswordFromSecret get secret error case", t, func() {
		m := mockGetSecret(nil, errors.New("mock error"))
		defer m.Reset()

		_, err := GetPasswordFromSecret(context.TODO(), "sec-name", "sec-namespace")
		So(err, ShouldBeError)
	})

	Convey("TestGetPasswordFromSecret secret data is nil case", t, func() {
		m := mockGetSecret(map[string][]byte{}, nil)
		defer m.Reset()

		_, err := GetPasswordFromSecret(context.TODO(), "sec-name", "sec-namespace")
		So(err, ShouldBeError)
	})

	Convey("TestGetPasswordFromSecret secret data dose not have password case", t, func() {
		m := mockGetSecret(map[string][]byte{"user": []byte("mock-user")}, nil)
		defer m.Reset()

		_, err := GetPasswordFromSecret(context.TODO(), "sec-name", "sec-namespace")
		So(err, ShouldBeError)
	})

	Convey("TestGetPasswordFromSecret normal case", t, func() {
		m := mockGetSecret(map[string][]byte{
			"user":     []byte("mock-user"),
			"password": []byte("mock-pw"),
		}, nil)
		defer m.Reset()

		pw, err := GetPasswordFromSecret(context.TODO(), "sec-name", "sec-namespace")
		So(err, ShouldBeNil)
		So(pw, ShouldEqual, "mock-pw")
	})
}

func TestGetCertFromSecretFailed(t *testing.T) {
	Convey("TestGetCertFromSecret secret is nil case", t, func() {
		m := mockGetSecret(nil, nil)
		defer m.Reset()

		_, err := GetCertFromSecret(context.TODO(), "sec-name", "sec-namespace")
		So(err, ShouldBeError)
	})

	Convey("TestGetCertFromSecret get secret error case", t, func() {
		m := mockGetSecret(nil, errors.New("mock error"))
		defer m.Reset()

		_, err := GetCertFromSecret(context.TODO(), "sec-name", "sec-namespace")
		So(err, ShouldBeError)
	})

	Convey("GetCertFromSecret secret data dose not have cert case", t, func() {
		m := mockGetSecret(map[string][]byte{}, nil)
		defer m.Reset()

		_, err := GetCertFromSecret(context.TODO(), "sec-name", "sec-namespace")
		So(err, ShouldBeError)
	})
}

func TestGetCertFromSecretSuccess(t *testing.T) {
	Convey("GetCertFromSecret normal case", t, func() {
		m := mockGetSecret(map[string][]byte{
			"tls.crt": []byte("mock-cert"),
		}, nil)
		defer m.Reset()

		pw, err := GetCertFromSecret(context.TODO(), "sec-name", "sec-namespace")
		So(err, ShouldBeNil)
		So(pw, ShouldEqual, []byte("mock-cert"))
	})
}

func TestIsContainFileType(t *testing.T) {
	type IsContainParam struct {
		target   constants.FileType
		list     []constants.FileType
		expected bool
	}
	var testCases = []IsContainParam{
		{
			"ext3",
			[]constants.FileType{constants.Ext3, constants.Ext4, constants.Ext2, constants.Xfs},
			true,
		},
		{
			"ext1",
			[]constants.FileType{constants.Ext3, constants.Ext4, constants.Ext2, constants.Xfs},
			false,
		},
	}

	for _, c := range testCases {
		expected := IsContain(c.target, c.list)
		assert.Equal(t, c.expected, expected)
	}
}

func TestMain(m *testing.M) {
	log.MockInitLogging(logName)
	defer log.MockStopLogging(logName)

	getGlobalConfig := gostub.StubFunc(&app.GetGlobalConfig, cfg.MockCompletedConfig())
	defer getGlobalConfig.Reset()

	m.Run()
}
