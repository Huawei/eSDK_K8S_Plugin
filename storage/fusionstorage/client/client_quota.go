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

package client

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/fusionstorage/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	quotaNotExist           int64 = 37767685
	manageQuotaPath               = "/api/v2/converged_service/quota"
	dtreeQuotaParentType          = 16445
	quotaSpaceUnitTypeBytes       = 0
	quotaTypeDirectory            = 1
)

// Quota is the interface for quota
type Quota interface {
	CreateQuota(ctx context.Context, params map[string]interface{}) error
	UpdateQuota(ctx context.Context, params map[string]interface{}) error
	GetQuotaByFileSystemById(ctx context.Context, fsID string) (map[string]interface{}, error)
	QueryQuotaByFsId(ctx context.Context, fsID string) (*QueryQuotaResponse, error)
	DeleteQuota(ctx context.Context, quotaID string) error

	GetQuotaByDTreeId(ctx context.Context, dTreeId string) (*DTreeQuotaResponse, error)
	CreateDTreeQuota(ctx context.Context, dTreeId string, capacity int64) (*DTreeQuotaResponse, error)
	DeleteDTreeQuota(ctx context.Context, quotaId string) error
	UpdateDTreeQuota(ctx context.Context, quotaId string, capacity int64) error
}

// CreateQuota creates quota by params
func (cli *RestClient) CreateQuota(ctx context.Context, params map[string]interface{}) error {
	resp, err := cli.post(ctx, "/api/v2/file_service/fs_quota", params)
	if err != nil {
		return err
	}

	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		msg := fmt.Sprintf("The result of response %v's format is not map[string]interface{}", resp)
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}
	errorCode := int64(result["code"].(float64))
	if errorCode != 0 {
		msg := fmt.Sprintf("Failed to create quota %v, error: %d", params, errorCode)
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}

	return nil
}

// UpdateQuota updates quota by params
func (cli *RestClient) UpdateQuota(ctx context.Context, params map[string]interface{}) error {
	resp, err := cli.put(ctx, "/api/v2/file_service/fs_quota", params)
	if err != nil {
		return err
	}
	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		msg := fmt.Sprintf("The result of response %v's format is not map[string]interface{}", resp)
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}
	errorCode := int64(result["code"].(float64))
	if errorCode != 0 {
		msg := fmt.Sprintf("Failed to Update quota %v, error: %d", params, errorCode)
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}
	return nil
}

// GetQuotaByFileSystemById query quota info by file system id
func (cli *RestClient) GetQuotaByFileSystemById(ctx context.Context, fsID string) (map[string]interface{}, error) {
	url := "/api/v2/file_service/fs_quota?parent_type=40&parent_id=" +
		fsID + "&range=%7B%22offset%22%3A0%2C%22limit%22%3A100%7D"
	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		msg := fmt.Sprintf("The result of response %v's format is not map[string]interface{}", resp)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}
	errorCode := int64(result["code"].(float64))
	if errorCode != 0 {
		return nil, fmt.Errorf("get quota by filesystem id %s error: %d", fsID, errorCode)
	}

	fsQuotas, exist := resp["data"].([]interface{})
	if !exist || len(fsQuotas) <= 0 {
		return nil, nil
	}

	for _, q := range fsQuotas {
		quota, ok := q.(map[string]interface{})
		if !ok {
			msg := fmt.Sprintf("The fsQuota %v's format is not map[string]interface{}", q)
			log.AddContext(ctx).Errorln(msg)
			return nil, errors.New(msg)
		}
		return quota, nil
	}
	return nil, nil
}

// QueryQuotaResponse defines the response of quota
type QueryQuotaResponse struct {
	Id             string  `json:"id"`
	SpaceHardQuota uint64  `json:"space_hard_quota"`
	SpaceSoftQuota uint64  `json:"space_soft_quota"`
	SpaceUnitType  float64 `json:"space_unit_type"`
}

// QueryQuotaByFsId query quotas by filesystem id
func (cli *RestClient) QueryQuotaByFsId(ctx context.Context, fsID string) (*QueryQuotaResponse, error) {
	url := "/api/v2/file_service/fs_quota?parent_type=40&parent_id=" +
		fsID + "&range=%7B%22offset%22%3A0%2C%22limit%22%3A100%7D"
	resp, err := gracefulNasGet[[]QueryQuotaResponse](ctx, cli, url)
	if err != nil {
		return nil, err
	}

	if resp.GetErrorCode() != 0 {
		return nil, fmt.Errorf("query quota by fsid %s failed: %v", fsID, resp.Result)
	}

	if len(resp.Data) == 0 {
		return nil, nil
	}

	return &resp.Data[0], nil
}

