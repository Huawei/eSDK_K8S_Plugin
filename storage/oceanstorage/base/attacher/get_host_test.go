/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
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

package attacher

import (
	"context"
	"fmt"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	cfg "github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app/config"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/client"
)

func setupMockConfig(t *testing.T) {
	t.Helper()
	origGetGlobalConfig := app.GetGlobalConfig
	t.Cleanup(func() { app.GetGlobalConfig = origGetGlobalConfig })
	mockCfg := cfg.MockCompletedConfig()
	mockCfg.HostNamePrefix = "k8s_"
	mockCfg.HostNamePrefixSet = true
	app.GetGlobalConfig = func() *cfg.CompletedConfig {
		return mockCfg
	}
}

func setupMockConfigNotSet(t *testing.T) {
	t.Helper()
	origGetGlobalConfig := app.GetGlobalConfig
	t.Cleanup(func() { app.GetGlobalConfig = origGetGlobalConfig })
	mockCfg := cfg.MockCompletedConfig()
	mockCfg.HostNamePrefix = ""
	mockCfg.HostNamePrefixSet = false
	app.GetGlobalConfig = func() *cfg.CompletedConfig {
		return mockCfg
	}
}

// TestGetHost_FullNameHit tests that GetHost returns immediately when fullName is found
func TestGetHost_FullNameHit(t *testing.T) {
	setupMockConfig(t)
	newClient := &client.OceanstorClient{}
	am := &AttachmentManager{Cli: newClient, Protocol: constants.ProtocolIscsi}
	params := map[string]interface{}{"HostName": "node1"}
	wantHost := map[string]interface{}{"ID": "1", "NAME": "k8s_node1"}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn(newClient, "GetHostByName", wantHost, nil)

	host, err := am.GetHost(context.Background(), params, true)
	assert.Nil(t, err)
	assert.Equal(t, wantHost, host)
}

// TestGetHost_NoHostName tests that GetHost returns error when HostName is missing
func TestGetHost_NoHostName(t *testing.T) {
	setupMockConfig(t)
	am := &AttachmentManager{Cli: &client.OceanstorClient{}, Protocol: constants.ProtocolIscsi}
	params := map[string]interface{}{}

	host, err := am.GetHost(context.Background(), params, true)
	assert.Nil(t, host)
	assert.Error(t, err)
}

// TestGetHost_GetHostByNameError tests that GetHost propagates GetHostByName error
func TestGetHost_GetHostByNameError(t *testing.T) {
	setupMockConfig(t)
	newClient := &client.OceanstorClient{}
	am := &AttachmentManager{Cli: newClient, Protocol: constants.ProtocolIscsi}
	params := map[string]interface{}{"HostName": "node1"}
	wantErr := fmt.Errorf("storage error")

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn(newClient, "GetHostByName", nil, wantErr)

	host, err := am.GetHost(context.Background(), params, true)
	assert.ErrorIs(t, err, wantErr)
	assert.Nil(t, host)
}

// TestGetHost_TruncatedNameValidationPass tests truncated-name fallback with initiator
// ownership validation passing (iSCSI initiator belongs to same host)
func TestGetHost_TruncatedNameValidationPass(t *testing.T) {
	setupMockConfig(t)
	newClient := &client.OceanstorClient{}
	am := &AttachmentManager{Cli: newClient, Protocol: constants.ProtocolIscsi}
	longHost := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" // 30 chars, k8s_ + 30 = 34 > 31
	params := map[string]interface{}{"HostName": longHost}
	truncatedHost := map[string]interface{}{"ID": "1", "NAME": "k8s_aaaaaaaaaaaaaaaaaaaaaaaaaaa"}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodSeq(newClient, "GetHostByName", []gomonkey.OutputCell{
		{Values: gomonkey.Params{nil, nil}},           // fullName not found
		{Values: gomonkey.Params{truncatedHost, nil}}, // truncatedName found
	})
	mock.ApplyFuncReturn(GetSingleInitiator, "iqn.2026-01.test:initiator", nil)
	mock.ApplyMethodReturn(newClient, "GetIscsiInitiatorByID",
		map[string]interface{}{"ISFREE": "false", "PARENTID": "1"}, nil)

	host, err := am.GetHost(context.Background(), params, true)
	assert.Nil(t, err)
	assert.Equal(t, truncatedHost, host)
}

