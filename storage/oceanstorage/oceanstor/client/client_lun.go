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

package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	pkgUtils "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	objectNotExist         int64 = 1077948996
	objectIdNotUnique      int64 = 1077948997
	lunAlreadyInGroup      int64 = 1077936862
	lunNotExist            int64 = 1077936859
	parameterIncorrect     int64 = 50331651
	objectNameAlreadyExist int64 = 1077948993

	maxLunNameLength = 31
)

// Lun defines interfaces for lun operations
type Lun interface {
	// QueryAssociateLunGroup used for query associate lun group by object type and object id
	QueryAssociateLunGroup(ctx context.Context, objType int, objID string) ([]interface{}, error)
	// GetLunByName used for get lun by name
	GetLunByName(ctx context.Context, name string) (map[string]interface{}, error)
	// MakeLunName create lun name based on different storage models
	MakeLunName(name string) string
	// GetLunByID used for get lun by id
	GetLunByID(ctx context.Context, id string) (map[string]interface{}, error)
	// GetLunGroupByName used for get lun group by name
	GetLunGroupByName(ctx context.Context, name string) (map[string]interface{}, error)
	// GetLunCountOfHost used for get lun count of host
	GetLunCountOfHost(ctx context.Context, hostID string) (int64, error)
	// GetLunCountOfMapping used for get lun count of mapping by mapping id
	GetLunCountOfMapping(ctx context.Context, mappingID string) (int64, error)
	// DeleteLunGroup used for delete lun group by lun group id
	DeleteLunGroup(ctx context.Context, id string) error
	// DeleteLun used for delete lun by lun id
	DeleteLun(ctx context.Context, id string) error
	// RemoveLunFromGroup used for remove lun from group
	RemoveLunFromGroup(ctx context.Context, lunID, groupID string) error
	// ExtendLun used for extend lun
	ExtendLun(ctx context.Context, lunID string, newCapacity int64) error
	// CreateLun used for create lun
	CreateLun(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error)
	// GetHostLunId used for get host lun id
	GetHostLunId(ctx context.Context, hostID, lunID string) (string, error)
	// UpdateLun used for update lun
	UpdateLun(ctx context.Context, lunID string, params map[string]interface{}) error
	// AddLunToGroup used for add lun to group
	AddLunToGroup(ctx context.Context, lunID string, groupID string) error
	// CreateLunGroup used for create lun group
	CreateLunGroup(ctx context.Context, name string) (map[string]interface{}, error)
}

// QueryAssociateLunGroup used for query associate lun group by object type and object id
func (cli *OceanstorClient) QueryAssociateLunGroup(ctx context.Context,
	objType int, objID string) ([]interface{}, error) {
	url := fmt.Sprintf("/lungroup/associate?ASSOCIATEOBJTYPE=%d&ASSOCIATEOBJID=%s", objType, objID)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("associate query lungroup by obj %s of type %d error: %d", objID, objType, code)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("obj %s of type %d doesn't associate to any lungroup", objID, objType)
		return nil, nil
	}

	respData, ok := resp.Data.([]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert respData to arr failed, data: %v", resp.Data)
	}
	return respData, nil
}

// GetLunByName used for get lun by name
func (cli *OceanstorClient) GetLunByName(ctx context.Context, name string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/lun?filter=NAME::%s&range=[0-100]", name)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Get lun %s info error: %d", name, code)
		return nil, errors.New(msg)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("Lun %s does not exist", name)
		return nil, nil
	}

	respData, ok := resp.Data.([]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert respData to arr failed, data: %v", resp.Data)
	}
	if len(respData) <= 0 {
		log.AddContext(ctx).Infof("Lun %s does not exist", name)
		return nil, nil
	}

	return cli.getObjByvStoreName(respData), nil
}

// MakeLunName v3/v5 storage support 1 to 31 characters
func (cli *OceanstorClient) MakeLunName(name string) string {
	if len(name) <= maxLunNameLength {
		return name
	}
	return name[:maxLunNameLength]
}

// GetLunByID used for get lun by id
func (cli *OceanstorClient) GetLunByID(ctx context.Context, id string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/lun/%s", id)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Get lun %s info error: %d", id, code)
		return nil, errors.New(msg)
	}

	lun, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert lun to map failed, data: %v", resp.Data)
	}

	return lun, nil
}

