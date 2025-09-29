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

// Package attacher provide storage mapping or unmapping
package attacher

import (
	"context"
	"errors"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/fusionstorage/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	logName = "attacherTest.log"
)

var testClient *client.RestClient

func TestMain(m *testing.M) {
	log.MockInitLogging(logName)
	defer log.MockStopLogging(logName)

	m.Run()
}

func TestVolumeAttacher_getTargetPortalsDynamic_Success(t *testing.T) {
	// arrange
	attacher := NewAttacher(VolumeAttacherConfig{Cli: testClient, IscsiLinks: 3})
	mockPortals := []*client.IscsiLink{{
		IP:            "ip1",
		IscsiLinksNum: 1,
		TargetName:    "target1",
		IscsiPortal:   "portal1",
	}}

	// mock
	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn(testClient, "IsSupportDynamicLinks", true, nil).
		ApplyMethodReturn(testClient, "QueryDynamicLinks", mockPortals, nil)

	// action
	tgtPortals, tgtIQNs, getErr := attacher.getTargetPortalsDynamic(context.Background(), "host", "pool")

	// assert
	assert.Nil(t, getErr)
	assert.Equal(t, 1, len(tgtPortals))
	assert.Equal(t, 1, len(tgtIQNs))
	assert.Equal(t, mockPortals[0].IscsiPortal, tgtPortals[0])
	assert.Equal(t, mockPortals[0].TargetName, tgtIQNs[0])
}

func TestVolumeAttacher_getTargetPortalsDynamic_UnsupportDynamic(t *testing.T) {
	// arrange
	attacher := NewAttacher(VolumeAttacherConfig{Cli: testClient, IscsiLinks: 3})

	// mock
	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn(testClient, "IsSupportDynamicLinks", false, nil)

	// act
	gotPortals, gotIQNs, gotErr := attacher.getTargetPortalsDynamic(context.Background(), "host", "pool")

	// assert
	assert.ErrorContains(t, gotErr, "the storage does not support query portals dynamically")
	assert.Nil(t, gotPortals)
	assert.Nil(t, gotIQNs)
}

func TestVolumeAttacher_getTargetPortalsDynamic_QueryDynamicFailed(t *testing.T) {
	// arrange
	wantErr := errors.New("mock query failed")

	attacher := NewAttacher(VolumeAttacherConfig{Cli: testClient, IscsiLinks: 3})

	// mock
	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn(testClient, "IsSupportDynamicLinks", true, nil).
		ApplyMethodReturn(testClient, "QueryDynamicLinks", nil, wantErr)

	// act
	gotPortals, gotIQNs, gotErr := attacher.getTargetPortalsDynamic(context.Background(), "host", "pool")

	// assert
	assert.ErrorIs(t, gotErr, wantErr)
	assert.Nil(t, gotPortals)
	assert.Nil(t, gotIQNs)
}

func TestVolumeAttacher_getMappingProperties_StaticPortalsSuccess(t *testing.T) {
	// arrange
	attacher := &VolumeAttacher{
		portals: []string{"portal1", "portal2"},
		cli:     testClient,
	}
	mockLun := &lunInfo{wwn: "wwn1", poolName: "pool1"}
	mockPortals := []string{"portal1", "portal2"}
	mockIQNs := []string{"iqn1", "iqn2"}
	mockHostLunId := "1"
	mockHost := "host1"

	// mock
	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyPrivateMethod(attacher, "getTargetPortalsStatic",
		func(_ context.Context) ([]string, []string, error) {
			return mockPortals, mockIQNs, nil
		})

	// act
	gotProps, gotErr := attacher.getMappingProperties(context.Background(), mockLun, mockHostLunId, mockHost)

	// assert
	assert.Nil(t, gotErr)
	assert.Equal(t, mockLun.wwn, gotProps["tgtLunWWN"])
	assert.Equal(t, mockPortals, gotProps["tgtPortals"])
	assert.Equal(t, mockIQNs, gotProps["tgtIQNs"])
	assert.Equal(t, []string{mockHostLunId, mockHostLunId}, gotProps["tgtHostLUNs"])
}

func TestVolumeAttacher_getMappingProperties_StaticPortalsError(t *testing.T) {
	// arrange
	attacher := &VolumeAttacher{
		portals: []string{"portal1", "portal2"},
		cli:     testClient,
	}
	mockLun := &lunInfo{wwn: "wwn1", poolName: "pool1"}
	mockHostLunId := "1"
	mockHost := "host1"
	wantErr := errors.New("failed to get portals")

	// mock
	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyPrivateMethod(attacher, "getTargetPortalsStatic",
		func(_ context.Context) ([]string, []string, error) {
			return nil, nil, wantErr
		})

	// act
	gotProps, gotErr := attacher.getMappingProperties(context.Background(), mockLun, mockHostLunId, mockHost)

	// assert
	assert.Equal(t, wantErr, gotErr)
	assert.Nil(t, gotProps)
}

func TestVolumeAttacher_getMappingProperties_DynamicPortalsSuccess(t *testing.T) {
	// arrange
	attacher := &VolumeAttacher{
		portals: []string{},
		cli:     testClient,
	}
	mockLun := &lunInfo{wwn: "wwn1", poolName: "pool1"}
	mockPortals := []string{"portal1", "portal2"}
	mockIQNs := []string{"iqn1", "iqn2"}
	mockHostLunId := "1"
	mockHost := "host1"

	// mock
	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyPrivateMethod(attacher, "getTargetPortalsDynamic", func(
		_ context.Context, hostName, poolName string) ([]string, []string, error) {
		return mockPortals, mockIQNs, nil
	})

	// act
	gotProps, gotErr := attacher.getMappingProperties(context.Background(), mockLun, mockHostLunId, mockHost)

	// assert
	assert.Nil(t, gotErr)
	assert.Equal(t, mockLun.wwn, gotProps["tgtLunWWN"])
	assert.Equal(t, mockPortals, gotProps["tgtPortals"])
	assert.Equal(t, mockIQNs, gotProps["tgtIQNs"])
	assert.Equal(t, []string{mockHostLunId, mockHostLunId}, gotProps["tgtHostLUNs"])
}

