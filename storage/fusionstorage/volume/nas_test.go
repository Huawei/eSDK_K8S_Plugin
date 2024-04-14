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

package volume

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"bou.ke/monkey"
	"github.com/agiledragon/gomonkey/v2"
	"github.com/smartystreets/goconvey/convey"

	"huawei-csi-driver/storage/fusionstorage/client"
	"huawei-csi-driver/storage/fusionstorage/types"
	"huawei-csi-driver/utils/log"
)

const (
	logName = "expandTest.log"
)

var testClient *client.Client
var ctx context.Context

func TestMain(m *testing.M) {
	log.MockInitLogging(logName)
	defer log.MockStopLogging(logName)

	ctx = context.TODO()

	m.Run()
}

func TestPreCreate(t *testing.T) {
	convey.Convey("Normal", t, func() {
		m := gomonkey.ApplyMethod(reflect.TypeOf(testClient),
			"GetPoolByName",
			func(_ *client.Client, ctx context.Context, poolName string) (map[string]interface{}, error) {
				return map[string]interface{}{"mock": "mock"}, nil
			})
		defer m.Reset()

		nas := NewNAS(testClient)
		err := nas.preCreate(context.TODO(), map[string]interface{}{
			"authclient": "*",
			"name":       "mock-name",
		})
		convey.So(err, convey.ShouldBeNil)
	})

	convey.Convey("Auth client empty", t, func() {
		nas := NewNAS(testClient)
		err := nas.preCreate(context.TODO(), map[string]interface{}{
			"name": "mock-name",
		})
		convey.So(err, convey.ShouldBeError)
	})

	convey.Convey("Name is empty", t, func() {
		m := gomonkey.ApplyMethod(reflect.TypeOf(testClient),
			"GetPoolByName",
			func(_ *client.Client, ctx context.Context, poolName string) (map[string]interface{}, error) {
				return map[string]interface{}{"mock": "mock"}, nil
			})
		defer m.Reset()

		nas := NewNAS(testClient)
		err := nas.preCreate(context.TODO(), map[string]interface{}{
			"authclient": "*",
		})
		convey.So(err, convey.ShouldBeError)
	})
}

func TestExpandWithNormal(t *testing.T) {
	convey.Convey("Normal", t, func() {
		_ = monkey.PatchInstanceMethod(reflect.TypeOf(testClient), "GetFileSystemByName",
			func(_ *client.Client, _ context.Context, _ string) (map[string]interface{}, error) {
				return map[string]interface{}{
					"id": float64(522),
				}, nil
			})
		_ = monkey.PatchInstanceMethod(reflect.TypeOf(testClient), "GetQuotaByFileSystemById",
			func(_ *client.Client, _ context.Context, _ string) (map[string]interface{}, error) {
				return map[string]interface{}{
					"id":               "522@2",
					"space_hard_quota": float64(2147483648),
					"space_soft_quota": float64(18446744073709551615),
				}, nil
			})
		_ = monkey.PatchInstanceMethod(reflect.TypeOf(testClient), "UpdateQuota",
			func(_ *client.Client, _ context.Context, param map[string]interface{}) error {
				_, exitId := param["id"]
				_, exitHardQuota := param["space_hard_quota"]
				_, exitSoftQuota := param["space_soft_quota"]
				if exitId && (exitSoftQuota || exitHardQuota) {
					return nil
				}
				return errors.New("fail")
			})
		nas := NewNAS(testClient)
		err := nas.Expand(context.TODO(), "123", 3221225472)
		convey.So(err, convey.ShouldBeNil)
	})
}

func TestExpandWithFileSystemNotExit(t *testing.T) {
	convey.Convey("File System Not Exist", t, func() {
		_ = monkey.PatchInstanceMethod(reflect.TypeOf(testClient), "GetFileSystemByName",
			func(_ *client.Client, _ context.Context, _ string) (map[string]interface{}, error) {
				return nil, nil
			})
		_ = monkey.PatchInstanceMethod(reflect.TypeOf(testClient), "GetQuotaByFileSystemById",
			func(_ *client.Client, _ context.Context, _ string) (map[string]interface{}, error) {
				return map[string]interface{}{
					"id":               "522@2",
					"space_hard_quota": float64(2147483648),
					"space_soft_quota": float64(18446744073709551615),
				}, nil
			})
		_ = monkey.PatchInstanceMethod(reflect.TypeOf(testClient), "UpdateQuota",
			func(_ *client.Client, _ context.Context, param map[string]interface{}) error {
				_, exitId := param["id"]
				_, exitHardQuota := param["space_hard_quota"]
				_, exitSoftQuota := param["space_soft_quota"]
				if exitId && (exitSoftQuota || exitHardQuota) {
					return nil
				}
				return errors.New("fail")
			})
		nas := NewNAS(testClient)
		err := nas.Expand(context.TODO(), "123", 3221225472)
		convey.So(err, convey.ShouldBeError)
	})
}

