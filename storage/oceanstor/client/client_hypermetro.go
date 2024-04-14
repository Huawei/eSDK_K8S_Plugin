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

	pkgUtils "huawei-csi-driver/pkg/utils"
	"huawei-csi-driver/utils/log"
)

const (
	hyperMetroNotExist int64 = 1077674242
)

// HyperMetro defines interfaces for hyper metro operations
type HyperMetro interface {
	// GetHyperMetroDomainByName used for get hyper metro domain by name
	GetHyperMetroDomainByName(ctx context.Context, name string) (map[string]interface{}, error)
	// GetHyperMetroDomain used for get hyper metro domain by domain id
	GetHyperMetroDomain(ctx context.Context, domainID string) (map[string]interface{}, error)
	// GetFSHyperMetroDomain used for get file system hyper metro domain by domain name
	GetFSHyperMetroDomain(ctx context.Context, domainName string) (map[string]interface{}, error)
	// GetHyperMetroPair used for get hyper metro pair by pair id
	GetHyperMetroPair(ctx context.Context, pairID string) (map[string]interface{}, error)
	// GetHyperMetroPairByLocalObjID used for get hyper metro pair by local object id
	GetHyperMetroPairByLocalObjID(ctx context.Context, objID string) (map[string]interface{}, error)
	// DeleteHyperMetroPair used for delete hyper metro pair by pair id
	DeleteHyperMetroPair(ctx context.Context, pairID string, onlineDelete bool) error
	// CreateHyperMetroPair used for create hyper metro pair
	CreateHyperMetroPair(ctx context.Context, data map[string]interface{}) (map[string]interface{}, error)
	// SyncHyperMetroPair used for synchronize hyper metro pair
	SyncHyperMetroPair(ctx context.Context, pairID string) error
	// StopHyperMetroPair used for stop hyper metro pair
	StopHyperMetroPair(ctx context.Context, pairID string) error
}

// GetHyperMetroDomainByName used for get hyper metro domain by name
func (cli *BaseClient) GetHyperMetroDomainByName(ctx context.Context, name string) (map[string]interface{}, error) {
	resp, err := cli.Get(ctx, "/HyperMetroDomain?range=[0-100]", nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get HyperMetroDomain of name %s error: %d", name, code)
	}
	if resp.Data == nil {
		log.AddContext(ctx).Infof("No HyperMetroDomain %s exist", name)
		return nil, nil
	}

	respData, ok := resp.Data.([]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert respData to arr failed, data: %v", resp.Data)
	}
	for _, i := range respData {
		domain, ok := i.(map[string]interface{})
		if !ok {
			log.AddContext(ctx).Warningf("convert domain to map failed, data: %v", i)
			continue
		}
		if domain["NAME"].(string) == name {
			return domain, nil
		}
	}

	return nil, nil
}

// GetHyperMetroDomain used for get hyper metro domain by domain id
func (cli *BaseClient) GetHyperMetroDomain(ctx context.Context, domainID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/HyperMetroDomain/%s", domainID)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get HyperMetroDomain %s error: %d", domainID, code)
	}

	respData, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert respData to arr failed, data: %v", resp.Data)
	}
	return respData, nil
}

// GetFSHyperMetroDomain used for get file system hyper metro domain by domain name
func (cli *BaseClient) GetFSHyperMetroDomain(ctx context.Context, domainName string) (map[string]interface{}, error) {
	url := "/FsHyperMetroDomain?RUNNINGSTATUS=0"
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get filesystem hyperMetro domain %s error: %d", domainName, code)
	}
	if resp.Data == nil {
		log.AddContext(ctx).Infof("hyperMetro domain %s does not exist", domainName)
		return nil, nil
	}

	respData, ok := resp.Data.([]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert respData to arr failed, data: %v", resp.Data)
	}
	for _, d := range respData {
		domain, ok := d.(map[string]interface{})
		if !ok {
			log.AddContext(ctx).Warningf("convert domain to map failed, data: %v", d)
			continue
		}
		if domain["NAME"].(string) == domainName {
			return domain, nil
		}
	}

	log.AddContext(ctx).Infof("FileSystem hyperMetro domain %s does not exist or is not normal", domainName)
	return nil, nil
}

