package nvme

import (
	"context"
	"errors"
	"flag"
	"os"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/prashantv/gostub"

	"huawei-csi-driver/connector"
	"huawei-csi-driver/connector/utils/lock"
	"huawei-csi-driver/utils/log"
)

const (
	logDir  = "/var/log/huawei/"
	logName = "nvmeTest.log"
)

func TestConnectVolume(t *testing.T) {
	var ctx = context.TODO()

	var mutex = sync.Mutex{}

	var GetSubSysInfoOutput = map[string]interface{}{
		"Subsystems": []interface{}{
			map[string]interface{}{
				"Paths": []interface{}{
					map[string]interface{}{
						"Transport": "fc",
						"State":     "live",
						"Name":      "channelName",
						"Address":   "address",
					},
				},
			},
		},
	}

	var normalConnMap = map[string]interface{}{
		"tgtLunGuid":         "LunGUID",
		"volumeUseMultiPath": true,
		"multiPathType":      "UseUltraPath",
		"portWWNList":        []PortWWNPair{{"address", "address"}},
	}
	var noTgtLunGuidConnMap = map[string]interface{}{}
	var noVolumeUseMultiPathConnMap = map[string]interface{}{
		"tgtLunGuid": "LunGUID",
	}
	var noMultiPathTypeConnMap = map[string]interface{}{
		"tgtLunGuid":         "LunGUID",
		"volumeUseMultiPath": true,
	}
	var noPortWWNListConnMap = map[string]interface{}{
		"tgtLunGuid":         "LunGUID",
		"volumeUseMultiPath": true,
		"multiPathType":      "UseUltraPath",
	}

	type args struct {
		ctx  context.Context
		conn map[string]interface{}
	}
	tests := []struct {
		name    string
		mutex   sync.Mutex
		args    args
		want    string
		wantErr bool
	}{
		{"Normal", mutex, args{ctx, normalConnMap}, "/dev/NVMeVirtualDevice", false},
		{"NoTgtLunGuid", mutex, args{ctx, noTgtLunGuidConnMap}, "", true},
		{"NoVolumeUseMultiPath", mutex, args{ctx, noVolumeUseMultiPathConnMap}, "", true},
		{"NoMultiPathType", mutex, args{ctx, noMultiPathTypeConnMap}, "", true},
		{"NoPortWWNList", mutex, args{ctx, noPortWWNListConnMap}, "", true},
	}

	stubs := gostub.StubFunc(&connector.GetSubSysInfo, GetSubSysInfoOutput, nil)
	defer stubs.Reset()
	stubs.StubFunc(&connector.DoScanNVMeDevice, nil)
	stubs.StubFunc(&connector.GetDevNameByLunWWN, "NVMeVirtualDevice", nil)
	stubs.StubFunc(&connector.IsUpNVMeResidualPath, false, nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fc := &FCNVMe{
				mutex: tt.mutex,
			}
			got, err := fc.ConnectVolume(tt.args.ctx, tt.args.conn)
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

	var mutex = sync.Mutex{}

	type args struct {
		ctx        context.Context
		tgtLunGuid string
	}
	tests := []struct {
		name    string
		mutex   sync.Mutex
		args    args
		wantErr bool
	}{
		{"Normal", mutex, args{ctx, "LunGUID"}, false},
		{"GetVirtualDeviceError", mutex, args{ctx, "errTgtLunGUID"}, true},
		{"emptyVirtualDevice", mutex, args{ctx, "emptyTgtLunGUID"}, false},
	}

	stubs := gostub.Stub(&connector.GetVirtualDevice, func(ctx context.Context, tgtLunGUID string) (string, int, error) {
		if tgtLunGUID == "errTgtLunGUID" {
			return "", 0, errors.New("test err")
		}
		if tgtLunGUID == "emptyTgtLunGUID" {
			return "", 0, nil
		}
		return "test", 1, nil
	})
	defer stubs.Reset()

	stubs.StubFunc(&connector.GetNVMePhysicalDevices, []string{}, nil)
	stubs.StubFunc(&connector.RemoveAllDevice, "test", nil)
	stubs.StubFunc(&connector.FlushDMDevice, nil)
	stubs.Stub(&flushTimeInterval, time.Microsecond)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fc := &FCNVMe{
				mutex: tt.mutex,
			}
			if err := fc.DisConnectVolume(tt.args.ctx, tt.args.tgtLunGuid); (err != nil) != tt.wantErr {
				t.Errorf("DisConnectVolume() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

const defaultDriverName = "csi.huawei.com"

var driverName = flag.String("driver-name", defaultDriverName, "CSI driver name")

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

	if err := lock.InitLock(*driverName); err != nil {
		log.Errorf("test lock init failed: %v", err)
		os.Exit(1)
	}

	m.Run()
}
