/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2024-2024. All rights reserved.
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

// Package client provides oceandisk storage client
package client

import (
	"context"
	"fmt"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/base"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

// OceandiskClientInterface defines interfaces for base client operations
type OceandiskClientInterface interface {
	base.RestClientInterface
	base.ApplicationType
	base.FC
	base.Host
	base.Iscsi
	base.Mapping
	base.Qos
	base.RoCE
	base.System

	Namespace
	NamespaceGroup

	GetBackendID() string
	GetDeviceSN() string
	GetStorageVersion() string
}

// OceandiskClient implements OceandiskClientInterface
type OceandiskClient struct {
	*base.ApplicationTypeClient
	*base.FCClient
	*base.HostClient
	*base.IscsiClient
	*base.MappingClient
	*base.QosClient
	*base.RoCEClient
	*base.SystemClient

	*RestClient
}

// NewClientConfig stores the information needed to create a new oceandisk client
type NewClientConfig struct {
	Urls            []string
	User            string
	SecretName      string
	SecretNamespace string
	ParallelNum     string
	BackendID       string
	UseCert         bool
	CertSecretMeta  string
	Storage         string
	Name            string
}

// NewClient inits a new client of oceandisk client
func NewClient(ctx context.Context, param *NewClientConfig) (*OceandiskClient, error) {
	restClient, err := NewRestClient(ctx, param)
	if err != nil {
		return nil, err
	}

	return &OceandiskClient{
		ApplicationTypeClient: &base.ApplicationTypeClient{RestClientInterface: restClient},
		FCClient:              &base.FCClient{RestClientInterface: restClient},
		HostClient:            &base.HostClient{RestClientInterface: restClient},
		IscsiClient:           &base.IscsiClient{RestClientInterface: restClient},
		MappingClient:         &base.MappingClient{RestClientInterface: restClient},
		QosClient:             &base.QosClient{RestClientInterface: restClient},
		RoCEClient:            &base.RoCEClient{RestClientInterface: restClient},
		SystemClient:          &base.SystemClient{RestClientInterface: restClient},
		RestClient:            restClient,
	}, nil
}

// ValidateLogin validates the login info
func (cli *OceandiskClient) ValidateLogin(ctx context.Context) error {
	var resp base.Response
	var err error

	password, err := utils.GetPasswordFromSecret(ctx, cli.SecretName, cli.SecretNamespace)
	if err != nil {
		return err
	}

	data := map[string]interface{}{
		"username": cli.User,
		"password": password,
		"scope":    base.LocalUserType,
	}

	cli.DeviceId = ""
	cli.Token = ""
	for i, url := range cli.Urls {
		cli.Url = url + "/deviceManager/rest"
		log.AddContext(ctx).Infof("try to login %s", cli.Url)
		resp, err = cli.BaseCall(ctx, "POST", "/xx/sessions", data)
		if err == nil {
			/* Sort the login Url to the last slot of san addresses, so that
			   if this connection error, next time will try other Url first. */
			cli.Urls[i], cli.Urls[len(cli.Urls)-1] = cli.Urls[len(cli.Urls)-1], cli.Urls[i]
			break
		} else if err.Error() != base.Unconnected {
			log.AddContext(ctx).Errorf("login %s error", cli.Url)
			break
		}

		log.AddContext(ctx).Warningf("login %s error due to connection failure, gonna try another Url", cli.Url)
	}

	if err != nil {
		return err
	}

	code, msg, err := utils.FormatRespErr(resp.Error)
	if err != nil {
		return fmt.Errorf("format login response data error: %v", err)
	}

	if code != 0 {
		return fmt.Errorf("validate login %s failed, error code: %d, error msg: %s", cli.Url, code, msg)
	}

	cli.setDeviceIdFromRespData(ctx, resp)

	log.AddContext(ctx).Infof("validate login %s success", cli.Url)
	return nil
}

func (cli *OceandiskClient) setDeviceIdFromRespData(ctx context.Context, resp base.Response) {
	respData, ok := resp.Data.(map[string]interface{})
	if !ok {
		log.AddContext(ctx).Warningf("convert response data to map[string]interface{} failed, data type: [%T]",
			resp.Data)
	}

	cli.DeviceId, ok = utils.GetValue[string](respData, "deviceid")
	if !ok {
		log.AddContext(ctx).Warningf("can not convert deviceId type %T to string", respData["deviceid"])
	}

	cli.Token, ok = utils.GetValue[string](respData, "iBaseToken")
	if !ok {
		log.AddContext(ctx).Warningf("can not convert iBaseToken type %T to string", respData["iBaseToken"])
	}
}