// TestGetHost_TruncatedNameEmptyHost tests truncated-name fallback where initiator
// is free (ISFREE=true) — code treats this as "not our host" and proceeds to creation
func TestGetHost_TruncatedNameEmptyHost(t *testing.T) {
	setupMockConfig(t)
	newClient := &client.OceanstorClient{}
	am := &AttachmentManager{Cli: newClient, Protocol: constants.ProtocolIscsi}
	longHost := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" // 30 chars
	params := map[string]interface{}{"HostName": longHost}
	truncatedHost := map[string]interface{}{"ID": "1", "NAME": "k8s_aaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	createdHost := map[string]interface{}{"ID": "2", "NAME": "k8s_aaaaaaaaaaaaaaaaaaaaaaaaaaaa"}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodSeq(newClient, "GetHostByName", []gomonkey.OutputCell{
		{Values: gomonkey.Params{nil, nil}},           // fullName not found
		{Values: gomonkey.Params{truncatedHost, nil}}, // truncatedName found
	})
	mock.ApplyFuncReturn(GetSingleInitiator, "iqn.2026-01.test:initiator", nil)
	mock.ApplyMethodReturn(newClient, "GetIscsiInitiatorByID",
		map[string]interface{}{"ISFREE": "true"}, nil)
	mock.ApplyMethodReturn(newClient, "CreateHost", createdHost, nil)

	host, err := am.GetHost(context.Background(), params, true)
	assert.Nil(t, err)
	assert.Equal(t, createdHost, host)
}

// TestGetHost_TruncatedNameOtherNode_Attach tests truncated-name fallback where initiator
// belongs to another node — Attach path: skip and proceed to creation
func TestGetHost_TruncatedNameOtherNode_Attach(t *testing.T) {
	setupMockConfig(t)
	newClient := &client.OceanstorClient{}
	am := &AttachmentManager{Cli: newClient, Protocol: constants.ProtocolIscsi}
	longHost := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" // 30 chars
	params := map[string]interface{}{"HostName": longHost}
	truncatedHost := map[string]interface{}{"ID": "1", "NAME": "k8s_aaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	createdHost := map[string]interface{}{"ID": "2", "NAME": "k8s_aaaaaaaaaaaaaaaaaaaaaaaaaaaa"}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodSeq(newClient, "GetHostByName", []gomonkey.OutputCell{
		{Values: gomonkey.Params{nil, nil}},           // fullName not found
		{Values: gomonkey.Params{truncatedHost, nil}}, // truncatedName found (other node)
	})
	mock.ApplyFuncReturn(GetSingleInitiator, "iqn.2026-01.test:initiator", nil)
	mock.ApplyMethodReturn(newClient, "GetIscsiInitiatorByID",
		map[string]interface{}{"ISFREE": "false", "PARENTID": "99"}, nil)
	mock.ApplyMethodReturn(newClient, "CreateHost", createdHost, nil)

	host, err := am.GetHost(context.Background(), params, true)
	assert.Nil(t, err)
	assert.Equal(t, createdHost, host)
}

