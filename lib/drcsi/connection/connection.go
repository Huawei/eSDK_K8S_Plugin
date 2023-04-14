/*
 Copyright (c) Huawei Technologies Co., Ltd. 2022-2022. All rights reserved.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at
      http://www.apache.org/licenses/LICENSE-2.0
 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

// Package connection connect to grpc
package connection

import (
	"context"
	"sync"

	"github.com/kubernetes-csi/csi-lib-utils/connection"
	"github.com/kubernetes-csi/csi-lib-utils/metrics"
	"google.golang.org/grpc"
)

// Connect opens insecure gRPC connection to a CSI driver. Address must be either absolute path to UNIX domain socket
// file or have format '<protocol>://', following gRPC name resolution mechanism at
// https://github.com/grpc/grpc/blob/master/doc/naming.md.
func Connect(ctx context.Context, drCSIAddress string, metricsManager metrics.CSIMetricsManager) (conn *grpc.ClientConn, err error) {
	var m sync.Mutex
	var canceled bool
	ready := make(chan bool)
	go func() {
		conn, err = connection.Connect(drCSIAddress, metricsManager)

		m.Lock()
		defer m.Unlock()
		if err != nil && canceled {
			_ = conn.Close()
		}

		close(ready)
	}()

	select {
	case <-ctx.Done():
		m.Lock()
		defer m.Unlock()
		canceled = true
		return nil, ctx.Err()

	case <-ready:
		return conn, err
	}
}
