/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2022-2025. All rights reserved.
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
	"errors"
	"net"
	"os"
	"path"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const FilePermission0777 = os.FileMode(0777)

func TestChmodFsPermission(t *testing.T) {
	currentDir, err := os.Getwd()
	if err != nil {
		log.Errorf("Get current directory failed.")
	}

	targetPath := path.Join(currentDir, "fsPermissionTest.txt")
	targetFile, err := os.Create(targetPath)
	if err != nil {
		log.Errorf("Create file/directory [%s] failed.", targetPath)
	}
	defer func() {
		if err := targetFile.Close(); err != nil {
			t.Errorf("close file %s failed, error: %v", targetFile.Name(), err)
		}
	}()
	err = targetFile.Chmod(FilePermission0777)
	if err != nil {
		log.Errorf("file targetFile chmod to 0600 failed, error: %v", err)
	}

	defer func() {
		err := os.Remove(targetPath)
		if err != nil {
			log.Errorf("Remove file/directory [%s] failed.", targetPath)
		}
	}()

	t.Run("Change target directory to 777 permission", func(t *testing.T) {
		ChmodFsPermission(context.TODO(), targetPath, "777")
		fileInfo, err := os.Stat(targetPath)
		require.NoError(t, err)

		filePerm := fileInfo.Mode().Perm()
		require.Equal(t, os.FileMode(0777), filePerm)
	})

	t.Run("Change target directory to 555 permission", func(t *testing.T) {
		ChmodFsPermission(context.TODO(), targetPath, "555")
		fileInfo, err := os.Stat(targetPath)
		require.NoError(t, err)

		filePerm := fileInfo.Mode().Perm()
		require.Equal(t, os.FileMode(0555), filePerm)
	})

	t.Run("Change target directory to 000 permission", func(t *testing.T) {
		ChmodFsPermission(context.TODO(), targetPath, "000")
		fileInfo, err := os.Stat(targetPath)
		require.NoError(t, err)

		filePerm := fileInfo.Mode().Perm()
		require.Equal(t, os.FileMode(0000), filePerm)
	})

	t.Run("Change target directory to 456 permission", func(t *testing.T) {
		ChmodFsPermission(context.TODO(), targetPath, "456")
		fileInfo, err := os.Stat(targetPath)
		require.NoError(t, err)

		filePerm := fileInfo.Mode().Perm()
		require.Equal(t, os.FileMode(0456), filePerm)
	})
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

func Test_GetHostIPs_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	wantIPs := []string{"192.168.0.2"}
	fakeInterfaces := []net.Interface{
		{Index: 1, Name: "eth0", Flags: net.FlagUp},
		{Index: 2, Name: "lo", Flags: net.FlagLoopback},
	}
	fakeAddrs := []net.Addr{
		&net.IPNet{IP: net.ParseIP("192.168.0.2")},
		&net.IPNet{IP: net.ParseIP("fe80::")},
	}

	// mock
	patches := gomonkey.ApplyFuncReturn(net.Interfaces, fakeInterfaces, nil).
		ApplyMethodReturn(&net.Interface{}, "Addrs", fakeAddrs, nil)
	defer patches.Reset()

	// action
	gotIPs, gotErr := GetHostIPs(ctx)

	// assert
	assert.NoError(t, gotErr)
	assert.Equal(t, wantIPs, gotIPs)
}

func Test_GetHostIPs_ErrorListingInterfaces(t *testing.T) {
	// arrange
	ctx := context.Background()
	wantErr := errors.New("fake error")

	// mock net.Interfaces to return an error
	patches := gomonkey.ApplyFuncReturn(net.Interfaces, nil, wantErr)
	defer patches.Reset()

	// action
	gotIPs, gotErr := GetHostIPs(ctx)

	// assert
	assert.ErrorContains(t, gotErr, "cannot list interfaces")
	assert.Nil(t, gotIPs)
}

func Test_FilterIPsByCIDRs(t *testing.T) {
	// arrange
	ctx := context.Background()

	tests := []struct {
		name          string
		ips           []string
		cidrs         []string
		want          []string
		wantErr       bool
		expectedError string
	}{
		{name: "ipv4", ips: []string{"192.168.1.1", "10.0.0.1", "172.16.0.1"},
			cidrs: []string{"192.168.0.0/16", "10.0.0.0/8"}, want: []string{"192.168.1.1", "10.0.0.1"}},
		{name: "ipv6", ips: []string{"fe80::1", "192.168.1.1"}, cidrs: []string{"fe80::/32"},
			want: []string{"fe80::1"}},
		{name: "mixed ipv4 and ipv6", ips: []string{"192.168.1.1", "fe80::1"},
			cidrs: []string{"192.168.0.0/8", "fe80::/32"}, want: []string{"192.168.1.1", "fe80::1"}},
		{name: "multiple cidrs matching same ip", ips: []string{"192.168.1.1"},
			cidrs: []string{"192.168.0.0/16", "192.168.1.0/24"}, want: []string{"192.168.1.1"}},
		{name: "empty ips slice", cidrs: []string{"192.168.0.0/16"}},
		{name: "empty cidrs slice", ips: []string{"192.168.1.1"}, want: []string{"192.168.1.1"}},
		{name: "no matching ips", ips: []string{"192.168.1.1", "10.0.0.1"}, cidrs: []string{"172.16.0.0/12"}},
		{name: "invalid cidr format", ips: []string{"192.168.1.1"}, cidrs: []string{"invalid-cidr"}, wantErr: true,
			expectedError: "failed to parse cidr invalid-cidr"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// action
			got, err := FilterIPsByCIDRs(ctx, tt.ips, tt.cidrs)

			// assert
			if tt.wantErr {
				assert.ErrorContains(t, err, tt.expectedError)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