// TestGetHost_TruncatedNameOtherNode_Detach tests truncated-name fallback where initiator
// belongs to another node — Detach path: return nil (idempotent)
func TestGetHost_TruncatedNameOtherNode_Detach(t *testing.T) {
	setupMockConfig(t)
	newClient := &client.OceanstorClient{}
	am := &AttachmentManager{Cli: newClient, Protocol: constants.ProtocolIscsi}
	longHost := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" // 30 chars
	params := map[string]interface{}{"HostName": longHost}
	truncatedHost := map[string]interface{}{"ID": "1", "NAME": "k8s_aaaaaaaaaaaaaaaaaaaaaaaaaaa"}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodSeq(newClient, "GetHostByName", []gomonkey.OutputCell{
		{Values: gomonkey.Params{nil, nil}},           // fullName not found
		{Values: gomonkey.Params{truncatedHost, nil}}, // truncatedName found (other node)
	})
	mock.ApplyFuncReturn(GetSingleInitiator, "iqn.2026-01.test:initiator", nil)
	mock.ApplyMethodReturn(newClient, "GetIscsiInitiatorByID",
		map[string]interface{}{"ISFREE": "false", "PARENTID": "99"}, nil)

	host, err := am.GetHost(context.Background(), params, false)
	assert.Nil(t, err)
	assert.Nil(t, host)
}

// TestGetHost_TruncatedNameInitiatorFetchFail_Attach tests that Attach path returns error
// when initiator fetch fails — cannot verify ownership safely
func TestGetHost_TruncatedNameInitiatorFetchFail_Attach(t *testing.T) {
	setupMockConfig(t)
	newClient := &client.OceanstorClient{}
	am := &AttachmentManager{Cli: newClient, Protocol: constants.ProtocolIscsi}
	longHost := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" // 30 chars
	params := map[string]interface{}{"HostName": longHost}
	truncatedHost := map[string]interface{}{"ID": "1", "NAME": "k8s_aaaaaaaaaaaaaaaaaaaaaaaaaaa"}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodSeq(newClient, "GetHostByName", []gomonkey.OutputCell{
		{Values: gomonkey.Params{nil, nil}},           // fullName not found
		{Values: gomonkey.Params{truncatedHost, nil}}, // truncatedName found
	})
	mock.ApplyFuncReturn(GetSingleInitiator, "", fmt.Errorf("secret unavailable"))

	host, err := am.GetHost(context.Background(), params, true)
	assert.Error(t, err)
	assert.Nil(t, host)
}

// TestGetHost_TruncatedNameInitiatorFetchFail_Detach tests that Detach path degrades
// gracefully when initiator fetch fails — safe to use host for detach (backward compat)
func TestGetHost_TruncatedNameInitiatorFetchFail_Detach(t *testing.T) {
	setupMockConfig(t)
	newClient := &client.OceanstorClient{}
	am := &AttachmentManager{Cli: newClient, Protocol: constants.ProtocolIscsi}
	longHost := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" // 30 chars
	params := map[string]interface{}{"HostName": longHost}
	truncatedHost := map[string]interface{}{"ID": "1", "NAME": "k8s_aaaaaaaaaaaaaaaaaaaaaaaaaaa"}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodSeq(newClient, "GetHostByName", []gomonkey.OutputCell{
		{Values: gomonkey.Params{nil, nil}},           // fullName not found
		{Values: gomonkey.Params{truncatedHost, nil}}, // truncatedName found
	})
	mock.ApplyFuncReturn(GetSingleInitiator, "", fmt.Errorf("secret unavailable"))

	host, err := am.GetHost(context.Background(), params, false)
	assert.Nil(t, err)
	assert.Equal(t, truncatedHost, host)
}

// TestGetHost_TruncatedNameInitiatorNotFound tests degraded behavior when initiator
// is not found on storage — host is reusable
func TestGetHost_TruncatedNameInitiatorNotFound(t *testing.T) {
	setupMockConfig(t)
	newClient := &client.OceanstorClient{}
	am := &AttachmentManager{Cli: newClient, Protocol: constants.ProtocolIscsi}
	longHost := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" // 30 chars
	params := map[string]interface{}{"HostName": longHost}
	truncatedHost := map[string]interface{}{"ID": "1", "NAME": "k8s_aaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	createdHost := map[string]interface{}{"ID": "1", "NAME": "k8s_node1"}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodSeq(newClient, "GetHostByName", []gomonkey.OutputCell{
		{Values: gomonkey.Params{nil, nil}},           // fullName not found
		{Values: gomonkey.Params{truncatedHost, nil}}, // truncatedName found
	})
	mock.ApplyFuncReturn(GetSingleInitiator, "iqn.2026-01.test:initiator", nil)
	mock.ApplyMethodReturn(newClient, "GetIscsiInitiatorByID",
		map[string]interface{}{}, nil)
	mock.ApplyMethodReturn(newClient, "CreateHost", createdHost, nil)

	host, err := am.GetHost(context.Background(), params, true)
	assert.Nil(t, err)
	assert.Equal(t, createdHost, host)
}

