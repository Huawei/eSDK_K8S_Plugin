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
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/api"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	objectNotExist          int64 = 1077948996
	objectIdNotUnique       int64 = 1077948997
	namespaceAlreadyInGroup int64 = 1077936862
	namespaceNotExist       int64 = 1077936859
	parameterIncorrect      int64 = 50331651
	objectNameAlreadyExist  int64 = 1077948993

	// NamespaceType is Associated object type of Namespace
	NamespaceType = "11"
	// AssociateObjTypeNamespace Namespace type
	AssociateObjTypeNamespace = 11
	// AssociateObjTypeNamespaceGroup Namespace group type
	AssociateObjTypeNamespaceGroup = 256
)

// Namespace defines interfaces for namespace operations
type Namespace interface {
	// GetNamespaceByName used for get namespace by name
	GetNamespaceByName(ctx context.Context, name string) (map[string]interface{}, error)
	// GetNamespaceByID used for get namespace by id
	GetNamespaceByID(ctx context.Context, id string) (map[string]interface{}, error)
	// GetNamespaceCountOfHost used for get namespace count of host
	GetNamespaceCountOfHost(ctx context.Context, hostID string) (int64, error)
	// GetNamespaceCountOfMapping used for get namespace count of mapping by mapping id
	GetNamespaceCountOfMapping(ctx context.Context, mappingID string) (int64, error)
	// DeleteNamespace used for delete namespace by namespace id
	DeleteNamespace(ctx context.Context, id string) error
	// ExtendNamespace used for extend namespace
	ExtendNamespace(ctx context.Context, namespaceID string, newCapacity int64) error
	// CreateNamespace used for create namespace
	CreateNamespace(ctx context.Context, params CreateNamespaceParams) (map[string]interface{}, error)
	// GetHostNamespaceId used for get host namespace id
	GetHostNamespaceId(ctx context.Context, hostID, namespaceID string) (string, error)
	// UpdateNamespace used for update namespace
	UpdateNamespace(ctx context.Context, namespaceID string, params map[string]interface{}) error
}

// NamespaceGroup defines interfaces for namespacegroup operations
type NamespaceGroup interface {
	// QueryAssociateNamespaceGroup used for query associate namespace group by object type and object id
	QueryAssociateNamespaceGroup(ctx context.Context, objType int, objID string) ([]interface{}, error)
	// GetNamespaceGroupByName used for get namespace group by name
	GetNamespaceGroupByName(ctx context.Context, name string) (map[string]interface{}, error)
	// DeleteNamespaceGroup used for delete namespace group by namespace group id
	DeleteNamespaceGroup(ctx context.Context, id string) error
	// RemoveNamespaceFromGroup used for remove namespace from group
	RemoveNamespaceFromGroup(ctx context.Context, namespaceID, groupID string) error
	// AddNamespaceToGroup used for add namespace to group
	AddNamespaceToGroup(ctx context.Context, namespaceID string, groupID string) error
	// CreateNamespaceGroup used for create namespace group
	CreateNamespaceGroup(ctx context.Context, name string) (map[string]interface{}, error)
}

// QueryAssociateNamespaceGroup used for query associate namespace group by object type and object id
func (cli *OceandiskClient) QueryAssociateNamespaceGroup(ctx context.Context,
	objType int, objID string) ([]interface{}, error) {
	url := fmt.Sprintf(api.QueryAssociateNamespaceGroup, objType, objID)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code, msg, err := utils.FormatRespErr(resp.Error)
	if err != nil {
		return nil, err
	}

	if code != 0 {
		return nil, fmt.Errorf("associate query namespacegroup by obj %s of type %d failed, "+
			"error code: %d, error msg: %s", objID, objType, code, msg)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("obj %s of type %d doesn't associate to any namespacegroup", objID, objType)
		return []interface{}{}, nil
	}

	respData, ok := resp.Data.([]interface{})
	if !ok {
		return nil, fmt.Errorf("convert respData to arr failed, data: %v", resp.Data)
	}
	return respData, nil
}

// GetNamespaceByName used for get namespace by name
func (cli *OceandiskClient) GetNamespaceByName(ctx context.Context, name string) (map[string]interface{}, error) {
	url := fmt.Sprintf(api.GetNamespaceByName, name)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code, msg, err := utils.FormatRespErr(resp.Error)
	if err != nil {
		return nil, err
	}

	if code != 0 {
		return nil, fmt.Errorf("get namespace %s info failed, error code: %d, error msg: %s", name, code, msg)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("namespace %s does not exist, a nil list is got", name)
		return map[string]interface{}{}, nil
	}

	respData, ok := resp.Data.([]interface{})
	if !ok {
		return nil, fmt.Errorf("convert respData to arr failed, data: %v", resp.Data)
	}
	if len(respData) <= 0 {
		log.AddContext(ctx).Infof("namespace %s does not exist", name)
		return map[string]interface{}{}, nil
	}

	namespace, ok := respData[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("convert namespace to map failed, data: %v", respData[0])
	}
	return namespace, nil
}

