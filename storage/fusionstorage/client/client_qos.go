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
	netUrl "net/url"

	pkgUtils "huawei-csi-driver/pkg/utils"
	"huawei-csi-driver/storage/fusionstorage/types"
	"huawei-csi-driver/storage/fusionstorage/utils"
	"huawei-csi-driver/utils/log"
)

// GetConvergedQoSNameByID used to get qos name by id
func (cli *RestClient) GetConvergedQoSNameByID(ctx context.Context, qosId int) (string, error) {
	url := fmt.Sprintf("/api/v2/dros_service/converged_qos_policy?qos_scale=%d&id=%d",
		types.QosScaleNamespace, qosId)
	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return "", err
	}

	if err := utils.CheckErrorCode(resp); err != nil {
		return "", err
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("convert data [%v] to map[string]interface{} failed", resp["data"])
	}

	qosName, ok := data["name"].(string)
	if !ok {
		return "", fmt.Errorf("convert qosName [%v] to string failed", data["name"])
	}

	return qosName, nil
}

// CreateConvergedQoS used to create converged QoS
func (cli *RestClient) CreateConvergedQoS(ctx context.Context, req *types.CreateConvergedQoSReq) (int, error) {
	data := map[string]interface{}{
		"account_id": cli.accountId,
		"qos_scale":  req.QosScale,
		"name":       req.Name,
		"qos_mode":   req.QosMode,
		"max_mbps":   req.MaxMbps,
		"max_iops":   req.MaxIops,
	}

	resp, err := cli.post(ctx, "/api/v2/dros_service/converged_qos_policy", data)
	if err != nil {
		return 0, err
	}

	if err := utils.CheckErrorCode(resp); err != nil {
		return 0, err
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("convert data [%v] to map[string]interface{} failed", resp["data"])
	}

	qosID, ok := data["id"].(float64)
	if !ok {
		return 0, fmt.Errorf("convert qosID [%v] to float64 failed", data["id"])
	}

	return int(qosID), nil
}

// DeleteConvergedQoS used to delete converged QoS by name
func (cli *RestClient) DeleteConvergedQoS(ctx context.Context, qosName string) error {
	url := fmt.Sprintf("/api/v2/dros_service/converged_qos_policy?qos_scale=%d&name=%s",
		types.QosScaleNamespace, qosName)
	resp, err := cli.delete(ctx, url, nil)
	if err != nil {
		return err
	}

	return utils.CheckErrorCode(resp)
}

// CreateQoS used to create QoS
func (cli *RestClient) CreateQoS(ctx context.Context, qosName string, qosData map[string]int) error {
	data := map[string]interface{}{
		"qosName":     qosName,
		"qosSpecInfo": qosData,
	}

	resp, err := cli.post(ctx, "/dsware/service/v1.3/qos/create", data)
	if err != nil {
		return err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		errorCode, _ := resp["errorCode"].(string)
		return fmt.Errorf("create QoS %v error: %s", data, errorCode)
	}

	return nil
}

// DeleteQoS used to delete QoS by name
func (cli *RestClient) DeleteQoS(ctx context.Context, qosName string) error {
	data := map[string]interface{}{
		"qosNames": []string{qosName},
	}

	resp, err := cli.post(ctx, "/dsware/service/v1.3/qos/delete", data)
	if err != nil {
		return err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		errorCode, _ := resp["errorCode"].(string)
		return fmt.Errorf("delete QoS %v error: %s", data, errorCode)
	}

	return nil
}

// DisassociateConvergedQoSWithVolume used to delete a converged QoS policy association
func (cli *RestClient) DisassociateConvergedQoSWithVolume(ctx context.Context, objectName string) error {
	url := fmt.Sprintf("/api/v2/dros_service/converged_qos_association?qos_scale=%d&object_name=%s",
		types.QosScaleNamespace, objectName)
	resp, err := cli.delete(ctx, url, nil)
	if err != nil {
		return err
	}

	return utils.CheckErrorCode(resp)
}

