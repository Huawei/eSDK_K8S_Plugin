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

package log

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	logFilePermission = 0640
	backupTimeFormat  = "20060102-150405"
)

// FileHook sends log entries to a file.
type FileHook struct {
	logFileHandle        *fileHandler
	logRotationThreshold int64
	formatter            logrus.Formatter
	logRotateMutex       *sync.Mutex
}

// ensure interface implementation
var _ flushable = &FileHook{}
var _ closable = &FileHook{}

// newFileHook creates a new log hook for writing to a file.
func newFileHook(logFilePath, logFileSize string, logFormat logrus.Formatter) (*FileHook, error) {
	logFileRootDir := filepath.Dir(logFilePath)
	dir, err := os.Lstat(logFileRootDir)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(logFileRootDir, 0750); err != nil {
			return nil, fmt.Errorf("could not create log directory %v. %v", logFileRootDir, err)
		}
	}
	if dir != nil && !dir.IsDir() {
		return nil, fmt.Errorf("log path %v exists and is not a directory, please remove it", logFileRootDir)
	}

	filesizeThreshold, err := getNumInByte(logFileSize)
	if err != nil {
		return nil, fmt.Errorf("error in evaluating max log file size: %v. Check 'logFileSize' flag", err)
	}

	return &FileHook{
		logRotationThreshold: filesizeThreshold,
		formatter:            logFormat,
		logFileHandle:        newFileHandler(logFilePath),
		logRotateMutex:       &sync.Mutex{}}, nil
}

// Close file handler
func (hook *FileHook) close() {
	// All writes are synced and no file descriptor are left to close with current implementation
}

// Flush commits the current contents of the file
func (hook *FileHook) flush() {
	// All writes are synced and no file descriptor are left to close with current implementation
}

// Levels returns all supported levels
func (hook *FileHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// Fire ensure logging of respective log entries
func (hook *FileHook) Fire(entry *logrus.Entry) error {
	// Get formatted entry
	lineBytes, err := hook.formatter.Format(entry)
	if err != nil {
		return fmt.Errorf("could not read log entry. %v", err)
	}

	// Write log entry to file
	_, err = hook.logFileHandle.writeString(string(lineBytes))
	if err != nil {
		// let logrus print error message
		return fmt.Errorf("write log message [%s] error. %s", lineBytes, err)
	}

	// Rotate the file as needed
	if err = hook.maybeDoLogfileRotation(); err != nil {
		return err
	}

	return nil
}

// logfileNeedsRotation checks to see if a file has grown too large
func (hook *FileHook) logfileNeedsRotation() bool {
	fileInfo, err := hook.logFileHandle.stat()
	if err != nil {
		return false
	}

	return fileInfo.Size() >= hook.logRotationThreshold
}

// maybeDoLogfileRotation check and perform log rotation
func (hook *FileHook) maybeDoLogfileRotation() error {
	if hook.logfileNeedsRotation() {
		hook.logRotateMutex.Lock()
		defer hook.logRotateMutex.Unlock()

		if hook.logfileNeedsRotation() {
			// Do the rotation.
			err := hook.logFileHandle.rotate()
			if err != nil {
				return err
			}
		}
	}

	return nil
}

type fileHandler struct {
	rwLock   *sync.RWMutex
	filePath string
}

func newFileHandler(logFilePath string) *fileHandler {
	return &fileHandler{
		filePath: logFilePath,
	}
}

func (f *fileHandler) stat() (os.FileInfo, error) {
	return os.Stat(f.filePath)
}

func (f *fileHandler) writeString(s string) (int, error) {
	file, err := os.OpenFile(f.filePath, os.O_CREATE|os.O_APPEND|os.O_RDWR, logFilePermission)
	if err != nil {
		return 0, fmt.Errorf("failed to open log file with error [%v]", err)
	}
	defer file.Close()
	return file.WriteString(s)
}

func (f *fileHandler) rotate() error {
	// Do the rotation.
	rotatedLogFileLocation := f.filePath + time.Now().Format(backupTimeFormat)
	if err := os.Rename(f.filePath, rotatedLogFileLocation); err != nil {
		return fmt.Errorf("failed to create backup file. %s", err)
	}
	if err := os.Chmod(rotatedLogFileLocation, 0440); err != nil {
		return fmt.Errorf("failed to chmod backup file. %s", err)
	}

	// try to remove old backup files
	backupFiles, err := f.sortedBackupLogFiles()
	if err != nil {
		return err
	}

	if maxBackups < uint(len(backupFiles)) {
		oldBackupFiles := backupFiles[maxBackups:]

		for _, file := range oldBackupFiles {
			err := os.Remove(filepath.Join(filepath.Dir(f.filePath), file.Name()))
			if err != nil {
				return fmt.Errorf("failed to remove old backup file [%s]. %s", file.Name(), err)
			}
		}
	}
	return nil
}

type logFileInfo struct {
	timestamp time.Time
	os.FileInfo
}

func (f *fileHandler) sortedBackupLogFiles() ([]logFileInfo, error) {
	files, err := ioutil.ReadDir(filepath.Dir(f.filePath))
	if err != nil {
		return nil, fmt.Errorf("can't read log file directory: %s", err)
	}

	logFiles := make([]logFileInfo, 0)
	baseLogFileName := filepath.Base(f.filePath)

	// take out log files from directory
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		// ignore files other than log file and current log file itself
		fileName := f.Name()
		if !strings.HasPrefix(fileName, baseLogFileName) || fileName == baseLogFileName {
			continue
		}

		timestamp, err := time.Parse(backupTimeFormat, fileName[len(baseLogFileName):])
		if err != nil {
			logrus.Warningf("Failed parsing log file suffix timestamp. %s", err)
			continue
		}

		logFiles = append(logFiles, logFileInfo{timestamp, f})
	}

	sort.Sort(byTimeFormat(logFiles))

	return logFiles, nil
}

type byTimeFormat []logFileInfo

func (by byTimeFormat) Less(i, j int) bool {
	return by[i].timestamp.After(by[j].timestamp)
}

func (by byTimeFormat) Swap(i, j int) {
	by[i], by[j] = by[j], by[i]
}

func (by byTimeFormat) Len() int {
	return len(by)
}

func getNumInByte(logFileSize string) (int64, error) {
	var sum int64 = 0
	var err error

	maxDataNum := strings.ToUpper(logFileSize)
	lastLetter := maxDataNum[len(maxDataNum)-1:]

	// 1.最后一位是M
	// 1.1 获取M前面的数字 * 1024 * 1024
	// 2.最后一位是K
	// 2.1 获取K前面的数字 * 1024
	// 3.最后一位是数字或者B
	// 3.1 若最后一位是数字，则直接返回 若最后一位是B，则获取前面的数字返回
	if lastLetter >= "0" && lastLetter <= "9" {
		sum, err = strconv.ParseInt(maxDataNum, 10, 64)
		if err != nil {
			return 0, err
		}
	} else {
		sum, err = strconv.ParseInt(maxDataNum[:len(maxDataNum)-1], 10, 64)
		if err != nil {
			return 0, err
		}

		if lastLetter == "M" {
			sum *= 1024 * 1024
		} else if lastLetter == "K" {
			sum *= 1024
		}
	}

	return sum, nil
}
