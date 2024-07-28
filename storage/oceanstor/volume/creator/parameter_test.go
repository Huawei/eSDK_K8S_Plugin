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

package creator_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"huawei-csi-driver/storage/oceanstor/volume/creator"
)

func TestParameters_AllSquash(t *testing.T) {
	// arrange
	want := 1
	in := map[string]any{"allsquash": want}
	params := creator.NewParameter(in)

	// act
	got := params.AllSquash()

	// assert
	require.Equal(t, want, got)
}

func TestParameters_AllocType(t *testing.T) {
	// arrange
	want := 1
	in := map[string]any{"alloctype": want}
	params := creator.NewParameter(in)

	// act
	got := params.AllocType()

	// assert
	require.Equal(t, want, got)
}

func TestParameters_AuthClient(t *testing.T) {
	// arrange
	want := "127.0.0.1"
	in := map[string]any{"authclient": want}
	params := creator.NewParameter(in)

	// act
	got := params.AuthClient()

	// assert
	require.Equal(t, want, got)
}

func TestParameters_Backend(t *testing.T) {
	// arrange
	want := "test-backend"
	in := map[string]any{"backend": want}
	params := creator.NewParameter(in)

	// act
	got := params.Backend()

	// assert
	require.Equal(t, want, got)
}

func TestParameters_Capacity(t *testing.T) {
	// arrange
	want := int64(20971520)
	in := map[string]any{"capacity": want}
	params := creator.NewParameter(in)

	// act
	got := params.Capacity()

	// assert
	require.Equal(t, want, got)
}

func TestParameters_CloneFrom(t *testing.T) {
	// arrange
	want := "pvc_1342a6d1_6aa5_4dc4_ad4f_508c65e4c57c"
	in := map[string]any{"clonefrom": want}
	params := creator.NewParameter(in)

	// act
	got := params.CloneFrom()

	// assert
	require.Equal(t, want, got)
}

func TestParameters_CloneSpeed(t *testing.T) {
	// arrange
	want := 4
	in := map[string]any{"clonespeed": want}
	params := creator.NewParameter(in)

	// act
	got := params.CloneSpeed()

	// assert
	require.Equal(t, want, got)
}

func TestParameters_Description(t *testing.T) {
	// arrange
	want := "description of filesystem"
	in := map[string]any{"description": want}
	params := creator.NewParameter(in)

	// act
	got := params.Description()

	// assert
	require.Equal(t, want, got)
}

func TestParameters_FsPermission(t *testing.T) {
	// arrange
	want := "777"
	in := map[string]any{"fspermission": want}
	params := creator.NewParameter(in)

	// act
	got := params.FsPermission()

	// assert
	require.Equal(t, want, got)
}

func TestParameters_IsClone_True(t *testing.T) {
	// arrange
	cloneFrom := "pvc_1342a6d1_6aa5_4dc4_ad4f_508c65e4c57c"
	in := map[string]any{"clonefrom": cloneFrom}
	params := creator.NewParameter(in)

	// act
	got := params.IsClone()

	// assert
	require.True(t, got)
}

func TestParameters_IsClone_False(t *testing.T) {
	// arrange
	in := map[string]any{}
	params := creator.NewParameter(in)

	// act
	got := params.IsClone()

	// assert
	require.False(t, got)
}

func TestParameters_IsHyperMetro_True(t *testing.T) {
	// arrange
	in := map[string]any{"hypermetro": true}
	params := creator.NewParameter(in)

	// act
	got := params.IsHyperMetro()

	// assert
	require.True(t, got)
}

func TestParameters_IsHyperMetro_False(t *testing.T) {
	// arrange
	cases := []map[string]any{
		{"hypermetro": false}, // value is false
		{},                    // key not exists
	}

	for _, c := range cases {
		// act
		params := creator.NewParameter(c)
		got := params.IsHyperMetro()

		// assert
		require.False(t, got)
	}
}

func TestParameters_IsReplication_True(t *testing.T) {
	// arrange
	in := map[string]any{"replication": true}
	params := creator.NewParameter(in)

	// act
	got := params.IsReplication()

	// assert
	require.True(t, got)
}

func TestParameters_IsReplication_False(t *testing.T) {
	// arrange
	cases := []map[string]any{
		{"replication": false}, // value is false
		{},                     // key not exists
	}

	for _, c := range cases {
		// act
		params := creator.NewParameter(c)
		got := params.IsReplication()

		// assert
		require.False(t, got)
	}
}

func TestParameters_IsSkipNfsShareAndQoS_True(t *testing.T) {
	// arrange
	in := map[string]any{"skipNfsShareAndQos": true}
	params := creator.NewParameter(in)

	// act
	got := params.IsSkipNfsShareAndQos()

	// assert
	require.True(t, got)
}
func TestParameters_IsSkipNfsShareAndQoS_False(t *testing.T) {
	// arrange
	cases := []map[string]any{
		{"skipNfsShareAndQos": false}, // value is false
		{},                            // key not exists
	}

	for _, c := range cases {
		// act
		params := creator.NewParameter(c)
		got := params.IsSkipNfsShareAndQos()

		// assert
		require.False(t, got)
	}

}

