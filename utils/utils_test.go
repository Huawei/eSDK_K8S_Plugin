/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2025. All rights reserved.
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
	"io"
	"os"
	"reflect"
	"testing"
	"unsafe"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	cfg "github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app/config"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/k8sutils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
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
	t.Run("TestGetPasswordFromSecret secret is nil case", func(t *testing.T) {
		m := mockGetSecret(nil, nil)
		defer m.Reset()

		_, err := GetPasswordFromSecret(context.TODO(), "sec-name", "sec-namespace")
		require.Error(t, err)
	})

	t.Run("TestGetPasswordFromSecret get secret error case", func(t *testing.T) {
		m := mockGetSecret(nil, errors.New("mock error"))
		defer m.Reset()

		_, err := GetPasswordFromSecret(context.TODO(), "sec-name", "sec-namespace")
		require.Error(t, err)
	})

	t.Run("TestGetPasswordFromSecret secret data is nil case", func(t *testing.T) {
		m := mockGetSecret(map[string][]byte{}, nil)
		defer m.Reset()

		_, err := GetPasswordFromSecret(context.TODO(), "sec-name", "sec-namespace")
		require.Error(t, err)
	})

	t.Run("TestGetPasswordFromSecret secret data dose not have password case", func(t *testing.T) {
		m := mockGetSecret(map[string][]byte{"user": []byte("mock-user")}, nil)
		defer m.Reset()

		_, err := GetPasswordFromSecret(context.TODO(), "sec-name", "sec-namespace")
		require.Error(t, err)
	})

	t.Run("TestGetPasswordFromSecret normal case", func(t *testing.T) {
		m := mockGetSecret(map[string][]byte{
			"user":     []byte("mock-user"),
			"password": []byte("mock-pw"),
		}, nil)
		defer m.Reset()

		pw, err := GetPasswordFromSecret(context.TODO(), "sec-name", "sec-namespace")
		require.NoError(t, err)
		require.Equal(t, "mock-pw", pw)
	})
}

func TestGetCertFromSecretFailed(t *testing.T) {
	t.Run("TestGetCertFromSecret secret is nil case", func(t *testing.T) {
		m := mockGetSecret(nil, nil)
		defer m.Reset()

		_, err := GetCertFromSecret(context.TODO(), "sec-name", "sec-namespace")
		require.Error(t, err)
	})

	t.Run("TestGetCertFromSecret get secret error case", func(t *testing.T) {
		m := mockGetSecret(nil, errors.New("mock error"))
		defer m.Reset()

		_, err := GetCertFromSecret(context.TODO(), "sec-name", "sec-namespace")
		require.Error(t, err)
	})

	t.Run("GetCertFromSecret secret data dose not have cert case", func(t *testing.T) {
		m := mockGetSecret(map[string][]byte{}, nil)
		defer m.Reset()

		_, err := GetCertFromSecret(context.TODO(), "sec-name", "sec-namespace")
		require.Error(t, err)
	})
}

func TestGetCertFromSecretSuccess(t *testing.T) {
	t.Run("GetCertFromSecret normal case", func(t *testing.T) {
		m := mockGetSecret(map[string][]byte{
			"tls.crt": []byte("mock-cert"),
		}, nil)
		defer m.Reset()

		pw, err := GetCertFromSecret(context.TODO(), "sec-name", "sec-namespace")
		require.NoError(t, err)
		require.Equal(t, pw, []byte("mock-cert"))
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

func TestIsCapacityAvailable_Success(t *testing.T) {
	// arrange
	const validSize int64 = 1024 * 1024 * 1024
	const notValidSize int64 = 1024*1024*1024 + 511
	cases := []struct {
		name     string
		size     int64
		params   map[string]any
		hasError bool
	}{
		{
			name:     "Empty_Map",
			size:     validSize,
			params:   map[string]any{},
			hasError: false,
		},
		{
			name:     "Empty_Value",
			size:     validSize,
			params:   map[string]any{constants.DisableVerifyCapacityKey: ""},
			hasError: false,
		},
		{
			name:     "False",
			size:     validSize,
			params:   map[string]any{constants.DisableVerifyCapacityKey: "false"},
			hasError: false,
		},
		{
			name:     "True_Valid",
			size:     validSize,
			params:   map[string]any{constants.DisableVerifyCapacityKey: "false"},
			hasError: false,
		},
		{
			name:     "True_Not_Valid",
			size:     notValidSize,
			params:   map[string]any{constants.DisableVerifyCapacityKey: "false"},
			hasError: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// action
			err := IsCapacityAvailable(c.size, constants.AllocationUnitBytes, c.params)

			// assert
			assert.Equal(t, c.hasError, err != nil)
		})
	}
}

func TestIsNil(t *testing.T) {
	tests := []struct {
		name string
		val  any
		want bool
	}{
		{name: "nil map", val: zeroValue[map[string]any](), want: true},
		{name: "map has value", val: map[string]any{"key": 1}, want: false},
		{name: "nil func", val: zeroValue[func()](), want: true},
		{name: "func has value", val: func() {}, want: false},
		{name: "nil chan", val: zeroValue[chan any](), want: true},
		{name: "chan has value", val: make(chan any), want: false},
		{name: "nil pointer", val: zeroValue[*int](), want: true},
		{name: "pointer has value", val: &struct{}{}, want: false},
		{name: "nil unsafe pointer", val: zeroValue[unsafe.Pointer](), want: true},
		{name: "unsafe pointer has value", val: unsafe.Pointer(&struct{}{}), want: false},
		{name: "nil interface", val: zeroValue[error](), want: true},
		{name: "interface has value", val: io.EOF, want: false},
		{name: "nil slice", val: zeroValue[[]int](), want: true},
		{name: "nil slice", val: make([]int, 0), want: false},
		{name: "base type", val: 0, want: false},
		{name: "empty type of variable", val: nil, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsNil(tt.val))
		})
	}
}
