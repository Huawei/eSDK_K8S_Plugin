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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	logName = "clientNamespaceTest.log"
)

// capturingTransport is a mock HTTP transport that captures request body data
type capturingTransport struct {
	response  *http.Response
	onRequest func(req *http.Request)
}

func (t *capturingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.onRequest != nil {
		t.onRequest(req)
	}
	return t.response, nil
}

func TestAllowNfsShareAccess(t *testing.T) {
	t.Run("Normal", func(t *testing.T) {
		guard := gomonkey.ApplyMethod(reflect.TypeOf(testClient), "Post",
			func(_ *RestClient, _ context.Context, _ string, _ map[string]interface{}) (map[string]interface{}, error) {
				return map[string]interface{}{
					"data":   map[string]interface{}{},
					"result": map[string]interface{}{"code": float64(0), "description": ""},
				}, nil
			})
		defer guard.Reset()

		err := testClient.AllowNfsShareAccess(context.TODO(), &AllowNfsShareAccessRequest{
			AccessName:  "test",
			ShareId:     "test",
			AccessValue: 0,
			AllSquash:   1,
			RootSquash:  1,
		})
		require.NoError(t, err)
	})

	t.Run("Result Code Not Exist", func(t *testing.T) {
		guard := gomonkey.ApplyMethod(reflect.TypeOf(testClient), "Post",
			func(_ *RestClient, _ context.Context, _ string, _ map[string]interface{}) (map[string]interface{}, error) {
				return map[string]interface{}{
					"data": map[string]interface{}{},
				}, nil
			})
		defer guard.Reset()

		err := testClient.AllowNfsShareAccess(context.TODO(), &AllowNfsShareAccessRequest{
			AccessName:  "test",
			ShareId:     "test",
			AccessValue: 0,
			AllSquash:   1,
			RootSquash:  1,
		})
		require.Error(t, err)
	})

	t.Run("RestClient Already Exist", func(t *testing.T) {
		guard := gomonkey.ApplyMethod(reflect.TypeOf(testClient), "Post",
			func(_ *RestClient, _ context.Context, _ string, _ map[string]interface{}) (map[string]interface{}, error) {
				return map[string]interface{}{
					"data":   map[string]interface{}{},
					"result": map[string]interface{}{"code": float64(clientAlreadyExist), "description": ""},
				}, nil
			})
		defer guard.Reset()

		err := testClient.AllowNfsShareAccess(context.TODO(), &AllowNfsShareAccessRequest{
			AccessName:  "test",
			ShareId:     "test",
			AccessValue: 0,
			AllSquash:   1,
			RootSquash:  1,
		})
		require.NoError(t, err)
	})

	t.Run("Error code is not zero", func(t *testing.T) {
		guard := gomonkey.ApplyMethod(reflect.TypeOf(testClient), "Post",
			func(_ *RestClient, _ context.Context, _ string, _ map[string]interface{}) (map[string]interface{}, error) {
				return map[string]interface{}{
					"data":   map[string]interface{}{},
					"result": map[string]interface{}{"code": float64(100), "description": ""},
				}, nil
			})
		defer guard.Reset()

		err := testClient.AllowNfsShareAccess(context.TODO(), &AllowNfsShareAccessRequest{
			AccessName:  "test",
			ShareId:     "test",
			AccessValue: 0,
			AllSquash:   1,
			RootSquash:  1,
		})
		require.Error(t, err)
	})
}

func TestRestClient_CreateFileSystem(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		params       map[string]interface{}
		mockResponse string
		mockError    error
		expectedData map[string]interface{}
		expectedErr  error
	}{
		{
			name: "Successful creation",
			params: map[string]interface{}{
				"name":     "testFS",
				"poolId":   int64(1),
				"protocol": "dpc",
			},
			mockResponse: `{"result":{"code":0},"data":{"id":"123"}}`,
			expectedData: map[string]interface{}{
				"id": "123",
			},
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// mock
			mockClient := getMockClient(200, tt.mockResponse)

			// action
			data, err := mockClient.CreateFileSystem(ctx, tt.params)

			// assert
			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.EqualError(t, err, tt.expectedErr.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedData, data)
			}
		})
	}
}

func TestRestClient_GetFileSystemByName(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		// arrange
		expectedResponse := `{"result":{"code":0},"data":{"key":"value"}}`

		// mock
		cli := getMockClient(200, expectedResponse)

		// action
		result, err := cli.GetFileSystemByName(ctx, "test")

		// assert
		assert.NoError(t, err)
		assert.Equal(t, map[string]interface{}{"key": "value"}, result)
	})
}