// AssociateConvergedQoSWithVolume used to add a converged QoS policy association
func (cli *RestClient) AssociateConvergedQoSWithVolume(ctx context.Context,
	req *types.AssociateConvergedQoSWithVolumeReq) error {

	data := map[string]interface{}{
		"account_id":    cli.accountId,
		"qos_scale":     req.QosScale,
		"object_name":   req.ObjectName,
		"qos_policy_id": req.QoSPolicyID,
	}

	resp, err := cli.post(ctx, "/api/v2/dros_service/converged_qos_association", data)
	if err != nil {
		return err
	}

	return utils.CheckErrorCode(resp)
}

// AssociateQoSWithVolume used to associate QoS with volume
func (cli *RestClient) AssociateQoSWithVolume(ctx context.Context, volName, qosName string) error {
	data := map[string]interface{}{
		"keyNames": []string{volName},
		"qosName":  qosName,
	}

	resp, err := cli.post(ctx, "/dsware/service/v1.3/qos/volume/associate", data)
	if err != nil {
		return err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		errorCode, _ := resp["errorCode"].(string)
		return fmt.Errorf("associate QoS %s with volume %s error: %s", qosName, volName, errorCode)
	}

	return nil
}

// DisassociateQoSWithVolume used to disassociate QoS with volume
func (cli *RestClient) DisassociateQoSWithVolume(ctx context.Context, volName, qosName string) error {
	data := map[string]interface{}{
		"keyNames": []string{volName},
		"qosName":  qosName,
	}

	resp, err := cli.post(ctx, "/dsware/service/v1.3/qos/volume/disassociate", data)
	if err != nil {
		return err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		errorCode, _ := resp["errorCode"].(string)
		return fmt.Errorf("disassociate QoS %s with volume %s error: %s", qosName, volName, errorCode)
	}

	return nil
}

// GetQoSPolicyAssociationCount used to get count of qos association
func (cli *RestClient) GetQoSPolicyAssociationCount(ctx context.Context, qosPolicyId int) (int, error) {
	filterRaw := fmt.Sprintf("{\"qos_policy_id\":\"%d\",\"qos_scale\":\"%d\",\"account_id\":\"%d\"}",
		qosPolicyId, types.QosScaleNamespace, cli.accountId)
	url := fmt.Sprintf("/api/v2/dros_service/converged_qos_association_count?filter=%s",
		netUrl.QueryEscape(filterRaw))

	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return 0, err
	}

	err = utils.CheckErrorCode(resp)
	if err != nil {
		return 0, err
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		return 0, pkgUtils.Errorln(ctx, fmt.Sprintf("convert data [%v] to map[string]interface{} failed",
			resp["data"]))
	}

	count, ok := data["count"].(float64)
	if !ok {
		return 0, pkgUtils.Errorln(ctx, fmt.Sprintf("convert data count [%v] to float64 failed", data["count"]))
	}

	return int(count), nil
}

// GetQoSPolicyIdByFsName used to get qos id by fs name
func (cli *RestClient) GetQoSPolicyIdByFsName(ctx context.Context, namespaceName string) (int, error) {
	rangeRaw := "{\"offset\":0,\"limit\":100}"
	filterRaw := fmt.Sprintf("{\"object_name\":\"%s\",\"qos_scale\":\"0\",\"account_id\":\"%d\"}",
		namespaceName, cli.accountId)
	url := fmt.Sprintf("/api/v2/dros_service/converged_qos_association?range=%s&filter=%s",
		netUrl.QueryEscape(rangeRaw), netUrl.QueryEscape(filterRaw))
	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return types.NoQoSPolicyId, err
	}

	err = utils.CheckErrorCode(resp)
	if err != nil {
		return types.NoQoSPolicyId, err
	}

	dataListInterfaces, ok := resp["data"].([]interface{})
	if !ok {
		return types.NoQoSPolicyId, fmt.Errorf("convert data [%v] to []map[string]interface{} failed", resp["data"])
	}
	for _, dataListInterface := range dataListInterfaces {
		data, ok := dataListInterface.(map[string]interface{})
		if !ok {
			return types.NoQoSPolicyId, pkgUtils.Errorln(ctx, fmt.Sprintf("convert dataListInterface: [%v] to "+
				"map[string]interface{} failed.", dataListInterface))
		}
		if data["object_name"] == namespaceName {
			qosPolicyId, ok := data["qos_policy_id"].(float64)
			if !ok {
				return types.NoQoSPolicyId, pkgUtils.Errorln(ctx, fmt.Sprintf("convert qos_policy_id: [%v] to "+
					"int failed.", data["qos_policy_id"]))
			}
			return int(qosPolicyId), nil
		}
	}

	return types.NoQoSPolicyId, nil
}

