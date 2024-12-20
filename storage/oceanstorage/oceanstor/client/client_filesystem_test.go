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
	"errors"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/require"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/base"
)

func TestAllowNfsShareAccess(t *testing.T) {
	t.Run("Normal", func(t *testing.T) {
		p := gomonkey.ApplyMethodReturn(testClient.RestClient, "Post", base.Response{
			Data: map[string]interface{}{
				"ID": "5",
			},
			Error: map[string]interface{}{
				"code":        float64(0),
				"description": "0",
			},
		}, nil)
		defer p.Reset()

		err := testClient.AllowNfsShareAccess(context.TODO(), &AllowNfsShareAccessRequest{
			Name:       "test",
			ParentID:   "test",
			AccessVal:  0,
			Sync:       1,
			AllSquash:  1,
			RootSquash: 1,
			VStoreID:   "0",
		})
		require.NoError(t, err)
	})

	t.Run("Error code is not zero", func(t *testing.T) {
		p := gomonkey.ApplyMethodReturn(testClient.RestClient, "Post",
			base.Response{
				Data: map[string]interface{}{
					"ID": "5",
				},
				Error: map[string]interface{}{
					"code":        float64(100),
					"description": "0",
				},
			}, nil)
		defer p.Reset()

		err := testClient.AllowNfsShareAccess(context.TODO(), &AllowNfsShareAccessRequest{
			Name:       "test",
			ParentID:   "test",
			AccessVal:  0,
			Sync:       1,
			AllSquash:  1,
			RootSquash: 1,
			VStoreID:   "0",
		})
		require.Error(t, err)
	})

	t.Run("Post quest return error", func(t *testing.T) {
		p := gomonkey.ApplyMethodReturn(testClient.RestClient, "Post",
			base.Response{}, errors.New("mock err"))
		defer p.Reset()

		err := testClient.AllowNfsShareAccess(context.TODO(), &AllowNfsShareAccessRequest{
			Name:       "test",
			ParentID:   "test",
			AccessVal:  0,
			Sync:       1,
			AllSquash:  1,
			RootSquash: 1,
			VStoreID:   "0",
		})
		require.Error(t, err)
	})
}
