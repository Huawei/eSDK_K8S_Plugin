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

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app/config"
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

func compareLogOptions(envCfg *config.AppConfig) error {
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

func compareConnectorOptions(envCfg *config.AppConfig) error {
	expectConnectorOptions := NewConnectorOptions()
	actuallyConnectorOptions := &connectorOptions{
		volumeUseMultiPath:   true,
		scsiMultiPathType:    dmMultiPath,
		nvmeMultiPathType:    hwUltraPathNVMe,
		deviceCleanupTimeout: defaultCleanupTimeout,
		scanVolumeTimeout:    defaultScanVolumeTimeout,
		connectorThreads:     defaultConnectorThreads,
		allPathOnline:        false,
		execCommandTimeout:   0,
		enableRoCEConnect:    true,
	}

	if !reflect.DeepEqual(expectConnectorOptions, actuallyConnectorOptions) {
		return fmt.Errorf("TestConfig failed, expectConnectorOptions %v, actuallyConnectorOptions %v",
			expectConnectorOptions, actuallyConnectorOptions)
	}
	return nil
}

func TestValidateFlags_NegativeQPS(t *testing.T) {
	// Arrange
	opt := &serviceOptions{
		kubeApiQps:   -1.0,
		kubeApiBurst: 10,
	}

	// Act
	errs := opt.ValidateFlags()

	// Assert
	if len(errs) == 0 {
		t.Fatal("expected error for negative QPS, got none")
	}
	if errs[0].Error() != "kube-api-qps must be >= 0, got -1.00" {
		t.Errorf("unexpected error message: %s", errs[0].Error())
	}
}

func TestValidateFlags_NegativeBurst(t *testing.T) {
	// Arrange
	opt := &serviceOptions{
		kubeApiQps:   5.0,
		kubeApiBurst: -1,
	}

	// Act
	errs := opt.ValidateFlags()

	// Assert
	if len(errs) == 0 {
		t.Fatal("expected error for negative Burst, got none")
	}
	if errs[0].Error() != "kube-api-burst must be >= 0, got -1" {
		t.Errorf("unexpected error message: %s", errs[0].Error())
	}
}

func TestValidateFlags_QPSGteBurst(t *testing.T) {
	// Arrange
	opt := &serviceOptions{
		kubeApiQps:   10.0,
		kubeApiBurst: 10,
	}

	// Act
	errs := opt.ValidateFlags()

	// Assert
	if len(errs) == 0 {
		t.Fatal("expected error for QPS >= Burst, got none")
	}
	if errs[0].Error() != "kube-api-burst (10) must be > kube-api-qps (10.00)" {
		t.Errorf("unexpected error message: %s", errs[0].Error())
	}
}

func TestValidateFlags_ValidInput(t *testing.T) {
	// Arrange
	opt := &serviceOptions{
		kubeApiQps:   5.0,
		kubeApiBurst: 10,
	}

	// Act
	errs := opt.ValidateFlags()

	// Assert
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid input, got: %v", errs)
	}
}
