/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2026. All rights reserved.
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

// Package driver provides csi driver with controller, node, identity services
package driver

import (
	"context"
	"errors"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/k8sutils"
)

func TestCsiDriver_NodeGetVolumeStats_EmptyVolumeId(t *testing.T) {
	// arrange
	ctx := context.Background()
	driver := NewServer(constants.DefaultDriverName, constants.ProviderVersion, &k8sutils.KubeClient{}, "node1")
	req := &csi.NodeGetVolumeStatsRequest{
		VolumeId:   "",
		VolumePath: "/var/lib/kubelet/plugins/kubernetes.io/csi/pv/test-volume/globalmount",
	}

	// action
	resp, err := driver.NodeGetVolumeStats(ctx, req)

	// assert
	require.Error(t, err)
	require.Nil(t, resp)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.InvalidArgument, st.Code())
}

func TestCsiDriver_NodeGetVolumeStats_EmptyVolumePath(t *testing.T) {
	// arrange
	ctx := context.Background()
	driver := NewServer(constants.DefaultDriverName, constants.ProviderVersion, &k8sutils.KubeClient{}, "node1")
	req := &csi.NodeGetVolumeStatsRequest{
		VolumeId:   "backend.vol-name",
		VolumePath: "",
	}

	// action
	resp, err := driver.NodeGetVolumeStats(ctx, req)

	// assert
	require.Error(t, err)
	require.Nil(t, resp)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.InvalidArgument, st.Code())
}

func TestCsiDriver_NodeGetVolumeStats_CheckBlockDeviceError(t *testing.T) {
	// arrange
	ctx := context.Background()
	driver := NewServer(constants.DefaultDriverName, constants.ProviderVersion, &k8sutils.KubeClient{}, "node1")
	req := &csi.NodeGetVolumeStatsRequest{
		VolumeId:   "backend.vol-name",
		VolumePath: "/dev/nonexistent",
	}

	patches := gomonkey.ApplyFuncReturn(utils.IsBlockDevice, false, errors.New("no such device"))
	defer patches.Reset()

	// action
	resp, err := driver.NodeGetVolumeStats(ctx, req)

	// assert
	require.Error(t, err)
	require.Nil(t, resp)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.Internal, st.Code())
}

func TestCsiDriver_NodeGetVolumeStats_BlockVolume(t *testing.T) {
	// arrange
	ctx := context.Background()
	driver := NewServer(constants.DefaultDriverName, constants.ProviderVersion, &k8sutils.KubeClient{}, "node1")
	req := &csi.NodeGetVolumeStatsRequest{
		VolumeId:   "backend.vol-name",
		VolumePath: "/dev/sda",
	}
	expectedSize := int64(10737418240)

	patches := gomonkey.ApplyFuncReturn(utils.IsBlockDevice, true, nil).
		ApplyFuncReturn(utils.GetBlockDeviceSize, expectedSize, nil)
	defer patches.Reset()

	// action
	resp, err := driver.NodeGetVolumeStats(ctx, req)

	// assert
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Usage, 1)
	require.Equal(t, expectedSize, resp.Usage[0].Total)
	require.Equal(t, csi.VolumeUsage_BYTES, resp.Usage[0].Unit)
	require.Equal(t, int64(0), resp.Usage[0].Available)
	require.Equal(t, int64(0), resp.Usage[0].Used)
}

func TestCsiDriver_NodeGetVolumeStats_BlockVolume_GetSizeError(t *testing.T) {
	// arrange
	ctx := context.Background()
	driver := NewServer(constants.DefaultDriverName, constants.ProviderVersion, &k8sutils.KubeClient{}, "node1")
	req := &csi.NodeGetVolumeStatsRequest{
		VolumeId:   "backend.vol-name",
		VolumePath: "/dev/sda",
	}

	patches := gomonkey.ApplyFuncReturn(utils.IsBlockDevice, true, nil).
		ApplyFuncReturn(utils.GetBlockDeviceSize, int64(0), errors.New("stat error"))
	defer patches.Reset()

	// action
	resp, err := driver.NodeGetVolumeStats(ctx, req)

	// assert
	require.Error(t, err)
	require.Nil(t, resp)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.Internal, st.Code())
}

func TestCsiDriver_NodeGetVolumeStats_FilesystemVolume(t *testing.T) {
	// arrange
	ctx := context.Background()
	driver := NewServer(constants.DefaultDriverName, constants.ProviderVersion, &k8sutils.KubeClient{}, "node1")
	req := &csi.NodeGetVolumeStatsRequest{
		VolumeId:   "backend.vol-name",
		VolumePath: "/var/lib/kubelet/plugins/kubernetes.io/csi/pv/test-volume/globalmount",
	}
	volumeMetrics := &utils.VolumeMetrics{
		Available:  resource.NewQuantity(500*4096, resource.BinarySI),
		Capacity:   resource.NewQuantity(1000*4096, resource.BinarySI),
		Used:       resource.NewQuantity((1000-600)*4096, resource.BinarySI),
		Inodes:     resource.NewQuantity(2000, resource.BinarySI),
		InodesFree: resource.NewQuantity(1500, resource.BinarySI),
		InodesUsed: resource.NewQuantity(500, resource.BinarySI),
	}

	patches := gomonkey.ApplyFuncReturn(utils.IsBlockDevice, false, nil).
		ApplyFuncReturn(utils.GetVolumeMetrics, volumeMetrics, nil)
	defer patches.Reset()

	// action
	resp, err := driver.NodeGetVolumeStats(ctx, req)

	// assert
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Usage, 2)

	require.Equal(t, int64(500*4096), resp.Usage[0].Available)
	require.Equal(t, int64(1000*4096), resp.Usage[0].Total)
	require.Equal(t, int64((1000-600)*4096), resp.Usage[0].Used)
	require.Equal(t, csi.VolumeUsage_BYTES, resp.Usage[0].Unit)

	require.Equal(t, int64(1500), resp.Usage[1].Available)
	require.Equal(t, int64(2000), resp.Usage[1].Total)
	require.Equal(t, int64(500), resp.Usage[1].Used)
	require.Equal(t, csi.VolumeUsage_INODES, resp.Usage[1].Unit)
}

func TestCsiDriver_NodeGetVolumeStats_FilesystemVolume_GetMetricsError(t *testing.T) {
	// arrange
	ctx := context.Background()
	driver := NewServer(constants.DefaultDriverName, constants.ProviderVersion, &k8sutils.KubeClient{}, "node1")
	req := &csi.NodeGetVolumeStatsRequest{
		VolumeId:   "backend.vol-name",
		VolumePath: "/var/lib/kubelet/plugins/kubernetes.io/csi/pv/test-volume/globalmount",
	}

	patches := gomonkey.ApplyFuncReturn(utils.IsBlockDevice, false, nil).
		ApplyFuncReturn(utils.GetVolumeMetrics, nil, errors.New("statfs error"))
	defer patches.Reset()

	// action
	resp, err := driver.NodeGetVolumeStats(ctx, req)

	// assert
	require.Error(t, err)
	require.Nil(t, resp)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.Internal, st.Code())
}
