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

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/require"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/fusionstorage/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/fusionstorage/types"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	logName = "expandTest.log"
)

var testClient *client.RestClient
var ctx context.Context

func TestMain(m *testing.M) {
	log.MockInitLogging(logName)
	defer log.MockStopLogging(logName)

	ctx = context.TODO()

	m.Run()
}

func TestPreCreate(t *testing.T) {
	t.Run("Normal", func(t *testing.T) {
		m := gomonkey.ApplyMethod(reflect.TypeOf(testClient),
			"GetPoolByName",
			func(_ *client.RestClient, ctx context.Context, poolName string) (map[string]interface{}, error) {
				return map[string]interface{}{"mock": "mock"}, nil
			})
		defer m.Reset()

		nas := NewNAS(testClient)
		err := nas.preCreate(context.TODO(), map[string]interface{}{
			"authclient": "*",
			"name":       "mock-name",
		})
		require.NoError(t, err)
	})

	t.Run("Auth client empty", func(t *testing.T) {
		nas := NewNAS(testClient)
		err := nas.preCreate(context.TODO(), map[string]interface{}{
			"name": "mock-name",
		})
		require.Error(t, err)
	})

	t.Run("Name is empty", func(t *testing.T) {
		m := gomonkey.ApplyMethod(reflect.TypeOf(testClient),
			"GetPoolByName",
			func(_ *client.RestClient, ctx context.Context, poolName string) (map[string]interface{}, error) {
				return map[string]interface{}{"mock": "mock"}, nil
			})
		defer m.Reset()

		nas := NewNAS(testClient)
		err := nas.preCreate(context.TODO(), map[string]interface{}{
			"authclient": "*",
		})
		require.Error(t, err)
	})
}

func TestExpandWithNormal(t *testing.T) {
	t.Run("Normal", func(t *testing.T) {
		p := gomonkey.ApplyMethod(reflect.TypeOf(testClient), "GetFileSystemByName",
			func(_ *client.RestClient, _ context.Context, _ string) (map[string]interface{}, error) {
				return map[string]interface{}{
					"id": float64(522),
				}, nil
			}).ApplyMethod(reflect.TypeOf(testClient), "GetQuotaByFileSystemById",
			func(_ *client.RestClient, _ context.Context, _ string) (map[string]interface{}, error) {
				return map[string]interface{}{
					"id":               "522@2",
					"space_hard_quota": float64(2147483648),
					"space_soft_quota": float64(18446744073709551615),
				}, nil
			}).ApplyMethod(reflect.TypeOf(testClient), "UpdateQuota",
			func(_ *client.RestClient, _ context.Context, param map[string]interface{}) error {
				_, exitId := param["id"]
				_, exitHardQuota := param["space_hard_quota"]
				_, exitSoftQuota := param["space_soft_quota"]
				if exitId && (exitSoftQuota || exitHardQuota) {
					return nil
				}
				return errors.New("fail")
			})
		defer p.Reset()

		nas := NewNAS(testClient)
		err := nas.Expand(context.TODO(), "123", 3221225472)
		require.NoError(t, err)
	})
}

func TestExpandWithFileSystemNotExit(t *testing.T) {
	t.Run("File System Not Exist", func(t *testing.T) {
		p := gomonkey.ApplyMethod(reflect.TypeOf(testClient), "GetFileSystemByName",
			func(_ *client.RestClient, _ context.Context, _ string) (map[string]interface{}, error) {
				return nil, nil
			}).ApplyMethod(reflect.TypeOf(testClient), "GetQuotaByFileSystemById",
			func(_ *client.RestClient, _ context.Context, _ string) (map[string]interface{}, error) {
				return map[string]interface{}{
					"id":               "522@2",
					"space_hard_quota": float64(2147483648),
					"space_soft_quota": float64(18446744073709551615),
				}, nil
			}).ApplyMethod(reflect.TypeOf(testClient), "UpdateQuota",
			func(_ *client.RestClient, _ context.Context, param map[string]interface{}) error {
				_, exitId := param["id"]
				_, exitHardQuota := param["space_hard_quota"]
				_, exitSoftQuota := param["space_soft_quota"]
				if exitId && (exitSoftQuota || exitHardQuota) {
					return nil
				}
				return errors.New("fail")
			})
		defer p.Reset()

		nas := NewNAS(testClient)
		err := nas.Expand(context.TODO(), "123", 3221225472)
		require.Error(t, err)
	})
}

