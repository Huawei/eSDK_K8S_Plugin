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
	"fmt"
	URL "net/url"
	"strings"

	pkgUtils "huawei-csi-driver/pkg/utils"
	"huawei-csi-driver/utils/log"
)

// RoCE defines interfaces for RoCE operations
type RoCE interface {
	// GetRoCEInitiator used for get RoCE initiator
	GetRoCEInitiator(ctx context.Context, initiator string) (map[string]interface{}, error)
	// GetRoCEInitiatorByID used for get RoCE initiator by id
	GetRoCEInitiatorByID(ctx context.Context, initiator string) (map[string]interface{}, error)
	// AddRoCEInitiator used for add RoCE initiator
	AddRoCEInitiator(ctx context.Context, initiator string) (map[string]interface{}, error)
	// AddRoCEInitiatorToHost used for add RoCE initiator to host
	AddRoCEInitiatorToHost(ctx context.Context, initiator, hostID string) error
	// GetRoCEPortalByIP used for get RoCE portal by ip
	GetRoCEPortalByIP(ctx context.Context, tgtPortal string) (map[string]interface{}, error)
}

// GetRoCEInitiator used for get RoCE initiator
func (cli *BaseClient) GetRoCEInitiator(ctx context.Context, initiator string) (map[string]interface{}, error) {
	id := URL.QueryEscape(strings.Replace(initiator, ":", "\\:", -1))
	url := fmt.Sprintf("/NVMe_over_RoCE_initiator?filter=ID::%s", id)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get RoCE initiator %s error: %d", initiator, code)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("RoCE initiator %s does not exist", initiator)
		return nil, nil
	}

	respData, ok := resp.Data.([]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert respData to arr failed, data: %v", resp.Data)
	}
	if len(respData) == 0 {
		return nil, nil
	}
	ini, ok := respData[0].(map[string]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert ini to map failed, data: %v", respData[0])
	}
	return ini, nil
}

// GetRoCEInitiatorByID used for get RoCE initiator by id
func (cli *BaseClient) GetRoCEInitiatorByID(ctx context.Context, initiator string) (map[string]interface{}, error) {
	id := URL.QueryEscape(strings.Replace(initiator, ":", "\\:", -1))
	url := fmt.Sprintf("/NVMe_over_RoCE_initiator/%s", id)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get RoCE initiator by ID %s error: %d", initiator, code)
	}

	respData, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert respData to map failed, data: %v", resp.Data)
	}
	return respData, nil
}

// AddRoCEInitiator used for add RoCE initiator
func (cli *BaseClient) AddRoCEInitiator(ctx context.Context, initiator string) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"ID": initiator,
	}

	resp, err := cli.Post(ctx, "/NVMe_over_RoCE_initiator", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code == objectIdNotUnique {
		log.AddContext(ctx).Infof("RoCE initiator %s already exists", initiator)
		return cli.GetRoCEInitiatorByID(ctx, initiator)
	}
	if code != 0 {
		return nil, fmt.Errorf("add RoCE initiator %s error: %d", initiator, code)
	}

	respData, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert respData to map failed, data: %v", resp.Data)
	}
	return respData, nil
}

// AddRoCEInitiatorToHost used for add RoCE initiator to host
func (cli *BaseClient) AddRoCEInitiatorToHost(ctx context.Context, initiator, hostID string) error {
	data := map[string]interface{}{
		"ID":               hostID,
		"ASSOCIATEOBJTYPE": 57870,
		"ASSOCIATEOBJID":   initiator,
	}
	resp, err := cli.Put(ctx, "/host/create_associate", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("add RoCE initiator %s to host %s error: %d", initiator, hostID, code)
	}

	return nil
}

// GetRoCEPortalByIP used for get RoCE portal by ip
func (cli *BaseClient) GetRoCEPortalByIP(ctx context.Context, tgtPortal string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/lif?filter=IPV4ADDR::%s", tgtPortal)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get RoCE by IP %s error: %d", tgtPortal, code)
	}
	if resp.Data == nil {
		log.AddContext(ctx).Infof("RoCE portal %s does not exist", tgtPortal)
		return nil, nil
	}

	respData, ok := resp.Data.([]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert respData to arr failed, data: %v", resp.Data)
	}
	if len(respData) == 0 {
		log.AddContext(ctx).Infof("RoCE portal %s does not exist", tgtPortal)
		return nil, nil
	}

	portal, ok := respData[0].(map[string]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert portal to string failed, data: %v", respData[0])
	}
	return portal, nil
}