// AddLunToGroup used for add lun to group
func (cli *OceanstorClient) AddLunToGroup(ctx context.Context, lunID string, groupID string) error {
	data := map[string]interface{}{
		"ID":               groupID,
		"ASSOCIATEOBJTYPE": "11",
		"ASSOCIATEOBJID":   lunID,
	}

	resp, err := cli.Post(ctx, "/lungroup/associate", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == objectIdNotUnique || code == lunAlreadyInGroup {
		log.AddContext(ctx).Warningf("Lun %s is already in group %s", lunID, groupID)
		return nil
	}
	if code != 0 {
		msg := fmt.Sprintf("Add lun %s to group %s error: %d", lunID, groupID, code)
		return errors.New(msg)
	}

	return nil
}

// RemoveLunFromGroup used for remove lun from group
func (cli *OceanstorClient) RemoveLunFromGroup(ctx context.Context, lunID, groupID string) error {
	data := map[string]interface{}{
		"ID":               groupID,
		"ASSOCIATEOBJTYPE": "11",
		"ASSOCIATEOBJID":   lunID,
	}

	resp, err := cli.Delete(ctx, "/lungroup/associate", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == objectNotExist {
		log.AddContext(ctx).Warningf("LUN %s is not in lungroup %s", lunID, groupID)
		return nil
	}
	if code != 0 {
		msg := fmt.Sprintf("Remove lun %s from group %s error: %d", lunID, groupID, code)
		return errors.New(msg)
	}

	return nil
}

// GetLunGroupByName used for get lun group by name
func (cli *OceanstorClient) GetLunGroupByName(ctx context.Context, name string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/lungroup?filter=NAME::%s", name)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Get lungroup %s info error: %d", name, code)
		return nil, errors.New(msg)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("Lungroup %s does not exist", name)
		return nil, nil
	}

	respData, ok := resp.Data.([]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert respData to arr failed, data: %v", resp.Data)
	}
	if len(respData) <= 0 {
		log.AddContext(ctx).Infof("Lungroup %s does not exist", name)
		return nil, nil
	}

	group, ok := respData[0].(map[string]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert group to arr failed, data: %v", respData[0])
	}

	return group, nil
}

// CreateLunGroup used for create lun group
func (cli *OceanstorClient) CreateLunGroup(ctx context.Context, name string) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"NAME":    name,
		"APPTYPE": 0,
	}
	resp, err := cli.Post(ctx, "/lungroup", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code == objectNameAlreadyExist {
		log.AddContext(ctx).Infof("Lungroup %s already exists", name)
		return cli.GetLunGroupByName(ctx, name)
	}
	if code != 0 {
		msg := fmt.Sprintf("Create lungroup %s error: %d", name, code)
		return nil, errors.New(msg)
	}

	lunGroup, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert lunGroup to map failed, data: %v", resp.Data)
	}
	return lunGroup, nil
}

// DeleteLunGroup used for delete lun group by lun group id
func (cli *OceanstorClient) DeleteLunGroup(ctx context.Context, id string) error {
	url := fmt.Sprintf("/lungroup/%s", id)
	resp, err := cli.Delete(ctx, url, nil)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == objectNotExist {
		log.AddContext(ctx).Infof("Lungroup %s does not exist while deleting", id)
		return nil
	}
	if code != 0 {
		msg := fmt.Sprintf("Delete lungroup %s error: %d", id, code)
		return errors.New(msg)
	}

	return nil
}

// CreateLun used for create lun
func (cli *OceanstorClient) CreateLun(ctx context.Context,
	params map[string]interface{}) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"NAME":        params["name"].(string),
		"PARENTID":    params["parentid"].(string),
		"CAPACITY":    params["capacity"].(int64),
		"DESCRIPTION": params["description"].(string),
		"ALLOCTYPE":   params["alloctype"].(int),
	}
	if val, ok := params["workloadTypeID"].(string); ok {
		data["WORKLOADTYPEID"] = val
	}

	resp, err := cli.Post(ctx, "/lun", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code == parameterIncorrect {
		return nil, fmt.Errorf("create Lun error. ErrorCode: %d. Reason: The input parameter is incorrect. "+
			"Suggestion: Delete current PVC and check the parameter of the storageClass and PVC and try again", code)
	}

	if code != 0 {
		return nil, fmt.Errorf("create volume %v error: %d", data, code)
	}

	respData, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert respData to map failed, data: %v", resp.Data)
	}
	return respData, nil
}

