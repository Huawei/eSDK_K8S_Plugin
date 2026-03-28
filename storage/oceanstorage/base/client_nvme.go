/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2022-2025. All rights reserved.
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

// Package base provide base operations for oceanstor base storage
package base

import (
	"context"
	"fmt"
	"strings"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/api"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/iputils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	hostType        = 21
	roceNVMeTypeNum = 57870
	roceNVMeKind    = "roce"
	tcpNVMeKind     = "tcp"
)

// NVMe defines interfaces for NVMe operations
type NVMe interface {
	// GetInitiatorByID used for get NVMe initiator by id
	GetInitiatorByID(ctx context.Context, initiator string) (map[string]interface{}, error)
	// AddInitiator used for add NVMe initiator
	AddInitiator(ctx context.Context, initiator string) (map[string]interface{}, error)
	// AddInitiatorToHost used for add NVMe initiator to host
	AddInitiatorToHost(ctx context.Context, initiator, hostID string) error
	// GetPortalByIP used for get NVMe portal by ip
	GetPortalByIP(ctx context.Context, tgtPortal string) (map[string]interface{}, error)
}

// RoCEClient defines client implements the NVMe interface
type RoCEClient struct {
	RestClientInterface
}

// GetInitiatorByID used for get initiator by id
func (cli *RoCEClient) GetInitiatorByID(ctx context.Context, initiator string) (map[string]interface{}, error) {
	url := fmt.Sprintf(api.GetRoCENVMeInitiatorByID, initiator)
	return getInitiatorByID(ctx, cli.RestClientInterface, initiator, url)
}

// AddInitiator used for add initiator
func (cli *RoCEClient) AddInitiator(ctx context.Context, initiator string) (map[string]interface{}, error) {
	return addInitiator(ctx, cli.RestClientInterface, initiator, api.CreateRoCENVMeInitiator)
}

// AddInitiatorToHost used for add initiator to host
func (cli *RoCEClient) AddInitiatorToHost(ctx context.Context, initiator, hostID string) error {
	data := map[string]interface{}{
		"ID":               hostID,
		"ASSOCIATEOBJTYPE": roceNVMeTypeNum,
		"ASSOCIATEOBJID":   initiator,
	}
	resp, err := cli.Put(ctx, api.AddRoCENVMeInitiatorToHost, data)
	if err != nil {
		return err
	}

	code, msg, err := utils.FormatRespErr(resp.Error)
	if err != nil {
		return err
	}

	if code != storage.SuccessCode {
		return fmt.Errorf("add roce-nvme initiator %s to host failed, "+
			"error code: %d, error msg: %s", initiator, code, msg)
	}

	return nil
}

// GetPortalByIP used for get portal by ip
func (cli *RoCEClient) GetPortalByIP(ctx context.Context, tgtPortal string) (map[string]interface{}, error) {
	return getPortalByIP(ctx, cli.RestClientInterface, tgtPortal)
}

// TcpClient defines client implements the NVMe interface
type TcpClient struct {
	RestClientInterface
}

// GetInitiatorByID used for get initiator by id
func (cli *TcpClient) GetInitiatorByID(ctx context.Context, initiator string) (map[string]interface{}, error) {
	url := fmt.Sprintf(api.GetTcpNVMeInitiatorByID, initiator)
	return getInitiatorByID(ctx, cli.RestClientInterface, initiator, url)
}

// AddInitiator used for add initiator
func (cli *TcpClient) AddInitiator(ctx context.Context, initiator string) (map[string]interface{}, error) {
	return addInitiator(ctx, cli.RestClientInterface, initiator, api.CreateTcpNVMeInitiator)
}

// AddInitiatorToHost used for add initiator to host
func (cli *TcpClient) AddInitiatorToHost(ctx context.Context, initiator, hostID string) error {
	data := map[string]interface{}{
		"ID":         initiator,
		"PARENTID":   hostID,
		"PARENTTYPE": hostType,
	}
	resp, err := cli.Put(ctx, api.AddTcpNVMeInitiatorToHost, data)
	if err != nil {
		return err
	}

	code, msg, err := utils.FormatRespErr(resp.Error)
	if err != nil {
		return err
	}

	if code != storage.SuccessCode {
		return fmt.Errorf("add tcp-nvme initiator %s to host failed, "+
			"error code: %d, error msg: %s", initiator, code, msg)
	}

	return nil
}

// GetPortalByIP used for get portal by ip
func (cli *TcpClient) GetPortalByIP(ctx context.Context, tgtPortal string) (map[string]interface{}, error) {
	return getPortalByIP(ctx, cli.RestClientInterface, tgtPortal)
}

func getInitiatorByID(ctx context.Context, cli RestClientInterface,
	initiator, url string) (map[string]interface{}, error) {
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code, msg, err := utils.FormatRespErr(resp.Error)
	if err != nil {
		return nil, err
	}

	if code == storage.ObjectNotExist {
		return map[string]interface{}{}, nil
	}

	if code != storage.SuccessCode {
		return nil, fmt.Errorf("get nvme initiator %s failed, error code: %d, error msg: %s", initiator, code, msg)
	}

	respData, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("convert initiator data to map failed, data: %v", resp.Data)
	}
	return respData, nil
}

func addInitiator(ctx context.Context, cli RestClientInterface, initiator, url string) (map[string]interface{}, error) {
	data := map[string]interface{}{"ID": initiator}
	resp, err := cli.Post(ctx, url, data)
	if err != nil {
		return nil, err
	}

	code, msg, err := utils.FormatRespErr(resp.Error)
	if err != nil {
		return nil, err
	}

	if code != storage.SuccessCode {
		return nil, fmt.Errorf("add nvme initiator %s failed, error code: %d, error msg: %s", initiator, code, msg)
	}

	respData, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("convert initiator data to map failed, data: %v", resp.Data)
	}
	return respData, nil
}

func getPortalByIP(ctx context.Context, cli RestClientInterface, tgtPortal string) (map[string]interface{}, error) {
	url, err := generateGetPortalUrlByIP(tgtPortal)
	if err != nil {
		return nil, err
	}

	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code, msg, err := utils.FormatRespErr(resp.Error)
	if err != nil {
		return nil, err
	}

	if code != storage.SuccessCode {
		return nil, fmt.Errorf("get logical ports failed, error code: %d, error msg: %s", code, msg)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("Portal %s does not exist", tgtPortal)
		return nil, nil
	}

	respData, ok := resp.Data.([]interface{})
	if !ok {
		return nil, fmt.Errorf("convert portals data to arr failed, data: %v", resp.Data)
	}
	if len(respData) == 0 {
		log.AddContext(ctx).Infof("Portal %s does not exist", tgtPortal)
		return nil, nil
	}

	portal, ok := respData[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("convert portal to string failed, data: %v", respData[0])
	}
	return portal, nil
}

func generateGetPortalUrlByIP(tgtPortal string) (string, error) {
	wrapper := iputils.NewIPDomainWrapper(tgtPortal)
	if wrapper == nil {
		return "", fmt.Errorf("tgtPortal %s is invalid", tgtPortal)
	}

	var url string
	if wrapper.IsIPv4() {
		url = fmt.Sprintf(api.GetIPV4Lif, tgtPortal)
	} else {
		url = fmt.Sprintf(api.GetIPV6Lif, strings.ReplaceAll(tgtPortal, ":", "\\:"))
	}

	return url, nil
}
