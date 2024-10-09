/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2024-2024. All rights reserved.
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

package connector

import (
	"context"
	"errors"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/require"
)

func TestGetUltraPathVLunByWWN(t *testing.T) {
	ctx := context.Background()
	wwn := "testWwn"

	t.Run("success", func(t *testing.T) {
		// arrange
		commandResult := "   1     deleted  testDisk  testWwn  Fault    1.00GB " +
			"       0B/0B       Huawei.Storage      61                   0/2"
		expectedVLun := &UltrapathVLun{
			ID:     "1",
			Disk:   "deleted",
			Name:   "testDisk",
			WWN:    "testWwn",
			Status: "Fault",
			upType: UltraPathCommand,
		}

		// mock
		//runUpCommand()
		gomonkey.ApplyFuncReturn(runUpCommand, commandResult, nil)

		// action
		vLun, err := GetUltrapathVLunByWWN(ctx, UltraPathCommand, wwn)

		// assert
		require.NoError(t, err)
		require.Equal(t, expectedVLun, vLun)
	})

	t.Run("no residual path", func(t *testing.T) {
		// arrange

		// mock
		gomonkey.ApplyFuncReturn(runUpCommand, nil, errors.New(exitStatus1))

		// action
		vLun, err := GetUltrapathVLunByWWN(ctx, UltraPathCommand, "testWwn")

		// assert
		require.NoError(t, err)
		require.Nil(t, vLun)
	})
}

func TestUltraPathVLun_CleanResidualPath(t *testing.T) {
	t.Run("doesn't need to clean", func(t *testing.T) {
		// arrange
		vLun := &UltrapathVLun{
			ID:     "1",
			Disk:   "sda",
			Name:   "testDisk",
			WWN:    "testWwn",
			Status: diskStatusNormal,
			upType: UltraPathCommand,
		}

		// action
		err := vLun.CleanResidualPath(context.Background())

		// assert
		require.NoError(t, err)
	})

	t.Run("clean successfully", func(t *testing.T) {
		// arrange
		vLun := &UltrapathVLun{
			ID:     "1",
			Disk:   "deleted",
			Name:   "testDisk",
			WWN:    "testWwn",
			Status: "Fault",
			upType: UltraPathCommand,
		}
		vLunDetails := "Path 62 [7:0:0:2] (up-270)  : Fault\nPath 63 [6:0:0:2] (up-269)  : Fault"
		expectedPhyDevices := []string{"7:0:0:2", "6:0:0:2"}
		var actualPhyDevices []string

		// mock
		gomonkey.ApplyFuncReturn(runUpCommand, vLunDetails, nil)
		gomonkey.ApplyFunc(deletePhysicalDevice, func(ctx context.Context, phyDevice string) error {
			actualPhyDevices = append(actualPhyDevices, phyDevice)
			return nil
		})

		// action
		err := vLun.CleanResidualPath(context.Background())

		// assert
		require.NoError(t, err)
		require.Equal(t, expectedPhyDevices, actualPhyDevices)
	})

	t.Run("clean physical paths", func(t *testing.T) {
		// arrange
		vLun := &UltrapathVLun{
			ID:     "1",
			Disk:   "deleted",
			Name:   "testDisk",
			WWN:    "testWwn",
			Status: "Fault",
			upType: UltraPathCommand,
		}
		vLunDetails := "Path 62 [7:0:0:2] (up-270)  : Fault\nPath 63 [6:0:0:2] (up-269)  : Normal\n "
		expectedPhyDevices := []string{"7:0:0:2", "6:0:0:2"}
		var actualPhyDevices []string

		// mock
		gomonkey.ApplyFuncReturn(runUpCommand, vLunDetails, nil)
		gomonkey.ApplyFunc(deletePhysicalDevice, func(ctx context.Context, phyDevice string) error {
			actualPhyDevices = append(actualPhyDevices, phyDevice)
			return nil
		})

		// action
		err := vLun.CleanResidualPath(context.Background())

		// assert
		require.NoError(t, err)
		require.Equal(t, expectedPhyDevices, actualPhyDevices)
	})
}