func TestExpandWithQuotaIdNotExist(t *testing.T) {
	convey.Convey("Quota Id Not Exist", t, func() {
		_ = monkey.PatchInstanceMethod(reflect.TypeOf(testClient), "GetFileSystemByName",
			func(_ *client.Client, _ context.Context, _ string) (map[string]interface{}, error) {
				return map[string]interface{}{
					"id": float64(522),
				}, nil
			})
		_ = monkey.PatchInstanceMethod(reflect.TypeOf(testClient), "GetQuotaByFileSystemById",
			func(_ *client.Client, _ context.Context, _ string) (map[string]interface{}, error) {
				return map[string]interface{}{
					"space_hard_quota": float64(2147483648),
					"space_soft_quota": float64(18446744073709551615),
				}, nil
			})
		_ = monkey.PatchInstanceMethod(reflect.TypeOf(testClient), "UpdateQuota",
			func(_ *client.Client, _ context.Context, param map[string]interface{}) error {
				_, exitId := param["id"]
				_, exitHardQuota := param["space_hard_quota"]
				_, exitSoftQuota := param["space_soft_quota"]
				if exitId && (exitSoftQuota || exitHardQuota) {
					return nil
				}
				return errors.New("fail")
			})
		nas := NewNAS(testClient)
		err := nas.Expand(context.TODO(), "123", 3221225472)
		convey.So(err, convey.ShouldBeError)
	})
}

func TestExpandWhenHardQuotaOrSoftQuotaNotExist(t *testing.T) {
	convey.Convey("space_hard_quota or space_soft_quota Not Exist", t, func() {
		_ = monkey.PatchInstanceMethod(reflect.TypeOf(testClient), "GetFileSystemByName",
			func(_ *client.Client, _ context.Context, _ string) (map[string]interface{}, error) {
				return map[string]interface{}{
					"id": float64(522),
				}, nil
			})
		_ = monkey.PatchInstanceMethod(reflect.TypeOf(testClient), "GetQuotaByFileSystemById",
			func(_ *client.Client, _ context.Context, _ string) (map[string]interface{}, error) {
				return map[string]interface{}{
					"id": "522@2",
				}, nil
			})
		_ = monkey.PatchInstanceMethod(reflect.TypeOf(testClient), "UpdateQuota",
			func(_ *client.Client, _ context.Context, param map[string]interface{}) error {
				_, exitId := param["id"]
				_, exitHardQuota := param["space_hard_quota"]
				_, exitSoftQuota := param["space_soft_quota"]
				if exitId && (exitSoftQuota || exitHardQuota) {
					return nil
				}
				return errors.New("fail")
			})
		nas := NewNAS(testClient)
		err := nas.Expand(context.TODO(), "123", 3221225472)
		convey.So(err, convey.ShouldBeError)
	})
}

func TestExpandWhenHardQuotaNotExist(t *testing.T) {
	convey.Convey("Hard Quota Not Exist", t, func() {

		_ = monkey.PatchInstanceMethod(reflect.TypeOf(testClient), "GetFileSystemByName",
			func(_ *client.Client, _ context.Context, _ string) (map[string]interface{}, error) {
				return map[string]interface{}{
					"id": float64(522),
				}, nil
			})
		_ = monkey.PatchInstanceMethod(reflect.TypeOf(testClient), "GetQuotaByFileSystemById",
			func(_ *client.Client, _ context.Context, _ string) (map[string]interface{}, error) {
				return map[string]interface{}{
					"id":               "522@2",
					"space_hard_quota": float64(18446744073709551615),
				}, nil
			})
		_ = monkey.PatchInstanceMethod(reflect.TypeOf(testClient), "UpdateQuota",
			func(_ *client.Client, _ context.Context, param map[string]interface{}) error {
				_, exitId := param["id"]
				_, exitHardQuota := param["space_hard_quota"]
				_, exitSoftQuota := param["space_soft_quota"]
				if exitId && (exitSoftQuota || exitHardQuota) {
					return nil
				}
				return errors.New("fail")
			})
		nas := NewNAS(testClient)
		err := nas.Expand(context.TODO(), "123", 3221225472)
		convey.So(err, convey.ShouldBeError)
	})
}