// DeleteQuota deletes quota by id
func (cli *RestClient) DeleteQuota(ctx context.Context, quotaID string) error {
	url := fmt.Sprintf("/api/v2/file_service/fs_quota/%s", quotaID)
	resp, err := cli.delete(ctx, url, nil)
	if err != nil {
		return err
	}

	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		msg := fmt.Sprintf("The result of response %v's format is not map[string]interface{}", resp)
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}
	errorCode := int64(result["code"].(float64))
	if errorCode != 0 {
		if errorCode == quotaNotExist {
			log.AddContext(ctx).Warningf("Quota %s doesn't exist while deleting.", quotaID)
			return nil
		}
		return fmt.Errorf("delete quota %s error: %d", quotaID, errorCode)
	}

	return nil
}

// DTreeQuotaResponse defines the fields of quota
type DTreeQuotaResponse struct {
	Id             string  `json:"id"`
	ParentId       string  `json:"parent_id"`
	SpaceHardQuota int64   `json:"space_hard_quota"`
	SpaceSoftQuota float64 `json:"space_soft_quota"`
	SpaceUnitType  int64   `json:"space_unit_type"`
}

// GetQuotaByDTreeId gets quota by dtree id
func (cli *RestClient) GetQuotaByDTreeId(ctx context.Context, dTreeId string) (*DTreeQuotaResponse, error) {
	restPath := utils.NewFusionRestPath(manageQuotaPath)
	restPath.SetQuery("parent_id", dTreeId)
	restPath.SetQuery("parent_type", strconv.Itoa(dtreeQuotaParentType))
	restPath.SetQuery("space_unit_type", strconv.Itoa(quotaSpaceUnitTypeBytes))
	restPath.SetDefaultRange()
	encodedPath, err := restPath.Encode()
	if err != nil {
		return nil, fmt.Errorf("failed to encode path and queries: %w", err)
	}

	resp, err := gracefulNasGet[[]*DTreeQuotaResponse](ctx, cli, encodedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get quota by dtree id: %w", err)
	}
	if resp.GetErrorCode() != 0 {
		return nil, fmt.Errorf("error %+v from get quota by dtree id restful response", resp.Result)
	}

	if len(resp.Data) == 0 {
		return nil, nil
	}

	return resp.Data[0], nil
}

// CreateDTreeQuota creates quota of dtree
func (cli *RestClient) CreateDTreeQuota(ctx context.Context, dtreeId string, capacity int64) (*DTreeQuotaResponse,
	error) {
	req := map[string]any{
		"parent_id":        dtreeId,
		"space_hard_quota": capacity,
		"quota_type":       quotaTypeDirectory,
		"space_unit_type":  quotaSpaceUnitTypeBytes,
		"parent_type":      dtreeQuotaParentType,
	}
	resp, err := gracefulNasPost[*DTreeQuotaResponse](ctx, cli, manageQuotaPath, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create dtree quota: %w", err)
	}
	if resp.GetErrorCode() != 0 {
		return nil, fmt.Errorf("error %+v from create quota restful response", resp.Result)
	}

	return resp.Data, nil
}

// DeleteDTreeQuota deletes quota of dtree
func (cli *RestClient) DeleteDTreeQuota(ctx context.Context, quotaId string) error {
	req := map[string]any{"id": quotaId}
	resp, err := gracefulNasDelete[any](ctx, cli, manageQuotaPath, req)
	if err != nil {
		return fmt.Errorf("failed to delete dtree quota: %w", err)
	}
	if resp.GetErrorCode() != 0 {
		return fmt.Errorf("error %+v from delete quota restful response", resp.Result)
	}
	return nil
}

// UpdateDTreeQuota update quota of dtree
func (cli *RestClient) UpdateDTreeQuota(ctx context.Context, quotaId string, capacity int64) error {
	req := map[string]any{
		"id":               quotaId,
		"space_hard_quota": capacity,
		"space_unit_type":  quotaSpaceUnitTypeBytes,
	}
	resp, err := gracefulNasPut[any](ctx, cli, manageQuotaPath, req)
	if err != nil {
		return fmt.Errorf("failed to update dtree quota: %w", err)
	}
	if resp.GetErrorCode() != 0 {
		return fmt.Errorf("error %+v from update quota restful response", resp.Result)
	}
	return nil
}
