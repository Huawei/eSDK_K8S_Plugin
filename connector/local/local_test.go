package local

import (
	"context"
	"flag"
	"os"
	"path"
	"testing"
	"time"

	"github.com/prashantv/gostub"

	"huawei-csi-driver/connector"
	"huawei-csi-driver/connector/utils/lock"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

const (
	logDir  = "/var/log/huawei/"
	logName = "localTest.log"
)

func TestConnectVolume(t *testing.T) {
	var ctx = context.TODO()

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
		{
			name: "NoTgtLunWWN",
			args: args{
				ctx:  ctx,
				conn: map[string]interface{}{}},
			want:    "",
			wantErr: true,
		},
		{
			name: "devPathNoExist",
			args: args{
				ctx:  ctx,
				conn: map[string]interface{}{"tgtLunWWN": "test"},
			},
			want:    "",
			wantErr: false,
		},
		{
			name: "Normal",
			args: args{
				ctx:  ctx,
				conn: map[string]interface{}{"tgtLunWWN": "tgtLunWWN"},
			},
			want:    "/dev/disk/by-id/wwn-0xtgtLunWWN",
			wantErr: false,
		},
	}

	stubs := gostub.Stub(&waitDevOnlineTimeInterval, time.Millisecond)
	defer stubs.Reset()

	stubs.Stub(&utils.ExecShellCmd, func(ctx context.Context, format string, args ...interface{}) (string, error) {
		if args[0] == "/dev/disk/by-id/wwn-0xtgtLunWWN" {
			return "/dev/disk/by-id/wwn-0xtgtLunWWN", nil
		}
		return "ls: cannot access '/dev/disk/by-id/wwn-0xtgtLunWWN': No such file or directory", nil
	})
	stubs.StubFunc(&connector.VerifySingleDevice, nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loc := &Local{}
			got, err := loc.ConnectVolume(tt.args.ctx, tt.args.conn)
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

	type args struct {
		ctx       context.Context
		tgtLunWWN string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "EmptyTgtLunWWN",
			args: args{
				ctx:       ctx,
				tgtLunWWN: "",
			},
			wantErr: false,
		},
		{
			name: "DeviceNotExist",
			args: args{
				ctx:       ctx,
				tgtLunWWN: "test",
			},
			wantErr: false,
		},
		{
			name: "Normal",
			args: args{
				ctx:       ctx,
				tgtLunWWN: "tgtLunWWN",
			},
			wantErr: true,
		},
	}

	stubs := gostub.Stub(&connector.DisconnectVolumeTimeOut, time.Millisecond)
	defer stubs.Reset()

	stubs.Stub(&connector.DisconnectVolumeTimeInterval, time.Millisecond)
	stubs.Stub(&utils.ExecShellCmd, func(ctx context.Context, format string, args ...interface{}) (string, error) {
		if args[0] == "tgtLunWWN" {
			return "./../../sd-tgtLunWWN\n", nil
		}
		return "No such file or directory", nil
	})
	stubs.StubFunc(&connector.GetPhysicalDevices, []string{"sd-tgtLunWWN"}, nil)
	stubs.StubFunc(&connector.RemoveAllDevice, "", nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loc := &Local{}
			err := loc.DisConnectVolume(tt.args.ctx, tt.args.tgtLunWWN)
			if (err != nil) != tt.wantErr {
				t.Errorf("DisConnectVolume() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

const defaultDriverName = "csi.huawei.com"

var driverName = flag.String("driver-name", defaultDriverName, "CSI driver name")

func TestMain(m *testing.M) {
	flag.Parse()

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

	if err := lock.InitLock(*driverName); err != nil {
		log.Errorf("test lock init failed: %v", err)
		os.Exit(1)
	}

	m.Run()
}
