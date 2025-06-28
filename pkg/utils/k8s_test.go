/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2023-2025. All rights reserved.
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
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/k8sutils"
)

func TestGetPasswordFromSecret(t *testing.T) {
	//arrange
	name := "sec-name"
	ns := "sec-namespace"
	ctx := context.TODO()
	cases := []struct {
		name        string
		data        map[string][]byte
		hasError    bool
		expectation string
		err         error
	}{
		{name: "TestGetPasswordFromSecret secret is nil case", data: nil, hasError: true},
		{name: "TestGetPasswordFromSecret get secret error case", data: map[string][]byte{},
			hasError: true, err: errors.New("mock error")},
		{name: "TestGetPasswordFromSecret secret data is nil case", data: map[string][]byte{}, hasError: true},
		{name: "TestGetPasswordFromSecret secret data dose not have password case",
			data: map[string][]byte{"user": []byte("mock-user")}, hasError: true},
		{name: "TestGetPasswordFromSecret normal case",
			data:     map[string][]byte{"user": []byte("mock-user"), "password": []byte("mock-pw")},
			hasError: false, expectation: "mock-pw"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			//mock
			m := mockGetSecret(tc.data, tc.err)
			//clean
			defer m.Reset()
			//active
			result, err := GetPasswordFromSecret(ctx, name, ns)
			//assert
			if tc.hasError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectation, result)
			}
		})
	}
}

func TestGetPwdAndAuthModeFromSecret(t *testing.T) {
	// arrange
	name := "sec-name"
	ns := "sec-namespace"
	ldap := constants.AuthModeScopeLDAP
	ctx := context.TODO()
	cases := []struct {
		name     string
		data     map[string][]byte
		hasError bool
		expRes   *BackendAuthInfo
	}{
		{name: "secret is nil case", data: nil, hasError: true},
		{name: "secret data have no value case", data: map[string][]byte{}, hasError: true},
		{name: "secret data dose not have password case",
			data: map[string][]byte{"user": []byte("mock-user")}, hasError: true},
		{name: "secret data dose not have authmode case",
			data:     map[string][]byte{"user": []byte("mock-user"), "password": []byte("mock-pw")},
			hasError: false, expRes: &BackendAuthInfo{User: "mock-user", Password: "mock-pw", Scope: "0"}},
		{name: "normal case",
			data: map[string][]byte{"user": []byte("mock-user"), "password": []byte("mock-pw"),
				"authenticationMode": []byte(ldap)},
			hasError: false, expRes: &BackendAuthInfo{User: "mock-user", Password: "mock-pw", Scope: "1"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			//mock
			m := mockGetSecret(c.data, nil)
			defer m.Reset()
			//action
			params, err := GetAuthInfoFromSecret(ctx, name, ns)
			//assert
			if !c.hasError {
				assert.Equal(t, params, c.expRes)
			} else {
				assert.Error(t, err, c.hasError)
			}
		})
	}
}

func mockGetSecret(data map[string][]byte, err error) *gomonkey.Patches {
	return gomonkey.ApplyMethod(reflect.TypeOf(app.GetGlobalConfig().K8sUtils),
		"GetSecret",
		func(_ *k8sutils.KubeClient, ctx context.Context, secretName, namespace string) (*corev1.Secret, error) {
			return &corev1.Secret{Data: data}, err
		})
}