// GetQoSNameByVolume used to get QoS name by volume name
func (cli *RestClient) GetQoSNameByVolume(ctx context.Context, volName string) (string, error) {
	url := fmt.Sprintf("/dsware/service/v1.3/volume/qos?volName=%s", volName)
	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return "", err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		errorCode, _ := resp["errorCode"].(string)
		return "", fmt.Errorf("get qos by volume %s error: %s", volName, errorCode)
	}

	qosName, exist := resp["qosName"].(string)
	if !exist {
		return "", nil
	}

	return qosName, nil
}

// GetAssociateCountOfQoS used to get associate count of QoS
func (cli *RestClient) GetAssociateCountOfQoS(ctx context.Context, qosName string) (int, error) {
	storagePools, err := cli.getAllPools(ctx)
	if err != nil {
		return 0, err
	}
	if storagePools == nil {
		return 0, nil
	}

	associatePools, err := cli.getAssociatePoolOfQoS(ctx, qosName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get associate snapshot of QoS %s error: %v", qosName, err)
		return 0, err
	}
	pools, ok := associatePools["pools"].([]interface{})
	if !ok {
		msg := fmt.Sprintf("There is no pools info in response %v.", associatePools)
		log.AddContext(ctx).Errorln(msg)
		return 0, errors.New(msg)
	}
	storagePoolsCount := len(pools)

	for _, p := range storagePools {
		pool, ok := p.(map[string]interface{})
		if !ok {
			msg := fmt.Sprintf("The storage pool %v's format is not map[string]interface{}", p)
			log.AddContext(ctx).Errorln(msg)
			return 0, errors.New(msg)
		}
		poolId := int64(pool["poolId"].(float64))
		volumes, err := cli.getAssociateObjOfQoS(ctx, qosName, "volume", poolId)
		if err != nil {
			log.AddContext(ctx).Errorf("Get associate volume of QoS %s error: %v", qosName, err)
			return 0, err
		}

		snapshots, err := cli.getAssociateObjOfQoS(ctx, qosName, "snapshot", poolId)
		if err != nil {
			log.AddContext(ctx).Errorf("Get associate snapshot of QoS %s error: %v", qosName, err)
			return 0, err
		}

		volumeCount := int(volumes["totalNum"].(float64))
		snapshotCount := int(snapshots["totalNum"].(float64))
		totalCount := volumeCount + snapshotCount + storagePoolsCount
		if totalCount != 0 {
			return totalCount, nil
		}
	}

	return 0, nil
}

func (cli *RestClient) getAssociateObjOfQoS(ctx context.Context,
	qosName, objType string,
	poolId int64) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"qosName": qosName,
		"poolId":  poolId,
	}

	resp, err := cli.post(ctx, "/dsware/service/v1.3/qos/volume/list?type=associated", data)
	if err != nil {
		return nil, err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		errorCode, _ := resp["errorCode"].(string)
		return nil, fmt.Errorf("get qos %s associate obj %s error: %s", qosName, objType, errorCode)
	}

	return resp, nil
}

func (cli *RestClient) getAssociatePoolOfQoS(ctx context.Context, qosName string) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"qosName": qosName,
	}

	resp, err := cli.post(ctx, "/dsware/service/v1.3/qos/storagePool/list?type=associated", data)
	if err != nil {
		return nil, err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		errorCode, _ := resp["errorCode"].(string)
		return nil, fmt.Errorf("get qos %s associate storagePool error: %s", qosName, errorCode)
	}

	return resp, nil
}