func TestVolumeAttacher_getMappingProperties_DynamicPortalsError(t *testing.T) {
	// arrange
	attacher := &VolumeAttacher{
		portals: []string{},
		cli:     testClient,
	}
	mockLun := &lunInfo{wwn: "wwn1", poolName: "pool1"}
	mockHostLunId := "1"
	mockHost := "host1"
	wantErr := errors.New("failed to get dynamic portals")

	// mock
	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyPrivateMethod(attacher, "getTargetPortalsDynamic", func(
		_ context.Context, hostName, poolName string) ([]string, []string, error) {
		return nil, nil, wantErr
	})

	// act
	gotProps, gotErr := attacher.getMappingProperties(context.Background(), mockLun, mockHostLunId, mockHost)

	// assert
	assert.Equal(t, wantErr, gotErr)
	assert.Nil(t, gotProps)
}

func TestVolumeAttacher_getLunInfo_Success(t *testing.T) {
	// arrange
	attacher := NewAttacher(VolumeAttacherConfig{Cli: testClient})
	lunMap := map[string]interface{}{
		"wwn":    "wwn1",
		"poolId": float64(1),
	}
	poolMap := map[string]interface{}{
		"poolName": "poolName1",
	}
	wantLun := &lunInfo{
		name:     "lun1",
		wwn:      "wwn1",
		poolName: "poolName1",
	}

	// mock
	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn(testClient, "GetVolumeByName", lunMap, nil)
	mock.ApplyMethodReturn(testClient, "GetPoolById", poolMap, nil)

	// act
	lun, gotErr := attacher.getLunInfo(context.Background(), "lun1")

	// assert
	assert.Nil(t, gotErr)
	assert.Equal(t, wantLun, lun)
}

func TestVolumeAttacher_getLunInfo_GetVolumeError(t *testing.T) {
	// arrange
	attacher := NewAttacher(VolumeAttacherConfig{Cli: testClient})
	wantErr := errors.New("get volume error")

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn(testClient, "GetVolumeByName", nil, wantErr)

	// act
	gotLun, gotErr := attacher.getLunInfo(context.Background(), "lun1")

	// assert
	assert.ErrorContains(t, gotErr, wantErr.Error())
	assert.Nil(t, gotLun)
}

func TestVolumeAttacher_getLunInfo_LunNotExist(t *testing.T) {
	// arrange
	attacher := NewAttacher(VolumeAttacherConfig{Cli: testClient})

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn(testClient, "GetVolumeByName", nil, nil)

	// act
	gotLun, gotErr := attacher.getLunInfo(context.Background(), "lun1")

	// assert
	assert.Nil(t, gotErr)
	assert.Nil(t, gotLun)
}

func TestVolumeAttacher_getLunInfo_WWNNotFound(t *testing.T) {
	// arrange
	attacher := NewAttacher(VolumeAttacherConfig{Cli: testClient})
	lunMap := map[string]interface{}{
		"poolId": float64(1),
	}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn(testClient, "GetVolumeByName", lunMap, nil)

	// act
	gotLun, gotErr := attacher.getLunInfo(context.Background(), "lun1")

	// assert
	assert.ErrorContains(t, gotErr, "can not find wwn in lun")
	assert.Nil(t, gotLun)
}

func TestVolumeAttacher_getLunInfo_PoolIDNotFound(t *testing.T) {
	// arrange
	attacher := NewAttacher(VolumeAttacherConfig{Cli: testClient})
	lunMap := map[string]interface{}{
		"wwn": "wwn1",
	}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn(testClient, "GetVolumeByName", lunMap, nil)

	// act
	gotLun, gotErr := attacher.getLunInfo(context.Background(), "lun1")

	// assert
	assert.ErrorContains(t, gotErr, "can not find poolId in lun")
	assert.Nil(t, gotLun)
}

func TestVolumeAttacher_getLunInfo_GetPoolError(t *testing.T) {
	// arrange
	attacher := NewAttacher(VolumeAttacherConfig{Cli: testClient})
	lunMap := map[string]interface{}{
		"wwn":    "wwn1",
		"poolId": float64(1),
	}
	wantErr := errors.New("get pool error")

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn(testClient, "GetVolumeByName", lunMap, nil)
	mock.ApplyMethodReturn(testClient, "GetPoolById", nil, wantErr)

	// act
	gotLun, gotErr := attacher.getLunInfo(context.Background(), "lun1")

	// assert
	assert.ErrorIs(t, gotErr, wantErr)
	assert.Nil(t, gotLun)
}

func TestVolumeAttacher_getLunInfo_PoolNameNotFound(t *testing.T) {
	// arrange
	attacher := NewAttacher(VolumeAttacherConfig{Cli: testClient})
	lunMap := map[string]interface{}{
		"wwn":    "wwn1",
		"poolId": float64(1),
	}
	poolMap := map[string]interface{}{}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn(testClient, "GetVolumeByName", lunMap, nil)
	mock.ApplyMethodReturn(testClient, "GetPoolById", poolMap, nil)

	// act
	gotLun, gotErr := attacher.getLunInfo(context.Background(), "lun1")

	// assert
	assert.ErrorContains(t, gotErr, "can not find poolName in pool")
	assert.Nil(t, gotLun)
}