// TestGetHost_CreateHostSuccess tests successful host creation when no host found
func TestGetHost_CreateHostSuccess(t *testing.T) {
	setupMockConfig(t)
	newClient := &client.OceanstorClient{}
	am := &AttachmentManager{Cli: newClient, Protocol: constants.ProtocolIscsi}
	params := map[string]interface{}{"HostName": "node1"}
	createdHost := map[string]interface{}{"ID": "1", "NAME": "k8s_node1"}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn(newClient, "GetHostByName", nil, nil)
	mock.ApplyMethodReturn(newClient, "CreateHost", createdHost, nil)

	host, err := am.GetHost(context.Background(), params, true)
	assert.Nil(t, err)
	assert.Equal(t, createdHost, host)
}

// TestGetHost_CreateHostTooLongFallback tests that CreateHost uses truncatedName
// when AllowTruncatedHostname is true and fullName exceeds maxHostNameLengthForV5
func TestGetHost_CreateHostTooLongFallback(t *testing.T) {
	setupMockConfig(t)
	// Assign
	newClient := &client.OceanstorClient{}
	am := &AttachmentManager{Cli: newClient, Protocol: constants.ProtocolIscsi, AllowTruncatedHostname: true}
	longHost := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" // 30 chars, k8s_ + 30 = 34 > 31
	params := map[string]any{"HostName": longHost}
	createdHost := map[string]any{"ID": "1", "NAME": "k8s_aaaaaaaaaaaaaaaaaaaaaaaaaaa"}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn(newClient, "GetHostByName", nil, nil)
	mock.ApplyMethodReturn(newClient, "CreateHost", createdHost, nil)

	// Action
	host, err := am.GetHost(context.Background(), params, true)

	// Assert
	assert.Nil(t, err)
	assert.Equal(t, createdHost, host)
}

// TestGetHost_CreateHostCollision tests that creation returns error when truncated-name
// creation fails (e.g. objectNameAlreadyExist collision)
func TestGetHost_CreateHostCollision(t *testing.T) {
	setupMockConfig(t)
	newClient := &client.OceanstorClient{}
	am := &AttachmentManager{Cli: newClient, Protocol: constants.ProtocolIscsi, AllowTruncatedHostname: true}
	longHost := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" // 30 chars
	params := map[string]interface{}{"HostName": longHost}
	collisionErr := fmt.Errorf("Create host %s error: %d", "k8s_aaaaaaaaaaaaaaaaaaaaaaaaaaa", 1077948993)

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn(newClient, "GetHostByName", nil, nil)
	mock.ApplyMethodReturn(newClient, "CreateHost", nil, collisionErr)

	host, err := am.GetHost(context.Background(), params, true)
	assert.Error(t, err)
	assert.Nil(t, host)
}

// TestGetHost_CreateHostOtherError tests that non-HostNameTooLong creation error is returned
func TestGetHost_CreateHostOtherError(t *testing.T) {
	setupMockConfig(t)
	newClient := &client.OceanstorClient{}
	am := &AttachmentManager{Cli: newClient, Protocol: constants.ProtocolIscsi}
	params := map[string]interface{}{"HostName": "node1"}
	createErr := fmt.Errorf("Create host k8s_node1 error: 1077949006")

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn(newClient, "GetHostByName", nil, nil)
	mock.ApplyMethodReturn(newClient, "CreateHost", nil, createErr)

	host, err := am.GetHost(context.Background(), params, true)
	assert.Error(t, err)
	assert.Nil(t, host)
}