// DeleteLun used for delete lun by lun id
func (cli *OceanstorClient) DeleteLun(ctx context.Context, id string) error {
	url := fmt.Sprintf("/lun/%s", id)
	resp, err := cli.Delete(ctx, url, nil)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == lunNotExist {
		log.AddContext(ctx).Infof("Lun %s does not exist while deleting", id)
		return nil
	}
	if code != 0 {
		msg := fmt.Sprintf("Delete lun %s error: %d", id, code)
		return errors.New(msg)
	}

	return nil
}

// ExtendLun used for extend lun
func (cli *OceanstorClient) ExtendLun(ctx context.Context, lunID string, newCapacity int64) error {
	data := map[string]interface{}{
		"CAPACITY": newCapacity,
		"ID":       lunID,
	}

	resp, err := cli.Put(ctx, "/lun/expand", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Extend LUN capacity to %d error: %d", newCapacity, code)
	}

	return nil
}

// GetLunCountOfMapping used for get lun count of mapping by mapping id
func (cli *OceanstorClient) GetLunCountOfMapping(ctx context.Context, mappingID string) (int64, error) {
	url := fmt.Sprintf("/lun/count?ASSOCIATEOBJTYPE=245&ASSOCIATEOBJID=%s", mappingID)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return 0, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Get mapped lun count of mapping %s error: %d", mappingID, code)
		return 0, errors.New(msg)
	}

	respData, ok := resp.Data.(map[string]interface{})
	if !ok {
		return 0, pkgUtils.Errorf(ctx, "convert respData to map failed, data: %v", resp.Data)
	}
	countStr, ok := respData["COUNT"].(string)
	if !ok {
		return 0, pkgUtils.Errorf(ctx, "convert countStr to string failed, data: %v", respData["COUNT"])
	}

	count := utils.ParseIntWithDefault(countStr, constants.DefaultIntBase, constants.DefaultIntBitSize, 0)
	return count, nil
}

// GetLunCountOfHost used for get lun count of host
func (cli *OceanstorClient) GetLunCountOfHost(ctx context.Context, hostID string) (int64, error) {
	url := fmt.Sprintf("/lun/count?ASSOCIATEOBJTYPE=21&ASSOCIATEOBJID=%s", hostID)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return 0, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Get mapped lun count of host %s error: %d", hostID, code)
		return 0, errors.New(msg)
	}

	respData, ok := resp.Data.(map[string]interface{})
	if !ok {
		return 0, pkgUtils.Errorf(ctx, "convert respData to map failed, data: %v", resp.Data)
	}

	countStr, ok := respData["COUNT"].(string)
	if !ok {
		return 0, pkgUtils.Errorf(ctx, "convert countStr to string failed, data: %v", respData["COUNT"])
	}
	count := utils.ParseIntWithDefault(countStr, constants.DefaultIntBase, constants.DefaultIntBitSize, 0)
	return count, nil
}

// GetHostLunId used for get host lun id
func (cli *OceanstorClient) GetHostLunId(ctx context.Context, hostID, lunID string) (string, error) {
	hostLunId := "1"
	url := fmt.Sprintf("/lun/associate?TYPE=11&ASSOCIATEOBJTYPE=21&ASSOCIATEOBJID=%s", hostID)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return "", err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return "", fmt.Errorf("Get hostLunId of host %s, lun %s error: %d", hostID, lunID, code)
	}

	respData, ok := resp.Data.([]interface{})
	if !ok {
		return "", pkgUtils.Errorf(ctx, "convert respData to arr failed, data: %v", resp.Data)
	}

	for _, i := range respData {
		hostLunInfo, ok := i.(map[string]interface{})
		if !ok {
			log.AddContext(ctx).Warningf(fmt.Sprintf("convert hostLunInfo to map failed, data: %v", i))
			continue
		}

		if hostLunInfo["ID"].(string) == lunID {
			var associateData map[string]interface{}
			associateDataBytes := []byte(hostLunInfo["ASSOCIATEMETADATA"].(string))
			err := json.Unmarshal(associateDataBytes, &associateData)
			if err != nil {
				return "", nil
			}
			hostLunIdFloat, ok := associateData["HostLUNID"].(float64)
			if ok {
				hostLunId = strconv.FormatInt(int64(hostLunIdFloat), constants.DefaultIntBase)
				break
			}
		}
	}

	return hostLunId, nil
}

// UpdateLun used for update lun
func (cli *OceanstorClient) UpdateLun(ctx context.Context, lunID string, params map[string]interface{}) error {
	url := fmt.Sprintf("/lun/%s", lunID)
	resp, err := cli.Put(ctx, url, params)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Update LUN %s by params %v error: %d", lunID, params, code)
		return errors.New(msg)
	}

	return nil
}