// GetNamespaceByID used for get namespace by id
func (cli *OceandiskClient) GetNamespaceByID(ctx context.Context, id string) (map[string]interface{}, error) {
	url := fmt.Sprintf(api.GetNamespaceByID, id)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code, msg, err := utils.FormatRespErr(resp.Error)
	if err != nil {
		return nil, err
	}

	if code != 0 {
		return nil, fmt.Errorf("get namespace %s info failed, error code: %d, error msg: %s", id, code, msg)
	}

	namespace, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("convert namespace to map failed, data: %v", resp.Data)
	}

	return namespace, nil
}

// AddNamespaceToGroup used for add namespace to group
func (cli *OceandiskClient) AddNamespaceToGroup(ctx context.Context, namespaceID string, groupID string) error {
	data := map[string]interface{}{
		"ID":               groupID,
		"ASSOCIATEOBJTYPE": NamespaceType,
		"ASSOCIATEOBJID":   namespaceID,
	}

	resp, err := cli.Post(ctx, api.AddNamespaceToGroup, data)
	if err != nil {
		return err
	}

	code, msg, err := utils.FormatRespErr(resp.Error)
	if err != nil {
		return err
	}

	if code == objectIdNotUnique || code == namespaceAlreadyInGroup {
		log.AddContext(ctx).Infof("namespace %s is already in group %s", namespaceID, groupID)
		return nil
	}

	if code != 0 {
		return fmt.Errorf("add namespace %s to group %s failed, "+
			"error code: %d, error msg: %s", namespaceID, groupID, code, msg)
	}

	return nil
}

// RemoveNamespaceFromGroup used for remove namespace from group
func (cli *OceandiskClient) RemoveNamespaceFromGroup(ctx context.Context, namespaceID, groupID string) error {
	data := map[string]interface{}{
		"ID":               groupID,
		"ASSOCIATEOBJTYPE": NamespaceType,
		"ASSOCIATEOBJID":   namespaceID,
	}

	resp, err := cli.Delete(ctx, api.RemoveNamespaceFromGroup, data)
	if err != nil {
		return err
	}

	code, msg, err := utils.FormatRespErr(resp.Error)
	if err != nil {
		return err
	}

	if code == objectNotExist {
		log.AddContext(ctx).Infof("namespace %s is not in namespacegroup %s", namespaceID, groupID)
		return nil
	}

	if code != 0 {
		return fmt.Errorf("remove namespace %s from group %s failed, "+
			"error code: %d, error msg: %s", namespaceID, groupID, code, msg)
	}

	return nil
}

// GetNamespaceGroupByName used for get namespace group by name
func (cli *OceandiskClient) GetNamespaceGroupByName(ctx context.Context, name string) (map[string]interface{}, error) {
	url := fmt.Sprintf(api.GetNamespaceGroupByName, name)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code, msg, err := utils.FormatRespErr(resp.Error)
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, fmt.Errorf("get namespacegroup %s info failed, "+
			"error code: %d, error msg: %s", name, code, msg)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("namespacegroup %s does not exist, a nil list is got", name)
		return map[string]interface{}{}, nil
	}

	respData, ok := resp.Data.([]interface{})
	if !ok {
		return nil, fmt.Errorf("convert respData to arr failed, data: %v", resp.Data)
	}

	if len(respData) <= 0 {
		log.AddContext(ctx).Infof("namespacegroup %s does not exist", name)
		return map[string]interface{}{}, nil
	}

	group, ok := respData[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("convert group to arr failed, data: %v", respData[0])
	}

	return group, nil
}

// CreateNamespaceGroup used for create namespace group
func (cli *OceandiskClient) CreateNamespaceGroup(ctx context.Context, name string) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"NAME": name,
	}
	resp, err := cli.Post(ctx, api.CreateNamespaceGroup, data)
	if err != nil {
		return nil, err
	}

	code, msg, err := utils.FormatRespErr(resp.Error)
	if err != nil {
		return nil, err
	}

	if code == objectNameAlreadyExist {
		log.AddContext(ctx).Infof("namespacegroup %s already exists", name)
		return cli.GetNamespaceGroupByName(ctx, name)
	}

	if code != 0 {
		return nil, fmt.Errorf("create namespacegroup %s failed, error code: %d, error msg: %s", name, code, msg)
	}

	namespaceGroup, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("convert namespaceGroup to map failed, data: %v", resp.Data)
	}
	return namespaceGroup, nil
}