// GetHyperMetroPair used for get hyper metro pair by pair id
func (cli *BaseClient) GetHyperMetroPair(ctx context.Context, pairID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/HyperMetroPair?filter=ID::%s", pairID)

	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get hypermetro %s error: %d", pairID, code)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("Hypermetro %s does not exist", pairID)
		return nil, nil
	}

	respData, ok := resp.Data.([]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert respData to arr failed, data: %v", resp.Data)
	}
	if len(respData) <= 0 {
		log.AddContext(ctx).Infof("Hypermetro %s does not exist", pairID)
		return nil, nil
	}

	pair, ok := respData[0].(map[string]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert pair to map failed, data: %v", respData[0])
	}
	return pair, nil
}

// GetHyperMetroPairByLocalObjID used for get hyper metro pair by local object id
func (cli *BaseClient) GetHyperMetroPairByLocalObjID(ctx context.Context, objID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/HyperMetroPair?filter=LOCALOBJID::%s", objID)

	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get hypermetro of local obj %s error: %d", objID, code)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("Hypermetro of local obj %s does not exist", objID)
		return nil, nil
	}

	respData, ok := resp.Data.([]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert respData to arr failed, data: %v", resp.Data)
	}
	for _, i := range respData {
		pair, ok := i.(map[string]interface{})
		if !ok {
			log.AddContext(ctx).Warningf("convert pair to map failed, data: %v", i)
			continue
		}
		if pair["LOCALOBJID"] == objID {
			return pair, nil
		}
	}

	log.AddContext(ctx).Infof("Hypermetro of local obj %s does not exist", objID)
	return nil, nil
}

// CreateHyperMetroPair used for create hyper metro pair
func (cli *BaseClient) CreateHyperMetroPair(ctx context.Context, data map[string]interface{}) (
	map[string]interface{}, error) {

	resp, err := cli.Post(ctx, "/HyperMetroPair", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Create hypermetro %v error: %d", data, code)
	}

	respData, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert respData to arr failed, data: %v", resp.Data)
	}
	return respData, nil
}

// SyncHyperMetroPair used for synchronize hyper metro pair
func (cli *BaseClient) SyncHyperMetroPair(ctx context.Context, pairID string) error {
	data := map[string]interface{}{
		"ID": pairID,
	}

	resp, err := cli.Put(ctx, "/HyperMetroPair/synchronize_hcpair", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Sync hypermetro %s error: %d", pairID, code)
	}

	return nil
}

// StopHyperMetroPair used for stop hyper metro pair
func (cli *BaseClient) StopHyperMetroPair(ctx context.Context, pairID string) error {
	data := map[string]interface{}{
		"ID": pairID,
	}

	resp, err := cli.Put(ctx, "/HyperMetroPair/disable_hcpair", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Stop hypermetro %s error: %d", pairID, code)
	}

	return nil
}

// DeleteHyperMetroPair used for delete hyper metro pair by pair id
func (cli *BaseClient) DeleteHyperMetroPair(ctx context.Context, pairID string, onlineDelete bool) error {
	url := fmt.Sprintf("/HyperMetroPair/%s", pairID)
	if !onlineDelete {
		url += "?isOnlineDeleting=0"
	}

	resp, err := cli.Delete(ctx, url, nil)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == hyperMetroNotExist {
		log.AddContext(ctx).Infof("Hypermetro %s to Delete does not exist", pairID)
		return nil
	} else if code != 0 {
		return fmt.Errorf("Delete hypermetro %s error: %d", pairID, code)
	}

	return nil
}
