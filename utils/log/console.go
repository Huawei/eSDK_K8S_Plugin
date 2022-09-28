/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2022. All rights reserved.
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

// Package log output logged entries to respective logging hooks
package log

import (
	"fmt"
	"io"
	"os"

	"github.com/sirupsen/logrus"
)

// ConsoleHook sends log entries to stdout/stderr.
type ConsoleHook struct {
	formatter logrus.Formatter
}

// newConsoleHook creates a new log hook for writing to stdout/stderr.
func newConsoleHook(logFormat logrus.Formatter) (*ConsoleHook, error) {
	return &ConsoleHook{logFormat}, nil
}

// Levels returns all supported levels
func (hook *ConsoleHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// Fire ensure logging of respective log entries
func (hook *ConsoleHook) Fire(entry *logrus.Entry) error {

	// Determine output stream
	var logWriter io.Writer
	switch entry.Level {
	case logrus.DebugLevel, logrus.InfoLevel, logrus.WarnLevel:
		logWriter = os.Stdout
	case logrus.ErrorLevel, logrus.FatalLevel:
		logWriter = os.Stderr
	default:
		return fmt.Errorf("unknown log level: %v", entry.Level)
	}

	lineBytes, err := hook.formatter.Format(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to read entry, %v", err)
		return err
	}

	if _, err := logWriter.Write(lineBytes); err != nil {
		return err
	}

	return nil
}