// DeleteNamespaceGroup used for delete namespace group by namespace group id
func (cli *OceandiskClient) DeleteNamespaceGroup(ctx context.Context, id string) error {
	url := fmt.Sprintf(api.DeleteNamespaceGroup, id)
	resp, err := cli.Delete(ctx, url, nil)
	if err != nil {
		return err
	}

	code, msg, err := utils.FormatRespErr(resp.Error)
	if err != nil {
		return err
	}

	if code == objectNotExist {
		log.AddContext(ctx).Infof("namespacegroup %s does not exist while deleting", id)
		return nil
	}
	if code != 0 {
		return fmt.Errorf("delete namespacegroup %s failed, error code: %d, error msg: %s", id, code, msg)
	}

	return nil
}

// CreateNamespaceParams defines create namespace params
type CreateNamespaceParams struct {
	Name           string
	ParentId       string
	Capacity       int64
	Description    string
	WorkLoadTypeId string
}

// MakeCreateNamespaceParams used to make parameters for CreateNamespace
func MakeCreateNamespaceParams(params map[string]interface{}) (*CreateNamespaceParams, error) {
	namespaceName, ok := params["name"].(string)
	if !ok {
		return nil, fmt.Errorf("assert namespace name: %v to string failed", params["name"])
	}

	parentId, ok := params["poolID"].(string)
	if !ok {
		return nil, fmt.Errorf("assert poolID: %v to string failed", params["poolID"])
	}

	capacity, ok := params["capacity"].(int64)
	if !ok {
		return nil, fmt.Errorf("assert capacity: %v to int64 failed", params["capacity"])
	}

	description, ok := params["description"].(string)
	if !ok {
		return nil, fmt.Errorf("assert description: %v to string failed", params["capacity"])
	}

	// params["workloadTypeID"] may not exist.
	// In this case, the value of workLoadTypeId is an empty string, which meets the expectation.
	workLoadTypeId, _ := utils.GetValue[string](params, "workloadTypeID")

	return &CreateNamespaceParams{
		Name:           namespaceName,
		ParentId:       parentId,
		Capacity:       capacity,
		Description:    description,
		WorkLoadTypeId: workLoadTypeId,
	}, nil
}

// CreateNamespace used for create namespace
func (cli *OceandiskClient) CreateNamespace(ctx context.Context,
	params CreateNamespaceParams) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"NAME":        params.Name,
		"PARENTID":    params.ParentId,
		"CAPACITY":    params.Capacity,
		"DESCRIPTION": params.Description,
	}

	if params.WorkLoadTypeId != "" {
		data["WORKLOADTYPEID"] = params.WorkLoadTypeId
	}

	resp, err := cli.Post(ctx, api.CreateNamespace, data)
	if err != nil {
		return nil, err
	}

	code, msg, err := utils.FormatRespErr(resp.Error)
	if err != nil {
		return nil, err
	}

	if code == parameterIncorrect {
		return nil, fmt.Errorf("create Namespace with incorrect parameters %v, "+
			"err code: %d, err msg: %s", data, code, msg)
	}

	if code != 0 {
		return nil, fmt.Errorf("create volume %v failed, error code: %d, error msg: %s", data, code, msg)
	}

	respData, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("convert respData to map failed, data: %v", resp.Data)
	}
	return respData, nil
}

// DeleteNamespace used for delete namespace by namespace id
func (cli *OceandiskClient) DeleteNamespace(ctx context.Context, id string) error {
	url := fmt.Sprintf(api.DeleteNamespace, id)
	resp, err := cli.Delete(ctx, url, nil)
	if err != nil {
		return err
	}

	code, msg, err := utils.FormatRespErr(resp.Error)
	if err != nil {
		return err
	}

	if code == namespaceNotExist {
		log.AddContext(ctx).Infof("namespace %s does not exist while deleting", id)
		return nil
	}

	if code != 0 {
		return fmt.Errorf("delete namespace %s failed, error code: %d, error msg: %s", id, code, msg)
	}

	return nil
}

// ExtendNamespace used for extend namespace
func (cli *OceandiskClient) ExtendNamespace(ctx context.Context, namespaceID string, newCapacity int64) error {
	data := map[string]interface{}{
		"CAPACITY": newCapacity,
		"ID":       namespaceID,
	}

	resp, err := cli.Put(ctx, api.ExtendNamespace, data)
	if err != nil {
		return err
	}

	code, msg, err := utils.FormatRespErr(resp.Error)
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("extend Namespace capacity to %d failed, "+
			"error code: %d, error msg: %s", newCapacity, code, msg)
	}

	return nil
}