func TestRestClient_CreateNfsShare(t *testing.T) {
	tests := []struct {
		name         string
		params       map[string]interface{}
		mockResponse string
		mockError    error
		expectedData map[string]interface{}
		expectedErr  error
	}{
		{
			name: "Successful creation of NFS share",
			params: map[string]interface{}{
				"sharepath":   "/path/to/share",
				"fsid":        "12345",
				"description": "Test NFS Share",
			},
			mockResponse: `{"result":{"code":0},"data":{"share_path":"/path/to/share",
	"file_system_id":"12345","description":"Test NFS Share","account_id":"1"}}`,
			expectedData: map[string]interface{}{
				"share_path":     "/path/to/share",
				"file_system_id": "12345",
				"description":    "Test NFS Share",
				"account_id":     "1",
			},
			expectedErr: nil,
		},
		{
			name: "Error creating NFS share",
			params: map[string]interface{}{
				"sharepath":   "/path/to/share",
				"fsid":        "12345",
				"description": "Test NFS Share",
			},
			mockResponse: `{"result":{"code":1}}`,
			expectedData: nil,
			expectedErr: errors.New("Create nfs share map[account_id:0 description:" +
				"Test NFS Share file_system_id:12345 share_path:/path/to/share] error: 1"),
		},
		{
			name: "Invalid response format",
			params: map[string]interface{}{
				"sharepath":   "/path/to/share",
				"fsid":        "12345",
				"description": "Test NFS Share",
			},
			mockResponse: `{"result": "invalid"}`,
			expectedData: nil,
			expectedErr: errors.New(
				"The result of response map[result:invalid]'s format is not map[string]interface{}"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// mock
			mockClient := getMockClient(200, tt.mockResponse)

			// action
			data, err := mockClient.CreateNfsShare(context.Background(), tt.params)

			// assert
			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.EqualError(t, err, tt.expectedErr.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedData, data)
			}
		})
	}
}

func TestRestClient_DeleteNfsShare(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		// mock
		id := "test-id"
		cli := getMockClient(200, `{"result":{"code":0}}`)

		// action
		err := cli.DeleteNfsShare(ctx, id)

		// assert
		assert.NoError(t, err)
	})

	t.Run("InvalidResponseFormat", func(t *testing.T) {
		// mock
		id := "test-id"
		cli := getMockClient(200, `{"result":"not a map"}`)

		// action
		err := cli.DeleteNfsShare(ctx, id)

		// assert
		assert.ErrorContains(t, err, "format is not map[string]interface{}")
	})

	t.Run("NonZeroErrorCode", func(t *testing.T) {
		// mock
		id := "test-id"
		cli := getMockClient(200, `{"result":{"code":1}}`)

		// action
		err := cli.DeleteNfsShare(ctx, id)

		// assert
		assert.ErrorContains(t, err, "Delete NFS share test-id error: 1")
	})
}

func TestRestClient_GetNfsShareByPath(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		// mock
		path := "/test/path"
		cli := getMockClient(200, `{"result":{"code":0},"data":[{"share_path":"/test/path","other_info":"some_info"}]}`)

		// action
		share, err := cli.GetNfsShareByPath(ctx, path)

		// assert
		assert.NoError(t, err)
		assert.Equal(t, path, share["share_path"])

	})

	t.Run("Non-zero error code in response", func(t *testing.T) {
		// mock
		path := "/test/path"
		cli := getMockClient(200, `{"result":{"code":1}}`)

		// action
		_, err := cli.GetNfsShareByPath(ctx, path)

		// assert
		assert.ErrorContains(t, err, "error: 1")
	})

	t.Run("No matching share path in response", func(t *testing.T) {
		// mock
		path := "/test/path"
		cli := getMockClient(200, `{"result":{"code":0},"data":[{"share_path":"/other/path"}]}`)

		// action
		share, err := cli.GetNfsShareByPath(ctx, path)

		// assert
		assert.NoError(t, err)
		assert.Nil(t, share)
	})
}

