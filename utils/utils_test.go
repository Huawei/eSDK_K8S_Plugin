/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2022. All rights reserved.
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

package utils

import (
	"context"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"

	"huawei-csi-driver/utils/log"
)

const (
	logDir  = "/var/log/huawei/"
	logName = "utilsTest.log"
)

func TestPathExist(t *testing.T) {
	type pathCase struct {
		name     string
		Path     string
		expected bool
	}
	var testCases = []pathCase{
		{
			"notExist",
			"wrongDir",
			false,
		},
		{
			"exist",
			"/rightDir",
			true,
		},
	}

	err := os.Mkdir("/rightDir", os.ModePerm)
	if err != nil {
		return
	}
	defer func() {
		err := os.Remove("/rightDir")
		if err != nil {

		}
	}()

	for _, c := range testCases {
		exist, err := PathExist(c.Path)
		assert.Equal(t, c.expected, exist, "case name is %s, result: %v, %v", c.name, exist, err)
	}
}

func TestMaskSensitiveInfo(t *testing.T) {
	type maskInfoCase struct {
		name       string
		info       interface{}
		expectInfo string
	}

	var testCases = []maskInfoCase{
		{
			"stringMaskInfo",
			"iscsiadm -m node -T iqn.2003-01.io.k8s:e2e.volume -p 192.168.0.2 --interface default --op new",
			"iscsiadm -m node -T ***-p 192.168.0.2 --interface default --op new",
		},
	}
	for _, c := range testCases {
		maskInfo := MaskSensitiveInfo(c.info)
		assert.Equal(t, c.expectInfo, maskInfo, "case name is %s, result: %v", c.name, maskInfo)
	}
}

func TestGetSnapshotName(t *testing.T) {
	shortName := GetSnapshotName("TestShortName")
	assert.Equal(t, "TestShortName", shortName)

	longName := GetSnapshotName("snapshot-f311b342-a4b4-4235-98b3-5a1c289849c0")
	assert.Equal(t, "snapshot-f311b342-a4b4-4235-98b", longName)
}

func TestGetFusionStorageLunName(t *testing.T) {
	shortName := GetFusionStorageLunName("pvc-331a3fcd-6380-4de5-9bc0-be95c801edeb")
	assert.Equal(t, "pvc-331a3fcd-6380-4de5-9bc0-be95c801edeb", shortName)

	longName := GetFusionStorageLunName("pvc-331a3fcd-6380-4de5-9bc0-be95c801edeb-pvc-331a3fcd-6380-4de5-" +
		"9bc0-be95c801edeb-pvc-331a3fcd-6380-4de5-9bc0-be95c801edeb")
	assert.Equal(t, "pvc-331a3fcd-6380-4de5-9bc0-be95c801edeb-pvc-331a3fcd-6380-4de5-9bc0-"+
		"be95c801edeb-pvc-331a3fcd-", longName)
}

func TestGetFusionStorageSnapshotName(t *testing.T) {
	shortName := GetFusionStorageSnapshotName("TestShortName")
	assert.Equal(t, "TestShortName", shortName)

	longName := GetFusionStorageSnapshotName("snapshot-331a3fcd-6380-4de5-9bc0-be95c801edeb-331a3fcd-6380-4de5-" +
		"9bc0-be95c801edeb-331a3fcd-6380-4de5-9bc0-be95c801edeb")
	assert.Equal(t, "snapshot-331a3fcd-6380-4de5-9bc0-be95c801edeb-331a3fcd-6380-4de5-9bc0-"+
		"be95c801edeb-331a3fcd-638", longName)
}

func TestGetFileSystemName(t *testing.T) {
	replaceName := GetFileSystemName("pvc-331a3fcd-6380-4de5-9bc0-be95c801edeb")
	assert.Equal(t, "pvc_331a3fcd_6380_4de5_9bc0_be95c801edeb", replaceName)
}

func TestGetFSSnapshotName(t *testing.T) {
	replaceName := GetFSSnapshotName("snapshot-331a3fcd-6380-4de5-9bc0-be95c801edeb")
	assert.Equal(t, "snapshot_331a3fcd_6380_4de5_9bc0_be95c801edeb", replaceName)
}

func TestGetSharePath(t *testing.T) {
	replaceName := GetSharePath("pvc-331a3fcd-6380-4de5-9bc0-be95c801edeb")
	assert.Equal(t, "/pvc_331a3fcd_6380_4de5_9bc0_be95c801edeb/", replaceName)
}

func TestGetFSSharePath(t *testing.T) {
	replaceName := GetFSSharePath("pvc-331a3fcd-6380-4de5-9bc0-be95c801edeb")
	assert.Equal(t, "/pvc_331a3fcd_6380_4de5_9bc0_be95c801edeb/", replaceName)
}

func TestGetHostName(t *testing.T) {
	temp := ExecShellCmd
	defer func() { ExecShellCmd = temp }()

	ExecShellCmd = func(_ context.Context, _ string, _ ...interface{}) (string, error) {
		return "worker-node1", nil
	}

	expectedHost, err := GetHostName(context.Background())
	assert.Equal(t, "worker-node1", expectedHost,
		"case name is testGetHostName, result: %v, error: %v", expectedHost, err)
}

func TestMain(m *testing.M) {
	if err := log.InitLogging(logName); err != nil {
		log.Errorf("init logging: %s failed. error: %v", logName, err)
		os.Exit(1)
	}
	logFile := path.Join(logDir, logName)
	defer func() {
		if err := os.RemoveAll(logFile); err != nil {
			log.Errorf("Remove file: %s failed. error: %s", logFile, err)
		}
	}()

	m.Run()
}
