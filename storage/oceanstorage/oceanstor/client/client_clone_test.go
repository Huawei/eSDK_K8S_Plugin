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

// Package client used to for client clone test
package client

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func (m *MockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.Response, m.Err
}

type MockTransport struct {
	Response *http.Response
	Err      error
}

func getMockClient(statusCode int, body string) *OceanstorClient {
	testClient.Client = &http.Client{
		Transport: &MockTransport{
			Response: &http.Response{
				StatusCode: statusCode,
				Body:       io.NopCloser(bytes.NewBufferString(body)),
			},
		},
	}

	return testClient
}

func TestDeleteClonePair_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	clonePairID := "test-clone-pair-id"
	successRespBody := `{
   "data": {},
   "error": {
       "code": 0,
       "description": "0"
   }}`

	// mock
	mockClient := getMockClient(200, successRespBody)

	// action
	err := mockClient.DeleteClonePair(ctx, clonePairID)

	// assert
	require.NoError(t, err)
}

func TestDeleteClonePair_NotExist(t *testing.T) {
	// arrange
	ctx := context.Background()
	clonePairID := "test-clone-pair-id"
	notExistRespBody := `{
   "data": {},
   "error": {
       "code": 1073798147,
       "description": "0"
   }}`

	// mock
	mockClient := getMockClient(200, notExistRespBody)

	// action
	err := mockClient.DeleteClonePair(ctx, clonePairID)

	// assert
	require.NoError(t, err)
}

func TestGetClonePairInfo_NotExist(t *testing.T) {
	// arrange
	ctx := context.Background()
	clonePairID := "test-clone-pair-id"
	successRespBody := `{
   "data": [{}],
   "error": {
       "code": 0,
       "description": "0"
   }}`

	// mock
	mockClient := getMockClient(200, successRespBody)

	// action
	_, err := mockClient.GetClonePairInfo(ctx, clonePairID)

	// assert
	require.NoError(t, err)
}

func TestGetClonePairInfo_Busy(t *testing.T) {
	// arrange
	ctx := context.Background()
	clonePairID := "test-clone-pair-id"
	successRespBody := `{
   "data": {
       "ID": "52",
       "consistencyGroupName": "",
       "vstoreName": "System_vStore"
   },
   "error": {
       "code": 1077949006,
       "description": "0"
   }}`

	// mock
	mockClient := getMockClient(200, successRespBody)

	// action
	_, err := mockClient.GetClonePairInfo(ctx, clonePairID)

	// assert
	require.Error(t, err)
}

func TestCreateClonePair_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	srcLunID := "test-srcLunID"
	dstLunID := "test-dstLunID"
	cloneSpeed := 3
	successRespBody := `{
    "data": {
        "copyRate": "3",
        "ID": "2",
        "consistencyGroupName": "",
        "description": "",
        "targetName": "lun0002",
        "name": "lun0002",
        "sourceName": "lun0000",
        "sourceID": "0",
        "targetID": "2",
        "TYPE": "57702",
        "consistencyGroupID": "4294967295",
        "copyEndTime": "18446744073709551615",
        "copyProcess": "0",
        "copyStatus": "0",
        "restoreEndTime": "18446744073709551615",
        "restoreStartTime": "18446744073709551615",
        "sourceType": "1",
        "syncEndTime": "18446744073709551615",
        "syncStartTime": "1562295530",
        "syncStatus": "1",
        "vstoreId":"0",
        "vstoreName":"System_vStore"
    },
    "error": {
        "code": 0,
        "description": ""
    }}`

	// mock
	mockClient := getMockClient(200, successRespBody)

	// action
	_, err := mockClient.CreateClonePair(ctx, srcLunID, dstLunID, cloneSpeed)

	// assert
	require.NoError(t, err)
}

func TestCreateClonePair_Failed(t *testing.T) {
	// arrange
	ctx := context.Background()
	srcLunID := "test-srcLunID"
	dstLunID := "test-dstLunID"
	cloneSpeed := 3
	failedRespBody := `{
    "data": {
    },
    "error": {
       "code": 1073798147,
       "description": ""
    }}`

	// mock
	mockClient := getMockClient(200, failedRespBody)

	// action
	_, err := mockClient.CreateClonePair(ctx, srcLunID, dstLunID, cloneSpeed)

	// assert
	require.Error(t, err)
}

func TestSyncClonePair_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	clonePairID := "test-clonePairID"
	RespBody := `{
    "data": {
    },
    "error": {
       "code": 0,
       "description": ""
    }}`

	// mock
	mockClient := getMockClient(200, RespBody)

	// action
	err := mockClient.SyncClonePair(ctx, clonePairID)

	// assert
	require.NoError(t, err)
}

func TestSyncClonePair_Failed(t *testing.T) {
	// arrange
	ctx := context.Background()
	clonePairID := "test-clonePairID"
	failedRespBody := `{
    "data": {
    },
    "error": {
       "code": 1073798175,
       "description": "The running status of the clone pair is not Normal or Unsynchronized."
    }}`

	// mock
	mockClient := getMockClient(200, failedRespBody)

	// action
	err := mockClient.SyncClonePair(ctx, clonePairID)

	// assert
	require.Error(t, err)
}

func TestCloneFileSystem_Failed(t *testing.T) {
	// arrange
	ctx := context.Background()
	name := "test-name"
	allocType := 1
	parentID := "test-parentID"
	parentSnapshotID := "test-parentSnapshotID"
	failedRespBody := `{
    "data": {
    },
    "error": {
       "code": 50331651,
       "description": "The entered parameter is incorrect."
    }}`

	// mock
	mockClient := getMockClient(200, failedRespBody)

	// action
	_, err := mockClient.CloneFileSystem(ctx, name, allocType, parentID, parentSnapshotID)

	// assert
	require.Error(t, err)
}
