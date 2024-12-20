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
	"strconv"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app/config"
)

const (
	defaultFileSize   = 1024 * 1024 * 20 // 20M
	defaultLogDir     = "/var/log/huawei"
	defaultLogLevel   = "info"
	defaultLogModule  = "file"
	defaultMaxBackups = 9
)

// loggingOptions include log's configuration
type loggingOptions struct {
	logFileSize   string
	loggingModule string
	logLevel      string
	logFileDir    string
	maxBackups    uint
}

// NewLoggingOptions returns logging configurations
func NewLoggingOptions() *loggingOptions {
	return &loggingOptions{
		loggingModule: defaultLogModule,
		logLevel:      defaultLogLevel,
		logFileDir:    defaultLogDir,
		logFileSize:   strconv.Itoa(defaultFileSize),
		maxBackups:    defaultMaxBackups,
	}
}

// AddFlags add the log flags
func (opt *loggingOptions) AddFlags(ff *flag.FlagSet) {
	ff.StringVar(&opt.logFileSize, "log-file-size",
		strconv.Itoa(defaultFileSize),
		"Maximum file size before log rotation")
	ff.UintVar(&opt.maxBackups, "max-backups",
		defaultMaxBackups,
		"maximum number of backup log file")
	ff.StringVar(&opt.loggingModule, "logging-module",
		defaultLogModule,
		"Flag enable one of available logging module (file, console)")
	ff.StringVar(&opt.logLevel, "log-level",
		defaultLogLevel,
		"Set logging level (debug, info, error, warning, fatal)")
	ff.StringVar(&opt.logFileDir, "log-file-dir",
		defaultLogDir,
		"The flag to specify logging directory. The flag is only supported if logging module is file")
}

// ApplyFlags assign the log flags
func (opt *loggingOptions) ApplyFlags(cfg *config.AppConfig) {
	cfg.MaxBackups = opt.maxBackups
	cfg.LoggingModule = opt.loggingModule
	cfg.LogFileDir = opt.logFileDir
	cfg.LogFileSize = opt.logFileSize
	cfg.LogLevel = opt.logLevel
}

// ValidateFlags validate the log flags
func (opt *loggingOptions) ValidateFlags() []error {
	errs := make([]error, 0)
	err := opt.validateLogLevel()
	if err != nil {
		errs = append(errs, err)
	}

	err = opt.validateLogModule()
	if err != nil {
		errs = append(errs, err)
	}

	return errs
}

func (opt *loggingOptions) validateLogLevel() error {
	switch opt.logLevel {
	case "debug", "info", "warning", "error", "fatal":
		return nil
	default:
		return fmt.Errorf("invalid logging level [%v]", opt.logLevel)
	}
}

func (opt *loggingOptions) validateLogModule() error {
	switch opt.loggingModule {
	case "file", "console":
		return nil
	default:
		return fmt.Errorf("invalid logging module [%v]. Support only 'file' or 'console'", opt.loggingModule)
	}
}
