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
	"reflect"
	"testing"

	"bou.ke/monkey"
	. "github.com/smartystreets/goconvey/convey"
)

func TestAllowNfsShareAccess(t *testing.T) {
	Convey("Normal", t, func() {
		guard := monkey.PatchInstanceMethod(reflect.TypeOf(testClient), "Post",
			func(_ *BaseClient, _ context.Context, _ string, _ map[string]interface{}) (Response, error) {
				return Response{
					Data: map[string]interface{}{
						"ID": "5",
					},
					Error: map[string]interface{}{
						"code":        float64(0),
						"description": "0",
					},
				}, nil
			})
		defer guard.Unpatch()

		err := testClient.AllowNfsShareAccess(context.TODO(), &AllowNfsShareAccessRequest{
			Name:       "test",
			ParentID:   "test",
			AccessVal:  0,
			Sync:       1,
			AllSquash:  1,
			RootSquash: 1,
			VStoreID:   "0",
		})
		So(err, ShouldBeNil)
	})

	Convey("Error code is not zero", t, func() {
		guard := monkey.PatchInstanceMethod(reflect.TypeOf(testClient), "Post",
			func(_ *BaseClient, _ context.Context, _ string, _ map[string]interface{}) (Response, error) {
				return Response{
					Data: map[string]interface{}{
						"ID": "5",
					},
					Error: map[string]interface{}{
						"code":        float64(100),
						"description": "0",
					},
				}, nil
			})
		defer guard.Unpatch()

		err := testClient.AllowNfsShareAccess(context.TODO(), &AllowNfsShareAccessRequest{
			Name:       "test",
			ParentID:   "test",
			AccessVal:  0,
			Sync:       1,
			AllSquash:  1,
			RootSquash: 1,
			VStoreID:   "0",
		})
		So(err, ShouldBeError)
	})

	Convey("Post quest return error", t, func() {
		guard := monkey.PatchInstanceMethod(reflect.TypeOf(testClient), "Post",
			func(_ *BaseClient, _ context.Context, _ string, _ map[string]interface{}) (Response, error) {
				return Response{}, errors.New("mock err")
			})
		defer guard.Unpatch()

		err := testClient.AllowNfsShareAccess(context.TODO(), &AllowNfsShareAccessRequest{
			Name:       "test",
			ParentID:   "test",
			AccessVal:  0,
			Sync:       1,
			AllSquash:  1,
			RootSquash: 1,
			VStoreID:   "0",
		})
		So(err, ShouldBeError)
	})
}
