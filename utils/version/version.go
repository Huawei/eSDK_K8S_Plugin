/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2025. All rights reserved.
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

// Package version offers version inits options
package version

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	versionFilePermission = 0644
	newline               = '\n'
)

var mutex sync.Mutex

// InitVersion used for init the version of the service
func InitVersion(versionFile, version string) error {
	mutex.Lock()
	defer mutex.Unlock()
	log.Infof("Init version is %s", version)
	fInfo, err := os.Lstat(versionFile)
	if err == nil {
		if fInfo.IsDir() {
			msg := fmt.Sprintf("Version file %v exists and is a directory, please remove it", versionFile)
			log.Errorln(msg)
			return errors.New(msg)
		}
		err := os.Remove(versionFile)
		if err != nil {
			log.Errorf("remove version file %v", err)
			return err
		}
	}
	file, err := os.OpenFile(versionFile, os.O_CREATE|os.O_SYNC|os.O_RDWR, versionFilePermission)
	if err != nil {
		if os.IsExist(err) {
			log.Infof("Open version file %s is exist.", versionFile)
			return nil
		}
		log.Errorln("Open version file %s failed", versionFile)
		return err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Warningf("Close file %s error: [%v]", versionFile, err)
		}
	}(file)

	versionByte := []byte(version)
	versionByte = append(versionByte, newline)
	_, err = file.Write(versionByte)
	if err != nil {
		msg := fmt.Sprintf("Failed to write the version to file %s, error: %v.", versionFile, err)
		log.Errorln(msg)
		return errors.New(msg)
	}
	return nil
}

// ClearVersion used for clear the version of the service
func ClearVersion(versionFile string) error {
	mutex.Lock()
	defer mutex.Unlock()
	fInfo, err := os.Lstat(versionFile)
	if err != nil {
		if os.IsNotExist(err) {
			log.Infof("version file %s does not exist, no need to clear.", versionFile)
			return nil
		}

		msg := fmt.Sprintf("Check version file %s error %v", versionFile, err)
		log.Errorln(msg)
		return errors.New(msg)
	}

	if fInfo.IsDir() {
		log.Warningf("Version file %v exists and is a directory.", versionFile)
		return nil
	}

	err = os.Remove(versionFile)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove version file [%s]. %s", versionFile, err)
		}
	}

	return nil
}

// CompareVersions compares two semantic versions
// Returns:
//
//	-1 if v1 < v2
//	 0 if v1 == v2
//	 1 if v1 > v2
func CompareVersions(v1, v2 string) int {
	// Split versions into numeric segments
	segments1 := strings.Split(v1, ".")
	segments2 := strings.Split(v2, ".")

	maxLen := max(len(segments1), len(segments2))

	for i := 0; i < maxLen; i++ {
		result := compareSegmentValue(segments1, segments2, i)
		if result == 0 {
			continue
		}
		return result
	}
	return 0
}

func compareSegmentValue(segments1, segments2 []string, index int) int {
	if index >= len(segments1) && index < len(segments2) {
		return -1
	}
	if index < len(segments1) && index >= len(segments2) {
		return 1
	}

	str1, str2 := segments1[index], segments2[index]

	num1, err := strconv.Atoi(str1)
	if err != nil {
		return strings.Compare(str1, str2)
	}
	num2, err := strconv.Atoi(str2)
	if err != nil {
		return strings.Compare(str1, str2)
	}

	if num1 > num2 {
		return 1
	} else if num1 < num2 {
		return -1
	}

	return 0
}
