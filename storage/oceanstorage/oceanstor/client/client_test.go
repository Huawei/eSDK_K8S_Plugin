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

package client

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	cfg "github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app/config"
	pkgUtils "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

var (
	testClient *OceanstorClient
)

type responseCode int

const (
	logName = "clientTest.log"

	successStatus responseCode = 200
)

func TestLogin(t *testing.T) {
	var cases = []struct {
		Name         string
		ResponseBody string
		wantErr      bool
	}{
		{
			"Normal",
			"{\"data\":{\"accountstate\":1,\"deviceid\":\"2102352TRW10KB000001\"," +
				"\"iBaseToken\":\"508C457614FEA5413316AC0945ED0EE047765A96DD6524462C93EA5BE834B440\"," +
				"\"lastloginip\":\"192.168.125.25\",\"lastlogintime\":1645117156,\"pwdchangetime\":1643562159," +
				"\"roleId\":\"1\",\"usergroup\":\"\",\"userid\":\"admin\",\"username\":\"dev-account\",\"userscope\":\"0\"}," +
				"\"error\":{\"code\":0,\"description\":\"0\"}}", false,
		},
		{
			"The user name or password is incorrect",
			"{\"data\":{},\"error\":{\"code\":1077949061,\"description\":\"The User name or PassWord is incorrect.\"," +
				"\"errorParam\":\"\",\"suggestion\":\"Check the User name and PassWord, and try again.\"}}", true,
		},
		{
			"The IP address has been locked",
			"{\"data\":{},\"error\":{\"code\":1077949071,\"description\":\"The IP address has been locked.\"," +
				"\"errorParam\":\"\",\"suggestion\":\"Contact the administrator.\"}}", true,
		},
	}

	m := getTestLoginPatches()
	defer m.Reset()

	for _, s := range cases {
		g := gomonkey.ApplyMethod(testClient.Client, "Do",
			func(_ *http.Client, req *http.Request) (*http.Response, error) {
				r := ioutil.NopCloser(bytes.NewReader([]byte(s.ResponseBody)))
				return &http.Response{
					StatusCode: 200,
					Body:       r,
				}, nil
			})

		err := testClient.Login(context.TODO())
		assert.Equal(t, s.wantErr, err != nil, "%s, err:%v", s.Name, err)
		g.Reset()
	}
}

func getTestLoginPatches() *gomonkey.Patches {
	m := gomonkey.ApplyFunc(pkgUtils.GetAuthInfoFromBackendID,
		func(ctx context.Context, backendID string) (*pkgUtils.BackendAuthInfo, error) {
			return &pkgUtils.BackendAuthInfo{Password: "mock", Scope: "0"}, nil
		})
	m.ApplyFunc(pkgUtils.GetCertSecretFromBackendID,
		func(ctx context.Context, backendID string) (bool, string, error) {
			return false, "", nil
		})
	m.ApplyFunc(pkgUtils.SetStorageBackendContentOnlineStatus,
		func(ctx context.Context, backendID string, online bool) error {
			return nil
		})
	return m
}

