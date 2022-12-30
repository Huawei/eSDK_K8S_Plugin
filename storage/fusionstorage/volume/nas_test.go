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

package volume

import (
	"context"
	"errors"
	"os"
	"path"
	"reflect"
	"testing"

	"bou.ke/monkey"
	. "github.com/smartystreets/goconvey/convey"

	"huawei-csi-driver/storage/fusionstorage/client"
	"huawei-csi-driver/utils/log"
)

const (
	logDir  = "/var/log/huawei/"
	logName = "expandTest.log"
)

var testClient *client.Client

func TestMain(m *testing.M) {
	if err := log.InitLogging(logName); err != nil {
		log.Errorf("init logging: %s failed. error: %v", logName, err)
		os.Exit(1)
	}
	logFile := path.Join(logDir, logName)
	defer func() {
		if err := os.RemoveAll(logFile); err != nil {
			log.Errorf("Remove file: %s failed. error: %s", logFile, err)
		}
	}()

	m.Run()
}

func TestExpandWithNormal(t *testing.T) {
	Convey("Normal", t, func() {
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
		So(err, ShouldBeNil)
	})
}

func TestExpandWithFileSystemNotExit(t *testing.T) {
	Convey("File System Not Exist", t, func() {
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
		So(err, ShouldBeError)
	})
}

func TestExpandWithQuotaIdNotExist(t *testing.T) {
	Convey("Quota Id Not Exist", t, func() {
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
		So(err, ShouldBeError)
	})
}

func TestExpandWhenHardQuotaOrSoftQuotaNotExist(t *testing.T) {
	Convey("space_hard_quota or space_soft_quota Not Exist", t, func() {
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
		So(err, ShouldBeError)
	})
}

func TestExpandWhenHardQuotaNotExist(t *testing.T) {
	Convey("Hard Quota Not Exist", t, func() {

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
		So(err, ShouldBeError)
	})
}

func TestExpandWithSoftQuotaNotExist(t *testing.T) {
	Convey("Soft Quota Not Exist", t, func() {

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
		So(err, ShouldBeError)
	})
}

func TestExpandWithUpdateQuotaFail(t *testing.T) {

	Convey("Update Quota Fail", t, func() {
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
		So(err, ShouldBeError)
	})
}
