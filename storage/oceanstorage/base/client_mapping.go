/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2022-2024. All rights reserved.
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

// Package base provide base operations for oceanstor and oceandisk storage
package base

import (
	"context"
	"errors"
	"fmt"

	pkgUtils "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	hostGroupNotInMapping     int64 = 1073804552
	lunGroupNotInMapping      int64 = 1073804554
	hostGroupAlreadyInMapping int64 = 1073804556
	lunGroupAlreadyInMapping  int64 = 1073804560
	mappingNotExist           int64 = 1077951819
)

// Mapping defines interfaces for mapping operations
type Mapping interface {
	// DeleteMapping used for delete mapping
	DeleteMapping(ctx context.Context, id string) error
	// GetMappingByName used for get mapping by name
	GetMappingByName(ctx context.Context, name string) (map[string]interface{}, error)
	// RemoveGroupFromMapping used for remove group from mapping
	RemoveGroupFromMapping(ctx context.Context, groupType int, groupID, mappingID string) error
	// CreateMapping used for create mapping
	CreateMapping(ctx context.Context, name string) (map[string]interface{}, error)
	// AddGroupToMapping used for add group to mapping
	AddGroupToMapping(ctx context.Context, groupType int, groupID, mappingID string) error
}

// MappingClient defines client implements the Mapping interface
type MappingClient struct {
	RestClientInterface
}

// CreateMapping used for create mapping
func (cli *MappingClient) CreateMapping(ctx context.Context, name string) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"NAME": name,
	}
	resp, err := cli.Post(ctx, "/mappingview", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code == objectNameAlreadyExist {
		log.AddContext(ctx).Infof("Mapping %s already exists", name)
		return cli.GetMappingByName(ctx, name)
	}
	if code != 0 {
		msg := fmt.Sprintf("Create mapping %s error: %d", name, code)
		return nil, errors.New(msg)
	}

	mapping, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert mapping to map failed, data: %v", resp.Data)
	}
	return mapping, nil
}

// GetMappingByName used for get mapping by name
func (cli *MappingClient) GetMappingByName(ctx context.Context, name string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/mappingview?filter=NAME::%s", name)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Get mapping %s error: %d", name, code)
		return nil, errors.New(msg)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("Mapping %s does not exist", name)
		return nil, nil
	}

	respData, ok := resp.Data.([]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert respData to arr failed, data: %v", resp.Data)
	}
	if len(respData) <= 0 {
		log.AddContext(ctx).Infof("Mapping %s does not exist", name)
		return nil, nil
	}

	mapping, ok := respData[0].(map[string]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert mapping to map failed, data: %v", respData[0])
	}
	return mapping, nil
}

// DeleteMapping used for delete mapping
func (cli *MappingClient) DeleteMapping(ctx context.Context, id string) error {
	url := fmt.Sprintf("/mappingview/%s", id)
	resp, err := cli.Delete(ctx, url, nil)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == mappingNotExist {
		log.AddContext(ctx).Infof("Mapping %s does not exist while deleting", id)
		return nil
	}
	if code != 0 {
		msg := fmt.Sprintf("Delete mapping %s error: %d", id, code)
		return errors.New(msg)
	}

	return nil
}

// AddGroupToMapping used for add group to mapping
func (cli *MappingClient) AddGroupToMapping(ctx context.Context, groupType int, groupID, mappingID string) error {
	data := map[string]interface{}{
		"ID":               mappingID,
		"ASSOCIATEOBJTYPE": groupType,
		"ASSOCIATEOBJID":   groupID,
	}
	resp, err := cli.Put(ctx, "/mappingview/create_associate", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == hostGroupAlreadyInMapping || code == lunGroupAlreadyInMapping {
		log.AddContext(ctx).Infof("Group %s of type %d is already in mapping %s",
			groupID, groupType, mappingID)
		return nil
	}
	if code != 0 {
		msg := fmt.Sprintf("Add group %s of type %d to mapping %s error: %d", groupID, groupType, mappingID, code)
		return errors.New(msg)
	}

	return nil
}

// RemoveGroupFromMapping used for remove group from mapping
func (cli *MappingClient) RemoveGroupFromMapping(ctx context.Context, groupType int, groupID, mappingID string) error {
	data := map[string]interface{}{
		"ID":               mappingID,
		"ASSOCIATEOBJTYPE": groupType,
		"ASSOCIATEOBJID":   groupID,
	}
	resp, err := cli.Put(ctx, "/mappingview/remove_associate", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == hostGroupNotInMapping ||
		code == lunGroupNotInMapping {
		log.AddContext(ctx).Infof("Group %s of type %d is not in mapping %s",
			groupID, groupType, mappingID)
		return nil
	}
	if code != 0 {
		msg := fmt.Sprintf("Remove group %s of type %d from mapping %s error: %d", groupID, groupType, mappingID, code)
		return errors.New(msg)
	}

	return nil
}