// TestGetHost_NotCreateNoHost tests that GetHost returns nil when toCreate=false and no host found
func TestGetHost_NotCreateNoHost(t *testing.T) {
	setupMockConfig(t)
	newClient := &client.OceanstorClient{}
	am := &AttachmentManager{Cli: newClient, Protocol: constants.ProtocolIscsi}
	params := map[string]interface{}{"HostName": "node1"}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn(newClient, "GetHostByName", nil, nil)

	host, err := am.GetHost(context.Background(), params, false)
	assert.Nil(t, err)
	assert.Nil(t, host)
}

// TestGetHost_ShortNameNoTruncation tests that no truncatedName fallback is attempted
// when fullName <= maxHostNameLengthForV5
func TestGetHost_ShortNameNoTruncation(t *testing.T) {
	setupMockConfig(t)
	newClient := &client.OceanstorClient{}
	am := &AttachmentManager{Cli: newClient, Protocol: constants.ProtocolIscsi}
	params := map[string]interface{}{"HostName": "node1"}
	createdHost := map[string]interface{}{"ID": "1", "NAME": "k8s_node1"}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn(newClient, "GetHostByName", nil, nil)
	mock.ApplyMethodReturn(newClient, "CreateHost", createdHost, nil)

	host, err := am.GetHost(context.Background(), params, true)
	assert.Nil(t, err)
	assert.Equal(t, createdHost, host)
}

// TestGetHost_FCValidationPass tests FC protocol initiator ownership validation
func TestGetHost_FCValidationPass(t *testing.T) {
	setupMockConfig(t)
	newClient := &client.OceanstorClient{}
	am := &AttachmentManager{Cli: newClient, Protocol: constants.ProtocolFC}
	longHost := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" // 30 chars
	params := map[string]interface{}{"HostName": longHost}
	truncatedHost := map[string]interface{}{"ID": "1", "NAME": "k8s_aaaaaaaaaaaaaaaaaaaaaaaaaaa"}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodSeq(newClient, "GetHostByName", []gomonkey.OutputCell{
		{Values: gomonkey.Params{nil, nil}},           // fullName not found
		{Values: gomonkey.Params{truncatedHost, nil}}, // truncatedName found
	})
	mock.ApplyFuncReturn(GetMultipleInitiators, []string{"20:00:00:00:00:00:00:01"}, nil)
	mock.ApplyMethodReturn(newClient, "GetFCInitiatorByID",
		map[string]interface{}{"ISFREE": "false", "PARENTID": "1"}, nil)

	host, err := am.GetHost(context.Background(), params, true)
	assert.Nil(t, err)
	assert.Equal(t, truncatedHost, host)
}

// TestGetHost_FCValidationOtherNode tests FC protocol where initiator belongs to another node
func TestGetHost_FCValidationOtherNode(t *testing.T) {
	setupMockConfig(t)
	newClient := &client.OceanstorClient{}
	am := &AttachmentManager{Cli: newClient, Protocol: constants.ProtocolFC}
	longHost := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" // 30 chars
	params := map[string]interface{}{"HostName": longHost}
	truncatedHost := map[string]interface{}{"ID": "1", "NAME": "k8s_aaaaaaaaaaaaaaaaaaaaaaaaaaa"}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodSeq(newClient, "GetHostByName", []gomonkey.OutputCell{
		{Values: gomonkey.Params{nil, nil}},           // fullName not found
		{Values: gomonkey.Params{truncatedHost, nil}}, // truncatedName found
	})
	mock.ApplyFuncReturn(GetMultipleInitiators, []string{"20:00:00:00:00:00:00:01"}, nil)
	mock.ApplyMethodReturn(newClient, "GetFCInitiatorByID",
		map[string]interface{}{"ISFREE": "false", "PARENTID": "99"}, nil)

	host, err := am.GetHost(context.Background(), params, false)
	assert.Nil(t, err)
	assert.Nil(t, host)
}

