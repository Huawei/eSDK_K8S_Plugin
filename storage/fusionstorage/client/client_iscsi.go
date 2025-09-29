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

// Package client provides fusion storage client
package client

import (
	"context"
	"fmt"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	queryPortInfoPath          = "/dsware/service/iscsi/queryPortInfo"
	createPortPath             = "/dsware/service/iscsi/createPort"
	queryIscsiPortalPath       = "/dsware/service/cluster/dswareclient/queryIscsiPortal"
	queryIscsiHostRelationPath = "/dsware/service/iscsi/queryIscsiHostRelation"
	queryIscsiLinksPath        = "/dsware/service/iscsi/queryIscsiLinks"

	initiatorAlreadyExist int64 = 50155102
	initiatorAddedToHost  int64 = 50157021
	initiatorNotExist     int64 = 50155103

	iscsiFlag = 0
)

// Iscsi is the interface for iSCSI
type Iscsi interface {
	GetInitiatorByName(ctx context.Context, name string) (map[string]interface{}, error)
	CreateInitiator(ctx context.Context, name string) error
	QueryIscsiPortal(ctx context.Context) ([]map[string]interface{}, error)
	IsSupportDynamicLinks(ctx context.Context, hostname string) (bool, error)
	QueryDynamicLinks(ctx context.Context, poolName, hostname string, amount int) ([]*IscsiLink, error)
}

// GetInitiatorByName used to get initiator by name
func (cli *RestClient) GetInitiatorByName(ctx context.Context, name string) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"portName": name,
	}

	resp, err := cli.post(ctx, queryPortInfoPath, data)
	if err != nil {
		return nil, err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		if !cli.checkErrorCode(ctx, resp, initiatorNotExist) {
			return nil, fmt.Errorf("get initiator %s error", name)
		}

		log.AddContext(ctx).Infof("Initiator %s does not exist", name)
		return nil, nil
	}

	portList, exist := resp["portList"].([]interface{})
	if !exist || len(portList) == 0 {
		log.AddContext(ctx).Infof("Initiator %s does not exist", name)
		return nil, nil
	}

	return portList[0].(map[string]interface{}), nil
}

// CreateInitiator used to create initiator by name
func (cli *RestClient) CreateInitiator(ctx context.Context, name string) error {
	data := map[string]interface{}{
		"portName": name,
	}

	resp, err := cli.post(ctx, createPortPath, data)
	if err != nil {
		return err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		if !cli.checkErrorCode(ctx, resp, initiatorAlreadyExist) {
			return fmt.Errorf("create initiator %s error", name)
		}
	}

	return nil
}

// QueryIscsiPortal used to query iscsi portal
func (cli *RestClient) QueryIscsiPortal(ctx context.Context) ([]map[string]interface{}, error) {
	data := make(map[string]interface{})
	resp, err := cli.post(ctx, queryIscsiPortalPath, data)
	if err != nil {
		return nil, err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		return nil, fmt.Errorf("query iscsi portal error: %d", result)
	}

	var nodeResultList []map[string]interface{}

	respData, exist := resp["nodeResultList"].([]interface{})
	if exist {
		for _, i := range respData {
			nodeResultList = append(nodeResultList, i.(map[string]interface{}))
		}
	}

	return nodeResultList, nil
}

// HostIscsiInfo defines the fields of IsSupportDynamicLinks request item
type HostIscsiInfo struct {
	Flag int    `json:"flag"`
	Key  string `json:"key"`
}

// IsSupportDynamicLinksResponse defines the fields of IsSupportDynamicLinks response
type IsSupportDynamicLinksResponse struct {
	*SanBaseResponse
	NewIscsi bool `json:"newIscsi"`
}

// IsSupportDynamicLinks return whether the storage support querying the iscsi links dynamically
func (cli *RestClient) IsSupportDynamicLinks(ctx context.Context, hostname string) (bool, error) {
	req := []HostIscsiInfo{{Flag: iscsiFlag, Key: hostname}}
	resp, err := gracefulSanPost[*IsSupportDynamicLinksResponse](ctx, cli, queryIscsiHostRelationPath, req)
	if err != nil {
		return false, fmt.Errorf("call IsSupportDynamicLinks failed, err: %w", err)
	}

	if resp.IsErrorCodeSet() {
		return false, fmt.Errorf("failed to get iscsi host relation by path, err: %s", resp.Error())
	}

	return resp.NewIscsi, nil
}

// QueryDynamicLinksRequest defines the fields of QueryDynamicLinks request
type QueryDynamicLinksRequest struct {
	Amount             int      `json:"amount"`
	IscsiServiceIpType int      `json:"iscsiServiceIpType"`
	PoolList           []string `json:"poolList"`
	HostKey            string   `json:"hostKey"`
}

// QueryDynamicLinksResponse defines the fields of QueryDynamicLinks response
type QueryDynamicLinksResponse struct {
	*SanBaseResponse
	IscsiLinks []*IscsiLink `json:"iscsiLinks"`
}

// IscsiLink defines the fields of QueryDynamicLinks response item
type IscsiLink struct {
	IP            string `json:"ip"`
	IscsiLinksNum int    `json:"iscsiLinksNum"`
	TargetName    string `json:"targetName"`
	IscsiPortal   string `json:"iscsiPortal"`
}

// QueryDynamicLinks return whether the storage support querying the iscsi links dynamically
func (cli *RestClient) QueryDynamicLinks(ctx context.Context,
	poolName, hostname string, amount int) ([]*IscsiLink, error) {
	req := QueryDynamicLinksRequest{
		Amount:             amount,
		IscsiServiceIpType: iscsiFlag,
		PoolList:           []string{poolName},
		HostKey:            hostname,
	}
	resp, err := gracefulSanPost[*QueryDynamicLinksResponse](ctx, cli, queryIscsiLinksPath, req)
	if err != nil {
		return nil, fmt.Errorf("call QueryDynamicLinks failed, err: %w", err)
	}

	if resp.IsErrorCodeSet() {
		return nil, fmt.Errorf("failed to query dynamic iscsi links, err: %s", resp.Error())
	}

	return resp.IscsiLinks, nil
}
