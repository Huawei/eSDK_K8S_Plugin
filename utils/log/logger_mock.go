/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2023-2025. All rights reserved.
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

// Package log provide the logging interfaces
package log

import (
	"os"
	"path"

	"github.com/sirupsen/logrus"
)

var (
	mockLogFileSize        = "1024"
	mockLoggingModule      = "console"
	mockLogLevel           = "info"
	mockLogFileDir         = "/var/log/huawei/"
	mockMaxBackups    uint = 3
)

// MockInitLogging mock init the logging service
func MockInitLogging(logName string) {
	if err := InitLogging(&Config{
		LogName:       logName,
		LogFileSize:   mockLogFileSize,
		LoggingModule: mockLoggingModule,
		LogLevel:      mockLogLevel,
		LogFileDir:    mockLogFileDir,
		MaxBackups:    mockMaxBackups,
	}); err != nil {
		logrus.Errorf("init logging: %s failed. error: %v", logName, err)
	}
}

// MockStopLogging mock stop the logging service
func MockStopLogging(logName string) {
	logFile := path.Join(mockLogFileDir, logName)
	if err := os.RemoveAll(logFile); err != nil {
		logrus.Errorf("Remove file: %s failed. error: %s", logFile, err)
	}
}
