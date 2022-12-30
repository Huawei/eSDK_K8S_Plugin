/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2022-2022. All rights reserved.
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

package driver

import (
	"context"
	"os"
	"path"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"huawei-csi-driver/utils/log"
)

const (
	logDir  = "/var/log/huawei/"
	logName = "controller_helper_test.log"
)

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

	m.Run()
}

func TestCheckReservedSnapshotSpaceRatio(t *testing.T) {
	Convey("Normal", t, func() {
		param := map[string]interface{}{
			"reservedSnapshotSpaceRatio": "50",
		}
		So(checkReservedSnapshotSpaceRatio(context.TODO(), param), ShouldBeNil)
	})

	Convey("Not int", t, func() {
		param := map[string]interface{}{
			"reservedSnapshotSpaceRatio": "20%",
		}
		So(checkReservedSnapshotSpaceRatio(context.TODO(), param), ShouldBeError)
	})

	Convey("Exceed the upper limit", t, func() {
		param := map[string]interface{}{
			"reservedSnapshotSpaceRatio": "60",
		}
		So(checkReservedSnapshotSpaceRatio(context.TODO(), param), ShouldBeError)
	})

	Convey("Below the lower limit", t, func() {
		param := map[string]interface{}{
			"reservedSnapshotSpaceRatio": "-10",
		}
		So(checkReservedSnapshotSpaceRatio(context.TODO(), param), ShouldBeError)
	})

}