func TestExpandWithSoftQuotaNotExist(t *testing.T) {
	convey.Convey("Soft Quota Not Exist", t, func() {

		_ = monkey.PatchInstanceMethod(reflect.TypeOf(testClient), "GetFileSystemByName",
			func(_ *client.Client, _ context.Context, _ string) (map[string]interface{}, error) {
				return map[string]interface{}{
					"id": float64(522),
				}, nil
			})
		_ = monkey.PatchInstanceMethod(reflect.TypeOf(testClient), "GetQuotaByFileSystemById",
			func(_ *client.Client, _ context.Context, _ string) (map[string]interface{}, error) {
				return map[string]interface{}{
					"id":               "522@2",
					"space_soft_quota": float64(18446744073709551615),
				}, nil
			})
		_ = monkey.PatchInstanceMethod(reflect.TypeOf(testClient), "UpdateQuota",
			func(_ *client.Client, _ context.Context, param map[string]interface{}) error {
				_, exitId := param["id"]
				_, exitHardQuota := param["space_hard_quota"]
				_, exitSoftQuota := param["space_soft_quota"]
				if exitId && (exitSoftQuota || exitHardQuota) {
					return nil
				}
				return errors.New("fail")
			})
		nas := NewNAS(testClient)
		err := nas.Expand(context.TODO(), "123", 3221225472)
		convey.So(err, convey.ShouldBeError)
	})
}

func TestExpandWithUpdateQuotaFail(t *testing.T) {

	convey.Convey("Update Quota Fail", t, func() {
		_ = monkey.PatchInstanceMethod(reflect.TypeOf(testClient), "GetFileSystemByName",
			func(_ *client.Client, _ context.Context, _ string) (map[string]interface{}, error) {
				return map[string]interface{}{
					"id": float64(522),
				}, nil
			})
		_ = monkey.PatchInstanceMethod(reflect.TypeOf(testClient), "GetQuotaByFileSystemById",
			func(_ *client.Client, _ context.Context, _ string) (map[string]interface{}, error) {
				return map[string]interface{}{
					"id":               "522@2",
					"space_hard_quota": float64(2147483648),
					"space_soft_quota": float64(18446744073709551615),
				}, nil
			})
		_ = monkey.PatchInstanceMethod(reflect.TypeOf(testClient), "UpdateQuota",
			func(_ *client.Client, _ context.Context, _ map[string]interface{}) error {
				return errors.New("fail")
			})
		nas := NewNAS(testClient)
		err := nas.Expand(context.TODO(), "123", 3221225472)
		convey.So(err, convey.ShouldBeError)
	})
}

func TestCreateConvergedQoS(t *testing.T) {
	convey.Convey("Empty", t, func() {
		nas := NewNAS(testClient)
		param := map[string]interface{}{}
		taskResult := map[string]interface{}{}
		_, err := nas.createConvergedQoS(ctx, param, taskResult)
		convey.So(err, convey.ShouldBeNil)
	})

	convey.Convey("No fsName", t, func() {
		nas := NewNAS(testClient)
		param := map[string]interface{}{
			"qos": map[string]int{"maxIOPS": 999, "maxMBPS": 999},
		}
		taskResult := map[string]interface{}{}
		_, err := nas.createConvergedQoS(ctx, param, taskResult)
		convey.So(err, convey.ShouldBeError)
	})
}

func TestPreProcessConvergedQoS(t *testing.T) {
	convey.Convey("Empty", t, func() {
		nas := NewNAS(testClient)
		param := map[string]interface{}{}
		err := nas.preProcessConvergedQoS(ctx, param)
		convey.So(err, convey.ShouldBeNil)
	})

	convey.Convey("not json", t, func() {
		nas := NewNAS(testClient)
		param := map[string]interface{}{
			"qos": "not json",
		}
		err := nas.preProcessConvergedQoS(ctx, param)
		convey.So(err, convey.ShouldBeError)
	})

	convey.Convey("normal", t, func() {
		nas := NewNAS(testClient)
		param := map[string]interface{}{
			"qos": "{\"maxMBPS\":999,\"maxIOPS\":999}",
		}
		err := nas.preProcessConvergedQoS(ctx, param)
		convey.So(err, convey.ShouldBeNil)
	})
}

func TestDeleteConvergedQoSByFsName(t *testing.T) {
	convey.Convey("GetQoSPolicyIdByFsName failed", t, func() {
		m := gomonkey.ApplyMethod(reflect.TypeOf(&client.Client{}),
			"GetQoSPolicyIdByFsName",
			func(_ *client.Client, _ context.Context, _ string) (int, error) { return 0, errors.New("mock-error") },
		)
		defer m.Reset()

		nas := NewNAS(testClient)
		err := nas.deleteConvergedQoSByFsName(ctx, "mock-fs-name")
		convey.So(err, convey.ShouldBeError)
	})

	convey.Convey("GetQoSPolicyIdByFsName empty", t, func() {
		m := gomonkey.ApplyMethod(reflect.TypeOf(&client.Client{}),
			"GetQoSPolicyIdByFsName",
			func(_ *client.Client, _ context.Context, _ string) (int, error) { return types.NoQoSPolicyId, nil },
		)
		defer m.Reset()

		nas := NewNAS(testClient)
		err := nas.deleteConvergedQoSByFsName(ctx, "mock-fs-name")
		convey.So(err, convey.ShouldBeNil)
	})
}