// GetNamespaceCountOfMapping used for get namespace count of mapping by mapping id
func (cli *OceandiskClient) GetNamespaceCountOfMapping(ctx context.Context, mappingID string) (int64, error) {
	url := fmt.Sprintf(api.GetNamespaceCountOfMapping, mappingID)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return 0, err
	}

	code, msg, err := utils.FormatRespErr(resp.Error)
	if err != nil {
		return 0, err
	}
	if code != 0 {
		return 0, fmt.Errorf("get mapped namespace count of mapping %s failed, "+
			"error code: %d, error msg: %s", mappingID, code, msg)
	}

	respData, ok := resp.Data.(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("convert respData to map failed, data: %v", resp.Data)
	}

	countStr, ok := respData["COUNT"].(string)
	if !ok {
		return 0, fmt.Errorf("convert countStr to string failed, data: %v", respData["COUNT"])
	}

	count := utils.ParseIntWithDefault(countStr, constants.DefaultIntBase, constants.DefaultIntBitSize, 0)
	return count, nil
}

// GetNamespaceCountOfHost used for get namespace count of host
func (cli *OceandiskClient) GetNamespaceCountOfHost(ctx context.Context, hostID string) (int64, error) {
	url := fmt.Sprintf(api.GetNamespaceCountOfHost, hostID)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return 0, err
	}

	code, msg, err := utils.FormatRespErr(resp.Error)
	if err != nil {
		return 0, err
	}
	if code != 0 {
		return 0, fmt.Errorf("get mapped namespace count of host %s failed, "+
			"error code: %d, error msg: %s", hostID, code, msg)
	}

	respData, ok := resp.Data.(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("convert respData to map failed, data: %v", resp.Data)
	}

	countStr, ok := respData["COUNT"].(string)
	if !ok {
		return 0, fmt.Errorf("convert countStr to string failed, data: %v", respData["COUNT"])
	}

	count := utils.ParseIntWithDefault(countStr, constants.DefaultIntBase, constants.DefaultIntBitSize, 0)
	return count, nil
}

// GetHostNamespaceId used for get host namespace id
func (cli *OceandiskClient) GetHostNamespaceId(ctx context.Context, hostID, namespaceID string) (string, error) {
	url := fmt.Sprintf(api.GetHostNamespaceId, hostID)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return "", err
	}

	code, msg, err := utils.FormatRespErr(resp.Error)
	if err != nil {
		return "", err
	}
	if code != 0 {
		return "", fmt.Errorf("get hostNamespaceId of host %s, namespace %s failed, "+
			"error code: %d, error msg: %s", hostID, namespaceID, code, msg)
	}

	respData, ok := resp.Data.([]interface{})
	if !ok {
		return "", fmt.Errorf("convert respData to arr failed, data: %v", resp.Data)
	}

	var hostNamespaceId string
	for _, i := range respData {
		hostNamespaceInfo, ok := i.(map[string]interface{})
		if !ok {
			log.AddContext(ctx).Warningf("convert hostNamespaceInfo to map failed, data: %v", i)
			continue
		}

		if hostNamespaceInfo["ID"].(string) == namespaceID {
			var associateData map[string]interface{}
			associateDataBytes := []byte(hostNamespaceInfo["ASSOCIATEMETADATA"].(string))
			err := json.Unmarshal(associateDataBytes, &associateData)
			if err != nil {
				return "", fmt.Errorf("unmarshal associateData fail while "+
					"getting the hostNamespaceId of host %s, namespace %s, error: %v", hostID, namespaceID, err)
			}

			hostNamespaceID, ok := associateData["hostNamespaceID"]
			if !ok {
				return "", fmt.Errorf("hostNamesapceID field is not exist "+
					"in unmarshaled associateData of host %s, namepsace %s", hostID, namespaceID)
			}
			hostNamespaceIdFloat, ok := hostNamespaceID.(float64)
			if ok {
				hostNamespaceId = strconv.FormatInt(int64(hostNamespaceIdFloat), constants.DefaultIntBase)
				break
			}
		}
	}

	if hostNamespaceId == "" {
		return "", fmt.Errorf("can not get the hostNamespaceId of host %s, namespace %s", hostID, namespaceID)
	}

	return hostNamespaceId, nil
}

// UpdateNamespace used for update namespace
func (cli *OceandiskClient) UpdateNamespace(ctx context.Context,
	namespaceID string, params map[string]interface{}) error {
	url := fmt.Sprintf(api.UpdateNamespace, namespaceID)
	resp, err := cli.Put(ctx, url, params)
	if err != nil {
		return err
	}

	code, msg, err := utils.FormatRespErr(resp.Error)
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("update Namespace %s by params %v failed, "+
			"error code: %d, error msg: %s", namespaceID, params, code, msg)
	}

	return nil
}
