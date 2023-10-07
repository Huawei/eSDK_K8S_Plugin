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
	"errors"
	"fmt"

	"huawei-csi-driver/utils/log"
)

type FC interface {
	// QueryFCInitiatorByHost used for get fc initiator by host id
	QueryFCInitiatorByHost(ctx context.Context, hostID string) ([]interface{}, error)
	// GetFCInitiator used for get fc initiator
	GetFCInitiator(ctx context.Context, wwn string) (map[string]interface{}, error)
	// GetFCInitiatorByID used for get fc initiator by id(wwn)
	GetFCInitiatorByID(ctx context.Context, wwn string) (map[string]interface{}, error)
	// UpdateFCInitiator used for update fc initiator
	UpdateFCInitiator(ctx context.Context, wwn string, alua map[string]interface{}) error
	// AddFCInitiatorToHost used for add fc initiator to host
	AddFCInitiatorToHost(ctx context.Context, initiator, hostID string) error
	// GetFCTargetWWNs used for get fc target WWNs
	GetFCTargetWWNs(ctx context.Context, initiatorWWN string) ([]string, error)
	// GetFCHostLink used for get fc host link by host id
	GetFCHostLink(ctx context.Context, hostID string) ([]interface{}, error)
}

// QueryFCInitiatorByHost used for get fc initiator by host id
func (cli *BaseClient) QueryFCInitiatorByHost(ctx context.Context, hostID string) ([]interface{}, error) {
	url := fmt.Sprintf("/fc_initiator?PARENTID=%s", hostID)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Query fc initiator of host %s error: %d", hostID, code)
		return nil, errors.New(msg)
	}
	if resp.Data == nil {
		log.AddContext(ctx).Infof("No fc initiator associated to host %s", hostID)
		return nil, nil
	}

	initiators, ok := resp.Data.([]interface{})
	if !ok {
		return nil, errors.New("convert resp.Data to []interface{} failed")
	}
	return initiators, nil
}

// GetFCInitiator used for get fc initiator by ID::wwn
func (cli *BaseClient) GetFCInitiator(ctx context.Context, wwn string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/fc_initiator?filter=ID::%s", wwn)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get fc initiator %s error: %d", wwn, code)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("FC initiator %s does not exist", wwn)
		return nil, nil
	}

	respData, ok := resp.Data.([]interface{})
	if !ok {
		return nil, errors.New("convert resp.Data to []interface{} failed")
	}
	initiator, ok := respData[0].(map[string]interface{})
	if !ok {
		return nil, errors.New("convert respData[0] to map[string]interface{} failed")
	}
	return initiator, nil
}

// GetFCInitiatorByID used for get fc initiator by id(wwn)
func (cli *BaseClient) GetFCInitiatorByID(ctx context.Context, wwn string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/fc_initiator/%s", wwn)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Get fc initiator by ID %s error: %d", wwn, code)
		return nil, errors.New(msg)
	}

	respData, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, errors.New("convert resp.Data to map[string]interface{} failed")
	}
	return respData, nil
}

// UpdateFCInitiator used for update fc initiator
func (cli *BaseClient) UpdateFCInitiator(ctx context.Context, wwn string, alua map[string]interface{}) error {
	url := fmt.Sprintf("/fc_initiator/%s", wwn)
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
		return fmt.Errorf("update fc initiator %s by %v error: %d", wwn, data, code)
	}

	return nil
}

// AddFCInitiatorToHost used for add fc initiator to host
func (cli *BaseClient) AddFCInitiatorToHost(ctx context.Context, initiator, hostID string) error {
	url := fmt.Sprintf("/fc_initiator/%s", initiator)
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
		msg := fmt.Sprintf("Add FC initiator %s to host %s error: %d", initiator, hostID, code)
		return errors.New(msg)
	}

	return nil
}

// GetFCTargetWWNs used for get fc target WWNs
func (cli *BaseClient) GetFCTargetWWNs(ctx context.Context, initiatorWWN string) ([]string, error) {
	url := fmt.Sprintf("/host_link?INITIATOR_TYPE=223&INITIATOR_PORT_WWN=%s", initiatorWWN)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get FC target wwns of initiator %s error: %d", initiatorWWN, code)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("There is no FC target wwn of host initiator wwn %s", initiatorWWN)
		return nil, nil
	}

	var tgtWWNs []string
	respData, ok := resp.Data.([]interface{})
	if !ok {
		return nil, errors.New("convert resp.Data to []interface{} failed")
	}
	for _, tgt := range respData {
		tgtPort, ok := tgt.(map[string]interface{})
		if !ok {
			return nil, errors.New("convert tgtPort to map[string]interface{} failed")
		}
		tgtWWNs = append(tgtWWNs, tgtPort["TARGET_PORT_WWN"].(string))
	}

	return tgtWWNs, nil
}

// GetFCHostLink used for get fc host link by host id
func (cli *BaseClient) GetFCHostLink(ctx context.Context, hostID string) ([]interface{}, error) {
	url := fmt.Sprintf("/host_link?INITIATOR_TYPE=223&PARENTID=%s", hostID)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get FC host link of host %s error: %d", hostID, code)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("There is no FC host link of host %s", hostID)
		return nil, nil
	}

	respData, ok := resp.Data.([]interface{})
	if !ok {
		return nil, errors.New("convert resp.Data to []interface{} failed")
	}
	return respData, nil
}
