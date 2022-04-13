package client

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"huawei-csi-driver/utils/log"
)

var testClient *Client

const (
	logDir  = "/var/log/huawei/"
	logName = "clientTest.log"
)

func init() {
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

	testClient = NewClient([]string{"https://192.168.125.25:8088"},
		"dev-account", "dev-password", "dev-vStore", "")
}

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
				"\"error\":{\"code\":0,\"description\":\"0\"}}",
			false,
		},
		{
			"The user name or password is incorrect",
			"{\"data\":{},\"error\":{\"code\":1077949061,\"description\":\"The user name or password is incorrect.\"," +
				"\"errorParam\":\"\",\"suggestion\":\"Check the user name and password, and try again.\"}}",
			true,
		},
		{
			"The IP address has been locked",
			"{\"data\":{},\"error\":{\"code\":1077949071,\"description\":\"The IP address has been locked.\"," +
				"\"errorParam\":\"\",\"suggestion\":\"Contact the administrator.\"}}",
			true,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockClient := NewMockHTTPClient(ctrl)

	temp := testClient.client
	defer func() { testClient.client = temp }()
	testClient.client = mockClient

	for _, s := range cases {
		mockClient.EXPECT().Do(gomock.Any()).DoAndReturn(func(req *http.Request) (*http.Response, error) {
			r := ioutil.NopCloser(bytes.NewReader([]byte(s.ResponseBody)))
			return &http.Response{
				StatusCode: 200,
				Body:       r,
			}, nil
		}).AnyTimes()

		err := testClient.Login(context.TODO())
		assert.Equal(t, s.wantErr, err != nil, "%s, err:%v", s.Name, err)
	}
}