func TestLogout(t *testing.T) {
	var cases = []struct {
		Name         string
		ResponseBody string
		wantErr      bool
	}{
		{
			"Normal",
			"{\"data\":{},\"error\":{\"code\":0,\"description\"：0}",
			false,
		},
		{
			"Call fail",
			"{\"data\":{},\"error\":{\"code\":1077949061,\"description\"：\"Call fail\"}",
			false,
		},
		{
			"Call error",
			"{\"data\":{},\"error\":{\"code\":1077949071,\"description\"：\"Call error\"}",
			false,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockClient := NewMockHTTPClient(ctrl)

	temp := testClient.Client
	defer func() { testClient.Client = temp }()
	testClient.Client = mockClient

	for _, s := range cases {
		mockClient.EXPECT().Do(gomock.Any()).DoAndReturn(func(req *http.Request) (*http.Response, error) {
			r := ioutil.NopCloser(bytes.NewReader([]byte(s.ResponseBody)))
			return &http.Response{
				StatusCode: int(successStatus),
				Body:       r,
			}, nil
		})
		testClient.Logout(context.TODO())
	}
}

func TestReLogin(t *testing.T) {
	var cases = []struct {
		Name         string
		ResponseBody string
		wantErr      bool
	}{
		{
			"Normal",
			"{\"data\":{\"accountstate\":1,\"deviceid\":\"2102352TRW10KB000001\"," +
				"\"iBaseToken\":\"508C457614FEA5413316AC0945ED0EE047765A96DD6524462C93EA5BE834B440\"," +
				"\"lastloginip\":\"192.168.125.25\",\"lastlogintime\":1645117156,\"pwdchangetime\":1643562159," +
				"\"roleId\":\"1\",\"usergroup\":\"\",\"userid\":\"admin\",\"username\":\"dev-account\", " +
				"\"userscope\":\"0\"},\"error\":{\"code\":0,\"description\":\"0\"}}", false,
		},
		{
			"The User name or PassWord is incorrect",
			"{\"data\":{},\"error\":{\"code\":1077949061,\"description\":\"The User name or PassWord " +
				"is incorrect.\",\"errorParam\":\"\",\"suggestion\":\"Check the User name and PassWord, and try again.\"}}",
			true,
		},
		{
			"The IP address has been locked",
			"{\"data\":{},\"error\":{\"code\":1077949071,\"description\":\"The IP address has been " +
				"locked.\",\"errorParam\":\"\",\"suggestion\":\"Contact the administrator.\"}}", true,
		},
	}

	m := getTestLoginPatches()
	defer m.Reset()

	for _, s := range cases {
		g := gomonkey.ApplyMethod(reflect.TypeOf(testClient.Client), "Do",
			func(_ *http.Client, req *http.Request) (*http.Response, error) {
				r := ioutil.NopCloser(bytes.NewReader([]byte(s.ResponseBody)))
				return &http.Response{
					StatusCode: 200,
					Body:       r,
				}, nil
			})

		err := testClient.ReLogin(context.TODO())
		assert.Equal(t, s.wantErr, err != nil, "%s, err:%v", s.Name, err)
		g.Reset()
	}
}

func TestGetLunByName(t *testing.T) {
	var cases = []struct {
		Name         string
		ResponseBody string
		wantErr      bool
	}{
		{
			"Normal",
			"{\"data\":[{\"ALLOCCAPACITY\":\"0\",\"ALLOCTYPE\":\"1\",\"CAPABILITY\":\"3\",\"CAPACITY\":" +
				"\"209715200\",\"CAPACITYALARMLEVEL\":\"2\",\"COMPRESSION\":\"0\",\"COMPRESSIONSAVEDCAPACITY\":\"0\"," +
				"\"COMPRESSIONSAVEDRATIO\":\"0\",\"DEDUPSAVEDCAPACITY\":\"0\",\"DEDUPSAVEDRATIO\":\"0\"," +
				"\"DESCRIPTION\":\"\",\"DISGUISEREMOTEARRAYID\":\"--\",\"DISGUISESTATUS\":\"0\",\"DRS_ENABLE\":" +
				"\"false\",\"ENABLECOMPRESSION\":\"true\",\"ENABLEISCSITHINLUNTHRESHOLD\":\"false\",\"" +
				"ENABLESMARTDEDUP\":\"true\",\"EXPOSEDTOINITIATOR\":\"false\",\"EXTENDIFSWITCH\":\"false\"," +
				"\"HASRSSOBJECT\":\"{\\\"SnapShot\\\":\\\"FALSE\\\",\\\"LunCopy\\\":\\\"FALSE\\\"," +
				"\\\"RemoteReplication\\\":\\\"FALSE\\\",\\\"SplitMirror\\\":\\\"FALSE\\\",\\\"LunMigration\\\":" +
				"\\\"FALSE\\\",\\\"LUNMirror\\\":\\\"FALSE\\\",\\\"HyperMetro\\\":\\\"FALSE\\\",\\\"LunClone\\\":" +
				"\\\"FALSE\\\",\\\"HyperCopy\\\":\\\"FALSE\\\",\\\"HyperCDP\\\":\\\"FALSE\\\",\\\"CloudBackup\\\":" +
				"\\\"FALSE\\\",\\\"drStar\\\":\\\"FALSE\\\"}\",\"HEALTHSTATUS\":\"1\",\"HYPERCDPSCHEDULEDISABLE\":" +
				"\"0\",\"ID\":\"0\",\"IOCLASSID\":\"\",\"IOPRIORITY\":\"1\",\"ISADD2LUNGROUP\":\"false\"," +
				"\"ISCHECKZEROPAGE\":\"true\",\"ISCLONE\":\"false\",\"ISCSITHINLUNTHRESHOLD\":\"90\"," +
				"\"MIRRORPOLICY\":\"1\",\"MIRRORTYPE\":\"0\",\"NAME\":\"zfy\",\"NGUID\":" +
				"\"710062d5870001f87817bec000000000\",\"OWNINGCONTROLLER\":\"--\",\"PARENTID\":\"0\",\"PARENTNAME\":" +
				"\"s0\",\"PREFETCHPOLICY\":\"0\",\"PREFETCHVALUE\":\"0\",\"REMOTELUNID\":\"--\"," +
				"\"REPLICATION_CAPACITY\":\"0\",\"RUNNINGSTATUS\":\"27\",\"RUNNINGWRITEPOLICY\":\"1\",\"SECTORSIZE\":" +
				"\"512\",\"SNAPSHOTSCHEDULEID\":\"--\",\"SUBTYPE\":\"0\",\"THINCAPACITYUSAGE\":\"0\"," +
				"\"TOTALSAVEDCAPACITY\":\"0\",\"TOTALSAVEDRATIO\":\"0\",\"TYPE\":11,\"USAGETYPE\":\"5\"," +
				"\"WORKINGCONTROLLER\":\"--\",\"WORKLOADTYPEID\":\"4294967295\",\"WORKLOADTYPENAME\":\"\"," +
				"\"WRITEPOLICY\":\"1\",\"WWN\":\"67817be10062d5870001f8c000000000\",\"blockDeviceName\":" +
				"\"GLOBAL_REPO_0\",\"devController\":\"all\",\"functionType\":\"1\",\"grainSize\":\"16\"," +
				"\"hyperCdpScheduleId\":\"0\",\"isShowDedupAndCompression\":\"false\",\"lunCgId\":\"0\",\"mapped\":" +
				"\"false\",\"remoteLunWwn\":\"--\",\"serviceEnabled\":\"true\",\"takeOverLunWwn\":\"--\"," +
				"\"SMARTCACHEPARTITIONID\":\"1\",\"SC_HITRAGE\":\"0\",\"createTime\":\"1646990907\"}],\"error\":" +
				"{\"code\":0,\"description\":\"0\"}}",
			false,
		},
		{
			"Get lun info by name fail",
			"{\"data\":[{}],\"error\":{\"code\":0,\"description\"：\"0\"}",
			true,
		},
		{
			"Get lun info by name error",
			"{\"data\":[{}],\"error\":{\"code\":1077949061,\"description\"：\"Get fail\"}",
			true,
		},
		{
			"Lun does not exist",
			"{\"data\":[{}],\"error\":{\"code\":0,\"description\"：\"0\"}",
			true,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	temp := testClient.Client
	defer func() { testClient.Client = temp }()

	for _, s := range cases {
		t.Run(s.Name, func(t *testing.T) {
			mockClient := NewMockHTTPClient(ctrl)
			testClient.Client = mockClient
			mockClient.EXPECT().Do(gomock.Any()).DoAndReturn(func(req *http.Request) (*http.Response, error) {
				r := ioutil.NopCloser(bytes.NewReader([]byte(s.ResponseBody)))
				return &http.Response{
					StatusCode: int(successStatus),
					Body:       r,
				}, nil
			}).AnyTimes()

			_, err := testClient.GetLunByName(context.TODO(), "zfy")
			assert.Equal(t, s.wantErr, err != nil, "%v", err)
		})
	}
}

func TestGetLunByID(t *testing.T) {
	var cases = []struct {
		Name         string
		ResponseBody string
		wantErr      bool
	}{
		{
			"Normal",
			"{ \"data\":{\"ALLOCCAPACITY\":\"0\",\"ALLOCTYPE\":\"1\",\"CAPABILITY\":\"3\"," +
				"\"CAPACITY\":\"209715200\",\"CAPACITYALARMLEVEL\":\"2\",\"CLONEIDS\":\"[]\",\"COMPRESSION\":\"0\"," +
				"\"COMPRESSIONSAVEDCAPACITY\":\"0\",\"COMPRESSIONSAVEDRATIO\":\"0\",\"DEDUPSAVEDCAPACITY\":\"0\"," +
				"\"DEDUPSAVEDRATIO\":\"0\",\"DESCRIPTION\":\"\",\"DISGUISEREMOTEARRAYID\":\"--\",\"DISGUISESTATUS\":" +
				"\"0\",\"DRS_ENABLE\":\"false\",\"ENABLECOMPRESSION\":\"true\",\"ENABLEISCSITHINLUNTHRESHOLD\":" +
				"\"false\",\"ENABLESMARTDEDUP\":\"true\",\"EXPOSEDTOINITIATOR\":\"false\",\"EXTENDIFSWITCH\":" +
				"\"false\",\"HASRSSOBJECT\":\"{\\\"SnapShot\\\":\\\"FALSE\\\",\\\"LunCopy\\\":\\\"FALSE\\\"," +
				"\\\"RemoteReplication\\\":\\\"FALSE\\\",\\\"SplitMirror\\\":\\\"FALSE\\\",\\\"LunMigration\\\":" +
				"\\\"FALSE\\\",\\\"LUNMirror\\\":\\\"FALSE\\\",\\\"HyperMetro\\\":\\\"FALSE\\\",\\\"LunClone\\\":" +
				"\\\"FALSE\\\",\\\"HyperCopy\\\":\\\"FALSE\\\",\\\"HyperCDP\\\":\\\"FALSE\\\",\\\"CloudBackup\\\":" +
				"\\\"FALSE\\\",\\\"drStar\\\":\\\"FALSE\\\"}\",\"HEALTHSTATUS\": \"1\",\"HYPERCDPSCHEDULEDISABLE\":" +
				"\"0\",\"ID\":\"0\",\"IOCLASSID\":\"\",\"IOPRIORITY\":\"1\",\"ISADD2LUNGROUP\":\"false\"," +
				"\"ISCHECKZEROPAGE\":\"true\",\"ISCLONE\":\"false\",\"ISCSITHINLUNTHRESHOLD\":\"90\",\"LUNCOPYIDS\":" +
				"\"[]\",\"LUNMigrationOrigin\":\"-1\",\"MIRRORPOLICY\":\"1\",\"MIRRORTYPE\":\"0\",\"NAME\":\"zfy\"," +
				"\"NGUID\":\"710062d5870001f87817bec000000000\",\"OWNINGCONTROLLER\":\"--\",\"PARENTID\":\"0\"," +
				"\"PARENTNAME\":\"s0\",\"PREFETCHPOLICY\":\"0\",\"PREFETCHVALUE\":\"0\",\"REMOTELUNID\":\"--\"," +
				"\"REMOTEREPLICATIONIDS\":\"[]\",\"REPLICATION_CAPACITY\":\"0\",\"RUNNINGSTATUS\":\"27\"," +
				"\"RUNNINGWRITEPOLICY\":\"1\",\"SECTORSIZE\":\"512\",\"SNAPSHOTIDS\":\"[]\",\"SNAPSHOTSCHEDULEID\":" +
				"\"--\",\"SUBTYPE\":\"0\",\"THINCAPACITYUSAGE\":\"0\",\"TOTALSAVEDCAPACITY\":\"0\"," +
				"\"TOTALSAVEDRATIO\":\"0\",\"TYPE\":11,\"USAGETYPE\":\"5\",\"WORKINGCONTROLLER\":\"--\"," +
				"\"WORKLOADTYPEID\":\"4294967295\",\"WORKLOADTYPENAME\":\"\",\"WRITEPOLICY\":\"1\",\"WWN\":" +
				"\"67817be10062d5870001f8c000000000\",\"blockDeviceName\":\"GLOBAL_REPO_0\",\"createTime\":\"\"," +
				"\"devController\":\"all\",\"functionType\":\"1\",\"grainSize\":\"16\",\"hyperCdpScheduleId\":\"0\"," +
				"\"isShowDedupAndCompression\":\"false\",\"lunCgId\":\"0\",\"mapped\":\"false\",\"protectGroupIds\":" +
				"\"\",\"remoteLunWwn\":\"--\",\"serviceEnabled\":\"true\",\"takeOverLunWwn\":\"--\"," +
				"\"SMARTCACHEPARTITIONID\":\"1\",\"SC_HITRAGE\":\"0\"},\"error\":{\"code\":0,\"description\":\"0\"}}",
			false,
		},
		{
			"Get lun info by id fail",
			"{\"data\":{},\"error\":{\"code\":0,\"description\"：\"0\"}",
			true,
		},
		{
			"Get lun info by id error",
			"{ \"data\":{\"ALLOCCAPACITY\":\"0\",\"ALLOCTYPE\":\"1\",\"CAPABILITY\":\"3\"," +
				"\"CAPACITY\":\"209715200\",\"CAPACITYALARMLEVEL\":\"2\",\"CLONEIDS\":\"[]\",\"COMPRESSION\":\"0\"," +
				"\"COMPRESSIONSAVEDCAPACITY\":\"0\",\"COMPRESSIONSAVEDRATIO\":\"0\",\"DEDUPSAVEDCAPACITY\":\"0\"," +
				"\"DEDUPSAVEDRATIO\":\"0\",\"DESCRIPTION\":\"\",\"DISGUISEREMOTEARRAYID\":\"--\",\"DISGUISESTATUS\":" +
				"\"0\",\"DRS_ENABLE\":\"false\",\"ENABLECOMPRESSION\":\"true\",\"ENABLEISCSITHINLUNTHRESHOLD\":" +
				"\"false\",\"ENABLESMARTDEDUP\":\"true\",\"EXPOSEDTOINITIATOR\":\"false\",\"EXTENDIFSWITCH\":" +
				"\"false\",\"HASRSSOBJECT\":\"{\\\"SnapShot\\\":\\\"FALSE\\\",\\\"LunCopy\\\":\\\"FALSE\\\"," +
				"\\\"RemoteReplication\\\":\\\"FALSE\\\",\\\"SplitMirror\\\":\\\"FALSE\\\",\\\"LunMigration\\\":" +
				"\\\"FALSE\\\",\\\"LUNMirror\\\":\\\"FALSE\\\",\\\"HyperMetro\\\":\\\"FALSE\\\",\\\"LunClone\\\":" +
				"\\\"FALSE\\\",\\\"HyperCopy\\\":\\\"FALSE\\\",\\\"HyperCDP\\\":\\\"FALSE\\\",\\\"CloudBackup\\\":" +
				"\\\"FALSE\\\",\\\"drStar\\\":\\\"FALSE\\\"}\",\"HEALTHSTATUS\": \"1\",\"HYPERCDPSCHEDULEDISABLE\":" +
				"\"0\",\"ID\":\"0\",\"IOCLASSID\":\"\",\"IOPRIORITY\":\"1\",\"ISADD2LUNGROUP\":\"false\"," +
				"\"ISCHECKZEROPAGE\":\"true\",\"ISCLONE\":\"false\",\"ISCSITHINLUNTHRESHOLD\":\"90\",\"LUNCOPYIDS\":" +
				"\"[]\",\"LUNMigrationOrigin\":\"-1\",\"MIRRORPOLICY\":\"1\",\"MIRRORTYPE\":\"0\",\"NAME\": \"zfy\"," +
				"\"NGUID\":\"710062d5870001f87817bec000000000\",\"OWNINGCONTROLLER\":\"--\",\"PARENTID\":\"0\"," +
				"\"PARENTNAME\":\"s0\",\"PREFETCHPOLICY\":\"0\",\"PREFETCHVALUE\":\"0\",\"REMOTELUNID\":\"--\"," +
				"\"REMOTEREPLICATIONIDS\":\"[]\",\"REPLICATION_CAPACITY\":\"0\",\"RUNNINGSTATUS\":\"27\"," +
				"\"RUNNINGWRITEPOLICY\":\"1\",\"SECTORSIZE\":\"512\",\"SNAPSHOTIDS\":\"[]\",\"SNAPSHOTSCHEDULEID\":" +
				"\"--\",\"SUBTYPE\":\"0\",\"THINCAPACITYUSAGE\":\"0\",\"TOTALSAVEDCAPACITY\":\"0\"," +
				"\"TOTALSAVEDRATIO\":\"0\",\"TYPE\":11,\"USAGETYPE\":\"5\",\"WORKINGCONTROLLER\":\"--\"," +
				"\"WORKLOADTYPEID\":\"4294967295\",\"WORKLOADTYPENAME\":\"\",\"WRITEPOLICY\":\"1\",\"WWN\":" +
				"\"67817be10062d5870001f8c000000000\",\"blockDeviceName\":\"GLOBAL_REPO_0\",\"createTime\":\"\"," +
				"\"devController\":\"all\",\"functionType\":\"1\",\"grainSize\":\"16\",\"hyperCdpScheduleId\":\"0\"," +
				"\"isShowDedupAndCompression\":\"false\",\"lunCgId\":\"0\",\"mapped\":\"false\",\"protectGroupIds\":" +
				"\"\",\"remoteLunWwn\":\"--\",\"serviceEnabled\":\"true\",\"takeOverLunWwn\":\"--\"," +
				"\"SMARTCACHEPARTITIONID\":\"1\",\"SC_HITRAGE\":\"0\"},\"error\":{\"code\":1077949061," +
				"\"description\":\"0\"}}",
			true,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	temp := testClient.Client
	defer func() { testClient.Client = temp }()

	for _, s := range cases {
		t.Run(s.Name, func(t *testing.T) {
			mockClient := NewMockHTTPClient(ctrl)
			testClient.Client = mockClient
			mockClient.EXPECT().Do(gomock.Any()).DoAndReturn(func(req *http.Request) (*http.Response, error) {
				r := ioutil.NopCloser(bytes.NewReader([]byte(s.ResponseBody)))
				return &http.Response{
					StatusCode: int(successStatus),
					Body:       r,
				}, nil
			}).AnyTimes()

			_, err := testClient.GetLunByID(context.TODO(), "0")
			assert.Equal(t, s.wantErr, err != nil, "err:%v", err)
		})
	}
}

func TestAddLunToGroup(t *testing.T) {
	var cases = []struct {
		Name         string
		ResponseBody string
		wantErr      bool
	}{
		{
			"Normal",
			"{\"data\":{},\"error\":{\"code\":0,\"description\":\"0\"}}",
			false,
		},
		{
			"Lun is already in group",
			"{\"data\":{},\"error\":{\"code\":1077948997,\"description\":\"0\"}}",
			false,
		},
		{
			"Lun is already in group",
			"{\"data\":{},\"error\":{\"code\":1077936862,\"description\":\"0\"}}",
			false,
		},
		{
			"Add lun to group error",
			"{\"data\":{},\"error\":{\"code\":1077949061,\"description\":\"0\"}}",
			true,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	temp := testClient.Client
	defer func() { testClient.Client = temp }()

	for _, s := range cases {
		mockClient := NewMockHTTPClient(ctrl)
		testClient.Client = mockClient
		t.Run(s.Name, func(t *testing.T) {
			mockClient.EXPECT().Do(gomock.Any()).DoAndReturn(func(req *http.Request) (*http.Response, error) {
				r := io.NopCloser(bytes.NewReader([]byte(s.ResponseBody)))
				return &http.Response{
					StatusCode: int(successStatus),
					Body:       r,
				}, nil
			}).AnyTimes()

			err := testClient.AddLunToGroup(context.TODO(), "", "")
			assert.Equal(t, s.wantErr, err != nil, "err:%v", err)
		})
	}
}

func TestRemoveLunFromGroup(t *testing.T) {
	var cases = []struct {
		Name         string
		ResponseBody string
		wantErr      bool
	}{
		{
			"Normal",
			"{\"data\":{},\"error\":{\"code\":0,\"description\": \"0\"}}",
			false,
		},
		{
			"LUN is not in lungroup",
			"{\"data\":{},\"error\":{\"code\":1077948996,\"description\": \"0\"}}",
			false,
		},
		{
			"Remove lun from group error",
			"{\"data\":{},\"error\":{\"code\":1077949061,\"description\": \"0\"}}",
			true,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	temp := testClient.Client
	defer func() { testClient.Client = temp }()

	for _, s := range cases {
		t.Run(s.Name, func(t *testing.T) {
			mockClient := NewMockHTTPClient(ctrl)
			testClient.Client = mockClient
			mockClient.EXPECT().Do(gomock.Any()).DoAndReturn(func(req *http.Request) (*http.Response, error) {
				r := io.NopCloser(bytes.NewReader([]byte(s.ResponseBody)))
				return &http.Response{
					StatusCode: int(successStatus),
					Body:       r,
				}, nil
			}).AnyTimes()

			err := testClient.RemoveLunFromGroup(context.TODO(), "", "")
			assert.Equal(t, s.wantErr, err != nil, "err:%v", err)
		})
	}
}

func TestGetLunGroupByName(t *testing.T) {
	var cases = []struct {
		Name         string
		ResponseBody string
		wantErr      bool
	}{
		{
			"Normal",
			"{\"data\":[{\"CAPCITY\":\"2097152\",\"DESCRIPTION\":\"\",\"ID\": \"0\",\"ISADD2MAPPINGVIEW\":" +
				"\"true\",\"NAME\":\"LUNGroup001\",\"SMARTQOSPOLICYID\":\"\",\"TYPE\":256,\"lunNumber\":\"256\"," +
				"\"allocatedCapacity\":\"1097152\",\"protectionCapacity\":\"40956\",\"cdpGroupNum\": \"0\"," +
				"\"cloneGroupNum\":\"0\",\"drStarNum\":\"0\",\"hyperMetroGroupNum\":\"0\",\"replicationGroupNum\": " +
				"\"0\",\"snapshotGroupNum\":\"0\"}],\"error\":{\"code\":0,\"description\":\"0\"}}",
			false,
		},
		{
			"Get lungroup info error",
			"{\"data\":[{}],\"error\":{\"code\":1077949061,\"description\": \"0\"}}",
			true,
		},
		{
			"Lungroup does not exist",
			"{\"data\":[{}],\"error\":{\"code\":0,\"description\": \"0\"}}",
			false,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	temp := testClient.Client
	defer func() { testClient.Client = temp }()

	for _, s := range cases {
		t.Run(s.Name, func(t *testing.T) {
			mockClient := NewMockHTTPClient(ctrl)
			testClient.Client = mockClient
			mockClient.EXPECT().Do(gomock.Any()).DoAndReturn(func(req *http.Request) (*http.Response, error) {
				r := io.NopCloser(bytes.NewReader([]byte(s.ResponseBody)))
				return &http.Response{
					StatusCode: int(successStatus),
					Body:       r,
				}, nil
			}).AnyTimes()

			_, err := testClient.GetLunGroupByName(context.TODO(), "")
			assert.Equal(t, s.wantErr, err != nil, "err:%v", err)
		})
	}
}

func TestCreateLunGroup(t *testing.T) {
	var cases = []struct {
		Name         string
		ResponseBody string
		wantErr      bool
	}{
		{
			"Normal",
			"{\"data\":{\"DESCRIPTION\":\"\",\"ID\":\"0\",\"ISADD2MAPPINGVIEW\":\"false\",\"NAME\":" +
				"\"LUNGroup002\",\"TYPE\":256},\"error\":{\"code\":0,\"description\":\"0\"}}",
			false,
		},
		{
			"Lungroup already exists",
			"{\"data\":{},\"error\":{\"code\":1077948993,\"description\": \"0\"}}",
			true,
		},
		{
			"Create lungroup error",
			"{\"data\":{},\"error\":{\"code\":1077949061,\"description\": \"0\"}}",
			true,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	temp := testClient.Client
	defer func() { testClient.Client = temp }()

	for _, s := range cases {
		t.Run(s.Name, func(t *testing.T) {
			mockClient := NewMockHTTPClient(ctrl)
			testClient.Client = mockClient
			mockClient.EXPECT().Do(gomock.Any()).DoAndReturn(func(req *http.Request) (*http.Response, error) {
				r := io.NopCloser(bytes.NewReader([]byte(s.ResponseBody)))
				return &http.Response{
					StatusCode: int(successStatus),
					Body:       r,
				}, nil
			}).AnyTimes()

			_, err := testClient.CreateLunGroup(context.TODO(), "")
			assert.Equal(t, s.wantErr, err != nil, "err:%v", err)
		})
	}
}

func TestDeleteLunGroup(t *testing.T) {
	var cases = []struct {
		Name         string
		ResponseBody string
		wantErr      bool
	}{
		{
			"Normal",
			"{\"data\":{},\"error\":{\"code\":0,\"description\":\"0\"}}",
			false,
		},
		{
			"Lungroup does not exist while deleting",
			"{\"data\":{},\"error\":{\"code\":1077948996,\"description\": \"0\"}}",
			false,
		},
		{
			"Delete lungroup error",
			"{\"data\":{},\"error\":{\"code\":1077949061,\"description\": \"0\"}}",
			true,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	temp := testClient.Client
	defer func() { testClient.Client = temp }()

	for _, s := range cases {
		t.Run(s.Name, func(t *testing.T) {
			mockClient := NewMockHTTPClient(ctrl)
			testClient.Client = mockClient
			mockClient.EXPECT().Do(gomock.Any()).DoAndReturn(func(req *http.Request) (*http.Response, error) {
				r := io.NopCloser(bytes.NewReader([]byte(s.ResponseBody)))
				return &http.Response{
					StatusCode: int(successStatus),
					Body:       r,
				}, nil
			}).AnyTimes()

			err := testClient.DeleteLunGroup(context.TODO(), "")
			assert.Equal(t, s.wantErr, err != nil, "err:%v", err)
		})
	}
}

func TestQueryAssociateLunGroup(t *testing.T) {
	var cases = []struct {
		Name         string
		ResponseBody string
		wantErr      bool
	}{
		{
			"Normal",
			"{\"data\":[{\"DESCRIPTION\": \"\",\"ID\": \"0\",\"ISADD2MAPPINGVIEW\":\"false\",\"NAME\":" +
				"\"LUNGroup001\",\"SMARTQOSPOLICYID\":\"\",\"TYPE\":256}],\"error\":{\"code\":0,\"description\":" +
				"\"0\"}}",
			false,
		},
		{
			"Associate query lungroup fail",
			"{\"data\":{[]},\"error\":{\"code\":0,\"description\": \"0\"}}",
			true,
		},
		{
			"Associate query lungroup error",
			"{\"data\":{},\"error\":{\"code\":1077949061,\"description\": \"0\"}}",
			true,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	temp := testClient.Client
	defer func() { testClient.Client = temp }()

	for _, s := range cases {
		t.Run(s.Name, func(t *testing.T) {
			mockClient := NewMockHTTPClient(ctrl)
			testClient.Client = mockClient
			mockClient.EXPECT().Do(gomock.Any()).DoAndReturn(func(req *http.Request) (*http.Response, error) {
				r := io.NopCloser(bytes.NewReader([]byte(s.ResponseBody)))
				return &http.Response{
					StatusCode: int(successStatus),
					Body:       r,
				}, nil
			}).AnyTimes()

			_, err := testClient.QueryAssociateLunGroup(context.TODO(), 245, "")
			assert.Equal(t, s.wantErr, err != nil, "err:%v", err)
		})
	}
}

func TestDeleteLun(t *testing.T) {
	var cases = []struct {
		Name         string
		ResponseBody string
		wantErr      bool
	}{
		{
			"Normal",
			"{\"data\":{},\"error\":{\"code\":0,\"description\":\"0\"}}",
			false,
		},
		{
			"Lun does not exist while deleting",
			"{\"data\":{},\"error\":{\"code\":1077936859,\"description\":\"0\"}}",
			false,
		},
		{
			"Delete lun error",
			"{\"data\":{},\"error\":{\"code\":1077949061,\"description\":\"0\"}}",
			true,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	temp := testClient.Client
	defer func() { testClient.Client = temp }()

	for _, s := range cases {
		t.Run(s.Name, func(t *testing.T) {
			mockClient := NewMockHTTPClient(ctrl)
			testClient.Client = mockClient
			mockClient.EXPECT().Do(gomock.Any()).DoAndReturn(func(req *http.Request) (*http.Response, error) {
				r := io.NopCloser(bytes.NewReader([]byte(s.ResponseBody)))
				return &http.Response{
					StatusCode: int(successStatus),
					Body:       r,
				}, nil
			}).AnyTimes()

			err := testClient.DeleteLun(context.TODO(), "")
			assert.Equal(t, s.wantErr, err != nil, "err:%v", err)
		})
	}
}

func TestGetPoolByName(t *testing.T) {
	var cases = []struct {
		Name         string
		ResponseBody string
		wantErr      bool
	}{
		{
			"Normal",
			"{\"data\":[{\"DESCRIPTION\":\"\",\"ENDINGUPTHRESHOLD\":\"90\",\"ID\":\"0\",\"TIER0CAPACITY\":" +
				"\"18446744073709551615\",\"TIER1CAPACITY\":\"6688675840\",\"TIER2CAPACITY\":" +
				"\"18446744073709551615\",\"NAME\":\"test\",\"PARENTID\":\"0\",\"PARENTTYPE\":266," +
				"\"PROVISIONINGLIMIT\":\"-1\",\"PROVISIONINGLIMITSWITCH\":\"false\",\"TIER0DISKTYPE\":\"3\"," +
				"\"TIER0RAIDLV\":\"5\",\"USAGETYPE\":\"1\",\"USERCONSUMEDCAPACITYTHRESHOLD\":\"80\"," +
				"\"autoDeleteSwitch\":\"0\",\"fsSpaceReductionRate\":\"\",\"lunSpaceReductionRate\":\"\"," +
				"\"poolProtectHighThreshold\":\"30\",\"poolProtectLowThreshold\":\"20\",\"COMPRESSEDCAPACITY\":\"0\"," +
				"\"COMPRESSINVOLVEDCAPACITY\":\"0\",\"COMPRESSIONRATE\":\"\",\"DATASPACE\":\"6688675840\"," +
				"\"DEDUPEDCAPACITY\":\"0\",\"DEDUPINVOLVEDCAPACITY\":\"0\",\"DEDUPLICATIONRATE\":\"\"," +
				"\"FSSHAREDCAPACITY\":\"0\",\"FSSUBSCRIBEDCAPACITY\":\"0\",\"FSUSEDCAPACITY\":\"0\",\"HEALTHSTATUS\":" +
				"\"1\",\"ISCONTAINERENABLE\":\"false\",\"LUNCONFIGEDCAPACITY\":\"104857600\",\"LUNMAPPEDCAPACITY\":" +
				"\"0\",\"NEWUSAGETYPE\":\"0\",\"PARENTNAME\":\"test\",\"REDUCTIONINVOLVEDCAPACITY\":\"0\"," +
				"\"REPLICATIONCAPACITY\":\"0\",\"RUNNINGSTATUS\":\"27\",\"SAVECAPACITYRATE\":\"\"," +
				"\"SPACEREDUCTIONRATE\":\"\",\"SUBSCRIBEDCAPACITY\":\"104857600\",\"THINPROVISIONSAVEPERCENTAGE\":" +
				"\"{\\\"numerator\\\":\\\"1000\\\",\\\"denominator\\\":\\\"10\\\",\\\"logic\\\":\\\"=\\\"}\"," +
				"\"TOTALFSCAPACITY\":\"0\",\"TOTALLUNWRITECAPACITY\":\"0\",\"USEDSUBSCRIBEDCAPACITY\":\"0\"," +
				"\"USERCONSUMEDCAPACITY\":\"0\",\"USERCONSUMEDCAPACITYPERCENTAGE\":\"0\"," +
				"\"USERCONSUMEDCAPACITYWITHOUTMETA\":\"0\",\"USERFREECAPACITY\":\"6688675840\",\"USERTOTALCAPACITY\":" +
				"\"6688675840\",\"USERWRITEALLOCCAPACITY\":\"0\",\"protectSize\":\"0\",\"TYPE\":216}]," +
				"\"error\":{\"code\":0,\"description\":\"0\"}}",
			false,
		},
		{
			"Pool does not exist",
			"{\"data\":[{}],\"error\":{\"code\":0,\"description\":\"0\"}}",
			false,
		},
		{
			"Get pool info error",
			"{\"data\":[{}],\"error\":{\"code\":1077949061,\"description\":\"0\"}}",
			true,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	temp := testClient.Client
	defer func() { testClient.Client = temp }()

	for _, s := range cases {
		t.Run(s.Name, func(t *testing.T) {
			mockClient := NewMockHTTPClient(ctrl)
			testClient.Client = mockClient
			mockClient.EXPECT().Do(gomock.Any()).DoAndReturn(func(req *http.Request) (*http.Response, error) {
				r := io.NopCloser(bytes.NewReader([]byte(s.ResponseBody)))
				return &http.Response{
					StatusCode: int(successStatus),
					Body:       r,
				}, nil
			}).AnyTimes()

			_, err := testClient.GetPoolByName(context.TODO(), "")
			assert.Equal(t, s.wantErr, err != nil, "err:%v", err)
		})
	}
}

func TestGetAllPools(t *testing.T) {
	var cases = []struct {
		Name         string
		ResponseBody string
		wantErr      bool
	}{
		{
			"Normal",
			"{\"data\":[{\"DESCRIPTION\":\"\",\"ENDINGUPTHRESHOLD\":\"90\",\"ID\":\"0\",\"TIER0CAPACITY\":" +
				"\"18446744073709551615\",\"TIER1CAPACITY\":\"6688675840\",\"TIER2CAPACITY\":" +
				"\"18446744073709551615\",\"NAME\":\"test\",\"PARENTID\":\"0\",\"PARENTTYPE\":266," +
				"\"PROVISIONINGLIMIT\":\"-1\",\"PROVISIONINGLIMITSWITCH\":\"false\",\"TIER0DISKTYPE\":\"3\"," +
				"\"TIER0RAIDLV\":\"5\",\"USAGETYPE\":\"1\",\"USERCONSUMEDCAPACITYTHRESHOLD\":\"80\"," +
				"\"autoDeleteSwitch\":\"0\",\"fsSpaceReductionRate\":\"\",\"lunSpaceReductionRate\":\"\"," +
				"\"poolProtectHighThreshold\":\"30\",\"poolProtectLowThreshold\":\"20\",\"COMPRESSEDCAPACITY\":\"0\"," +
				"\"COMPRESSINVOLVEDCAPACITY\":\"0\",\"COMPRESSIONRATE\":\"\",\"DATASPACE\":\"6688675840\"," +
				"\"DEDUPEDCAPACITY\":\"0\",\"DEDUPINVOLVEDCAPACITY\":\"0\",\"DEDUPLICATIONRATE\":\"\"," +
				"\"FSSHAREDCAPACITY\":\"0\",\"FSSUBSCRIBEDCAPACITY\":\"0\",\"FSUSEDCAPACITY\":\"0\",\"HEALTHSTATUS\":" +
				"\"1\",\"ISCONTAINERENABLE\":\"false\",\"LUNCONFIGEDCAPACITY\":\"104857600\",\"LUNMAPPEDCAPACITY\":" +
				"\"0\",\"NEWUSAGETYPE\":\"0\",\"PARENTNAME\":\"test\",\"REDUCTIONINVOLVEDCAPACITY\":\"0\"," +
				"\"REPLICATIONCAPACITY\":\"0\",\"RUNNINGSTATUS\":\"27\",\"SAVECAPACITYRATE\":\"\"," +
				"\"SPACEREDUCTIONRATE\":\"\",\"SUBSCRIBEDCAPACITY\":\"104857600\",\"THINPROVISIONSAVEPERCENTAGE\":" +
				"\"{\\\"numerator\\\":\\\"1000\\\",\\\"denominator\\\":\\\"10\\\",\\\"logic\\\":\\\"=\\\"}\"," +
				"\"TOTALFSCAPACITY\":\"0\",\"TOTALLUNWRITECAPACITY\":\"0\",\"USEDSUBSCRIBEDCAPACITY\":\"0\"," +
				"\"USERCONSUMEDCAPACITY\":\"0\",\"USERCONSUMEDCAPACITYPERCENTAGE\":\"0\"," +
				"\"USERCONSUMEDCAPACITYWITHOUTMETA\":\"0\",\"USERFREECAPACITY\":\"6688675840\",\"USERTOTALCAPACITY\":" +
				"\"6688675840\",\"USERWRITEALLOCCAPACITY\":\"0\",\"protectSize\":\"0\",\"TYPE\":216}],\"error\":" +
				"{\"code\":0,\"description\":\"0\"}}",
			false,
		},
		{
			"Get all pools info error",
			"{\"data\":[{}],\"error\":{\"code\":1077949061,\"description\":\"0\"}}",
			true,
		},
		{
			"There's no pools exist",
			"{\"data\":[{}],\"error\":{\"code\": ,\"description\":\"0\"}}",
			true,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	temp := testClient.Client
	defer func() { testClient.Client = temp }()

	for _, s := range cases {
		t.Run(s.Name, func(t *testing.T) {
			mockClient := NewMockHTTPClient(ctrl)
			testClient.Client = mockClient
			mockClient.EXPECT().Do(gomock.Any()).DoAndReturn(func(req *http.Request) (*http.Response, error) {
				r := io.NopCloser(bytes.NewReader([]byte(s.ResponseBody)))
				return &http.Response{
					StatusCode: int(successStatus),
					Body:       r,
				}, nil
			}).AnyTimes()

			_, err := testClient.GetAllPools(context.TODO())
			assert.Equal(t, s.wantErr, err != nil, "err:%v", err)
		})
	}
}

func TestCreateHost(t *testing.T) {
	var cases = []struct {
		Name         string
		ResponseBody string
		wantErr      bool
	}{
		{
			"Normal",
			"{\"data\":{\"DESCRIPTION\":\"host\",\"HEALTHSTATUS\":\"1\",\"ID\":\"15\",\"INITIATORNUM\": " +
				"\"0\",\"IP\":\"\",\"ISADD2HOSTGROUP\":\"false\",\"LOCATION\":\"\",\"MODEL\":\"\",\"NAME\":" +
				"\"host003\",\"NETWORKNAME\":\"\",\"OPERATIONSYSTEM\":\"9\",\"RUNNINGSTATUS\":\"1\",\"TYPE\":21," +
				"\"accessMode\":\"1\",\"allocatedCapacity\":\"0\",\"capacity\":\"0\",\"enableInbandCommand\":" +
				"\"false\",\"hyperMetroPathOptimized\":\"true\",\"mappingLunNumber\":\"0\",\"protectionCapacity\":" +
				"\"0\"},\"error\":{\"code\":0,\"description\":\"0\"}}",
			false,
		},
		{
			"Host already exists",
			"{\"data\":{},\"error\":{\"code\":1077948993,\"description\":\"\"}}",
			true,
		},
		{
			"Create host error",
			"{\"data\":{},\"error\":{\"code\":1077949061,\"description\":\"\"}}",
			true,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	temp := testClient.Client
	defer func() { testClient.Client = temp }()

	for _, s := range cases {
		t.Run(s.Name, func(t *testing.T) {
			mockClient := NewMockHTTPClient(ctrl)
			testClient.Client = mockClient
			mockClient.EXPECT().Do(gomock.Any()).DoAndReturn(func(req *http.Request) (*http.Response, error) {
				r := io.NopCloser(bytes.NewReader([]byte(s.ResponseBody)))
				return &http.Response{
					StatusCode: int(successStatus),
					Body:       r,
				}, nil
			}).AnyTimes()

			_, err := testClient.CreateHost(context.TODO(), "")
			assert.Equal(t, s.wantErr, err != nil, "err:%v", err)
		})
	}
}

func TestGetHostByName(t *testing.T) {
	var cases = []struct {
		Name         string
		ResponseBody string
		wantErr      bool
	}{
		{
			"Normal",
			"{\"data\":[{\"DESCRIPTION\":\"\",\"HEALTHSTATUS\":\"1\",\"ID\":\"1\",\"INITIATORNUM\":\"0\"," +
				"\"IP\":\"\",\"ISADD2HOSTGROUP\":\"false\",\"LOCATION\":\"\",\"MODEL\":\"\",\"NAME\":\"Host002\"," +
				"\"NETWORKNAME\":\"\",\"OPERATIONSYSTEM\":\"0\",\"RUNNINGSTATUS\":\"1\",\"TYPE\":21,\"accessMode\":" +
				"\"0\",\"mappingLunNumber\":\"256\",\"capacity\":\"2097152\",\"allocatedCapacity\":\"1097152\"," +
				"\"protectionCapacity\":\"40956\",\"enableInbandCommand\":\"false\"},{\"DESCRIPTION\":\"\"," +
				"\"HEALTHSTATUS\":\"1\",\"ID\":\"2\",\"INITIATORNUM\":\"1\",\"IP\":\"\",\"ISADD2HOSTGROUP\":\"true\"," +
				"\"LOCATION\":\"\",\"MODEL\":\"\",\"NAME\":\"Host002\",\"NETWORKNAME\":\"\",\"OPERATIONSYSTEM\":" +
				"\"0\",\"PARENTID\":\"0\",\"PARENTNAME\":\"HG002\",\"PARENTTYPE\":14,\"RUNNINGSTATUS\":\"1\"," +
				"\"TYPE\":21,\"accessMode\":\"1\",\"allocatedCapacity\":\"0\",\"capacity\":\"0\"," +
				"\"enableInbandCommand\":\"false\",\"hyperMetroPathOptimized\":\"true\",\"mappingLunNumber\":\"0\"," +
				"\"protectionCapacity\":\"0\"}],\"error\":{\"code\":0,\"description\":\"0\"}}",
			false,
		},
		{
			"Get host error",
			"{\"data\":[{}],\"error\":{\"code\":1077949061,\"description\":\"\"}}",
			true,
		},
		{
			"Host does not exist",
			"{\"data\":[{}],\"error\":{\"code\":0,\"description\":\"\"}}",
			false,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	temp := testClient.Client
	defer func() { testClient.Client = temp }()

	for _, s := range cases {
		t.Run(s.Name, func(t *testing.T) {
			mockClient := NewMockHTTPClient(ctrl)
			testClient.Client = mockClient
			mockClient.EXPECT().Do(gomock.Any()).DoAndReturn(func(req *http.Request) (*http.Response, error) {
				r := io.NopCloser(bytes.NewReader([]byte(s.ResponseBody)))
				return &http.Response{
					StatusCode: int(successStatus),
					Body:       r,
				}, nil
			}).AnyTimes()

			_, err := testClient.GetHostByName(context.TODO(), "")
			assert.Equal(t, s.wantErr, err != nil, "err:%v", err)
		})
	}
}

func TestDeleteHost(t *testing.T) {
	var cases = []struct {
		Name         string
		ResponseBody string
		wantErr      bool
	}{
		{
			"Normal",
			"{\"data\":{},\"error\":{\"code\":0,\"description\":\"0\"}}",
			false,
		},
		{
			"Host does not exist",
			"{\"data\":{},\"error\":{\"code\":1077937498,\"description\":\"0\"}}",
			false,
		},
		{
			"Delete host error",
			"{\"data\":{},\"error\":{\"code\":1077949061,\"description\":\"0\"}}",
			true,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	temp := testClient.Client
	defer func() { testClient.Client = temp }()

	for _, s := range cases {
		t.Run(s.Name, func(t *testing.T) {
			mockClient := NewMockHTTPClient(ctrl)
			testClient.Client = mockClient
			mockClient.EXPECT().Do(gomock.Any()).DoAndReturn(func(req *http.Request) (*http.Response, error) {
				r := io.NopCloser(bytes.NewReader([]byte(s.ResponseBody)))
				return &http.Response{
					StatusCode: int(successStatus),
					Body:       r,
				}, nil
			}).AnyTimes()

			err := testClient.DeleteHost(context.TODO(), "")
			assert.Equal(t, s.wantErr, err != nil, "err:%v", err)
		})
	}
}

func TestCreateHostGroup(t *testing.T) {
	var cases = []struct {
		Name         string
		ResponseBody string
		wantErr      bool
	}{
		{
			"Normal",
			"{\"data\":{\"DESCRIPTION\":\"test create hostGroup rest\",\"ID\":\"0\",\"ISADD2MAPPINGVIEW\":" +
				"\"false\",\"NAME\":\"HostGroup001\",\"TYPE\":14,\"allocatedCapacity\":\"\",\"capacity\":\"\"," +
				"\"hostNumbe\":\"0\",\"hostNumber\":\"0\",\"mappingLunNumber\":\"0\",\"protectionCapacity\":\"\"}," +
				"\"error\":{\"code\":0,\"description\":\"0\"}}",
			false,
		},
		{
			"Hostgroup already exists",
			"{\"data\":{},\"error\":{\"code\":1077948993,\"description\":\"0\"}}",
			true,
		},
		{
			"Create hostgroup error",
			"{\"data\":{},\"error\":{\"code\":1077949061,\"description\":\"0\"}}",
			true,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	temp := testClient.Client
	defer func() { testClient.Client = temp }()

	for _, s := range cases {
		t.Run(s.Name, func(t *testing.T) {
			mockClient := NewMockHTTPClient(ctrl)
			testClient.Client = mockClient
			mockClient.EXPECT().Do(gomock.Any()).DoAndReturn(func(req *http.Request) (*http.Response, error) {
				r := io.NopCloser(bytes.NewReader([]byte(s.ResponseBody)))
				return &http.Response{
					StatusCode: int(successStatus),
					Body:       r,
				}, nil
			}).AnyTimes()

			_, err := testClient.CreateHostGroup(context.TODO(), "")
			assert.Equal(t, s.wantErr, err != nil, "err:%v", err)
		})
	}
}

func TestGetHostGroupByName(t *testing.T) {
	var cases = []struct {
		name         string
		responseBody string
		wantErr      bool
		err          error
	}{
		{
			"Normal",
			"{\"data\":[{\"DESCRIPTION\":\"\",\"ID\":\"0\",\"ISADD2MAPPINGVIEW\":\"false\",\"NAME\":" +
				"\"hg1\",\"TYPE\":14,\"allocatedCapacity\":\"\",\"capacity\":\"\",\"hostNumber\":\"0\",\"hostNumbe\":" +
				"\"0\",\"mappingLunNumber\":\"0\",\"protectionCapacity\":\"\"},{\"DESCRIPTION\":\"\",\"ID\":\"1\"," +
				"\"ISADD2MAPPINGVIEW\":\"false\",\"NAME\":\"hg2\",\"TYPE\":14,\"allocatedCapacity\":\"\"," +
				"\"capacity\":\"\",\"hostNumbe\":\"0\",\"hostNumber\":\"0\",\"mappingLunNumber\":\"0\"," +
				"\"protectionCapacity\":\"\"}],\"error\":{\"code\":0,\"description\":\"0\"}}",
			false,
			nil,
		},
		{
			"Get hostgroup by name fail",
			"{\"data\":[{}],\"error\":{\"code\":1077949061,\"description\":\"0\"}}",
			true,
			errors.New("get hostgroup by name fail"),
		},
		{
			"Get hostgroup by name error",
			"{\"data\":[{}],\"error\":{\"code\":1077949061,\"description\":\"0\"}}",
			true,
			errors.New("get hostgroup by name error"),
		},
		{
			"Hostgroup %s does not exist",
			"{\"data\":[{}],\"error\":{\"code\":0,\"description\":\"0\"}}",
			false,
			nil,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	temp := testClient.Client
	defer func() { testClient.Client = temp }()

	for _, s := range cases {
		t.Run(s.name, func(t *testing.T) {
			mockClient := NewMockHTTPClient(ctrl)
			testClient.Client = mockClient
			mockClient.EXPECT().Do(gomock.Any()).DoAndReturn(func(req *http.Request) (*http.Response, error) {
				r := io.NopCloser(bytes.NewReader([]byte(s.responseBody)))
				return &http.Response{
					StatusCode: int(successStatus),
					Body:       r,
				}, nil
			}).Times(1)

			_, err := testClient.GetHostGroupByName(context.TODO(), "")
			assert.Equal(t, s.wantErr, err != nil, "err:%v", err)
		})
	}
}

func TestDeleteHostGroup(t *testing.T) {
	var cases = []struct {
		name         string
		responseBody string
		wantErr      bool
		err          error
	}{
		{
			"Normal",
			"{\"data\":{},\"error\":{\"code\": 0,\"description\":\"0\"}}",
			false,
			nil,
		},
		{
			"Delete hostgroup fail",
			"{\"data\":{},\"error\":{\"code\":1077949061,\"description\":\"0\"}}",
			true,
			errors.New("delete hostgroup fail"),
		},
		{
			"Delete hostgroup error",
			"{\"data\":{},\"error\":{\"code\":1077949061,\"description\":\"0\"}}",
			true,
			errors.New("delete hostgroup error"),
		},
		{
			"Hostgroup does not exist",
			"{\"data\":[{}],\"error\":{\"code\":0,\"description\":\"0\"}}",
			false,
			nil,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	temp := testClient.Client
	defer func() { testClient.Client = temp }()

	for _, s := range cases {
		t.Run(s.name, func(t *testing.T) {
			mockClient := NewMockHTTPClient(ctrl)
			testClient.Client = mockClient
			mockClient.EXPECT().Do(gomock.Any()).DoAndReturn(func(req *http.Request) (*http.Response, error) {
				r := io.NopCloser(bytes.NewReader([]byte(s.responseBody)))
				return &http.Response{
					StatusCode: int(successStatus),
					Body:       r,
				}, nil
			}).Times(1)

			err := testClient.DeleteHostGroup(context.TODO(), "")
			assert.Equal(t, s.wantErr, err != nil, "err:%v", err)
		})
	}
}

func TestCreateMapping(t *testing.T) {
	var cases = []struct {
		name         string
		responseBody string
		wantErr      bool
	}{
		{
			"Normal",
			"{\"data\":{\"AVAILABLEHOSTLUNIDLIST\":\"[0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19," +
				"20,21,22,23,24,25,26,27,28,29,30,31,32,33,34,35,36,37,38,39,40,41,42,43,44,45,46,47,48,49,50,51,52," +
				"53,54,55,56,57,58,59,60,61,62,63,64,65,66,67,68,69,70,71,72,73,74,75,76,77,78,79,80,81,82,83,84,85," +
				"86,87,88,89,90,91,92,93,94,95,96,97,98,99,100,101,102,103,104,105,106,107,108,109,110,111,112,113," +
				"114,115,116,117,118,119,120,121,122,123,124,125,126,127,128,129,130,131,132,133,134,135,136,137,138," +
				"139,140,141,142,143,144,145,146,147,148,149,150,151,152,153,154,155,156,157,158,159,160,161,162,163," +
				"164,165,166,167,168,169,170,171,172,173,174,175,176,177,178,179,180,181,182,183,184,185,186,187,188," +
				"189,190,191,192,193,194,195,196,197,198,199,200,201,202,203,204,205,206,207,208,209,210,211,212,213," +
				"214,215,216,217,218,219,220,221,222,223,224,225,226,227,228,229,230,231,232,233,234,235,236,237,238," +
				"239,240,241,242,243,244,245,246,247,248,249,250,251,252,253,254,255,256,257,258,259,260,261,262,263," +
				"264,265,266,267,268,269,270,271,272,273,274,275,276,277,278,279,280,281,282,283,284,285,286,287,288," +
				"289,290,291,292,293,294,295,296,297,298,299,300,301,302,303,304,305,306,307,308,309,310,311,312,313," +
				"314,315,316,317,318,319,320,321,322,323,324,325,326,327,328,329,330,331,332,333,334,335,336,337,338," +
				"339,340,341,342,343,344,345,346,347,348,349,350,351,352,353,354,355,356,357,358,359,360,361,362,363," +
				"364,365,366,367,368,369,370,371,372,373,374,375,376,377,378,379,380,381,382,383,384,385,386,387,388," +
				"389,390,391,392,393,394,395,396,397,398,399,400,401,402,403,404,405,406,407,408,409,410,411,412,413," +
				"414,415,416,417,418,419,420,421,422,423,424,425,426,427,428,429,430,431,432,433,434,435,436,437,438," +
				"439,440,441,442,443,444,445,446,447,448,449,450,451,452,453,454,455,456,457,458,459,460,461,462,463," +
				"464,465,466,467,468,469,470,471,472,473,474,475,476,477,478,479,480,481,482,483,484,485,486,487,488," +
				"489,490,491,492,493,494,495,496,497,498,499,500,501,502,503,504,505,506,507,508,509,510,511]\"," +
				"\"DESCRIPTION\":\"\",\"ENABLEINBANDCOMMAND\":\"true\",\"ID\":\"1\",\"INBANDLUNWWN\":\"\",\"NAME\":" +
				"\"MappingView001\",\"TYPE\":245},\"error\":{\"code\":0,\"description\":\"0\"}}",
			false,
		},
		{
			"Create mapping success",
			"{\"data\":{},\"error\":{\"code\":0,\"description\":\"0\"}}",
			false,
		},
		{
			"Mapping already exists",
			"{\"data\":{},\"error\":{\"code\":1077948993,\"description\":\"0\"}}",
			true,
		},
		{
			"Create mapping error",
			"{\"data\":{},\"error\":{\"code\":1077949061,\"description\":\"0\"}}",
			true,
		},
	}

	g := gomonkey.ApplyFunc(pkgUtils.GetPasswordFromBackendID,
		func(ctx context.Context, backendID string) (string, error) {
			return "mock", nil
		})
	defer g.Reset()

	for _, s := range cases {
		m := gomonkey.ApplyMethod(reflect.TypeOf(testClient.Client),
			"Do",
			func(_ *http.Client, req *http.Request) (*http.Response, error) {
				r := ioutil.NopCloser(bytes.NewReader([]byte(s.responseBody)))
				return &http.Response{
					StatusCode: 200,
					Body:       r,
				}, nil
			})

		_, err := testClient.CreateMapping(context.TODO(), "")
		assert.Equal(t, s.wantErr, err != nil, "%s, err:%v", s.name, err)
		m.Reset()
	}
}

func TestGetMappingByName(t *testing.T) {
	var cases = []struct {
		name         string
		responseBody string
		wantErr      bool
		err          error
	}{
		{
			"Normal",
			"{\"data\":[{\"DESCRIPTION\":\"\",\"ENABLEINBANDCOMMAND\":\"false\",\"ID\":\"0\"," +
				"\"INBANDLUNWWN\":\"\",\"NAME\":\"map_1607754116393916_idx1\",\"TYPE\":245,\"hostGroupId\":\"0\"," +
				"\"hostGroupName\":\"HostGroup001\",\"hostName\":\"\",\"lunGroupId\":\"0\",\"lunGroupName\":" +
				"\"LUNGroup001\",\"portGroupId\":\"0\",\"portGroupName\":\"PortGroup001\"}],\"error\":{\"code\":0," +
				"\"description\":\"0\"}}",
			false,
			nil,
		},
		{
			"Get mapping by name fail",
			"{\"data\":[{}],\"error\":{\"code\":0,\"description\":\"0\"}}",
			false,
			errors.New("get mapping by name fail"),
		},
		{
			"Get mapping by name error",
			"{\"data\":[{}],\"error\":{\"code\":1077949061,\"description\":\"0\"}}",
			true,
			errors.New("get mapping by name error"),
		},
		{
			"Mapping does not exist",
			"{\"data\":[{}],\"error\":{\"code\":0,\"description\":\"0\"}}",
			false,
			nil,
		},
	}

	for _, s := range cases {
		g := gomonkey.ApplyMethod(reflect.TypeOf(testClient.Client), "Do",
			func(*http.Client, *http.Request) (*http.Response, error) {
				r := ioutil.NopCloser(bytes.NewReader([]byte(s.responseBody)))
				return &http.Response{
					StatusCode: 200,
					Body:       r,
				}, nil
			})

		_, err := testClient.GetMappingByName(context.TODO(), "")
		assert.Equal(t, s.wantErr, err != nil, "%s, err:%v", s.name, err)
		g.Reset()
	}
}

func TestDeleteMapping(t *testing.T) {
	var cases = []struct {
		name         string
		responseBody string
		wantErr      bool
		err          error
	}{
		{
			"Normal",
			"{\"data\":{},\"error\":{\"code\":0,\"description\":\"0\"}}",
			false,
			nil,
		},
		{
			"Delete mapping fail",
			"{\"data\":{},\"error\":{\"code\":0,\"description\":\"0\"}}",
			false,
			errors.New("delete mapping fail"),
		},
		{
			"Mapping does not exist while deleting",
			"{\"data\":{},\"error\":{\"code\":1077951819,\"description\":\"0\"}}",
			false,
			nil,
		},
		{
			"Delete mapping error",
			"{\"data\":{},\"error\":{\"code\":1077949061,\"description\":\"0\"}}",
			true,
			errors.New("delete mapping error"),
		},
	}

	g := gomonkey.ApplyFunc(pkgUtils.GetPasswordFromBackendID,
		func(ctx context.Context, backendID string) (string, error) {
			return "mock", nil
		})
	defer g.Reset()

	for _, s := range cases {
		c := gomonkey.ApplyMethod(reflect.TypeOf(testClient.Client), "Do",
			func(*http.Client, *http.Request) (*http.Response, error) {
				r := ioutil.NopCloser(bytes.NewReader([]byte(s.responseBody)))
				return &http.Response{
					StatusCode: 200,
					Body:       r,
				}, nil
			})

		err := testClient.DeleteMapping(context.TODO(), "")
		assert.Equal(t, s.wantErr, err != nil, "%s, err:%v", s.name, err)
		c.Reset()
	}
}

func TestAddHostToGroup(t *testing.T) {
	var cases = []struct {
		name         string
		responseBody string
		wantErr      bool
		err          error
	}{
		{
			"Normal",
			"{\"data\":{ },\"error\":{\"code\":0,\"description\":\"0\"}}",
			false,
			nil,
		},
		{
			"Host is already in hostgroup",
			"{\"data\":{},\"error\":{\"code\":1077937501,\"description\":\"0\"}}",
			false,
			nil,
		},
		{
			"Add host to hostgroup error",
			"{\"data\":{},\"error\":{\"code\":1077949061,\"description\":\"0\"}}",
			true,
			nil,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	temp := testClient.Client
	defer func() { testClient.Client = temp }()

	for _, s := range cases {
		t.Run(s.name, func(t *testing.T) {
			mockClient := NewMockHTTPClient(ctrl)
			testClient.Client = mockClient
			mockClient.EXPECT().Do(gomock.Any()).DoAndReturn(func(req *http.Request) (*http.Response, error) {
				r := io.NopCloser(bytes.NewReader([]byte(s.responseBody)))
				return &http.Response{
					StatusCode: int(successStatus),
					Body:       r,
				}, s.err
			}).Times(1)
			err := testClient.AddHostToGroup(context.TODO(), "", "")
			assert.Equal(t, s.wantErr, err != nil, "err:%v", err)
		})
	}
}

func TestRemoveHostFromGroup(t *testing.T) {
	var cases = []struct {
		name         string
		responseBody string
		wantErr      bool
		err          error
	}{
		{
			"Normal",
			"{\"data\":{},\"error\":{\"code\":0,\"description\":\"0\"}}",
			false,
			nil,
		},
		{
			"Remove host from hostgroup fail",
			"{\"data\":{},\"error\":{\"code\":0,\"description\":\"0\"}}",
			false,
			errors.New("remove host from hostgroup fail"),
		},
		{
			"Host is not in hostgroup",
			"{\"data\":{},\"error\":{\"code\":1073745412,\"description\":\"0\"}}",
			false,
			nil,
		},
		{
			"Remove host from hostgroup error",
			"{\"data\":{},\"error\":{\"code\":1073745413,\"description\":\"0\"}}",
			true,
			errors.New("remove host from hostgroup error"),
		},
	}

	g := gomonkey.ApplyFunc(pkgUtils.GetPasswordFromBackendID,
		func(ctx context.Context, backendID string) (string, error) {
			return "mock", nil
		})
	defer g.Reset()

	for _, s := range cases {
		d := gomonkey.ApplyMethod(reflect.TypeOf(testClient.Client), "Do",
			func(*http.Client, *http.Request) (*http.Response, error) {
				r := io.NopCloser(bytes.NewReader([]byte(s.responseBody)))
				return &http.Response{
					StatusCode: 200,
					Body:       r,
				}, nil
			})

		err := testClient.RemoveHostFromGroup(context.TODO(), "", "")
		assert.Equal(t, s.wantErr, err != nil, "%s, err:%v", s.name, err)
		d.Reset()
	}
}

func TestQueryAssociateHostGroup(t *testing.T) {
	var cases = []struct {
		name         string
		responseBody string
		wantErr      bool
		err          error
	}{
		{
			"Normal",
			"{\"data\":[{\"DESCRIPTION\":\"\",\"ID\":\"1\",\"ISADD2MAPPINGVIEW\":\"true\",\"NAME\":" +
				"\"HostGroup002\",\"TYPE\":21}],\"error\":{\"code\":0,\"description\":\"0\"}}",
			false,
			nil,
		},
		{
			"Associate query hostgroup by obj fail",
			"{\"data\":[{}],\"error\":{\"code\":0,\"description\":\"0\"}}",
			false,
			errors.New("associate query hostgroup by obj fail"),
		},
		{
			"Associate query hostgroup by obj error",
			"{\"data\":[{}],\"error\":{\"code\":1077949061,\"description\":\"0\"}}",
			true,
			errors.New("associate query hostgroup by obj error"),
		},
		{
			"Obj doesn't associate to any hostgroup",
			"{\"data\":[{}],\"error\":{\"code\":0,\"description\":\"0\"}}",
			false,
			nil,
		},
	}

	g := gomonkey.ApplyFunc(pkgUtils.GetPasswordFromBackendID,
		func(ctx context.Context, backendID string) (string, error) {
			return "mock", nil
		})
	defer g.Reset()

	for _, s := range cases {
		d := gomonkey.ApplyMethod(reflect.TypeOf(testClient.Client), "Do",
			func(*http.Client, *http.Request) (*http.Response, error) {
				r := ioutil.NopCloser(bytes.NewReader([]byte(s.responseBody)))
				return &http.Response{
					StatusCode: 200,
					Body:       r,
				}, nil
			})

		_, err := testClient.QueryAssociateHostGroup(context.TODO(), 21, "")
		assert.Equal(t, s.wantErr, err != nil, "%s, err:%v", s.name, err)
		d.Reset()
	}
}

func TestAddGroupToMapping(t *testing.T) {
	var cases = []struct {
		name         string
		responseBody string
		wantErr      bool
		err          error
	}{
		{
			"Normal",
			"{\"data\":{},\"error\":{\"code\":0,\"description\":\"0\"}}",
			false,
			nil,
		},
		{
			"Group is already in mapping",
			"{\"data\":{},\"error\":{\"code\":1073804556,\"description\":\"0\"}}",
			false,
			nil,
		},
		{
			"Group is already in mapping",
			"{\"data\":{},\"error\":{\"code\":1073804560,\"description\":\"0\"}}",
			false,
			nil,
		},
		{
			"Add group to mapping error",
			"{\"data\":{},\"error\":{\"code\":1077949061,\"description\":\"0\"}}",
			true,
			errors.New("add group to mapping error"),
		},
	}

	g := gomonkey.ApplyFunc(pkgUtils.GetPasswordFromBackendID,
		func(ctx context.Context, backendID string) (string, error) {
			return "mock", nil
		})
	defer g.Reset()

	for _, s := range cases {
		d := gomonkey.ApplyMethod(reflect.TypeOf(testClient.Client), "Do",
			func(*http.Client, *http.Request) (*http.Response, error) {
				r := ioutil.NopCloser(bytes.NewReader([]byte(s.responseBody)))
				return &http.Response{
					StatusCode: 200,
					Body:       r,
				}, nil
			})

		err := testClient.AddGroupToMapping(context.TODO(), 14, "", "")
		assert.Equal(t, s.wantErr, err != nil, "%s, err:%v", s.name, err)
		d.Reset()
	}
}

func TestRemoveGroupFromMapping(t *testing.T) {
	var cases = []struct {
		name         string
		responseBody string
		wantErr      bool
		err          error
	}{
		{
			"Normal",
			"{\"data\":{},\"error\":{\"code\":0,\"description\":\"0\"}}",
			false,
			nil,
		},
		{
			"Group is not in mapping",
			"{\"data\":{},\"error\":{\"code\":1073804552,\"description\":\"0\"}}",
			false,
			nil,
		},
		{
			"Group is not in mapping",
			"{\"data\":{},\"error\":{\"code\":1073804554,\"description\":\"0\"}}",
			false,
			nil,
		},
		{
			"Remove group from mapping error",
			"{\"data\":{},\"error\":{\"code\":1077949061,\"description\":\"0\"}}",
			true,
			errors.New("remove group from mapping error"),
		},
	}

	g := gomonkey.ApplyFunc(pkgUtils.GetPasswordFromBackendID,
		func(ctx context.Context, backendID string) (string, error) {
			return "mock", nil
		})
	defer g.Reset()

	for _, s := range cases {
		d := gomonkey.ApplyMethod(reflect.TypeOf(testClient.Client), "Do",
			func(*http.Client, *http.Request) (*http.Response, error) {
				r := ioutil.NopCloser(bytes.NewReader([]byte(s.responseBody)))
				return &http.Response{
					StatusCode: 200,
					Body:       r,
				}, nil
			})

		err := testClient.RemoveGroupFromMapping(context.TODO(), 256, "", "")
		assert.Equal(t, s.wantErr, err != nil, "%s, err:%v", s.name, err)
		d.Reset()
	}
}

func TestGetLunCountOfHost(t *testing.T) {
	var cases = []struct {
		name         string
		responseBody string
		wantErr      bool
		err          error
	}{
		{
			"Normal",
			"{\"data\":{\"COUNT\":\"10\"},\"error\":{\"code\":0,\"description\":\"0\"}}",
			false,
			nil,
		},
		{
			"Get mapped lun count of host fail",
			"{\"data\":{},\"error\":{\"code\":0,\"description\":\"0\"}}",
			true,
			errors.New("get mapped lun count of host fail"),
		},
		{
			"Get mapped lun count of host error",
			"{\"data\":{},\"error\":{\"code\":1077949061,\"description\":\"0\"}}",
			true,
			errors.New("get mapped lun count of host error"),
		},
	}

	g := gomonkey.ApplyFunc(pkgUtils.GetPasswordFromBackendID,
		func(ctx context.Context, backendID string) (string, error) {
			return "mock", nil
		})
	defer g.Reset()

	for _, s := range cases {
		d := gomonkey.ApplyMethod(reflect.TypeOf(testClient.Client), "Do",
			func(*http.Client, *http.Request) (*http.Response, error) {
				r := ioutil.NopCloser(bytes.NewReader([]byte(s.responseBody)))
				return &http.Response{
					StatusCode: 200,
					Body:       r,
				}, nil
			})

		_, err := testClient.GetLunCountOfHost(context.TODO(), "")
		assert.Equal(t, s.wantErr, err != nil, "%s, err:%v", s.name, err)

		d.Reset()
	}
}

func TestGetLunCountOfMapping(t *testing.T) {
	var cases = []struct {
		name         string
		responseBody string
		wantErr      bool
		err          error
	}{
		{
			"Normal",
			"{\"data\":{\"COUNT\":\"10\"},\"error\":{\"code\":0,\"description\":\"0\"}}",
			false,
			nil,
		},
		{
			"Get mapped lun count of mapping fail",
			"{\"data\":{},\"error\":{\"code\":0,\"description\":\"0\"}}",
			true,
			errors.New("Get mapped lun count of mapping fail"),
		},
		{
			"Get mapped lun count of mapping error",
			"{\"data\":{},\"error\":{\"code\":1077949061,\"description\":\"0\"}}",
			true,
			errors.New("Get mapped lun count of mapping error"),
		},
	}

	g := gomonkey.ApplyFunc(pkgUtils.GetPasswordFromBackendID,
		func(ctx context.Context, backendID string) (string, error) {
			return "mock", nil
		})
	defer g.Reset()

	for _, s := range cases {
		d := gomonkey.ApplyMethod(reflect.TypeOf(testClient.Client), "Do",
			func(*http.Client, *http.Request) (*http.Response, error) {
				r := ioutil.NopCloser(bytes.NewReader([]byte(s.responseBody)))
				return &http.Response{
					StatusCode: 200,
					Body:       r,
				}, nil
			})

		_, err := testClient.GetLunCountOfMapping(context.TODO(), "")
		assert.Equal(t, s.wantErr, err != nil, "%s, err:%v", s.name, err)

		d.Reset()
	}
}

func TestMain(m *testing.M) {
	log.MockInitLogging(logName)
	defer log.MockStopLogging(logName)

	getGlobalConfig := gostub.StubFunc(&app.GetGlobalConfig, cfg.MockCompletedConfig())
	defer getGlobalConfig.Reset()

	testClient, _ = NewClient(context.Background(), &NewClientConfig{
		Urls:            []string{"https://127.0.0.1:8088"},
		User:            "dev-account",
		SecretName:      "mock-sec-name",
		SecretNamespace: "mock-sec-namespace",
		ParallelNum:     "",
		BackendID:       "mock-backend-id",
		VstoreName:      "dev-vStore",
	})

	m.Run()
}
