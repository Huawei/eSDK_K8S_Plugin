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

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	cfg "github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app/config"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/fusionstorage/client"
	baseAttacher "github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/base/attacher"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
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
		_ context.Context, hostName, poolName string,
	) ([]string, []string, error) {
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
		_ context.Context, hostName, poolName string,
	) ([]string, []string, error) {
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

func TestVolumeAttacher_getHostName_EmptyPrefix(t *testing.T) {
	origGetGlobalConfig := app.GetGlobalConfig
	defer func() { app.GetGlobalConfig = origGetGlobalConfig }()

	mockCfg := cfg.MockCompletedConfig()
	mockCfg.HostNamePrefix = ""
	mockCfg.HostNamePrefixSet = false
	app.GetGlobalConfig = func() *cfg.CompletedConfig {
		return mockCfg
	}

	attacher := &VolumeAttacher{}

	got := attacher.getHostNameWithPrefix("mynode")
	assert.Equal(t, "mynode", got)
}

func TestVolumeAttacher_getHostName_ExplicitEmptyPrefix(t *testing.T) {
	origGetGlobalConfig := app.GetGlobalConfig
	defer func() { app.GetGlobalConfig = origGetGlobalConfig }()

	// When --host-name-prefix="" is explicitly set, FusionStorage should still have no prefix
	mockCfg := cfg.MockCompletedConfig()
	mockCfg.HostNamePrefix = ""
	mockCfg.HostNamePrefixSet = true
	app.GetGlobalConfig = func() *cfg.CompletedConfig {
		return mockCfg
	}

	attacher := &VolumeAttacher{}

	got := attacher.getHostNameWithPrefix("mynode")
	assert.Equal(t, "mynode", got) // no prefix for FusionStorage
}

func TestVolumeAttacher_getHostName_CustomPrefix(t *testing.T) {
	origGetGlobalConfig := app.GetGlobalConfig
	defer func() { app.GetGlobalConfig = origGetGlobalConfig }()

	mockCfg := cfg.MockCompletedConfig()
	mockCfg.HostNamePrefix = "prod_"
	mockCfg.HostNamePrefixSet = true
	app.GetGlobalConfig = func() *cfg.CompletedConfig {
		return mockCfg
	}

	attacher := &VolumeAttacher{}

	got := attacher.getHostNameWithPrefix("mynode")
	assert.Equal(t, "prod_mynode", got)
}

func TestVolumeAttacher_getHostName_LongHostNameNoTruncation(t *testing.T) {
	origGetGlobalConfig := app.GetGlobalConfig
	defer func() { app.GetGlobalConfig = origGetGlobalConfig }()

	mockCfg := cfg.MockCompletedConfig()
	mockCfg.HostNamePrefix = "k8s_"
	mockCfg.HostNamePrefixSet = true
	app.GetGlobalConfig = func() *cfg.CompletedConfig {
		return mockCfg
	}

	attacher := &VolumeAttacher{}
	longHostname := "a-really-very-long-hostname-that-exceeds-limit"

	got := attacher.getHostNameWithPrefix(longHostname)
	assert.Equal(t, "k8s_"+longHostname, got)
	// FusionStorage does not truncate
	assert.Greater(t, len(got), 31)
}

func TestVolumeAttacher_getRawHostName_Success(t *testing.T) {
	attacher := &VolumeAttacher{}
	parameters := map[string]interface{}{"HostName": "mynode"}

	got, err := attacher.getRawHostName(parameters)
	assert.Nil(t, err)
	assert.Equal(t, "mynode", got)
}

func TestVolumeAttacher_getRawHostName_HostNameNotFound(t *testing.T) {
	attacher := &VolumeAttacher{}
	parameters := map[string]interface{}{}

	got, err := attacher.getRawHostName(parameters)
	assert.ErrorContains(t, err, "can not find host name")
	assert.Empty(t, got)
}

func TestVolumeAttacher_getRawHostName_HostNameNotString(t *testing.T) {
	attacher := &VolumeAttacher{}
	parameters := map[string]interface{}{"HostName": 123}

	got, err := attacher.getRawHostName(parameters)
	assert.ErrorContains(t, err, "can not find host name")
	assert.Empty(t, got)
}

func TestVolumeAttacher_ControllerAttach_iSCSISuccess(t *testing.T) {
	attacher := &VolumeAttacher{cli: testClient, protocol: "iscsi"}
	mockMapping := map[string]interface{}{"tgtLunWWN": "wwn1"}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyPrivateMethod(attacher, "getLunInfo",
		func(_ *VolumeAttacher, _ context.Context, _ string) (*lunInfo, error) {
			return &lunInfo{name: "lun1", wwn: "wwn1", poolName: "pool1"}, nil
		})
	mock.ApplyPrivateMethod(attacher, "iSCSIControllerAttach",
		func(_ *VolumeAttacher, _ context.Context, _ *lunInfo, _ map[string]any) (map[string]any, error) {
			return mockMapping, nil
		})

	gotProps, gotErr := attacher.ControllerAttach(context.Background(),
		"lun1", map[string]any{"HostName": "mynode"})
	assert.Nil(t, gotErr)
	assert.Equal(t, mockMapping, gotProps)
}

func TestVolumeAttacher_ControllerAttach_SCSISuccess(t *testing.T) {
	attacher := &VolumeAttacher{cli: testClient, protocol: "fc", hosts: map[string]string{"mynode": "10.0.0.1"}}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyPrivateMethod(attacher, "getLunInfo",
		func(_ *VolumeAttacher, _ context.Context, _ string) (*lunInfo, error) {
			return &lunInfo{name: "lun1", wwn: "wwn1", poolName: "pool1"}, nil
		})
	mock.ApplyMethodReturn(testClient, "AttachVolume", nil)

	gotProps, gotErr := attacher.ControllerAttach(context.Background(),
		"lun1", map[string]interface{}{"HostName": "mynode"})
	assert.Nil(t, gotErr)
	assert.Equal(t, "wwn1", gotProps["tgtLunWWN"])
}

func TestVolumeAttacher_SCSIControllerAttach_Success(t *testing.T) {
	attacher := &VolumeAttacher{cli: testClient, hosts: map[string]string{"mynode": "10.0.0.1"}}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn(testClient, "AttachVolume", nil)

	wwn, gotErr := attacher.SCSIControllerAttach(context.Background(),
		&lunInfo{name: "lun1", wwn: "wwn1"}, map[string]any{"HostName": "mynode"})
	assert.Nil(t, gotErr)
	assert.Equal(t, "wwn1", wwn)
}

func TestVolumeAttacher_ControllerDetach_Success(t *testing.T) {
	attacher := &VolumeAttacher{cli: testClient}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyPrivateMethod(attacher, "doUnmapping",
		func(_ *VolumeAttacher, _ context.Context, _, _ string) (string, error) {
			return "wwn1", nil
		})

	wwn, gotErr := attacher.ControllerDetach(context.Background(),
		"lun1", map[string]interface{}{"HostName": "mynode"})
	assert.Nil(t, gotErr)
	assert.Equal(t, "wwn1", wwn)
}

func TestVolumeAttacher_doUnmapping_iSCSIVolumeAdded(t *testing.T) {
	origGetGlobalConfig := app.GetGlobalConfig
	defer func() { app.GetGlobalConfig = origGetGlobalConfig }()
	mockCfg := cfg.MockCompletedConfig()
	mockCfg.HostNamePrefix = ""
	mockCfg.HostNamePrefixSet = false
	app.GetGlobalConfig = func() *cfg.CompletedConfig { return mockCfg }

	attacher := &VolumeAttacher{cli: testClient, protocol: "iscsi"}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyPrivateMethod(attacher, "getLunInfo",
		func(_ *VolumeAttacher, _ context.Context, _ string) (*lunInfo, error) {
			return &lunInfo{name: "lun1", wwn: "wwn1"}, nil
		})
	mock.ApplyPrivateMethod(attacher, "isVolumeAddToHost",
		func(_ *VolumeAttacher, _ context.Context, _, _ string) (bool, error) {
			return true, nil
		})
	mock.ApplyMethodReturn(testClient, "DeleteLunFromHost", nil)

	wwn, gotErr := attacher.doUnmapping(context.Background(), "lun1", "host1")
	assert.Nil(t, gotErr)
	assert.Equal(t, "wwn1", wwn)
}

func TestVolumeAttacher_doUnmapping_FC(t *testing.T) {
	attacher := &VolumeAttacher{
		cli:      testClient,
		protocol: "fc",
		hosts:    map[string]string{"host1": "10.0.0.1"},
	}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyPrivateMethod(attacher, "getLunInfo",
		func(_ *VolumeAttacher, _ context.Context, _ string) (*lunInfo, error) {
			return &lunInfo{name: "lun1", wwn: "wwn1"}, nil
		})
	mock.ApplyMethodReturn(testClient, "DetachVolume", nil)

	wwn, gotErr := attacher.doUnmapping(context.Background(), "lun1", "host1")
	assert.Nil(t, gotErr)
	assert.Equal(t, "wwn1", wwn)
}

func TestVolumeAttacher_getTargetPortalsStatic_Success(t *testing.T) {
	attacher := &VolumeAttacher{cli: testClient, portals: []string{"192.168.1.1"}}
	portalResult := []map[string]interface{}{
		{
			"status": "successful",
			"iscsiPortalList": []interface{}{
				map[string]interface{}{
					"iscsiStatus": "active",
					"iscsiPortal": "192.168.1.1:3260",
					"targetName":  "iqn.test:t1",
				},
			},
		},
	}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn(testClient, "QueryIscsiPortal", portalResult, nil)

	tgtPortals, tgtIQNs, gotErr := attacher.getTargetPortalsStatic(context.Background())
	assert.Nil(t, gotErr)
	assert.Equal(t, []string{"192.168.1.1:3260"}, tgtPortals)
	assert.Equal(t, []string{"iqn.test:t1"}, tgtIQNs)
}

func TestVolumeAttacher_getTargetPortalsStatic_QueryError(t *testing.T) {
	attacher := &VolumeAttacher{cli: testClient, portals: []string{"192.168.1.1"}}
	wantErr := errors.New("query failed")

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn(testClient, "QueryIscsiPortal", nil, wantErr)

	_, _, gotErr := attacher.getTargetPortalsStatic(context.Background())
	assert.ErrorIs(t, gotErr, wantErr)
}

func TestVolumeAttacher_getTargetPortalsStatic_AllPortalsInvalid(t *testing.T) {
	attacher := &VolumeAttacher{cli: testClient, portals: []string{"10.0.0.1"}}
	portalResult := []map[string]interface{}{
		{"status": "successful", "iscsiPortalList": []interface{}{}},
	}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn(testClient, "QueryIscsiPortal", portalResult, nil)

	_, _, gotErr := attacher.getTargetPortalsStatic(context.Background())
	assert.ErrorContains(t, gotErr, "All config portal")
}

func TestVolumeAttacher_attachIscsiInitiatorToHost_CreateInitiator(t *testing.T) {
	attacher := &VolumeAttacher{cli: testClient}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyFuncReturn(baseAttacher.GetSingleInitiator, "iqn.test:123456", nil)
	mock.ApplyMethodReturn(testClient, "GetInitiatorByName", nil, nil)
	mock.ApplyMethodReturn(testClient, "CreateInitiator", nil)
	mock.ApplyMethodReturn(testClient, "AddPortToHost", nil)

	gotErr := attacher.attachIscsiInitiatorToHost(context.Background(), "host1", "rawhost1")
	assert.Nil(t, gotErr)
}

func TestVolumeAttacher_isVolumeAddToHost_True(t *testing.T) {
	attacher := &VolumeAttacher{cli: testClient}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn(testClient, "QueryHostOfVolume",
		[]map[string]interface{}{{"hostName": "host1"}}, nil)

	added, gotErr := attacher.isVolumeAddToHost(context.Background(), "lun1", "host1")
	assert.Nil(t, gotErr)
	assert.True(t, added)
}

func TestVolumeAttacher_createIscsiHost_CreateNew(t *testing.T) {
	attacher := &VolumeAttacher{cli: testClient}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn(testClient, "GetHostByName", nil, nil)
	mock.ApplyFuncReturn(utils.GetAlua, nil)
	mock.ApplyMethodReturn(testClient, "CreateHost", nil)

	gotErr := attacher.createIscsiHost(context.Background(), "host1")
	assert.Nil(t, gotErr)
}

func TestVolumeAttacher_needUpdateIscsiHost_Changed(t *testing.T) {
	attacher := &VolumeAttacher{}
	host := map[string]interface{}{"switchoverMode": "old"}
	hostAlua := map[string]interface{}{"switchoverMode": "new"}

	assert.True(t, attacher.needUpdateIscsiHost(host, hostAlua))
}

func TestVolumeAttacher_parseISCSIPortal_Active(t *testing.T) {
	attacher := &VolumeAttacher{}
	portal := map[string]interface{}{"iscsiStatus": "active", "iscsiPortal": "192.168.1.1:3260"}

	ip := attacher.parseISCSIPortal(context.Background(), portal)
	assert.Equal(t, "192.168.1.1", ip)
}

func TestVolumeAttacher_parseiSCSIPortalList_Success(t *testing.T) {
	attacher := &VolumeAttacher{}
	validIPs := make(map[string]bool)
	validIQNs := make(map[string]string)

	portalList := []interface{}{
		map[string]interface{}{
			"iscsiStatus": "active",
			"iscsiPortal": "192.168.1.1:3260",
			"targetName":  "iqn.test:t1",
		},
	}

	gotErr := attacher.parseiSCSIPortalList(context.Background(), portalList, validIPs, validIQNs)
	assert.Nil(t, gotErr)
	assert.True(t, validIPs["192.168.1.1"])
}

func TestVolumeAttacher_iSCSIControllerAttach_GetRawHostNameError(t *testing.T) {
	attacher := &VolumeAttacher{cli: testClient}

	// getRawHostName will fail because parameters has no HostName

	// act
	gotProps, gotErr := attacher.iSCSIControllerAttach(context.Background(), &lunInfo{}, map[string]interface{}{})

	// assert
	assert.ErrorContains(t, gotErr, "can not find host name")
	assert.Nil(t, gotProps)
}

func TestVolumeAttacher_iSCSIControllerAttach_AttachInitiatorError(t *testing.T) {
	origGetGlobalConfig := app.GetGlobalConfig
	defer func() { app.GetGlobalConfig = origGetGlobalConfig }()

	mockCfg := cfg.MockCompletedConfig()
	mockCfg.HostNamePrefix = "prod_"
	mockCfg.HostNamePrefixSet = true
	app.GetGlobalConfig = func() *cfg.CompletedConfig {
		return mockCfg
	}

	attacher := &VolumeAttacher{cli: testClient}
	wantErr := errors.New("attach initiator failed")

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyPrivateMethod(attacher, "createIscsiHost",
		func(_ *VolumeAttacher, _ context.Context, _ string) error {
			return nil
		})
	mock.ApplyPrivateMethod(attacher, "attachIscsiInitiatorToHost",
		func(_ *VolumeAttacher, _ context.Context, _, _ string) error {
			return wantErr
		})

	parameters := map[string]interface{}{"HostName": "mynode"}

	// act
	gotProps, gotErr := attacher.iSCSIControllerAttach(context.Background(), &lunInfo{}, parameters)

	// assert
	assert.ErrorIs(t, gotErr, wantErr)
	assert.Nil(t, gotProps)
}
