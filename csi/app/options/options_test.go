/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2023-2023. All rights reserved.
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

// Package options control the service configurations, include env and config
package options

import (
	"flag"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/sirupsen/logrus"

	"huawei-csi-driver/csi/app/config"
)

func initOptions() *optionsManager {
	return NewOptionsManager()
}

func TestConfig(t *testing.T) {
	flagSet := flag.NewFlagSet("fake-huawei-csi", flag.ContinueOnError)
	if err := flagSet.Parse(os.Args); err != nil {
		logrus.Fatalf("Parse flag error: %v", err)
	}

	m := gomonkey.ApplyFunc(GetCurrentPodNameSpace, func() string {
		return "mock-namespace"
	})
	defer m.Reset()

	optManager := initOptions()
	optManager.AddFlags(flagSet)

	var err error
	envCfg, err := optManager.Config()
	if err != nil {
		logrus.Fatalf("Failed to get configuration, %v", err)
	}

	if err := compareLogOptions(envCfg); err != nil {
		t.Error(err.Error())
	}

	if err := compareConnectorOptions(envCfg); err != nil {
		t.Error(err.Error())
	}
}

func compareLogOptions(envCfg *config.Config) error {
	expectLogOptions := NewLoggingOptions()
	actuallyLogOptions := &loggingOptions{
		logFileSize:   envCfg.LogFileSize,
		loggingModule: envCfg.LoggingModule,
		logLevel:      envCfg.LogLevel,
		logFileDir:    envCfg.LogFileDir,
		maxBackups:    envCfg.MaxBackups,
	}

	if !reflect.DeepEqual(expectLogOptions, actuallyLogOptions) {
		return fmt.Errorf("TestConfig failed, expectLogOptions %v, actuallyLogOptions %v",
			expectLogOptions, actuallyLogOptions)
	}
	return nil
}

func compareConnectorOptions(envCfg *config.Config) error {
	expectConnectorOptions := NewConnectorOptions()
	actuallyConnectorOptions := &connectorOptions{
		volumeUseMultiPath:   true,
		scsiMultiPathType:    dmMultiPath,
		nvmeMultiPathType:    hwUltraPathNVMe,
		deviceCleanupTimeout: defaultCleanupTimeout,
		scanVolumeTimeout:    defaultScanVolumeTimeout,
		connectorThreads:     defaultConnectorThreads,
	}

	if !reflect.DeepEqual(expectConnectorOptions, actuallyConnectorOptions) {
		return fmt.Errorf("TestConfig failed, expectConnectorOptions %v, actuallyConnectorOptions %v",
			expectConnectorOptions, actuallyConnectorOptions)
	}
	return nil
}
