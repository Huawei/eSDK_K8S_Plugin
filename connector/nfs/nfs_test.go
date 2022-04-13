package nfs

import (
	"context"
	"errors"
	"os"
	"path"
	"testing"

	"github.com/prashantv/gostub"

	"huawei-csi-driver/connector"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

const (
	logDir  = "/var/log/huawei/"
	logName = "nfsTest.log"
)

func testExecShellCmd(_ context.Context, format string, args ...interface{}) (string, error) {
	if format == "blkid -o udev %s" {
		return "ID_FS_TYPE=xxx\n", nil
	}

	if args[0] == "err-targetPath" {
		return "err output", errors.New("not found")
	}

	return "", nil
}

func TestConnectVolume(t *testing.T) {
	var ctx = context.TODO()

	if err := os.MkdirAll("test-sourcePath", 0750); err != nil {
		t.Fatal("can not create a source path")
	}
	defer utils.RemoveDir("test-sourcePath", "test-sourcePath")
	defer utils.RemoveDir("test-targetPath", "test-targetPath")

	var blockConnMap = map[string]interface{}{
		"srcType":    "block",
		"sourcePath": "test-sourcePath",
		"targetPath": "test-targetPath",
		"fsType":     "",
		"mountFlags": "",
	}
	var existFsTypeIsEmptyMap = map[string]interface{}{
		"srcType":    "block",
		"sourcePath": "sourcePath",
		"targetPath": "test-targetPath",
		"fsType":     "",
		"mountFlags": "",
	}
	var fsConnMap = map[string]interface{}{
		"srcType":    "fs",
		"sourcePath": "test-sourcePath",
		"targetPath": "test",
		"fsType":     "",
		"mountFlags": "test-flag",
	}
	var otherSrcTypeMap = map[string]interface{}{
		"srcType": "test",
	}
	var emptySrcTypeMap = map[string]interface{}{
		"srcType": "",
	}
	var emptySourcePathMap = map[string]interface{}{
		"srcType":    "block",
		"sourcePath": "",
	}
	var emptyTargetPathMap = map[string]interface{}{
		"srcType":    "fs",
		"sourcePath": "testSourcePath",
		"targetPath": "",
	}

	type args struct {
		ctx  context.Context
		conn map[string]interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{"EmptySrcType", args{ctx, emptySrcTypeMap}, "", true},
		{"EmptySourcePath", args{ctx, emptySourcePathMap}, "", true},
		{"EmptyTargetPath", args{ctx, emptyTargetPathMap}, "", true},

		{"SrcTypeIsOther", args{ctx, otherSrcTypeMap}, "", true},
		{"SrcTypeIsFS", args{ctx, fsConnMap}, "", false},

		{"SrcTypeIsBlock", args{ctx, blockConnMap}, "", false},
		{"ExistFsTypeIsEmpty", args{ctx, existFsTypeIsEmptyMap}, "", true},
	}

	stubs := gostub.StubFunc(&connector.ReadDevice, []byte{}, nil)
	defer stubs.Reset()

	stubs.StubFunc(&utils.PathExist, true, nil)
	stubs.StubFunc(&readFile, []byte("test test\n"), nil)
	stubs.StubFunc(&connector.ResizeMountPath, nil)
	stubs.StubFunc(&connector.IsInFormatting, false, nil)
	stubs.StubFunc(&connector.GetDeviceSize, int64(halfTiSizeBytes), nil)
	stubs.Stub(&utils.ExecShellCmd, testExecShellCmd)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nfs := &NFS{}
			got, err := nfs.ConnectVolume(tt.args.ctx, tt.args.conn)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConnectVolume() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ConnectVolume() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDisConnectVolume(t *testing.T) {
	var ctx = context.TODO()

	if err := os.MkdirAll("test-targetPath", 0750); err != nil {
		t.Error("can not create a source path")
	}
	if err := os.MkdirAll("err-targetPath", 0750); err != nil {
		t.Error("can not create a source path")
	}
	defer utils.RemoveDir("err-targetPath", "err-targetPath")

	type args struct {
		ctx        context.Context
		targetPath string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{"TargetPathNotExist", args{ctx, "test"}, false},
		{"UnmontFailed", args{ctx, "err-targetPath"}, true},
		{"Normal", args{ctx, "test-targetPath"}, false},
	}

	stubs := gostub.Stub(&utils.ExecShellCmd, testExecShellCmd)
	defer stubs.Reset()

	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			nfs := &NFS{}
			if err := nfs.DisConnectVolume(tt.args.ctx, tt.args.targetPath); (err != nil) != tt.wantErr {
				t.Errorf("DisConnectVolume() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
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
