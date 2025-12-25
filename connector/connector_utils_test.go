/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2023. All rights reserved.
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

package connector

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/assert"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
)

func TestGetDevice(t *testing.T) {
	const (
		hasPrefixDM       = "/test/../../dm-test"
		hasPrefixNVMe     = "/test/../../nvme-test"
		hasPrefixSD       = "/test/../../sd-test"
		invalidDeviceLink = "../../"
		emptyDeviceLink   = ""
	)
	type args struct {
		findDeviceMap map[string]string
		deviceLink    string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"HasPrefixDM", args{map[string]string{}, hasPrefixDM}, "dm-test"},
		{"HasPrefixNVMe", args{map[string]string{}, hasPrefixNVMe}, "nvme-test"},
		{"HasPrefixSD", args{map[string]string{}, hasPrefixSD}, "sd-test"},
		{"DeviceLinkIsEmpty", args{map[string]string{}, emptyDeviceLink}, ""},
		{"TheSplitLengthIsLessThenTwo", args{map[string]string{}, invalidDeviceLink}, ""},
		{"HasPrefixNvmeButExist", args{map[string]string{"nvme-test": "test"}, hasPrefixNVMe}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getDevice(tt.args.findDeviceMap, tt.args.deviceLink); got != tt.want {
				t.Errorf("getDevice() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetDeviceLink(t *testing.T) {
	var stubCtx = context.TODO()

	type args struct {
		ctx        context.Context
		tgtLunGUID string
	}
	type outputs struct {
		output string
		err    error
	}
	tests := []struct {
		name    string
		args    args
		outputs outputs
		want    string
		wantErr bool
	}{
		{"Normal", args{stubCtx, "test123456"},
			outputs{"test output", nil}, "test output", false},
		{"EmptyCmdResult", args{stubCtx, "test123456"},
			outputs{"", errors.New("test")}, "", false},
		{"CmdResultIsFileOrDirectoryNoExist", args{stubCtx, "test123456"},
			outputs{"No such file or directory", errors.New("test")}, "", false},
		{"CmdResultIsOtherError", args{stubCtx, "test123456"},
			outputs{"other result", errors.New("test")}, "", true},
	}

	stub := utils.ExecShellCmd
	defer func() {
		utils.ExecShellCmd = stub
	}()
	for _, tt := range tests {
		utils.ExecShellCmd = func(_ context.Context, format string, args ...interface{}) (string, error) {
			return tt.outputs.output, tt.outputs.err
		}
		t.Run(tt.name, func(t *testing.T) {
			got, err := getDeviceLink(tt.args.ctx, tt.args.tgtLunGUID)
			if (err != nil) != tt.wantErr {
				t.Errorf("getDeviceLink() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getDeviceLink() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsUltraPathDevice(t *testing.T) {
	const device = "dm-test"
	var stubCtx = context.TODO()

	type args struct {
		ctx    context.Context
		device string
	}
	type outputs struct {
		output string
		err    error
	}
	tests := []struct {
		name    string
		args    args
		outputs outputs
		want    bool
	}{
		{"Normal", args{stubCtx, device}, outputs{"test output dm-test", nil}, true},
		{"CmdError", args{stubCtx, device}, outputs{"test output", errors.New("test")}, false},
	}

	stub := utils.ExecShellCmd
	defer func() {
		utils.ExecShellCmd = stub
	}()
	for _, tt := range tests {
		utils.ExecShellCmd = func(_ context.Context, format string, args ...interface{}) (string, error) {
			return tt.outputs.output, tt.outputs.err
		}

		t.Run(tt.name, func(t *testing.T) {
			if got := isUltraPathDevice(tt.args.ctx, tt.args.device); got != tt.want {
				t.Errorf("isUltraPathDevice() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetDevices(t *testing.T) {
	const (
		normalDeviceLink  = "/test/../../dm-test\n/test/../../sd-test"
		emptyDeviceLink   = ""
		invalidDeviceLink = "/test/../../"
	)
	var emptyDevices []string

	type args struct {
		deviceLink string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{"Normal", args{normalDeviceLink}, []string{"dm-test", "sd-test"}},
		{"EmptyDeviceLink", args{emptyDeviceLink}, emptyDevices},
		{"TheSplitLengthLessThenTwo", args{invalidDeviceLink}, emptyDevices},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getDevices(tt.args.deviceLink); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getDevices() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckConnectSuccess(t *testing.T) {
	type args struct {
		ctx       context.Context
		device    string
		tgtLunWWN string
	}

	tests := []struct {
		name string
		args args
		want bool
	}{
		{"Normal", args{context.TODO(), "normal-device", "normal-lunWWN"}, true},
		{"DeviceIsNotReadable", args{context.TODO(), "test-device", "normal-lunWWN"}, false},
		{"DeviceIsNotAvailable", args{context.TODO(), "normal-device", "test-tgtLunWWN"}, false},
	}

	stubIsDeviceReadable := IsDeviceReadable
	defer func() {
		IsDeviceReadable = stubIsDeviceReadable
	}()

	stubIsDeviceAvailable := IsDeviceAvailable
	defer func() {
		IsDeviceAvailable = stubIsDeviceAvailable
	}()
	for _, tt := range tests {
		IsDeviceReadable = func(_ context.Context, devicePath string) (bool, error) {
			if devicePath != "/dev/normal-device" {
				return false, errors.New("test")
			}

			return true, nil
		}
		IsDeviceAvailable = func(_ context.Context, device, lunWWN string) (bool, error) {
			if lunWWN != "normal-lunWWN" {
				return false, errors.New("test")
			}

			return true, nil
		}
		t.Run(tt.name, func(t *testing.T) {
			if got := CheckConnectSuccess(tt.args.ctx, tt.args.device, tt.args.tgtLunWWN); got != tt.want {
				t.Errorf("CheckConnectSuccess() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsDeviceAvailable(t *testing.T) {
	type args struct {
		ctx    context.Context
		device string
		lunWWN string
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{"SCSIWwn", args{context.TODO(), "/dev/dm-device", "dm-device"}, true, false},
		{"NVMeWwn", args{context.TODO(), "/dev/nvme-device", "nvme-device"}, true, false},
		{"CanontGetWwn", args{context.TODO(), "/dev/other-device", "test lunWWN"}, false, true},
	}

	var stubGetSCSIWwn = GetSCSIWwn
	defer func() {
		GetSCSIWwn = stubGetSCSIWwn
	}()

	var stubGetNVMeWwn = GetNVMeWwn
	defer func() {
		GetNVMeWwn = stubGetNVMeWwn
	}()
	for _, tt := range tests {
		GetSCSIWwn = func(_ context.Context, hostDevice string) (string, error) {
			if hostDevice == "/dev/dm-device" {
				return hostDevice, nil
			}
			return "", errors.New("test error")
		}

		GetNVMeWwn = func(_ context.Context, device string) (string, error) {
			return device, nil
		}

		t.Run(tt.name, func(t *testing.T) {
			got, err := IsDeviceAvailable(tt.args.ctx, tt.args.device, tt.args.lunWWN)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsDeviceAvailable() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("IsDeviceAvailable() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsDeviceReadable(t *testing.T) {
	type args struct {
		ctx        context.Context
		devicePath string
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{"CanReadDevice", args{context.TODO(), "Normal"}, true, false},
		{"CanNotReadDevice", args{context.TODO(), "UnNormal"}, false, true},
	}

	stub := ReadDevice
	defer func() {
		ReadDevice = stub
	}()
	for _, tt := range tests {
		ReadDevice = func(_ context.Context, dev string) ([]byte, error) {
			if dev != "Normal" {
				return []byte{}, errors.New("test error")
			}
			return []byte(dev), nil
		}
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsDeviceReadable(tt.args.ctx, tt.args.devicePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsDeviceReadable() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("IsDeviceReadable() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestXfsResize(t *testing.T) {
	type args struct {
		ctx        context.Context
		devicePath string
	}
	type outputs struct {
		output string
		err    error
	}
	tests := []struct {
		name    string
		args    args
		outputs outputs
		wantErr bool
	}{
		{"Normal", args{context.TODO(), "device path"}, outputs{"normal cmd output", nil}, false},
		{"ErrorOutput", args{context.TODO(), "device path"}, outputs{"unnormal cmd output", errors.New("test error")},
			true},
	}

	stub := utils.ExecShellCmd
	defer func() {
		utils.ExecShellCmd = stub
	}()
	for _, tt := range tests {
		utils.ExecShellCmd = func(_ context.Context, format string, args ...interface{}) (string, error) {
			return tt.outputs.output, tt.outputs.err
		}
		t.Run(tt.name, func(t *testing.T) {
			if err := xfsResize(tt.args.ctx, tt.args.devicePath); (err != nil) != tt.wantErr {
				t.Errorf("xfsResize() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetVirtualDevice(t *testing.T) {
	stub := GetDevicesByGUID
	stub2 := utils.ExecShellCmd
	defer func() {
		GetDevicesByGUID = stub
		utils.ExecShellCmd = stub2
	}()

	tests := getVirtualDeviceTest()
	for _, tt := range tests {
		GetDevicesByGUID = func(_ context.Context, tgtLunGUID string) ([]string, error) {
			return tt.mockOutputs.output, tt.mockOutputs.err
		}
		utils.ExecShellCmd = func(ctx context.Context, format string, args ...interface{}) (string, error) {
			return tt.mockOutputs.cmdOutput, tt.mockOutputs.cmdErr
		}

		t.Run(tt.name, func(t *testing.T) {
			dev, kind, err := GetVirtualDevice(tt.args.ctx, tt.args.LunWWN)
			if (err != nil) != tt.wantErr || dev != tt.wantDeviceName || kind != tt.wantDeviceKind {
				t.Errorf("GetVirtualDevice() error = %v, wantErr %v; dev: %s, want: %s; kind: %d, want: %d",
					err, tt.wantErr, dev, tt.wantDeviceName, kind, tt.wantDeviceKind)
			}
		})
	}
}

type VirtualDeviceArgs struct {
	ctx    context.Context
	LunWWN string
}
type VirtualDeviceOutputs struct {
	output    []string
	cmdOutput string
	err       error
	cmdErr    error
}

func getVirtualDeviceTest() []struct {
	name           string
	args           VirtualDeviceArgs
	mockOutputs    VirtualDeviceOutputs
	wantDeviceName string
	wantDeviceKind int
	wantErr        bool
} {
	return []struct {
		name           string
		args           VirtualDeviceArgs
		mockOutputs    VirtualDeviceOutputs
		wantDeviceName string
		wantDeviceKind int
		wantErr        bool
	}{
		{"NormalUltrapath*",
			VirtualDeviceArgs{context.TODO(), "7100e98b8e19b76d00e4069a00000003"},
			VirtualDeviceOutputs{[]string{"ultrapathh"},
				"", nil, nil}, "ultrapathh", UseUltraPathNVMe, false},
		{"NormalDm-*",
			VirtualDeviceArgs{context.TODO(), "7100e98b8e19b76d00e4069a00000003"},
			VirtualDeviceOutputs{[]string{"dm-2"},
				"lrwxrwxrwx. 1 root root       7 Mar 14 10:26 mpatha -> ../dm-2", nil, nil},
			"dm-2", UseDMMultipath, false},
		{"NormalPhysicalSd*",
			VirtualDeviceArgs{context.TODO(), "7100e98b8e19b76d00e4069a00000003"},
			VirtualDeviceOutputs{[]string{"sdd"},
				"", nil, nil}, "sdd", NotUseMultipath, false},
		{"NormalPhysicalNVMe*",
			VirtualDeviceArgs{context.TODO(), "7100e98b8e19b76d00e4069a00000003"},
			VirtualDeviceOutputs{[]string{"nvme1n1"},
				"", nil, nil}, "nvme1n1", NotUseMultipath, false},
		{"ErrorMultiUltrapath*",
			VirtualDeviceArgs{context.TODO(), "7100e98b8e19b76d00e4069a00000003"},
			VirtualDeviceOutputs{[]string{"ultrapathh", "ultrapathi"},
				"", nil, nil}, "", 0, true},
		{"ErrorPartitionUltrapath*",
			VirtualDeviceArgs{context.TODO(), "7100e98b8e19b76d00e4069a00000003"},
			VirtualDeviceOutputs{[]string{"ultrapathh", "ultrapathh2"},
				"", nil, nil}, "ultrapathh", UseUltraPathNVMe, false},
		{"ErrorPartitionDm-*",
			VirtualDeviceArgs{context.TODO(), "7100e98b8e19b76d00e4069a00000003"},
			VirtualDeviceOutputs{[]string{"dm-2"},
				"lrwxrwxrwx. 1 root root       7 Mar 14 10:26 mpatha2 -> ../dm-2", nil, nil},
			"", 0, false},
		{"ErrorPartitionNvme*",
			VirtualDeviceArgs{context.TODO(), "7100e98b8e19b76d00e4069a00000003"},
			VirtualDeviceOutputs{[]string{"nvme1n1", "nvme1n1p1"},
				"", nil, nil}, "nvme1n1", 0, false},
	}
}

func TestWatchDMDevice(t *testing.T) {
	var cases = []struct {
		name             string
		lunWWN           string
		lunName          string
		expectPathNumber int
		devices          []string
		aggregatedTime   time.Duration
		pathCompleteTime time.Duration
		err              error
	}{
		{"Normal", "6582575100bc510f12345678000103e8", "dm-0", 3, []string{"sdb", "sdc", "sdd"},
			100 * time.Millisecond, 100 * time.Millisecond, nil},
		{"PathIncomplete", "6582575100bc510f12345678000103e8", "dm-0", 3, []string{"sdb", "sdc"},
			100 * time.Millisecond, 100 * time.Millisecond, errors.New(VolumePathIncomplete)},
		{"Timeout", "6582575100bc510f12345678000103e8", "dm-0", 3, []string{"sdb", "sdc", "sdd"},
			100 * time.Millisecond, 10000 * time.Millisecond, errors.New(VolumeNotFound)},
	}

	stubs := gostub.New()
	defer stubs.Reset()

	for _, c := range cases {
		var startTime = time.Now()

		stubs.Stub(&utils.ExecShellCmd, func(ctx context.Context, format string, args ...interface{}) (string, error) {
			if time.Now().Sub(startTime) > c.aggregatedTime {
				return fmt.Sprintf("name    sysfs uuid                             \nmpathja %s  %s", c.lunName,
					c.lunWWN), nil
			} else {
				return "", errors.New("err")
			}
		})

		stubs.Stub(&getDeviceFromDM, func(dm string) ([]string, error) {
			if time.Now().Sub(startTime) > c.pathCompleteTime {
				return c.devices, nil
			} else {
				return nil, errors.New(VolumeNotFound)
			}
		})

		_, err := WatchDMDevice(context.TODO(), c.lunWWN, c.expectPathNumber)
		assert.Equal(t, c.err, err, "%s, err:%v", c.name, err)
	}
}

func TestGetFsTypeByDevPath(t *testing.T) {
	type args struct {
		ctx     context.Context
		devPath string
	}
	type outputs struct {
		output string
		err    error
	}

	tests := []struct {
		name       string
		args       args
		mockOutput outputs
		want       string
		wantErr    bool
	}{
		{"Normal", args{context.TODO(), "/dev/dm-2"}, outputs{"xfs\n", nil}, "xfs", false},
		{"RunCommandError", args{context.TODO(), "/dev/dm-3"}, outputs{"", errors.New("mock error")}, "", true},
	}

	stub := utils.ExecShellCmd
	defer func() {
		utils.ExecShellCmd = stub
	}()

	for _, tt := range tests {
		utils.ExecShellCmd = func(ctx context.Context, format string, args ...interface{}) (string, error) {
			return tt.mockOutput.output, tt.mockOutput.err
		}

		t.Run(tt.name, func(t *testing.T) {
			fsType, err := GetFsTypeByDevPath(tt.args.ctx, tt.args.devPath)
			if (err != nil) != tt.wantErr || fsType != tt.want {
				t.Errorf("Test GetFsTypeByDevPath() error = %v, wantErr: [%v]; fsType: [%s], want: [%s]",
					err, tt.wantErr, fsType, tt.want)
			}
		})
	}
}

func TestGetDeviceTypeByName(t *testing.T) {
	tests := []struct {
		name       string
		deviceName string
		want       int
		wantErr    bool
	}{
		{name: "test_for_dm", deviceName: "dm-1", want: UseDMMultipath, wantErr: false},
		{name: "test_for_dm", deviceName: "mpathib", want: UseDMMultipath, wantErr: false},
		{name: "test_for_sd", deviceName: "sda", want: NotUseMultipath, wantErr: false},
		{name: "test_for_nvme", deviceName: "nvme1n1", want: NotUseMultipath, wantErr: false},
		{name: "test_for_ultrapath", deviceName: "sdu", want: UseUltraPath, wantErr: false},
		{name: "test_for_ultrapath-nvme", deviceName: "ultrapatha", want: UseUltraPathNVMe, wantErr: false},
		{name: "test_for_not_found", deviceName: "tty", want: 0, wantErr: true},
	}

	isUltraPath := gomonkey.ApplyFunc(isUltraPathDevice, func(ctx context.Context, device string) bool {
		return device == "sdu"
	})
	defer isUltraPath.Reset()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getDeviceTypeByName(context.Background(), tt.deviceName)
			if (err != nil) != tt.wantErr {
				t.Errorf("getDeviceTypeByName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getDeviceTypeByName() want = %v, get = %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetDeviceFromMountFile(t *testing.T) {
	tests := []struct {
		name        string
		targetPath  string
		checkDevRef bool
		want        string
		mountMap    map[string]string
		wantErr     bool
	}{
		{name: "test_device_not_exist", targetPath: "/mnt/test",
			checkDevRef: true, want: "", mountMap: map[string]string{}, wantErr: true},
		{name: "test_device_exist_and_ref_one_path", targetPath: "/mnt/test1", checkDevRef: true, want: "/dev/sda",
			mountMap: map[string]string{"/mnt/test1": "/dev/sda", "/mnt/test2": "/dev/sdb"}, wantErr: false},
		{name: "test_device_exist_and_ref_multiple_path", targetPath: "/mnt/test1", checkDevRef: true, want: "",
			mountMap: map[string]string{"/mnt/test1": "/dev/sda", "/mnt/test2": "/dev/sda"}, wantErr: true},
		{name: "test_device_exist_and_ref_multiple_path_and_no_check", targetPath: "/mnt/test1", checkDevRef: false,
			want: "/dev/sda", mountMap: map[string]string{"/mnt/test1": "/dev/sda", "/mnt/test2": "/dev/sda"},
			wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			readMount := gomonkey.ApplyFunc(ReadMountPoints, func(ctx context.Context) (map[string]string, error) {
				return tt.mountMap, nil
			})
			defer readMount.Reset()

			got, err := GetDeviceFromMountFile(context.Background(), tt.targetPath, tt.checkDevRef)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetDeviceFromMountFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetDeviceFromMountFile() want = %v, got = %v", tt.want, got)
			}
		})
	}
}

func TestGetDeviceFromSymLink(t *testing.T) {
	mockDeviceName := "mock-target-device"
	mockTargetName := "mock-target-path"
	mockTargetNameWithoutLink := "mock-target-path-without-link"

	tempDir, done := helperFuncForTestGetDeviceFromSymLink(t, mockTargetNameWithoutLink,
		mockDeviceName, mockTargetName)
	if done {
		return
	}

	tests := []struct {
		name       string
		targetPath string
		want       string
		wantErr    bool
	}{
		{
			name:       "test_get_link_device_from_target_path",
			targetPath: filepath.Join(tempDir, mockTargetName),
			want:       filepath.Join(tempDir, mockDeviceName),
			wantErr:    false,
		},
		{
			name:       "test_target_path_without_link_device",
			targetPath: filepath.Join(tempDir, mockTargetNameWithoutLink),
			want:       "",
			wantErr:    true,
		},
		{
			name:       "test_target_path_not_exist",
			targetPath: "not-exist-path",
			want:       "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetDeviceFromSymLink(tt.targetPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetDeviceFromSymLink() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetDeviceFromSymLink() want = %s, got = %s", tt.want, got)
			}
		})
	}
	if err := os.RemoveAll(tempDir); err != nil {
		t.Errorf("remove dir %s failed, error: %+v", tempDir, err)
	}
}

func helperFuncForTestGetDeviceFromSymLink(t *testing.T, mockTargetNameWithoutLink string, mockDeviceName string,
	mockTargetName string) (string, bool) {
	tempDir, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Errorf("create temp dir failed, error: %v", err)
		return "", true
	}

	withoutLinkPath, err := os.Create(path.Join(tempDir, mockTargetNameWithoutLink))
	if err != nil {
		t.Errorf("create mock target without link file failed, error: %v", err)
		return "", true
	}
	defer func() {
		if err = withoutLinkPath.Close(); err != nil {
			t.Errorf("close file %s failed, error: %v", withoutLinkPath.Name(), err)
		}
	}()
	err = withoutLinkPath.Chmod(0600)
	if err != nil {
		t.Errorf("file withoutLinkPath chmod to 0600 failed, error: %v", err)
		return "", true
	}
	deviceFile, err := os.Create(path.Join(tempDir, mockDeviceName))
	if err != nil {
		t.Errorf("create mock device file failed, error: %v", err)
		return "", true
	}
	defer func() {
		if err := deviceFile.Close(); err != nil {
			t.Errorf("close file %s failed, error: %v", deviceFile.Name(), err)
		}
	}()
	err = deviceFile.Chmod(0600)
	if err != nil {
		t.Errorf("file deviceFile chmod to 0600 failed, error: %v", err)
		return "", true
	}

	if err := os.Symlink(path.Join(tempDir, mockDeviceName), path.Join(tempDir, mockTargetName)); err != nil {
		t.Errorf("create symlink failed, error: %v", err)
		return "", true
	}

	return tempDir, false
}

func TestRemoveWwnType(t *testing.T) {
	tests := []struct {
		wwn  string
		want string
	}{
		{wwn: "t10.1600", want: "600"}, {wwn: "t10.600", want: "600"},
		{wwn: "1600", want: "600"}, {wwn: "eui.2600", want: "600"},
		{wwn: "eui.600", want: "600"}, {wwn: "2600", want: "600"},
		{wwn: "naa.3600", want: "600"}, {wwn: "naa.600", want: "600"},
		{wwn: "3600", want: "600"},
	}
	for _, tt := range tests {
		t.Run("TestRemoveWwnType", func(t *testing.T) {
			assert.Equalf(t, tt.want, removeWwnType(tt.wwn), "removeWwnType(%v)", tt.wwn)
		})
	}
}