// TestGetHost_FCValidationMultiWWN_Mixed tests FC protocol with multiple WWNs where
// one matches our host and one belongs to another node — should verify as owned
func TestGetHost_FCValidationMultiWWN_Mixed(t *testing.T) {
	setupMockConfig(t)
	newClient := &client.OceanstorClient{}
	am := &AttachmentManager{Cli: newClient, Protocol: constants.ProtocolFC}
	longHost := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" // 30 chars
	params := map[string]interface{}{"HostName": longHost}
	truncatedHost := map[string]interface{}{"ID": "1", "NAME": "k8s_aaaaaaaaaaaaaaaaaaaaaaaaaaa"}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodSeq(newClient, "GetHostByName", []gomonkey.OutputCell{
		{Values: gomonkey.Params{nil, nil}},           // fullName not found
		{Values: gomonkey.Params{truncatedHost, nil}}, // truncatedName found
	})
	// Two WWNs: first belongs to another node, second belongs to our host
	mock.ApplyFuncReturn(GetMultipleInitiators, []string{
		"20:00:00:00:00:00:00:01", // WWN1 - other node
		"20:00:00:00:00:00:00:02", // WWN2 - our node
	}, nil)
	mock.ApplyMethodSeq(newClient, "GetFCInitiatorByID", []gomonkey.OutputCell{
		{Values: gomonkey.Params{map[string]interface{}{"ISFREE": "false", "PARENTID": "99"}, nil}}, // WWN1 → other host
		{Values: gomonkey.Params{map[string]interface{}{"ISFREE": "false", "PARENTID": "1"}, nil}},  // WWN2 → our host
	})

	host, err := am.GetHost(context.Background(), params, true)
	assert.Nil(t, err)
	assert.Equal(t, truncatedHost, host)
}

// TestGetHost_FCValidationMultiWWN_AllOtherNode tests FC with multiple WWNs all
// belonging to other nodes — should return false (not our host)
func TestGetHost_FCValidationMultiWWN_AllOtherNode(t *testing.T) {
	setupMockConfig(t)
	newClient := &client.OceanstorClient{}
	am := &AttachmentManager{Cli: newClient, Protocol: constants.ProtocolFC}
	longHost := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" // 30 chars
	params := map[string]interface{}{"HostName": longHost}
	truncatedHost := map[string]interface{}{"ID": "1", "NAME": "k8s_aaaaaaaaaaaaaaaaaaaaaaaaaaa"}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodSeq(newClient, "GetHostByName", []gomonkey.OutputCell{
		{Values: gomonkey.Params{nil, nil}},           // fullName not found
		{Values: gomonkey.Params{truncatedHost, nil}}, // truncatedName found
	})
	mock.ApplyFuncReturn(GetMultipleInitiators, []string{
		"20:00:00:00:00:00:00:01",
		"20:00:00:00:00:00:00:02",
	}, nil)
	mock.ApplyMethodReturn(newClient, "GetFCInitiatorByID",
		map[string]interface{}{"ISFREE": "false", "PARENTID": "99"}, nil)

	host, err := am.GetHost(context.Background(), params, false)
	assert.Nil(t, err)
	assert.Nil(t, host)
}

