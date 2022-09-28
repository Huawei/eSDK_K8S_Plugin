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
	"strings"

	"huawei-csi-driver/utils/log"
)

type Iscsi interface {
	// GetIscsiInitiator used for get iscsi initiator
	GetIscsiInitiator(ctx context.Context, initiator string) (map[string]interface{}, error)
	// GetIscsiInitiatorByID used for get iscsi initiator by id
	GetIscsiInitiatorByID(ctx context.Context, initiator string) (map[string]interface{}, error)
	// UpdateIscsiInitiator used for update iscsi initiator
	UpdateIscsiInitiator(ctx context.Context, initiator string, alua map[string]interface{}) error
	// AddIscsiInitiator used for add iscsi initiator
	AddIscsiInitiator(ctx context.Context, initiator string) (map[string]interface{}, error)
	// AddIscsiInitiatorToHost used for add iscsi initiator to host
	AddIscsiInitiatorToHost(ctx context.Context, initiator, hostID string) error
	// GetIscsiTgtPort used for get iscsi target port
	GetIscsiTgtPort(ctx context.Context) ([]interface{}, error)
	// GetISCSIHostLink used for get iscsi host link
	GetISCSIHostLink(ctx context.Context, hostID string) ([]interface{}, error)
}

// GetISCSIHostLink used for get iscsi host link
func (cli *BaseClient) GetISCSIHostLink(ctx context.Context, hostID string) ([]interface{}, error) {
	url := fmt.Sprintf("/host_link?INITIATOR_TYPE=222&PARENTID=%s", hostID)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get ISCSI host link of host %s error: %d", hostID, code)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("There is no ISCSI host link of host %s", hostID)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	return respData, nil
}

// GetIscsiInitiator used for get iscsi initiator
func (cli *BaseClient) GetIscsiInitiator(ctx context.Context, initiator string) (map[string]interface{}, error) {
	id := strings.Replace(initiator, ":", "\\:", -1)
	url := fmt.Sprintf("/iscsi_initiator?filter=ID::%s", id)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Get ISCSI initiator %s error: %d", initiator, code)
		return nil, errors.New(msg)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("ISCSI initiator %s does not exist", initiator)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	ini := respData[0].(map[string]interface{})
	return ini, nil
}

// GetIscsiInitiatorByID used for get iscsi initiator by id
func (cli *BaseClient) GetIscsiInitiatorByID(ctx context.Context, initiator string) (map[string]interface{}, error) {
	id := strings.Replace(initiator, ":", "\\:", -1)
	url := fmt.Sprintf("/iscsi_initiator/%s", id)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Get ISCSI initiator by ID %s error: %d", initiator, code)
		return nil, errors.New(msg)
	}

	respData := resp.Data.(map[string]interface{})
	return respData, nil
}

// AddIscsiInitiator used for add iscsi initiator
func (cli *BaseClient) AddIscsiInitiator(ctx context.Context, initiator string) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"ID": initiator,
	}

	resp, err := cli.Post(ctx, "/iscsi_initiator", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code == objectIdNotUnique {
		log.AddContext(ctx).Infof("Iscsi initiator %s already exists", initiator)
		return cli.GetIscsiInitiatorByID(ctx, initiator)
	}
	if code != 0 {
		msg := fmt.Sprintf("Add iscsi initiator %s error: %d", initiator, code)
		return nil, errors.New(msg)
	}

	respData := resp.Data.(map[string]interface{})
	return respData, nil
}

// UpdateIscsiInitiator used for update iscsi initiator
func (cli *BaseClient) UpdateIscsiInitiator(ctx context.Context, initiator string, alua map[string]interface{}) error {
	url := fmt.Sprintf("/iscsi_initiator/%s", initiator)
	data := map[string]interface{}{}

	if multiPathType, ok := alua["MULTIPATHTYPE"]; ok {
		data["MULTIPATHTYPE"] = multiPathType
	}

	if failoverMode, ok := alua["FAILOVERMODE"]; ok {
		data["FAILOVERMODE"] = failoverMode
	}

	if specialModeType, ok := alua["SPECIALMODETYPE"]; ok {
		data["SPECIALMODETYPE"] = specialModeType
	}

	if pathType, ok := alua["PATHTYPE"]; ok {
		data["PATHTYPE"] = pathType
	}

	resp, err := cli.Put(ctx, url, data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("update iscsi initiator %s by %v error: %d", initiator, data, code)
	}

	return nil
}

// AddIscsiInitiatorToHost used for add iscsi initiator to host
func (cli *BaseClient) AddIscsiInitiatorToHost(ctx context.Context, initiator, hostID string) error {
	url := fmt.Sprintf("/iscsi_initiator/%s", initiator)
	data := map[string]interface{}{
		"PARENTTYPE": 21,
		"PARENTID":   hostID,
	}
	resp, err := cli.Put(ctx, url, data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Add iscsi initiator %s to host %s error: %d", initiator, hostID, code)
		return errors.New(msg)
	}

	return nil
}

// GetIscsiTgtPort used for get iscsi target port
func (cli *BaseClient) GetIscsiTgtPort(ctx context.Context) ([]interface{}, error) {
	resp, err := cli.Get(ctx, "/iscsi_tgt_port", nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get ISCSI tgt port error: %d", code)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("ISCSI tgt port does not exist")
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	return respData, nil
}