func TestRestClient_DeleteNfsShareAccess(t *testing.T) {
	ctx := context.Background()
	accessID := "test-access-id"

	tests := []struct {
		name        string
		mockResp    string
		mockErr     error
		expectedErr error
	}{
		{
			name:        "Success case",
			mockResp:    `{"result":{"code":0}}`,
			expectedErr: nil,
		},
		{
			name:        "Non-zero error code",
			mockResp:    `{"result":{"code":1}}`,
			expectedErr: errors.New("Delete nfs share test-access-id access error: 1"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// mock
			mockClient := getMockClient(200, tt.mockResp)

			// action
			err := mockClient.DeleteNfsShareAccess(ctx, accessID)

			// assert
			if tt.expectedErr != nil {
				assert.EqualError(t, err, tt.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetNfsShareAccess(t *testing.T) {
	// arrange
	ctx := context.Background()
	shareID := "test_share_id"

	t.Run("SuccessfulResponse", func(t *testing.T) {
		expectedResponse := map[string]interface{}{
			"result": map[string]interface{}{
				"code": 0,
			},
			"data": map[string]interface{}{
				"key": "value",
			},
		}

		// mock
		mockClient := getMockClient(200, `{"result":{"code":0},"data":{"key":"value"}}`)

		// action
		result, err := mockClient.GetNfsShareAccess(ctx, shareID)

		// assert
		assert.NoError(t, err)
		assert.Equal(t, expectedResponse["data"], result)
	})
}

func TestRestClient_CreateFileSystem_AdvancedOptions(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name               string
		params             map[string]interface{}
		expectedDataFields map[string]interface{}
		unexpectedDataKeys []string
		expectedErr        bool
	}{
		{
			name: "Trash and audit params via advancedOptions",
			params: map[string]interface{}{
				"name":   "testFS",
				"poolId": int64(1),
				"advancedoptions": `{"is_trash_enable":true,"checkpoint_trash":30,` +
					`"interval_trash":7,"is_trash_visible":true,` +
					`"audit_log_switch":1,"audit_log_rule":1}`,
			},
			expectedDataFields: map[string]interface{}{
				"is_trash_enable":  true,
				"checkpoint_trash": float64(30),
				"interval_trash":   float64(7),
				"is_trash_visible": true,
				"audit_log_switch": float64(1),
				"audit_log_rule":   float64(1),
			},
			unexpectedDataKeys: nil,
		},
		{
			name: "AdvancedOptions does not override existing keys",
			params: map[string]interface{}{
				"name":            "testFS",
				"poolId":          int64(1),
				"protocol":        "dpc",
				"advancedoptions": `{"forbidden_dpc":999}`,
			},
			expectedDataFields: map[string]interface{}{
				"forbidden_dpc": float64(notForbidden),
			},
			unexpectedDataKeys: nil,
		},
		{
			name: "No advancedOptions",
			params: map[string]interface{}{
				"name":     "testFS",
				"poolId":   int64(1),
				"protocol": "dpc",
			},
			expectedDataFields: nil,
			unexpectedDataKeys: []string{
				"is_trash_enable", "checkpoint_trash", "interval_trash",
				"is_trash_visible", "audit_log_switch", "audit_log_rule",
			},
		},
		{
			name: "Invalid JSON in advancedOptions returns error",
			params: map[string]interface{}{
				"name":            "testFS",
				"poolId":          int64(1),
				"advancedoptions": `{invalid}`,
			},
			expectedDataFields: nil,
			unexpectedDataKeys: nil,
			expectedErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedData map[string]interface{}
			var parseErr error

			// mock HTTP transport to capture the request body and return a success response
			transport := &capturingTransport{
				response: &http.Response{
					StatusCode: 200,
					Body: io.NopCloser(bytes.NewBufferString(
						`{"result":{"code":0},"data":{"id":"123"}}`)),
				},
				onRequest: func(req *http.Request) {
					if req.Body != nil {
						bodyBytes, err := io.ReadAll(req.Body)
						if err != nil {
							parseErr = err
							return
						}
						parseErr = json.Unmarshal(bodyBytes, &capturedData)
					}
				},
			}
			testClient.client = &http.Client{Transport: transport}

			// action
			_, err := testClient.CreateFileSystem(ctx, tt.params)

			// assert
			if tt.expectedErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NoError(t, parseErr, "failed to parse request body")

			if capturedData != nil {
				for key, expectedVal := range tt.expectedDataFields {
					actualVal, exists := capturedData[key]
					if exists {
						assert.Equal(t, expectedVal, actualVal, "value mismatch for key %s", key)
					} else {
						assert.Failf(t, "key missing", "expected key %s to exist in API data", key)
					}
				}

				for _, key := range tt.unexpectedDataKeys {
					_, exists := capturedData[key]
					if exists {
						assert.Failf(t, "unexpected key", "unexpected key %s should not exist in API data", key)
					}
				}
			} else {
				assert.Fail(t, "capturedData should not be nil")
			}
		})
	}
}
