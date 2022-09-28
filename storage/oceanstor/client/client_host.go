/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2022-2022. All rights reserved.
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
	"errors"
	"fmt"

	"huawei-csi-driver/utils/log"
)

const (
	hostAlreadyInHostGroup int64 = 1077937501
	hostNotInHostGroup     int64 = 1073745412
	objectNameAlreadyExist int64 = 1077948993
	hostNotExist           int64 = 1077937498
	hostGroupNotExist      int64 = 1077937500
)

type Host interface {
	// QueryAssociateHostGroup used for query associate host group
	QueryAssociateHostGroup(ctx context.Context, objType int, objID string) ([]interface{}, error)
	// GetHostByName used to get host by name
	GetHostByName(ctx context.Context, name string) (map[string]interface{}, error)
	// GetHostGroupByName used for get host group by name
	GetHostGroupByName(ctx context.Context, name string) (map[string]interface{}, error)
	// DeleteHost used for delete host by id
	DeleteHost(ctx context.Context, id string) error
	// DeleteHostGroup used for delete host group
	DeleteHostGroup(ctx context.Context, id string) error
	// CreateHost used for create  host
	CreateHost(ctx context.Context, name string) (map[string]interface{}, error)
	// UpdateHost used for update host
	UpdateHost(ctx context.Context, id string, alua map[string]interface{}) error
	// AddHostToGroup used for add host to group
	AddHostToGroup(ctx context.Context, hostID, hostGroupID string) error
	// CreateHostGroup used for create host group
	CreateHostGroup(ctx context.Context, name string) (map[string]interface{}, error)
	// RemoveHostFromGroup used for remove host from group
	RemoveHostFromGroup(ctx context.Context, hostID, hostGroupID string) error
}

// AddHostToGroup used for add host to group
func (cli *BaseClient) AddHostToGroup(ctx context.Context, hostID, hostGroupID string) error {
	data := map[string]interface{}{
		"ID":               hostGroupID,
		"ASSOCIATEOBJTYPE": 21,
		"ASSOCIATEOBJID":   hostID,
	}
	resp, err := cli.Post(ctx, "/hostgroup/associate", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == hostAlreadyInHostGroup {
		log.AddContext(ctx).Infof("Host %s is already in hostgroup %s", hostID, hostGroupID)
		return nil
	}
	if code != 0 {
		msg := fmt.Sprintf("Add host %s to hostgroup %s error: %d", hostID, hostGroupID, code)
		return errors.New(msg)
	}

	return nil
}

// RemoveHostFromGroup used for remove host from group
func (cli *BaseClient) RemoveHostFromGroup(ctx context.Context, hostID, hostGroupID string) error {
	data := map[string]interface{}{
		"ID":               hostGroupID,
		"ASSOCIATEOBJTYPE": 21,
		"ASSOCIATEOBJID":   hostID,
	}
	resp, err := cli.Delete(ctx, "/host/associate", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == hostNotInHostGroup {
		log.AddContext(ctx).Infof("Host %s is not in hostgroup %s", hostID, hostGroupID)
		return nil
	}
	if code != 0 {
		msg := fmt.Sprintf("Remove host %s from hostgroup %s error: %d", hostID, hostGroupID, code)
		return errors.New(msg)
	}

	return nil
}

// QueryAssociateHostGroup used for query associate host group
func (cli *BaseClient) QueryAssociateHostGroup(ctx context.Context, objType int, objID string) ([]interface{}, error) {
	url := fmt.Sprintf("/hostgroup/associate?ASSOCIATEOBJTYPE=%d&ASSOCIATEOBJID=%s", objType, objID)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("associate query hostgroup by obj %s of type %d error: %d", objID, objType, code)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("obj %s of type %d doesn't associate to any hostgroup", objID, objType)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	return respData, nil
}

// CreateHost used for create  host
func (cli *BaseClient) CreateHost(ctx context.Context, name string) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"NAME":            name,
		"OPERATIONSYSTEM": 0,
	}

	resp, err := cli.Post(ctx, "/host", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code == objectNameAlreadyExist {
		log.AddContext(ctx).Infof("Host %s already exists", name)
		return cli.GetHostByName(ctx, name)
	}
	if code != 0 {
		msg := fmt.Sprintf("Create host %s error: %d", name, code)
		return nil, errors.New(msg)
	}

	host := resp.Data.(map[string]interface{})
	return host, nil
}

// UpdateHost used for update host
func (cli *BaseClient) UpdateHost(ctx context.Context, id string, alua map[string]interface{}) error {
	url := fmt.Sprintf("/host/%s", id)
	data := map[string]interface{}{}

	if accessMode, ok := alua["accessMode"]; ok {
		data["accessMode"] = accessMode
	}

	if hyperMetroPathOptimized, ok := alua["hyperMetroPathOptimized"]; ok {
		data["hyperMetroPathOptimized"] = hyperMetroPathOptimized
	}

	resp, err := cli.Put(ctx, url, data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("update host %s by %v error: %d", id, data, code)
	}

	return nil
}

// GetHostByName used to get host by name
func (cli *BaseClient) GetHostByName(ctx context.Context, name string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/host?filter=NAME::%s&range=[0-100]", name)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Get host %s error: %d", name, code)
		return nil, errors.New(msg)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("Host %s does not exist", name)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	if len(respData) <= 0 {
		log.AddContext(ctx).Infof("Host %s does not exist", name)
		return nil, nil
	}

	host := respData[0].(map[string]interface{})
	return host, nil
}

// DeleteHost used for delete host by id
func (cli *BaseClient) DeleteHost(ctx context.Context, id string) error {
	url := fmt.Sprintf("/host/%s", id)
	resp, err := cli.Delete(ctx, url, nil)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == hostNotExist {
		log.AddContext(ctx).Infof("Host %s does not exist", id)
		return nil
	}
	if code != 0 {
		msg := fmt.Sprintf("Delete host %s error: %d", id, code)
		return errors.New(msg)
	}

	return nil
}

// CreateHostGroup used for create host group
func (cli *BaseClient) CreateHostGroup(ctx context.Context, name string) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"NAME": name,
	}
	resp, err := cli.Post(ctx, "/hostgroup", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code == objectNameAlreadyExist {
		log.AddContext(ctx).Infof("Hostgroup %s already exists", name)
		return cli.GetHostGroupByName(ctx, name)
	}
	if code != 0 {
		msg := fmt.Sprintf("Create hostgroup %s error: %d", name, code)
		return nil, errors.New(msg)
	}

	hostGroup := resp.Data.(map[string]interface{})
	return hostGroup, nil
}

// GetHostGroupByName used for get host group by name
func (cli *BaseClient) GetHostGroupByName(ctx context.Context, name string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/hostgroup?filter=NAME::%s", name)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Get hostgroup %s error: %d", name, code)
		return nil, errors.New(msg)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("Hostgroup %s does not exist", name)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	if len(respData) <= 0 {
		log.AddContext(ctx).Infof("Hostgroup %s does not exist", name)
		return nil, nil
	}

	hostGroup := respData[0].(map[string]interface{})
	return hostGroup, nil
}

// DeleteHostGroup used for delete host group
func (cli *BaseClient) DeleteHostGroup(ctx context.Context, id string) error {
	url := fmt.Sprintf("/hostgroup/%s", id)
	resp, err := cli.Delete(ctx, url, nil)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == hostGroupNotExist {
		log.AddContext(ctx).Infof("Hostgroup %s does not exist while deleting", id)
		return nil
	}
	if code != 0 {
		msg := fmt.Sprintf("Delete hostgroup %s error: %d", id, code)
		return errors.New(msg)
	}

	return nil
}
