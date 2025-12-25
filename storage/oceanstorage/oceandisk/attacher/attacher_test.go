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

// Package attacher provide operations of volume attach
package attacher

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage"
	baseAttacher "github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/base/attacher"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceandisk/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	logName = "oceandisk_attacher.log"
)

func TestMain(m *testing.M) {
	log.MockInitLogging(logName)
	defer log.MockStopLogging(logName)

	m.Run()
}

func TestOceandiskAttacher_addToNamespaceGroupMapping_Success(t *testing.T) {
	// arrange
	groupName, groupID, mappingID := "group1", "1", "1"
	newClient, err := client.NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}
	attacher := NewOceanDiskAttacher(OceanDiskAttacherConfig{Cli: newClient})

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodReturn(newClient, "QueryAssociateNamespaceGroup", []interface{}{}, nil).
		ApplyMethodReturn(newClient, "AddGroupToMapping", nil)

	// action
	getErr := attacher.addToNamespaceGroupMapping(context.Background(), groupName, groupID, mappingID)

	// assert
	if getErr != nil {
		t.Errorf("TestOceandiskAttacher_addToNamespaceGroupMapping_Success failed, "+
			"wantErr = nil, gotErr = %v", getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestOceandiskAttacher_addToNamespaceGroupMapping_AlreadyExistSuccess(t *testing.T) {
	// arrange
	groupName, groupID, mappingID := "group1", "1", "1"
	newClient, err := client.NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}
	attacher := NewOceanDiskAttacher(OceanDiskAttacherConfig{Cli: newClient})

	var mockResponse []interface{}
	group := map[string]interface{}{
		"NAME": groupName,
	}
	mockResponse = append(mockResponse, group)

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodReturn(newClient, "QueryAssociateNamespaceGroup", mockResponse, nil)

	// action
	getErr := attacher.addToNamespaceGroupMapping(context.Background(), groupName, groupID, mappingID)

	// assert
	if getErr != nil {
		t.Errorf("TestOceandiskAttacher_addToNamespaceGroupMapping_AlreadyExistSuccess failed, "+
			"wantErr = nil, gotErr = %v", getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestOceandiskAttacher_addToNamespaceGroupMapping_QueryGroupError(t *testing.T) {
	// arrange
	groupName, groupID, mappingID := "group1", "1", "1"
	newClient, err := client.NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}
	attacher := NewOceanDiskAttacher(OceanDiskAttacherConfig{Cli: newClient})
	mockErr := errors.New("query group err")
	wantErr := fmt.Errorf("query associated namespace groups of mapping %s error: %v", mappingID, mockErr)

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodReturn(newClient, "QueryAssociateNamespaceGroup", nil, mockErr)

	// action
	getErr := attacher.addToNamespaceGroupMapping(context.Background(), groupName, groupID, mappingID)

	// assert
	if !reflect.DeepEqual(wantErr, getErr) {
		t.Errorf("TestOceandiskAttacher_addToNamespaceGroupMapping_QueryGroupError failed, "+
			"wantErr = %v, gotErr = %v", wantErr, getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestOceandiskAttacher_addToNamespaceGroupMapping_FormatGroupError(t *testing.T) {
	// arrange
	groupName, groupID, mappingID := "group1", "1", "1"
	newClient, err := client.NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}
	attacher := NewOceanDiskAttacher(OceanDiskAttacherConfig{Cli: newClient})

	var mockResponse []interface{}
	group := "invalid format group"
	mockResponse = append(mockResponse, group)

	wantErr := fmt.Errorf("invalid group type. Expected 'map[string]interface{}', found %T", group)

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodReturn(newClient, "QueryAssociateNamespaceGroup", mockResponse, nil)

	// action
	getErr := attacher.addToNamespaceGroupMapping(context.Background(), groupName, groupID, mappingID)

	// assert
	if !reflect.DeepEqual(wantErr, getErr) {
		t.Errorf("TestOceandiskAttacher_addToNamespaceGroupMapping_FormatGroupError failed, "+
			"wantErr = %v, gotErr = %v", wantErr, getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestOceandiskAttacher_addToNamespaceGroupMapping_AddGroupToMappingError(t *testing.T) {
	// arrange
	groupName, groupID, mappingID := "group1", "1", "1"
	newClient, err := client.NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}
	attacher := NewOceanDiskAttacher(OceanDiskAttacherConfig{Cli: newClient})
	mockErr := errors.New("add group to mapping err")
	wantErr := fmt.Errorf("add namespace group %s to mapping %s error: %v", groupID, mappingID, mockErr)

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodReturn(newClient, "QueryAssociateNamespaceGroup", []interface{}{}, nil).
		ApplyMethodReturn(newClient, "AddGroupToMapping", mockErr)

	// action
	getErr := attacher.addToNamespaceGroupMapping(context.Background(), groupName, groupID, mappingID)

	// assert
	if !reflect.DeepEqual(wantErr, getErr) {
		t.Errorf("TestOceandiskAttacher_addToNamespaceGroupMapping_AddGroupToMappingError failed, "+
			"wantErr = %v, gotErr = %v", wantErr, getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestOceandiskAttacher_createNamespaceGroup_Success(t *testing.T) {
	// arrange
	namespaceId, hostID, mappingID := "1", "1", "1"
	newClient, err := client.NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}
	attacher := NewOceanDiskAttacher(OceanDiskAttacherConfig{Cli: newClient})

	group := map[string]interface{}{
		"NAME": attacher.getNamespaceGroupName(hostID),
		"ID":   "1",
	}

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodReturn(newClient, "QueryAssociateNamespaceGroup", []interface{}{}, nil).
		ApplyMethodReturn(newClient, "GetNamespaceGroupByName", map[string]interface{}{}, nil).
		ApplyMethodReturn(newClient, "CreateNamespaceGroup", group, nil).
		ApplyMethodReturn(newClient, "AddNamespaceToGroup", nil).
		ApplyPrivateMethod(&OceandiskAttacher{}, "addToNamespaceGroupMapping",
			func(ctx context.Context, groupName, groupID, mappingID string) error {
				return nil
			})

	// action
	getErr := attacher.createNamespaceGroup(context.Background(), namespaceId, hostID, mappingID)

	// assert
	if getErr != nil {
		t.Errorf("TestOceandiskAttacher_createNamespaceGroup_Success failed, "+
			"wantErr = nil, gotErr = %v", getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestOceandiskAttacher_createNamespaceGroup_GroupAlreadyExistSuccess(t *testing.T) {
	// arrange
	namespaceId, hostID, mappingID := "1", "1", "1"
	newClient, err := client.NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}
	attacher := NewOceanDiskAttacher(OceanDiskAttacherConfig{Cli: newClient})

	var mockResponse []interface{}
	group := map[string]interface{}{
		"NAME": attacher.getNamespaceGroupName(hostID),
		"ID":   "1",
	}
	mockResponse = append(mockResponse, group)

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodReturn(newClient, "QueryAssociateNamespaceGroup", mockResponse, nil).
		ApplyPrivateMethod(&OceandiskAttacher{}, "addToNamespaceGroupMapping",
			func(ctx context.Context, groupName, groupID, mappingID string) error {
				return nil
			})

	// action
	getErr := attacher.createNamespaceGroup(context.Background(), namespaceId, hostID, mappingID)

	// assert
	if getErr != nil {
		t.Errorf("TestOceandiskAttacher_createNamespaceGroup_GroupAlreadyExistSuccess failed, "+
			"wantErr = nil, gotErr = %v", getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestOceandiskAttacher_createNamespaceGroup_GetGroupError(t *testing.T) {
	// arrange
	namespaceId, hostID, mappingID := "1", "1", "1"
	newClient, err := client.NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}
	attacher := NewOceanDiskAttacher(OceanDiskAttacherConfig{Cli: newClient})
	namespaceGroupName := attacher.getNamespaceGroupName(hostID)
	mockErr := errors.New("get group err")
	wantErr := fmt.Errorf("get namespacegroup by name %s error: %v", namespaceGroupName, mockErr)

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodReturn(newClient, "QueryAssociateNamespaceGroup", []interface{}{}, nil).
		ApplyMethodReturn(newClient, "GetNamespaceGroupByName",
			map[string]interface{}{}, mockErr)

	// action
	getErr := attacher.createNamespaceGroup(context.Background(), namespaceId, hostID, mappingID)

	// assert
	if !reflect.DeepEqual(wantErr, getErr) {
		t.Errorf("TestOceandiskAttacher_createNamespaceGroup_GetGroupError failed, "+
			"wantErr = %v, gotErr = %v", wantErr, getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestOceandiskAttacher_doMapping_Success(t *testing.T) {
	// arrange
	hostID, namespaceName, uniqueId, hostNamespaceId := "1", "namespace1", "uniqueID1", "5"
	newClient, err := client.NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}
	attacher := NewOceanDiskAttacher(OceanDiskAttacherConfig{Cli: newClient, Protocol: "roce"})

	namespace := map[string]interface{}{
		"NAME":  namespaceName,
		"ID":    "1",
		"NGUID": uniqueId,
	}

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodReturn(newClient, "GetNamespaceByName", namespace, nil).
		ApplyMethodReturn(&baseAttacher.AttachmentManager{}, "CreateMapping", "1", nil).
		ApplyMethodReturn(&baseAttacher.AttachmentManager{}, "CreateHostGroup", nil).
		ApplyPrivateMethod(&OceandiskAttacher{}, "createNamespaceGroup",
			func(ctx context.Context, namespaceID, hostID, mappingID string) error {
				return nil
			}).
		ApplyMethodReturn(newClient, "GetHostNamespaceId", hostNamespaceId, nil)

	// action
	getNamespaceUniqueId, getHostNamespaceId, getErr := attacher.doMapping(context.Background(), hostID, namespaceName)

	// assert
	if getNamespaceUniqueId != uniqueId || getHostNamespaceId != hostNamespaceId || getErr != nil {
		t.Errorf("TestOceandiskAttacher_doMapping_Success failed, "+
			"wantUniqueId = %s, gotUniqueId = %s, wantHostNamespaceId = %s, gotHostNamespaceId = %s, "+
			"wantErr = nil, gotErr = %v", uniqueId, getNamespaceUniqueId, hostNamespaceId, getHostNamespaceId, getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestOceandiskAttacher_doMapping_NotExistError(t *testing.T) {
	// arrange
	hostID, namespaceName := "1", "namespace1"
	newClient, err := client.NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}
	attacher := NewOceanDiskAttacher(OceanDiskAttacherConfig{Cli: newClient})
	wantErr := fmt.Errorf("namespace %s not exist for attaching", namespaceName)

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodReturn(newClient, "GetNamespaceByName", map[string]interface{}{}, nil)

	// action
	getNamespaceUniqueId, getHostNamespaceId, getErr := attacher.doMapping(context.Background(), hostID, namespaceName)

	// assert
	if getNamespaceUniqueId != "" || getHostNamespaceId != "" || !reflect.DeepEqual(wantErr, getErr) {
		t.Errorf("TestOceandiskAttacher_doMapping_Success failed, "+
			"wantUniqueId = , gotUniqueId = %s, wantHostNamespaceId = , gotHostNamespaceId = %s, "+
			"wantErr = %v, gotErr = %v", getNamespaceUniqueId, getHostNamespaceId, wantErr, getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestOceandiskAttacher_ControllerAttach_Success(t *testing.T) {
	// arrange
	namespaceName, hostName := "namespace1", "host1"
	parameters := map[string]interface{}{}
	newClient, err := client.NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}
	attacher := NewOceanDiskAttacher(OceanDiskAttacherConfig{Cli: newClient})
	attacher.Alua = map[string]interface{}{
		hostName: map[string]interface{}{
			"accessMode": 0,
		},
	}
	wantRes := map[string]interface{}{}

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodReturn(&baseAttacher.AttachmentManager{},
		"GetHost", map[string]interface{}{"ID": "1", "NAME": hostName}, nil).
		ApplyMethodReturn(newClient, "UpdateHost", nil).
		ApplyMethodReturn(&baseAttacher.AttachmentManager{}, "AttachRoCE", map[string]interface{}{}, nil).
		ApplyPrivateMethod(attacher, "doMapping",
			func(ctx context.Context, hostID, namespaceName string) (string, string, error) {
				return "", "", nil
			}).
		ApplyMethodReturn(&baseAttacher.AttachmentManager{}, "GetMappingProperties", wantRes, nil)

	// action
	getRes, getErr := attacher.ControllerAttach(context.Background(), namespaceName, parameters)

	// assert
	if !reflect.DeepEqual(wantRes, getRes) || getErr != nil {
		t.Errorf("TestOceandiskAttacher_ControllerAttach_Success failed, "+
			"wantRes = %v, getRes = %s, wantErr = nil, gotErr = %v", wantRes, getRes, getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestOceandiskAttacher_doUnmapping_Success(t *testing.T) {
	// arrange
	hostID, namespaceName, uniqueId := "1", "namespace1", "uniqueID1"
	newClient, err := client.NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}
	attacher := NewOceanDiskAttacher(OceanDiskAttacherConfig{Cli: newClient, Protocol: "roce"})

	namespace := map[string]interface{}{
		"NAME":  namespaceName,
		"ID":    "1",
		"NGUID": uniqueId,
	}
	var mockResponse []interface{}
	group := map[string]interface{}{
		"NAME": attacher.getNamespaceGroupName(hostID),
		"ID":   "1",
	}
	mockResponse = append(mockResponse, group)
	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodReturn(newClient, "GetNamespaceByName", namespace, nil).
		ApplyMethodReturn(newClient, "QueryAssociateNamespaceGroup", mockResponse, nil).
		ApplyMethodReturn(newClient, "RemoveNamespaceFromGroup", nil)

	// action
	getWwn, getErr := attacher.doUnmapping(context.Background(), hostID, namespaceName)

	// assert
	if getWwn != uniqueId || getErr != nil {
		t.Errorf("TestOceandiskAttacher_doUnmapping_Success failed, "+
			"wantUniqueId = %s, gotUniqueId = %s, wantErr = nil, gotErr = %v", uniqueId, getWwn, getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestOceandiskAttacher_ControllerDetach_Success(t *testing.T) {
	// arrange
	namespaceName := "namespace1"
	parameters := map[string]interface{}{}
	newClient, err := client.NewClient(context.Background(), &storage.NewClientConfig{})
	if err != nil {
		return
	}
	attacher := NewOceanDiskAttacher(OceanDiskAttacherConfig{Cli: newClient})

	wantWwn := "want wwn"

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodReturn(&baseAttacher.AttachmentManager{},
		"GetHost", map[string]interface{}{"ID": "1"}, nil).
		ApplyPrivateMethod(attacher, "doUnmapping",
			func(ctx context.Context, hostID, namespaceName string) (string, error) {
				return wantWwn, nil
			})

	// action
	getWwn, getErr := attacher.ControllerDetach(context.Background(), namespaceName, parameters)

	// assert
	if wantWwn != getWwn || getErr != nil {
		t.Errorf("TestOceandiskAttacher_ControllerDetach_Success failed, "+
			"wantWwn = %s, getWwn = %s, wantErr = nil, gotErr = %v", wantWwn, getWwn, getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}
