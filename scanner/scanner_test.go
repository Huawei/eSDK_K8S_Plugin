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

package scanner

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/manage"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const logName = "scanner_test.log"

func TestMain(m *testing.M) {
	log.MockInitLogging(logName)
	defer log.MockStopLogging(logName)
	m.Run()
}

func TestBuildISCSILunInfos(t *testing.T) {
	// arrange
	info := &manage.ControllerPublishInfo{
		TgtPortals:  []string{"192.168.1.100:3260", "192.168.1.101:3260"},
		TgtIQNs:     []string{"iqn.xxx.com:disk1", "iqn.xxx.com:disk2"},
		TgtHostLUNs: []string{"0", "1"},
	}

	// act
	lunInfos := buildISCSILunInfos(info)

	// assert
	assert.Equal(t, 2, len(lunInfos))
	assert.Equal(t, "192.168.1.100:3260", lunInfos[0].Portal)
	assert.Equal(t, "iqn.xxx.com:disk1", lunInfos[0].IQN)
	assert.Equal(t, "0", lunInfos[0].HostLUN)
	assert.Equal(t, "192.168.1.101:3260", lunInfos[1].Portal)
	assert.Equal(t, "iqn.xxx.com:disk2", lunInfos[1].IQN)
	assert.Equal(t, "1", lunInfos[1].HostLUN)
}

func TestBuildISCSILunInfos_MismatchedLengths(t *testing.T) {
	// arrange
	info := &manage.ControllerPublishInfo{
		TgtPortals:  []string{"192.168.1.100:3260"},
		TgtIQNs:     []string{"iqn.xxx.com:disk1", "iqn.xxx.com:disk2"},
		TgtHostLUNs: []string{"0"},
	}

	// act
	lunInfos := buildISCSILunInfos(info)

	// assert
	assert.Equal(t, 2, len(lunInfos))
	assert.Equal(t, "192.168.1.100:3260", lunInfos[0].Portal)
	assert.Equal(t, "", lunInfos[1].Portal)
	assert.Equal(t, "0", lunInfos[0].HostLUN)
	assert.Equal(t, "", lunInfos[1].HostLUN)
}

func TestBuildFCLunInfos(t *testing.T) {
	// arrange
	info := &manage.ControllerPublishInfo{
		TgtWWNs:     []string{"50000974c0004321", "50000974c0004322"},
		TgtHostLUNs: []string{"0", "1"},
	}

	// act
	lunInfos := buildFCLunInfos(info)

	// assert
	assert.Equal(t, 2, len(lunInfos))
	assert.Equal(t, "50000974c0004321", lunInfos[0].WWPN)
	assert.Equal(t, "0", lunInfos[0].HostLUN)
	assert.Equal(t, "50000974c0004322", lunInfos[1].WWPN)
	assert.Equal(t, "1", lunInfos[1].HostLUN)
}

func TestBuildFCLunInfos_MismatchedLengths(t *testing.T) {
	// arrange
	info := &manage.ControllerPublishInfo{
		TgtWWNs:     []string{"50000974c0004321", "50000974c0004322"},
		TgtHostLUNs: []string{"0"},
	}

	// act
	lunInfos := buildFCLunInfos(info)

	// assert
	assert.Equal(t, 2, len(lunInfos))
	assert.Equal(t, "0", lunInfos[0].HostLUN)
	assert.Equal(t, "", lunInfos[1].HostLUN)
}

func TestScannerFactory_ISCSIProtocol(t *testing.T) {
	// arrange
	publishInfo := &manage.ControllerPublishInfo{
		TgtPortals:  []string{"192.168.1.100:3260"},
		TgtIQNs:     []string{"iqn.xxx.com:disk1"},
		TgtHostLUNs: []string{"0"},
		TgtLunWWN:   "test-wwn",
	}

	p := gomonkey.NewPatches()
	defer p.Reset()
	p.ApplyMethodReturn(&ISCSIScanner{}, "Scan", nil).
		ApplyFuncReturn(filepath.Glob, []string{"path1"}, nil)

	factory := GetFactory()

	// act
	err := factory.Scan(context.Background(), publishInfo)

	// assert
	assert.NoError(t, err)
}

func TestScannerFactory_FCProtocol(t *testing.T) {
	// arrange
	publishInfo := &manage.ControllerPublishInfo{
		TgtWWNs:     []string{"50000974c0004321"},
		TgtHostLUNs: []string{"0"},
		TgtLunWWN:   "test-wwn",
	}

	p := gomonkey.NewPatches()
	defer p.Reset()
	p.ApplyMethodReturn(&FCScanner{}, "Scan", nil).
		ApplyFuncReturn(filepath.Glob, []string{"path1"}, nil)

	factory := GetFactory()

	// act
	err := factory.Scan(context.Background(), publishInfo)

	// assert
	assert.NoError(t, err)
}

func TestScannerFactory_UnknownProtocol(t *testing.T) {
	// arrange
	publishInfo := &manage.ControllerPublishInfo{TgtLunWWN: "test-wwn"}
	factory := GetFactory()

	// mock
	p := gomonkey.NewPatches()
	defer p.Reset()
	p.ApplyFuncReturn(filepath.Glob, []string{"path1"}, nil)

	// act
	err := factory.Scan(context.Background(), publishInfo)

	// assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot determine protocol from publishInfo")
}

func TestScannerFactory_SkipScan(t *testing.T) {
	// arrange
	publishInfo := &manage.ControllerPublishInfo{TgtLunWWN: "test-wwn"}
	factory := GetFactory()

	// act
	err := factory.Scan(context.Background(), publishInfo)

	// assert
	assert.NoError(t, err)
}
