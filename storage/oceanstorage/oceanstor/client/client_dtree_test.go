/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2025-2025. All rights reserved.
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

// Package client used to for client DTree test
package client

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateDTree_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	param := map[string]interface{}{
		"NAME":          "dtree1",
		"PARENTTYPE":    "40",
		"PARENTID":      "1",
		"QUOTASWITCH":   "false",
		"securityStyle": "3",
	}
	successRespBody := `{
    "data": {
        "ID": "1@4097",
        "NAME": "dtree1",
        "PARENTTYPE": 40,
        "PARENTID": "1",
        "PARENTNAME": "fs1",
        "QUOTASWITCH": "false",
        "QUOTASWITCHSTATUS": "0",
        "path": "/",
        "securityStyle": "3",
        "vstoreId":"0", 
        "vstoreName":"System_vStore"
    },
    "error": {
        "code": 0,
        "description": "0"
    }}`

	// mock
	mockClient := getMockClient(200, successRespBody)

	// action
	_, err := mockClient.CreateDTree(ctx, param)

	// assert
	require.NoError(t, err)
}

func TestCreateDTree_Failed(t *testing.T) {
	// arrange
	ctx := context.Background()
	param := map[string]interface{}{
		"NAME":          "dtree1",
		"PARENTTYPE":    "40",
		"PARENTID":      "1",
		"QUOTASWITCH":   "false",
		"securityStyle": "3",
	}
	failedRespBody := `{
    "data": {},
    "error": {
        "code": 1077949006,
        "description": "0"
    }}`

	// mock
	mockClient := getMockClient(200, failedRespBody)

	// action
	_, err := mockClient.CreateDTree(ctx, param)

	// assert
	require.Error(t, err)
}

func TestGetDTreeByName_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	parentID := "test-parentID"
	parentName := "test-parentName"
	vStoreID := "test-vStoreID"
	name := "test-name"
	successRespBody := `{
    "data": {
        "ID": "1@4097",
        "NAME": "dtree1",
        "PARENTTYPE": 40,
        "PARENTID": "1",
        "PARENTNAME": "fs1",
        "QUOTASWITCH": "false",
        "QUOTASWITCHSTATUS": "0",
        "path": "/",
        "securityStyle": "3",
        "vstoreId":"0", 
        "vstoreName":"System_vStore"
    },
    "error": {
        "code": 0,
        "description": "0"
    }}`

	// mock
	mockClient := getMockClient(200, successRespBody)

	// action
	_, err := mockClient.GetDTreeByName(ctx, parentID, parentName, vStoreID, name)

	// assert
	require.NoError(t, err)
}

func TestGetDTreeByName_NotExist(t *testing.T) {
	// arrange
	ctx := context.Background()
	parentID := "test-parentID"
	parentName := "test-parentName"
	vStoreID := "test-vStoreID"
	name := "test-name"
	successRespBody := `{
    "data": {},
    "error": {
        "code": 1077955336,
        "description": "The specified dtree does not exist."
    }}`

	// mock
	mockClient := getMockClient(200, successRespBody)

	// action
	_, err := mockClient.GetDTreeByName(ctx, parentID, parentName, vStoreID, name)

	// assert
	require.NoError(t, err)
}

func TestDeleteDTreeByID_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	vStoreId := "test-vStoreId"
	dTreeID := "test-dTree-Id"
	successRespBody := `{
    "data": {},
    "error": {
        "code": 0,
        "description": "0"
    }}`

	// mock
	mockClient := getMockClient(200, successRespBody)

	// action
	err := mockClient.DeleteDTreeByID(ctx, vStoreId, dTreeID)

	// assert
	require.NoError(t, err)
}

func TestDeleteDTreeByID_Failed(t *testing.T) {
	// arrange
	ctx := context.Background()
	vStoreId := "test-vStoreId"
	dTreeID := "test-dTree-Id"
	failedRespBody := `{
    "data": {},
    "error": {
        "code": 1077949006,
        "description": "0"
    }}`

	// mock
	mockClient := getMockClient(200, failedRespBody)

	// action
	err := mockClient.DeleteDTreeByID(ctx, vStoreId, dTreeID)

	// assert
	require.Error(t, err)
}

func TestDeleteDTreeByName_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	parentName := "test-parentName"
	dTreeName := "test-dTreeName"
	vStoreId := "test-vStoreId"
	successRespBody := `{
    "data": {},
    "error": {
        "code": 0,
        "description": "0"
    }}`

	// mock
	mockClient := getMockClient(200, successRespBody)

	// action
	err := mockClient.DeleteDTreeByName(ctx, parentName, dTreeName, vStoreId)

	// assert
	require.NoError(t, err)
}

func TestDeleteDTreeByName_Failed(t *testing.T) {
	// arrange
	ctx := context.Background()
	parentName := "test-parentName"
	dTreeName := "test-dTreeName"
	vStoreId := "test-vStoreId"
	failedRespBody := `{
    "data": {},
    "error": {
        "code": 1077949006,
        "description": "0"
    }}`

	// mock
	mockClient := getMockClient(200, failedRespBody)

	// action
	err := mockClient.DeleteDTreeByName(ctx, parentName, dTreeName, vStoreId)

	// assert
	require.Error(t, err)
}
