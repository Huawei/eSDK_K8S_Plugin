package plugin

import (
	"context"
	"os"
	"path"
	"reflect"
	"testing"

	"bou.ke/monkey"

	"huawei-csi-driver/storage/oceanstor/client"
	"huawei-csi-driver/utils/log"
)

const (
	logName string = "oceanstor-nas_test.log"
	logDir  string = "/var/log/huawei"
)

func TestInit(t *testing.T) {
	tests := []struct {
		name       string
		config     map[string]interface{}
		parameters map[string]interface{}
		keepLogin  bool
		wantErr    bool
	}{
		{"Normal",
			map[string]interface{}{"urls": []interface{}{"*.*.*.*"}, "user": "testUser", "password": "2e0273ba51d5c30866", "keyText": "0NuSPbY4r6rANmmAipqPTMRpSlz3OULX"},
			map[string]interface{}{"protocol": "nfs", "portals": []interface{}{"*.*.*.*"}},
			false, false,
		},
		{"ProtocolErr",
			map[string]interface{}{"urls": []interface{}{"*.*.*.*"}, "user": "testUser", "password": "2e0273ba51d5c30866", "keyText": "0NuSPbY4r6rANmmAipqPTMRpSlz3OULX"},
			map[string]interface{}{"protocol": "wrong", "portals": []interface{}{"*.*.*.1"}},
			false, true,
		},
		{"PortNotUnique",
			map[string]interface{}{"urls": []interface{}{"*.*.*.*"}, "user": "testUser", "password": "2e0273ba51d5c30866", "keyText": "0NuSPbY4r6rANmmAipqPTMRpSlz3OULX"},
			map[string]interface{}{"protocol": "wrong", "portals": []interface{}{"*.*.*.1", "*.*.*.2"}},
			false, true,
		},
	}

	var cli *client.Client
	monkey.PatchInstanceMethod(reflect.TypeOf(cli), "Logout", func(*client.Client, context.Context) {})
	monkey.PatchInstanceMethod(reflect.TypeOf(cli), "Login", func(*client.Client, context.Context) error {
		return nil
	})
	monkey.PatchInstanceMethod(reflect.TypeOf(cli), "GetSystem", func(*client.Client, context.Context) (map[string]interface{}, error) {
		return map[string]interface{}{"PRODUCTVERSION": "Test"}, nil
	})
	defer monkey.UnpatchAll()

	for _, tt := range tests {
		var p = &OceanstorNasPlugin{}
		t.Run(tt.name, func(t *testing.T) {
			if err := p.Init(tt.config, tt.parameters, tt.keepLogin); (err != nil) != tt.wantErr {
				t.Errorf("Init error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMain(m *testing.M) {
	if err := log.InitLogging(logName); err != nil {
		log.Errorf("Init logging: %s failed. error: %v", logName, err)
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