func TestExpandWithQuotaIdNotExist(t *testing.T) {
	t.Run("Quota Id Not Exist", func(t *testing.T) {
		p := gomonkey.ApplyMethod(reflect.TypeOf(testClient), "GetFileSystemByName",
			func(_ *client.RestClient, _ context.Context, _ string) (map[string]interface{}, error) {
				return map[string]interface{}{
					"id": float64(522),
				}, nil
			}).ApplyMethod(reflect.TypeOf(testClient), "GetQuotaByFileSystemById",
			func(_ *client.RestClient, _ context.Context, _ string) (map[string]interface{}, error) {
				return map[string]interface{}{
					"space_hard_quota": float64(2147483648),
					"space_soft_quota": float64(18446744073709551615),
				}, nil
			}).ApplyMethod(reflect.TypeOf(testClient), "UpdateQuota",
			func(_ *client.RestClient, _ context.Context, param map[string]interface{}) error {
				_, exitId := param["id"]
				_, exitHardQuota := param["space_hard_quota"]
				_, exitSoftQuota := param["space_soft_quota"]
				if exitId && (exitSoftQuota || exitHardQuota) {
					return nil
				}
				return errors.New("fail")
			})
		defer p.Reset()

		nas := NewNAS(testClient)
		err := nas.Expand(context.TODO(), "123", 3221225472)
		require.Error(t, err)
	})
}

func TestExpandWhenHardQuotaOrSoftQuotaNotExist(t *testing.T) {
	t.Run("space_hard_quota or space_soft_quota Not Exist", func(t *testing.T) {
		p := gomonkey.ApplyMethod(reflect.TypeOf(testClient), "GetFileSystemByName",
			func(_ *client.RestClient, _ context.Context, _ string) (map[string]interface{}, error) {
				return map[string]interface{}{
					"id": float64(522),
				}, nil
			}).ApplyMethod(reflect.TypeOf(testClient), "GetQuotaByFileSystemById",
			func(_ *client.RestClient, _ context.Context, _ string) (map[string]interface{}, error) {
				return map[string]interface{}{
					"id": "522@2",
				}, nil
			}).ApplyMethod(reflect.TypeOf(testClient), "UpdateQuota",
			func(_ *client.RestClient, _ context.Context, param map[string]interface{}) error {
				_, exitId := param["id"]
				_, exitHardQuota := param["space_hard_quota"]
				_, exitSoftQuota := param["space_soft_quota"]
				if exitId && (exitSoftQuota || exitHardQuota) {
					return nil
				}
				return errors.New("fail")
			})
		defer p.Reset()

		nas := NewNAS(testClient)
		err := nas.Expand(context.TODO(), "123", 3221225472)
		require.Error(t, err)
	})
}

func TestExpandWhenHardQuotaNotExist(t *testing.T) {
	t.Run("Hard Quota Not Exist", func(t *testing.T) {
		p := gomonkey.ApplyMethod(reflect.TypeOf(testClient), "GetFileSystemByName",
			func(_ *client.RestClient, _ context.Context, _ string) (map[string]interface{}, error) {
				return map[string]interface{}{
					"id": float64(522),
				}, nil
			}).ApplyMethod(reflect.TypeOf(testClient), "GetQuotaByFileSystemById",
			func(_ *client.RestClient, _ context.Context, _ string) (map[string]interface{}, error) {
				return map[string]interface{}{
					"id":               "522@2",
					"space_hard_quota": float64(18446744073709551615),
				}, nil
			}).ApplyMethod(reflect.TypeOf(testClient), "UpdateQuota",
			func(_ *client.RestClient, _ context.Context, param map[string]interface{}) error {
				_, exitId := param["id"]
				_, exitHardQuota := param["space_hard_quota"]
				_, exitSoftQuota := param["space_soft_quota"]
				if exitId && (exitSoftQuota || exitHardQuota) {
					return nil
				}
				return errors.New("fail")
			})
		defer p.Reset()

		nas := NewNAS(testClient)
		err := nas.Expand(context.TODO(), "123", 3221225472)
		require.Error(t, err)
	})
}