// TestGetHost_NVMeValidationPass tests NVMe protocol initiator ownership validation
func TestGetHost_NVMeValidationPass(t *testing.T) {
	setupMockConfig(t)
	newClient := &client.OceanstorClient{}
	am := &AttachmentManager{Cli: newClient, Protocol: constants.ProtocolRoceNVMe}
	longHost := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" // 30 chars
	params := map[string]interface{}{"HostName": longHost}
	truncatedHost := map[string]interface{}{"ID": "1", "NAME": "k8s_aaaaaaaaaaaaaaaaaaaaaaaaaaa"}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodSeq(newClient, "GetHostByName", []gomonkey.OutputCell{
		{Values: gomonkey.Params{nil, nil}},           // fullName not found
		{Values: gomonkey.Params{truncatedHost, nil}}, // truncatedName found
	})
	mock.ApplyFuncReturn(GetSingleInitiator, "nqn.2026-01.test:initiator", nil)
	mock.ApplyMethodReturn(newClient, "GetInitiatorByID",
		map[string]interface{}{"ISFREE": "false", "PARENTID": "1"}, nil)

	host, err := am.GetHost(context.Background(), params, true)
	assert.Nil(t, err)
	assert.Equal(t, truncatedHost, host)
}

// TestGetHost_NVMeValidationOtherNode tests NVMe protocol where initiator belongs to another node
func TestGetHost_NVMeValidationOtherNode(t *testing.T) {
	setupMockConfig(t)
	newClient := &client.OceanstorClient{}
	am := &AttachmentManager{Cli: newClient, Protocol: constants.ProtocolRoceNVMe}
	longHost := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" // 30 chars
	params := map[string]interface{}{"HostName": longHost}
	truncatedHost := map[string]interface{}{"ID": "1", "NAME": "k8s_aaaaaaaaaaaaaaaaaaaaaaaaaaa"}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodSeq(newClient, "GetHostByName", []gomonkey.OutputCell{
		{Values: gomonkey.Params{nil, nil}},           // fullName not found
		{Values: gomonkey.Params{truncatedHost, nil}}, // truncatedName found
	})
	mock.ApplyFuncReturn(GetSingleInitiator, "nqn.2026-01.test:initiator", nil)
	mock.ApplyMethodReturn(newClient, "GetInitiatorByID",
		map[string]interface{}{"ISFREE": "false", "PARENTID": "99"}, nil)

	host, err := am.GetHost(context.Background(), params, false)
	assert.Nil(t, err)
	assert.Nil(t, host)
}

// TestGetHost_TruncatedNameHostIDMissing tests that verifyHostOwnership returns error
// when host["ID"] is missing or not a string
func TestGetHost_TruncatedNameHostIDMissing(t *testing.T) {
	setupMockConfig(t)
	newClient := &client.OceanstorClient{}
	am := &AttachmentManager{Cli: newClient, Protocol: constants.ProtocolIscsi}
	longHost := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" // 30 chars
	params := map[string]interface{}{"HostName": longHost}
	// Host without ID field
	truncatedHost := map[string]interface{}{"NAME": "k8s_aaaaaaaaaaaaaaaaaaaaaaaaaaa"}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodSeq(newClient, "GetHostByName", []gomonkey.OutputCell{
		{Values: gomonkey.Params{nil, nil}},           // fullName not found
		{Values: gomonkey.Params{truncatedHost, nil}}, // truncatedName found but no ID
	})
	mock.ApplyFuncReturn(GetSingleInitiator, "iqn.2026-01.test:initiator", nil)
	mock.ApplyMethodReturn(newClient, "GetIscsiInitiatorByID",
		map[string]interface{}{"ISFREE": "false", "PARENTID": "99"}, nil)

	host, err := am.GetHost(context.Background(), params, true)
	assert.Error(t, err)
	assert.Nil(t, host)
}

// TestGetHost_FallbackNotSet tests the fallback to "k8s_" when HostNamePrefix is not set
func TestGetHost_FallbackNotSet(t *testing.T) {
	setupMockConfigNotSet(t)
	newClient := &client.OceanstorClient{}
	am := &AttachmentManager{Cli: newClient, Protocol: constants.ProtocolIscsi}
	params := map[string]interface{}{"HostName": "node1"}
	wantHost := map[string]interface{}{"ID": "1", "NAME": "k8s_node1"}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn(newClient, "GetHostByName", wantHost, nil)

	host, err := am.GetHost(context.Background(), params, true)
	assert.Nil(t, err)
	assert.Equal(t, wantHost, host)
}
