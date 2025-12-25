/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2024-2025. All rights reserved.
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
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/base"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
)

func TestBaseClient_QueryAssociateNamespaceGroup_Success(t *testing.T) {
	// arrange
	objType := 11
	objID := "1"
	client, err := NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}

	namespaceGroups := make([]interface{}, 0)
	namespaceGroup := map[string]string{
		"DESCRIPTION": "",
		"ID":          "1",
	}
	namespaceGroups = append(namespaceGroups, namespaceGroup)

	mockResponse := base.Response{
		Error: map[string]interface{}{
			"code":        float64(0),
			"description": "0",
		},
		Data: namespaceGroups,
	}

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodFunc(&base.RestClient{}, "Get", func(ctx context.Context,
		url string, data map[string]interface{}) (base.Response, error) {
		return mockResponse, nil
	})

	// action
	getRes, getErr := client.QueryAssociateNamespaceGroup(context.Background(), objType, objID)

	// assert
	if !reflect.DeepEqual(namespaceGroups, getRes) || getErr != nil {
		t.Errorf("TestBaseClient_QueryAssociateNamespaceGroup_Success failed, "+
			"wantRes = %v, gotRes = %v, wantErr = nil, gotErr = %v", namespaceGroups, getRes, getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestBaseClient_QueryAssociateNamespaceGroup_CodeError(t *testing.T) {
	// arrange
	objType := 11
	objID := "1"
	client, err := NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}

	errCode := 1
	errMsg := "unknown error"
	mockResponse := base.Response{
		Error: map[string]interface{}{
			"code":        float64(errCode),
			"description": errMsg,
		},
		Data: nil,
	}
	wantErr := fmt.Errorf("associate query namespacegroup by obj %s of type %d failed, "+
		"error code: %d, error msg: %s", objID, objType, errCode, errMsg)

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodFunc(&base.RestClient{}, "Get", func(ctx context.Context,
		url string, data map[string]interface{}) (base.Response, error) {
		return mockResponse, nil
	})

	// action
	getRes, getErr := client.QueryAssociateNamespaceGroup(context.Background(), objType, objID)

	// assert
	if getRes != nil || !reflect.DeepEqual(wantErr, getErr) {
		t.Errorf("TestBaseClient_QueryAssociateNamespaceGroup_CodeError failed, "+
			"wantRes = nil, gotRes = %v, wantErr = %v, gotErr = %v", getRes, wantErr, getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestBaseClient_QueryAssociateNamespaceGroup_RespNil(t *testing.T) {
	// arrange
	objType := 11
	objID := "1"
	client, err := NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}

	mockResponse := base.Response{
		Error: map[string]interface{}{
			"code":        float64(0),
			"description": "0",
		},
		Data: nil,
	}

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodFunc(&base.RestClient{}, "Get", func(ctx context.Context,
		url string, data map[string]interface{}) (base.Response, error) {
		return mockResponse, nil
	})

	// action
	getRes, getErr := client.QueryAssociateNamespaceGroup(context.Background(), objType, objID)

	// assert
	if len(getRes) != 0 || getErr != nil {
		t.Errorf("TestBaseClient_QueryAssociateNamespaceGroup_RespNil failed, "+
			"wantRes = [], gotRes = %v, wantErr = nil, gotErr = %v", getRes, getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestBaseClient_QueryAssociateNamespaceGroup_RespFormatError(t *testing.T) {
	// arrange
	objType := 11
	objID := "1"
	client, err := NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}

	mockResponse := base.Response{
		Error: map[string]interface{}{
			"code":        float64(0),
			"description": "0",
		},
		Data: "",
	}
	wantErr := fmt.Errorf("convert respData to arr failed, data: %v", mockResponse.Data)

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodFunc(&base.RestClient{}, "Get", func(ctx context.Context,
		url string, data map[string]interface{}) (base.Response, error) {
		return mockResponse, nil
	})

	// action
	getRes, getErr := client.QueryAssociateNamespaceGroup(context.Background(), objType, objID)

	// assert
	if getRes != nil || !reflect.DeepEqual(wantErr, getErr) {
		t.Errorf("TestBaseClient_QueryAssociateNamespaceGroup_RespFormatError failed, "+
			"wantRes = nil, gotRes = %v, wantErr = %v, gotErr = %v", getRes, wantErr, getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestBaseClient_GetNamespaceByName_Success(t *testing.T) {
	// arrange
	name := "namespace1"
	client, err := NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}

	namespaces := make([]interface{}, 0)
	namespace := map[string]interface{}{
		"name": name,
		"ID":   "1",
	}
	namespaces = append(namespaces, namespace)

	mockResponse := base.Response{
		Error: map[string]interface{}{
			"code":        float64(0),
			"description": "0",
		},
		Data: namespaces,
	}

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodFunc(&base.RestClient{}, "Get", func(ctx context.Context,
		url string, data map[string]interface{}) (base.Response, error) {
		return mockResponse, nil
	})

	// action
	getRes, getErr := client.GetNamespaceByName(context.Background(), name)

	// assert
	if !reflect.DeepEqual(namespace, getRes) || getErr != nil {
		t.Errorf("TestBaseClient_GetNamespaceByName_Success failed, "+
			"wantRes = %v, gotRes = %v, wantErr = nil, gotErr = %v", namespace, getRes, getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestBaseClient_GetNamespaceByName_NamespaceFormatError(t *testing.T) {
	// arrange
	name := "namespace1"
	client, err := NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}

	namespaces := make([]interface{}, 0)
	namespace := map[string]string{
		"name": name,
		"ID":   "1",
	}
	namespaces = append(namespaces, namespace)

	mockResponse := base.Response{
		Error: map[string]interface{}{
			"code":        float64(0),
			"description": "0",
		},
		Data: namespaces,
	}
	wantErr := fmt.Errorf("convert namespace to map failed, data: %v", namespace)

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodFunc(&base.RestClient{}, "Get", func(ctx context.Context,
		url string, data map[string]interface{}) (base.Response, error) {
		return mockResponse, nil
	})

	// action
	getRes, getErr := client.GetNamespaceByName(context.Background(), name)

	// assert
	if getRes != nil || !reflect.DeepEqual(wantErr, getErr) {
		t.Errorf("TestBaseClient_GetNamespaceByName_NamespaceFormatError failed, "+
			"wantRes = nil, gotRes = %v, wantErr = %v, gotErr = %v", getRes, wantErr, getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestBaseClient_GetNamespaceByID_Success(t *testing.T) {
	// arrange
	id := "1"
	client, err := NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}

	namespace := map[string]interface{}{
		"name": "namespace1",
		"ID":   id,
	}
	mockResponse := base.Response{
		Error: map[string]interface{}{
			"code":        float64(0),
			"description": "0",
		},
		Data: namespace,
	}

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodFunc(&base.RestClient{}, "Get", func(ctx context.Context,
		url string, data map[string]interface{}) (base.Response, error) {
		return mockResponse, nil
	})

	// action
	getRes, getErr := client.GetNamespaceByID(context.Background(), id)

	// assert
	if !reflect.DeepEqual(namespace, getRes) || getErr != nil {
		t.Errorf("TestBaseClient_GetNamespaceByID_Success failed, "+
			"wantRes = %v, gotRes = %v, wantErr = nil, gotErr = %v", namespace, getRes, getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestBaseClient_GetNamespaceByID_FormatNamespaceError(t *testing.T) {
	// arrange
	id := "1"
	client, err := NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}

	namespace := "invalid format"
	mockResponse := base.Response{
		Error: map[string]interface{}{
			"code":        float64(0),
			"description": "0",
		},
		Data: namespace,
	}
	wantErr := fmt.Errorf("convert namespace to map failed, data: %v", namespace)

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodFunc(&base.RestClient{}, "Get", func(ctx context.Context,
		url string, data map[string]interface{}) (base.Response, error) {
		return mockResponse, nil
	})

	// action
	getRes, getErr := client.GetNamespaceByID(context.Background(), id)

	// assert
	if getRes != nil || !reflect.DeepEqual(wantErr, getErr) {
		t.Errorf("TestBaseClient_GetNamespaceByID_FormatNamespaceError failed, "+
			"wantRes = nil, gotRes = %v, wantErr = %v, gotErr = %v", getRes, wantErr, getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestBaseClient_AddNamespaceToGroup_Success(t *testing.T) {
	// arrange
	namespaceID := "1"
	groupID := "1"
	client, err := NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}

	mockResponse := base.Response{
		Error: map[string]interface{}{
			"code":        float64(0),
			"description": "0",
		},
	}

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodFunc(&base.RestClient{}, "Post", func(ctx context.Context,
		url string, data map[string]interface{}) (base.Response, error) {
		return mockResponse, nil
	})

	// action
	getErr := client.AddNamespaceToGroup(context.Background(), namespaceID, groupID)

	// assert
	if getErr != nil {
		t.Errorf("TestBaseClient_AddNamespaceToGroup_Success failed, wantErr = nil, gotErr = %v", getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestBaseClient_AddNamespaceToGroup_CodeError(t *testing.T) {
	// arrange
	namespaceID := "1"
	groupID := "1"
	client, err := NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}

	errCode := 1
	errMsg := "unknown error"
	mockResponse := base.Response{
		Error: map[string]interface{}{
			"code":        float64(errCode),
			"description": errMsg,
		},
	}
	wantErr := fmt.Errorf("add namespace %s to group %s failed, "+
		"error code: %d, error msg: %s", namespaceID, groupID, errCode, errMsg)

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodFunc(&base.RestClient{}, "Post", func(ctx context.Context,
		url string, data map[string]interface{}) (base.Response, error) {
		return mockResponse, nil
	})

	// action
	getErr := client.AddNamespaceToGroup(context.Background(), namespaceID, groupID)

	// assert
	if !reflect.DeepEqual(wantErr, getErr) {
		t.Errorf("TestBaseClient_AddNamespaceToGroup_Success failed, wantErr = %v, gotErr = %v", wantErr, getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestBaseClient_RemoveNamespaceFromGroup_Success(t *testing.T) {
	// arrange
	namespaceID := "1"
	groupID := "1"
	client, err := NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}

	mockResponse := base.Response{
		Error: map[string]interface{}{
			"code":        float64(0),
			"description": "0",
		},
	}

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodFunc(&base.RestClient{}, "Delete", func(ctx context.Context,
		url string, data map[string]interface{}) (base.Response, error) {
		return mockResponse, nil
	})

	// action
	getErr := client.RemoveNamespaceFromGroup(context.Background(), namespaceID, groupID)

	// assert
	if getErr != nil {
		t.Errorf("TestBaseClient_RemoveNamespaceFromGroup_Success failed, wantErr = nil, gotErr = %v", getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestBaseClient_RemoveNamespaceFromGroup_CodeError(t *testing.T) {
	// arrange
	namespaceID := "1"
	groupID := "1"
	client, err := NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}

	errCode := 1
	errMsg := "unknown error"
	mockResponse := base.Response{
		Error: map[string]interface{}{
			"code":        float64(errCode),
			"description": errMsg,
		},
	}
	wantErr := fmt.Errorf("remove namespace %s from group %s failed, "+
		"error code: %d, error msg: %s", namespaceID, groupID, errCode, errMsg)

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodFunc(&base.RestClient{}, "Delete", func(ctx context.Context,
		url string, data map[string]interface{}) (base.Response, error) {
		return mockResponse, nil
	})

	// action
	getErr := client.RemoveNamespaceFromGroup(context.Background(), namespaceID, groupID)

	// assert
	if !reflect.DeepEqual(wantErr, getErr) {
		t.Errorf("TestBaseClient_RemoveNamespaceFromGroup_CodeError failed, "+
			"wantErr = %v, gotErr = %v", wantErr, getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestBaseClient_GetNamespaceGroupByName_Success(t *testing.T) {
	// arrange
	name := "group1"
	client, err := NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}

	groups := make([]interface{}, 0)
	group := map[string]interface{}{
		"name": name,
		"ID":   "1",
	}
	groups = append(groups, group)

	mockResponse := base.Response{
		Error: map[string]interface{}{
			"code":        float64(0),
			"description": "0",
		},
		Data: groups,
	}

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodFunc(&base.RestClient{}, "Get", func(ctx context.Context,
		url string, data map[string]interface{}) (base.Response, error) {
		return mockResponse, nil
	})

	// action
	getRes, getErr := client.GetNamespaceGroupByName(context.Background(), name)

	// assert
	if !reflect.DeepEqual(group, getRes) || getErr != nil {
		t.Errorf("TestBaseClient_GetNamespaceGroupByName_Success failed, "+
			"wantRes = %v, gotRes = %v, wantErr = nil, gotErr = %v", group, getRes, getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestBaseClient_GetNamespaceGroupByName_RespFormatError(t *testing.T) {
	// arrange
	name := "group1"
	client, err := NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}

	mockResponse := base.Response{
		Error: map[string]interface{}{
			"code":        float64(0),
			"description": "0",
		},
		Data: "unknown format",
	}
	wantErr := fmt.Errorf("convert respData to arr failed, data: %v", mockResponse.Data)

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodFunc(&base.RestClient{}, "Get", func(ctx context.Context,
		url string, data map[string]interface{}) (base.Response, error) {
		return mockResponse, nil
	})

	// action
	getRes, getErr := client.GetNamespaceGroupByName(context.Background(), name)

	// assert
	if getRes != nil || !reflect.DeepEqual(wantErr, getErr) {
		t.Errorf("TestBaseClient_GetNamespaceGroupByName_RespFormatError failed, "+
			"wantRes = nil, gotRes = %v, wantErr = %v, gotErr = %v", getRes, wantErr, getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestBaseClient_GetNamespaceGroupByName_GroupFormatError(t *testing.T) {
	// arrange
	name := "group1"
	client, err := NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}

	groups := make([]interface{}, 0)
	group := "unknown format"
	groups = append(groups, group)

	mockResponse := base.Response{
		Error: map[string]interface{}{
			"code":        float64(0),
			"description": "0",
		},
		Data: groups,
	}
	wantErr := fmt.Errorf("convert group to arr failed, data: %v", group)

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodFunc(&base.RestClient{}, "Get", func(ctx context.Context,
		url string, data map[string]interface{}) (base.Response, error) {
		return mockResponse, nil
	})

	// action
	getRes, getErr := client.GetNamespaceGroupByName(context.Background(), name)

	// assert
	if getRes != nil || !reflect.DeepEqual(wantErr, getErr) {
		t.Errorf("TestBaseClient_GetNamespaceGroupByName_GroupFormatError failed, "+
			"wantRes = nil, gotRes = %v, wantErr = %v, gotErr = %v", getRes, wantErr, getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestBaseClient_CreateNamespaceGroup_Success(t *testing.T) {
	// arrange
	name := "group1"
	client, err := NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}

	group := map[string]interface{}{
		"name": name,
		"ID":   "1",
	}

	mockResponse := base.Response{
		Error: map[string]interface{}{
			"code":        float64(0),
			"description": "0",
		},
		Data: group,
	}

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodFunc(&base.RestClient{}, "Post", func(ctx context.Context,
		url string, data map[string]interface{}) (base.Response, error) {
		return mockResponse, nil
	})

	// action
	getRes, getErr := client.CreateNamespaceGroup(context.Background(), name)

	// assert
	if !reflect.DeepEqual(group, getRes) || getErr != nil {
		t.Errorf("TestBaseClient_CreateNamespaceGroup_Success failed,"+
			"wantRes = %v, gotRes = %v, wantErr = nil, gotErr = %v", group, getRes, getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestBaseClient_CreateNamespaceGroup_ExistSuccess(t *testing.T) {
	// arrange
	name := "group1"
	client, err := NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}

	group := map[string]interface{}{
		"name": name,
		"ID":   "1",
	}

	mockResponse := base.Response{
		Error: map[string]interface{}{
			"code":        float64(objectNameAlreadyExist),
			"description": "0",
		},
		Data: group,
	}

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodFunc(&base.RestClient{}, "Post", func(ctx context.Context,
		url string, data map[string]interface{}) (base.Response, error) {
		return mockResponse, nil
	}).ApplyMethodFunc(client, "GetNamespaceGroupByName", func(ctx context.Context,
		name string) (map[string]interface{}, error) {
		return group, nil
	})

	// action
	getRes, getErr := client.CreateNamespaceGroup(context.Background(), name)

	// assert
	if !reflect.DeepEqual(group, getRes) || getErr != nil {
		t.Errorf("TestBaseClient_CreateNamespaceGroup_ExistSuccess failed,"+
			"wantRes = %v, gotRes = %v, wantErr = nil, gotErr = %v", group, getRes, getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestBaseClient_CreateNamespaceGroup_CodeError(t *testing.T) {
	// arrange
	name := "group1"
	client, err := NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}

	errCode := 1
	errMsg := "unknown error"
	mockResponse := base.Response{
		Error: map[string]interface{}{
			"code":        float64(errCode),
			"description": errMsg,
		},
		Data: nil,
	}
	wantErr := fmt.Errorf("create namespacegroup %s failed, "+
		"error code: %d, error msg: %s", name, errCode, errMsg)

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodFunc(&base.RestClient{}, "Post", func(ctx context.Context,
		url string, data map[string]interface{}) (base.Response, error) {
		return mockResponse, nil
	})

	// action
	getRes, getErr := client.CreateNamespaceGroup(context.Background(), name)

	// assert
	if getRes != nil || !reflect.DeepEqual(wantErr, getErr) {
		t.Errorf("TestBaseClient_CreateNamespaceGroup_CodeError failed,"+
			"wantRes = nil, gotRes = %v, wantErr = %v, gotErr = %v", getRes, wantErr, getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestBaseClient_CreateNamespaceGroup_GroupFormatError(t *testing.T) {
	// arrange
	name := "group1"
	client, err := NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}

	mockResponse := base.Response{
		Error: map[string]interface{}{
			"code":        float64(0),
			"description": "0",
		},
		Data: nil,
	}
	wantErr := fmt.Errorf("convert namespaceGroup to map failed, data: %v", mockResponse.Data)

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodFunc(&base.RestClient{}, "Post", func(ctx context.Context,
		url string, data map[string]interface{}) (base.Response, error) {
		return mockResponse, nil
	})

	// action
	getRes, getErr := client.CreateNamespaceGroup(context.Background(), name)

	// assert
	if getRes != nil || !reflect.DeepEqual(wantErr, getErr) {
		t.Errorf("TestBaseClient_CreateNamespaceGroup_GroupFormatError failed,"+
			"wantRes = nil, gotRes = %v, wantErr = %v, gotErr = %v", getRes, wantErr, getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestBaseClient_DeleteNamespaceGroup_Success(t *testing.T) {
	// arrange
	id := "1"
	client, err := NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}

	mockResponse := base.Response{
		Error: map[string]interface{}{
			"code":        float64(0),
			"description": "0",
		},
	}

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodReturn(&base.RestClient{}, "Delete", mockResponse, nil)

	// action
	getErr := client.DeleteNamespaceGroup(context.Background(), id)

	// assert
	if getErr != nil {
		t.Errorf("TestBaseClient_DeleteNamespaceGroup_Success failed, wantErr = nil, gotErr = %v", getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestBaseClient_DeleteNamespaceGroup_NotExistSuccess(t *testing.T) {
	// arrange
	id := "1"
	client, err := NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}

	mockResponse := base.Response{
		Error: map[string]interface{}{
			"code":        float64(objectNotExist),
			"description": "namespacegroup is already exist",
		},
	}

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodReturn(&base.RestClient{}, "Delete", mockResponse, nil)

	// action
	getErr := client.DeleteNamespaceGroup(context.Background(), id)

	// assert
	if getErr != nil {
		t.Errorf("TestBaseClient_DeleteNamespaceGroup_NotExistSuccess failed, "+
			"wantErr = nil, gotErr = %v", getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestBaseClient_DeleteNamespaceGroup_CodeError(t *testing.T) {
	// arrange
	id := "1"
	client, err := NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}

	errCode := 1
	errMsg := "unknown error"
	mockResponse := base.Response{
		Error: map[string]interface{}{
			"code":        float64(errCode),
			"description": errMsg,
		},
	}
	wantErr := fmt.Errorf("delete namespacegroup %s failed, error code: %d, error msg: %s", id, errCode, errMsg)

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodReturn(&base.RestClient{}, "Delete", mockResponse, nil)

	// action
	getErr := client.DeleteNamespaceGroup(context.Background(), id)

	// assert
	if !reflect.DeepEqual(wantErr, getErr) {
		t.Errorf("TestBaseClient_DeleteNamespaceGroup_CodeError failed, "+
			"wantErr = %v, gotErr = %v", wantErr, getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestBaseClient_ExtendNamespace_Success(t *testing.T) {
	// arrange
	namespaceId := "1"
	capacity := int64(10)
	client, err := NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}

	mockResponse := base.Response{
		Error: map[string]interface{}{
			"code":        float64(0),
			"description": "0",
		},
	}

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodReturn(&base.RestClient{}, "Put", mockResponse, nil)

	// action
	getErr := client.ExtendNamespace(context.Background(), namespaceId, capacity)

	// assert
	if getErr != nil {
		t.Errorf("TestBaseClient_ExtendNamespace_Success failed, wantErr = nil, gotErr = %v", getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestBaseClient_ExtendNamespace_CodeFormatError(t *testing.T) {
	// arrange
	namespaceId := "1"
	capacity := int64(10)
	errorCode := 0
	client, err := NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}

	mockResponse := base.Response{
		Error: map[string]interface{}{
			"code":        errorCode,
			"description": "0",
		},
	}
	wantErr := fmt.Errorf("can not convert resp code %v to float64", errorCode)

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodReturn(&base.RestClient{}, "Put", mockResponse, nil)

	// action
	getErr := client.ExtendNamespace(context.Background(), namespaceId, capacity)

	// assert
	if !reflect.DeepEqual(wantErr, getErr) {
		t.Errorf("TestBaseClient_ExtendNamespace_CodeFormatError failed, "+
			"wantErr = %v, gotErr = %v", wantErr, getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestBaseClient_CreateNamespace_Success(t *testing.T) {
	// arrange
	params := CreateNamespaceParams{}
	client, err := NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}

	data := map[string]interface{}{
		"ID":   "1",
		"Name": "test",
	}
	mockResponse := base.Response{
		Error: map[string]interface{}{
			"code":        float64(0),
			"description": "0",
		},
		Data: data,
	}

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodReturn(&base.RestClient{}, "Post", mockResponse, nil)

	// action
	getRes, getErr := client.CreateNamespace(context.Background(), params)

	// assert
	if !reflect.DeepEqual(data, getRes) || getErr != nil {
		t.Errorf("TestBaseClient_CreateNamespace_Success failed, "+
			"wantRes = %v, gotRes = %v,  wantErr = nil, gotErr = %v", data, getRes, getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestBaseClient_CreateNamespace_InvalidParamsError(t *testing.T) {
	// arrange
	params := CreateNamespaceParams{}
	data := map[string]interface{}{
		"NAME":        params.Name,
		"PARENTID":    params.ParentId,
		"CAPACITY":    params.Capacity,
		"DESCRIPTION": params.Description,
	}
	client, err := NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}

	msg := "Enter a correct parameter."
	mockResponse := base.Response{
		Error: map[string]interface{}{
			"code":        float64(parameterIncorrect),
			"description": msg,
		},
	}
	wantErr := fmt.Errorf("create Namespace with incorrect parameters %v, "+
		"err code: %d, err msg: %s", data, parameterIncorrect, msg)

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodReturn(&base.RestClient{}, "Post", mockResponse, nil)

	// action
	getRes, getErr := client.CreateNamespace(context.Background(), params)

	// assert
	if getRes != nil || !reflect.DeepEqual(wantErr, getErr) {
		t.Errorf("TestBaseClient_CreateNamespace_InvalidParamsError failed, "+
			"wantRes = nil, gotRes = %v,  wantErr = %v, gotErr = %v", getRes, wantErr, getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestBaseClient_DeleteNamespace_Success(t *testing.T) {
	// arrange
	id := "1"
	client, err := NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}

	mockResponse := base.Response{
		Error: map[string]interface{}{
			"code":        float64(0),
			"description": "0",
		},
	}

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodReturn(&base.RestClient{}, "Delete", mockResponse, nil)

	// action
	getErr := client.DeleteNamespace(context.Background(), id)

	// assert
	if getErr != nil {
		t.Errorf("TestBaseClient_DeleteNamespace_Success failed, wantErr = nil, gotErr = %v", getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestBaseClient_DeleteNamespace_NotExistSuccess(t *testing.T) {
	// arrange
	id := "1"
	client, err := NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}

	mockResponse := base.Response{
		Error: map[string]interface{}{
			"code":        float64(namespaceNotExist),
			"description": "namespacegroup is already exist",
		},
	}

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodReturn(&base.RestClient{}, "Delete", mockResponse, nil)

	// action
	getErr := client.DeleteNamespace(context.Background(), id)

	// assert
	if getErr != nil {
		t.Errorf("TestBaseClient_DeleteNamespace_NotExistSuccess failed, "+
			"wantErr = nil, gotErr = %v", getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestBaseClient_DeleteNamespace_CodeError(t *testing.T) {
	// arrange
	id := "1"
	client, err := NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}

	errCode := 1
	errMsg := "unknown error"
	mockResponse := base.Response{
		Error: map[string]interface{}{
			"code":        float64(errCode),
			"description": errMsg,
		},
	}
	wantErr := fmt.Errorf("delete namespace %s failed, error code: %d, error msg: %s", id, errCode, errMsg)

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodReturn(&base.RestClient{}, "Delete", mockResponse, nil)

	// action
	getErr := client.DeleteNamespace(context.Background(), id)

	// assert
	if !reflect.DeepEqual(wantErr, getErr) {
		t.Errorf("TestBaseClient_DeleteNamespace_CodeError failed, "+
			"wantErr = %v, gotErr = %v", wantErr, getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestBaseClient_GetNamespaceCountOfMapping_Success(t *testing.T) {
	// arrange
	mappingId := "1"
	countStr := "10"
	client, err := NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}

	data := map[string]interface{}{
		"COUNT": countStr,
	}
	mockResponse := base.Response{
		Error: map[string]interface{}{
			"code":        float64(0),
			"description": "0",
		},
		Data: data,
	}
	wantCount := utils.ParseIntWithDefault(countStr, constants.DefaultIntBase, constants.DefaultIntBitSize, 0)

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodReturn(&base.RestClient{}, "Get", mockResponse, nil)

	// action
	getCount, getErr := client.GetNamespaceCountOfMapping(context.Background(), mappingId)

	// assert
	if wantCount != getCount || getErr != nil {
		t.Errorf("TestBaseClient_GetNamespaceCountOfMapping_Success failed, "+
			"wantCount = %d, gotCount = %d, wantErr = nil, gotErr = %v", wantCount, getCount, getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestBaseClient_GetNamespaceCountOfMapping_CountFormatError(t *testing.T) {
	// arrange
	mappingId := "1"
	countStr := float64(10)
	client, err := NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}

	data := map[string]interface{}{
		"COUNT": countStr,
	}
	mockResponse := base.Response{
		Error: map[string]interface{}{
			"code":        float64(0),
			"description": "0",
		},
		Data: data,
	}
	wantErr := fmt.Errorf("convert countStr to string failed, data: %v", countStr)

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodReturn(&base.RestClient{}, "Get", mockResponse, nil)

	// action
	getCount, getErr := client.GetNamespaceCountOfMapping(context.Background(), mappingId)

	// assert
	if getCount != 0 || !reflect.DeepEqual(wantErr, getErr) {
		t.Errorf("TestBaseClient_GetNamespaceCountOfMapping_CountFormatError failed, "+
			"wantCount = 0, gotCount = %d, wantErr = %v, gotErr = %v", getCount, wantErr, getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestBaseClient_GetNamespaceCountOfHost_Success(t *testing.T) {
	// arrange
	mappingId := "1"
	countStr := "10"
	client, err := NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}

	data := map[string]interface{}{
		"COUNT": countStr,
	}
	mockResponse := base.Response{
		Error: map[string]interface{}{
			"code":        float64(0),
			"description": "0",
		},
		Data: data,
	}
	wantCount := utils.ParseIntWithDefault(countStr, constants.DefaultIntBase, constants.DefaultIntBitSize, 0)

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodReturn(&base.RestClient{}, "Get", mockResponse, nil)

	// action
	getCount, getErr := client.GetNamespaceCountOfHost(context.Background(), mappingId)

	// assert
	if wantCount != getCount || getErr != nil {
		t.Errorf("TestBaseClient_GetNamespaceCountOfHost_Success failed, "+
			"wantCount = %d, gotCount = %d, wantErr = nil, gotErr = %v", wantCount, getCount, getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestBaseClient_GetNamespaceCountOfHost_CountFormatError(t *testing.T) {
	// arrange
	mappingId := "1"
	countStr := float64(10)
	client, err := NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}

	data := map[string]interface{}{
		"COUNT": countStr,
	}
	mockResponse := base.Response{
		Error: map[string]interface{}{
			"code":        float64(0),
			"description": "0",
		},
		Data: data,
	}
	wantErr := fmt.Errorf("convert countStr to string failed, data: %v", countStr)

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodReturn(&base.RestClient{}, "Get", mockResponse, nil)

	// action
	getCount, getErr := client.GetNamespaceCountOfHost(context.Background(), mappingId)

	// assert
	if getCount != 0 || !reflect.DeepEqual(wantErr, getErr) {
		t.Errorf("TestBaseClient_GetNamespaceCountOfHost_CountFormatError failed, "+
			"wantCount = 0, gotCount = %d, wantErr = %v, gotErr = %v", getCount, wantErr, getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestBaseClient_GetHostNamespaceId_Success(t *testing.T) {
	// arrange
	hostId := "1"
	namespaceId := "1"
	client, err := NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}

	namespaces := make([]interface{}, 0)
	namespace := map[string]interface{}{
		"name":              "namespace1",
		"ID":                "1",
		"ASSOCIATEMETADATA": "{\"hostNamespaceID\":1}",
	}
	namespaces = append(namespaces, namespace)
	mockResponse := base.Response{
		Error: map[string]interface{}{
			"code":        float64(0),
			"description": "0",
		},
		Data: namespaces,
	}

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodReturn(&base.RestClient{}, "Get", mockResponse, nil)

	// action
	getId, getErr := client.GetHostNamespaceId(context.Background(), hostId, namespaceId)

	// assert
	if getId != "1" || getErr != nil {
		t.Errorf("TestBaseClient_GetHostNamespaceId_Success failed, "+
			"wantId = 1, getId = %s, wantErr = nil, gotErr = %v", getId, getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestBaseClient_GetHostNamespaceId_NotExistIdError(t *testing.T) {
	// arrange
	hostId := "1"
	namespaceId := "1"
	client, err := NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}

	namespaces := make([]interface{}, 0)
	mockResponse := base.Response{
		Error: map[string]interface{}{
			"code":        float64(0),
			"description": "0",
		},
		Data: namespaces,
	}
	wantErr := fmt.Errorf("can not get the hostNamespaceId of host %s, namespace %s", hostId, namespaceId)

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodReturn(&base.RestClient{}, "Get", mockResponse, nil)

	// action
	getId, getErr := client.GetHostNamespaceId(context.Background(), hostId, namespaceId)

	// assert
	if getId != "" || !reflect.DeepEqual(wantErr, getErr) {
		t.Errorf("TestBaseClient_GetHostNamespaceId_NotExistIdError failed, "+
			"wantId = , getId = %s, wantErr = %v, gotErr = %v", getId, wantErr, getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestBaseClient_GetHostNamespaceId_JsonUnmarshalError(t *testing.T) {
	// arrange
	hostId := "1"
	namespaceId := "1"
	client, err := NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}

	namespaces := make([]interface{}, 0)
	namespace := map[string]interface{}{
		"name":              "namespace1",
		"ID":                "1",
		"ASSOCIATEMETADATA": "{\"hostNamespaceID\":1}",
	}
	namespaces = append(namespaces, namespace)
	mockResponse := base.Response{
		Error: map[string]interface{}{
			"code":        float64(0),
			"description": "0",
		},
		Data: namespaces,
	}
	mockErr := errors.New("json unmarshal err")
	wantErr := fmt.Errorf("unmarshal associateData fail while "+
		"getting the hostNamespaceId of host %s, namespace %s, error: %v", hostId, namespaceId, mockErr)

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodReturn(&base.RestClient{}, "Get", mockResponse, nil).
		ApplyFunc(json.Unmarshal, func(data []byte, v any) error {
			return mockErr
		})

	// action
	getId, getErr := client.GetHostNamespaceId(context.Background(), hostId, namespaceId)

	// assert
	if getId != "" || !reflect.DeepEqual(wantErr, getErr) {
		t.Errorf("TestBaseClient_GetHostNamespaceId_JsonUnmarshalError failed, "+
			"wantId = , getId = %s, wantErr = %v, gotErr = %v", getId, wantErr, getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestBaseClient_UpdateNamespace_Success(t *testing.T) {
	// arrange
	namespaceID := "1"
	params := map[string]interface{}{}
	client, err := NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}

	mockResponse := base.Response{
		Error: map[string]interface{}{
			"code":        float64(0),
			"description": "0",
		},
	}

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodReturn(&base.RestClient{}, "Put", mockResponse, nil)

	// action
	getErr := client.UpdateNamespace(context.Background(), namespaceID, params)

	// assert
	if getErr != nil {
		t.Errorf("TestBaseClient_UpdateNamespace_Success failed, wantErr = nil, gotErr = %v", getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}