func TestExpandWithSoftQuotaNotExist(t *testing.T) {
	t.Run("Soft Quota Not Exist", func(t *testing.T) {
		p := gomonkey.ApplyMethod(reflect.TypeOf(testClient), "GetFileSystemByName",
			func(_ *client.RestClient, _ context.Context, _ string) (map[string]interface{}, error) {
				return map[string]interface{}{
					"id": float64(522),
				}, nil
			}).ApplyMethod(reflect.TypeOf(testClient), "GetQuotaByFileSystemById",
			func(_ *client.RestClient, _ context.Context, _ string) (map[string]interface{}, error) {
				return map[string]interface{}{
					"id":               "522@2",
					"space_soft_quota": float64(18446744073709551615),
				}, nil
			}).ApplyMethod(reflect.TypeOf(testClient), "UpdateQuota",
			func(_ *client.RestClient, _ context.Context, param map[string]interface{}) error {
				_, exitId := param["id"]
				_, exitHardQuota := param["space_hard_quota"]
				_, exitSoftQuota := param["space_soft_quota"]
				if exitId && (exitSoftQuota || exitHardQuota) {
					return nil
				}
				return errors.New("fail")
			})
		defer p.Reset()

		nas := NewNAS(testClient)
		err := nas.Expand(context.TODO(), "123", 3221225472)
		require.Error(t, err)
	})
}

func TestExpandWithUpdateQuotaFail(t *testing.T) {
	t.Run("Update Quota Fail", func(t *testing.T) {
		p := gomonkey.ApplyMethod(reflect.TypeOf(testClient), "GetFileSystemByName",
			func(_ *client.RestClient, _ context.Context, _ string) (map[string]interface{}, error) {
				return map[string]interface{}{
					"id": float64(522),
				}, nil
			}).ApplyMethod(reflect.TypeOf(testClient), "GetQuotaByFileSystemById",
			func(_ *client.RestClient, _ context.Context, _ string) (map[string]interface{}, error) {
				return map[string]interface{}{
					"id":               "522@2",
					"space_hard_quota": float64(2147483648),
					"space_soft_quota": float64(18446744073709551615),
				}, nil
			}).ApplyMethod(reflect.TypeOf(testClient), "UpdateQuota",
			func(_ *client.RestClient, _ context.Context, _ map[string]interface{}) error {
				return errors.New("fail")
			})
		defer p.Reset()

		nas := NewNAS(testClient)
		err := nas.Expand(context.TODO(), "123", 3221225472)
		require.Error(t, err)
	})
}

func TestCreateConvergedQoS(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		nas := NewNAS(testClient)
		param := map[string]interface{}{}
		taskResult := map[string]interface{}{}
		_, err := nas.createConvergedQoS(ctx, param, taskResult)
		require.NoError(t, err)
	})

	t.Run("No fsName", func(t *testing.T) {
		nas := NewNAS(testClient)
		param := map[string]interface{}{
			"qos": map[string]int{"maxIOPS": 999, "maxMBPS": 999},
		}
		taskResult := map[string]interface{}{}
		_, err := nas.createConvergedQoS(ctx, param, taskResult)
		require.Error(t, err)
	})
}

func TestPreProcessConvergedQoS(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		nas := NewNAS(testClient)
		param := map[string]interface{}{}
		err := nas.preProcessConvergedQoS(ctx, param)
		require.NoError(t, err)
	})

	t.Run("not json", func(t *testing.T) {
		nas := NewNAS(testClient)
		param := map[string]interface{}{
			"qos": "not json",
		}
		err := nas.preProcessConvergedQoS(ctx, param)
		require.Error(t, err)
	})

	t.Run("normal", func(t *testing.T) {
		nas := NewNAS(testClient)
		param := map[string]interface{}{
			"qos": "{\"maxMBPS\":999,\"maxIOPS\":999}",
		}
		err := nas.preProcessConvergedQoS(ctx, param)
		require.NoError(t, err)
	})
}

func TestDeleteConvergedQoSByFsName(t *testing.T) {
	t.Run("GetQoSPolicyIdByFsName failed", func(t *testing.T) {
		m := gomonkey.ApplyMethod(reflect.TypeOf(&client.RestClient{}),
			"GetQoSPolicyIdByFsName",
			func(_ *client.RestClient, _ context.Context, _ string) (int, error) {
				return 0, errors.New("mock-error")
			},
		)
		defer m.Reset()

		nas := NewNAS(testClient)
		err := nas.deleteConvergedQoSByFsName(ctx, "mock-fs-name")
		require.Error(t, err)
	})

	t.Run("GetQoSPolicyIdByFsName empty", func(t *testing.T) {
		m := gomonkey.ApplyMethod(reflect.TypeOf(&client.RestClient{}),
			"GetQoSPolicyIdByFsName",
			func(_ *client.RestClient, _ context.Context, _ string) (int, error) { return types.NoQoSPolicyId, nil },
		)
		defer m.Reset()

		nas := NewNAS(testClient)
		err := nas.deleteConvergedQoSByFsName(ctx, "mock-fs-name")
		require.NoError(t, err)
	})
}