func TestParameters_IsSnapshot_True(t *testing.T) {
	// arrange
	snapshotName := "snapshot_72b7d526_bec8_4e5b_81ee_ca16a3dcb000"
	in := map[string]any{"fromSnapshot": snapshotName}
	params := creator.NewParameter(in)

	// act
	got := params.IsSnapshot()

	// assert
	require.True(t, got)
}

func TestParameters_IsSnapshot_False(t *testing.T) {
	// arrange
	in := map[string]any{} // fromSnapshot key is not exists
	params := creator.NewParameter(in)

	// act
	got := params.IsSnapshot()

	// assert
	require.False(t, got)
}

func TestParameters_MetroDomainID(t *testing.T) {
	// arrange
	want := "8070607d40000104"
	in := map[string]any{"metroDomainID": want}
	params := creator.NewParameter(in)

	// act
	got := params.MetroDomainID()

	// assert
	require.Equal(t, want, got)
}

func TestParameters_PoolID(t *testing.T) {
	// arrange
	want := "1"
	in := map[string]any{"poolID": want}
	params := creator.NewParameter(in)

	// act
	got := params.PoolID()

	// assert
	require.Equal(t, want, got)
}

func TestParameters_PvcName(t *testing.T) {
	// arrange
	want := "pvc_e598a267_0b64_4a55_8b03_2c6c362f2dce"
	in := map[string]any{"name": want}
	params := creator.NewParameter(in)

	// act
	got := params.PvcName()

	// assert
	require.Equal(t, want, got)
}

func TestParameters_RemoteStoragePool(t *testing.T) {
	// arrange
	want := "StoragePool001"
	in := map[string]any{"remotestoragepool": want}
	params := creator.NewParameter(in)

	// act
	got := params.RemoteStoragePool()

	// assert
	require.Equal(t, want, got)
}

func TestParameters_RootSquash(t *testing.T) {
	// arrange
	want := 1
	in := map[string]any{"rootsquash": want}
	params := creator.NewParameter(in)

	// act
	got := params.RootSquash()

	// assert
	require.Equal(t, want, got)
}

func TestParameters_SnapshotParentId(t *testing.T) {
	// arrange
	want := "1"
	in := map[string]any{"snapshotparentid": want}
	params := creator.NewParameter(in)

	// act
	got := params.SnapshotParentId()

	// assert
	require.Equal(t, want, got)
}

func TestParameters_SourceSnapshotName(t *testing.T) {
	// arrange
	want := "snapshot-82759d80-1df8-4eeb-a053-1d3b3e390477"
	in := map[string]any{"sourcesnapshotname": want}
	params := creator.NewParameter(in)

	// act
	got := params.SourceSnapshotName()

	// assert
	require.Equal(t, want, got)
}

func TestParameters_SourceVolumeName(t *testing.T) {
	// arrange
	want := "pvc_1342a6d1_6aa5_4dc4_ad4f_508c65e4c57c"
	in := map[string]any{"sourcevolumename": want}
	params := creator.NewParameter(in)

	// act
	got := params.SourceVolumeName()

	// assert
	require.Equal(t, want, got)
}

func TestParameters_StoragePool(t *testing.T) {
	// arrange
	want := "StoragePool001"
	in := map[string]any{"storagepool": want}
	params := creator.NewParameter(in)

	// act
	got := params.StoragePool()

	// assert
	require.Equal(t, want, got)
}

func TestParameters_ActiveVStoreID(t *testing.T) {
	// arrange
	want := "1"
	in := map[string]any{"localVStoreID": want}
	params := creator.NewParameter(in)

	// act
	got := params.ActiveVStoreID()

	// assert
	require.Equal(t, want, got)
}

func TestParameters_StandByVStoreID(t *testing.T) {
	// arrange
	want := "1"
	in := map[string]any{"remoteVStoreID": want}
	params := creator.NewParameter(in)

	// act
	got := params.StandByVStoreID()

	// assert
	require.Equal(t, want, got)
}

func TestParameters_VStorePairId(t *testing.T) {
	// arrange
	want := "21008070607d40000000000600000000"
	in := map[string]any{"vstorepairid": want}
	params := creator.NewParameter(in)

	// act
	got := params.VStorePairId()

	// assert
	require.Equal(t, want, got)
}

func TestParameters_Product(t *testing.T) {
	// arrange
	want := creator.StorageProduct("DoradoV6")
	in := map[string]any{"product": "DoradoV6"}
	params := creator.NewParameter(in)

	// act
	got := params.Product()

	// assert
	require.Equal(t, want, got)
}

func TestParameters_ProductIsV6(t *testing.T) {
	// arrange
	in := map[string]any{"product": "DoradoV6"}
	params := creator.NewParameter(in)

	// act
	got := params.Product().IsV6()

	// assert
	require.True(t, got)
}
