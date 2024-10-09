/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2022-2023. All rights reserved.
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

package client

import (
	"context"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/require"
)

const (
	logName = "clientNamespaceTest.log"
)

func TestAllowNfsShareAccess(t *testing.T) {
	t.Run("Normal", func(t *testing.T) {
		guard := gomonkey.ApplyMethod(reflect.TypeOf(testClient), "Post",
			func(_ *RestClient, _ context.Context, _ string, _ map[string]interface{}) (map[string]interface{}, error) {
				return map[string]interface{}{
					"data":   map[string]interface{}{},
					"result": map[string]interface{}{"code": float64(0), "description": ""},
				}, nil
			})
		defer guard.Reset()

		err := testClient.AllowNfsShareAccess(context.TODO(), &AllowNfsShareAccessRequest{
			AccessName:  "test",
			ShareId:     "test",
			AccessValue: 0,
			AllSquash:   1,
			RootSquash:  1,
			AccountId:   "0",
		})
		require.NoError(t, err)
	})

	t.Run("Result Code Not Exist", func(t *testing.T) {
		guard := gomonkey.ApplyMethod(reflect.TypeOf(testClient), "Post",
			func(_ *RestClient, _ context.Context, _ string, _ map[string]interface{}) (map[string]interface{}, error) {
				return map[string]interface{}{
					"data": map[string]interface{}{},
				}, nil
			})
		defer guard.Reset()

		err := testClient.AllowNfsShareAccess(context.TODO(), &AllowNfsShareAccessRequest{
			AccessName:  "test",
			ShareId:     "test",
			AccessValue: 0,
			AllSquash:   1,
			RootSquash:  1,
			AccountId:   "0",
		})
		require.Error(t, err)
	})

	t.Run("RestClient Already Exist", func(t *testing.T) {
		guard := gomonkey.ApplyMethod(reflect.TypeOf(testClient), "Post",
			func(_ *RestClient, _ context.Context, _ string, _ map[string]interface{}) (map[string]interface{}, error) {
				return map[string]interface{}{
					"data":   map[string]interface{}{},
					"result": map[string]interface{}{"code": float64(clientAlreadyExist), "description": ""},
				}, nil
			})
		defer guard.Reset()

		err := testClient.AllowNfsShareAccess(context.TODO(), &AllowNfsShareAccessRequest{
			AccessName:  "test",
			ShareId:     "test",
			AccessValue: 0,
			AllSquash:   1,
			RootSquash:  1,
			AccountId:   "0",
		})
		require.NoError(t, err)
	})

	t.Run("Error code is not zero", func(t *testing.T) {
		guard := gomonkey.ApplyMethod(reflect.TypeOf(testClient), "Post",
			func(_ *RestClient, _ context.Context, _ string, _ map[string]interface{}) (map[string]interface{}, error) {
				return map[string]interface{}{
					"data":   map[string]interface{}{},
					"result": map[string]interface{}{"code": float64(100), "description": ""},
				}, nil
			})
		defer guard.Reset()

		err := testClient.AllowNfsShareAccess(context.TODO(), &AllowNfsShareAccessRequest{
			AccessName:  "test",
			ShareId:     "test",
			AccessValue: 0,
			AllSquash:   1,
			RootSquash:  1,
			AccountId:   "0",
		})
		require.Error(t, err)
	})
}
