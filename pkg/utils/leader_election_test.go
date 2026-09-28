/*
 Copyright (c) Huawei Technologies Co., Ltd. 2024-2026. All rights reserved.

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

package utils

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRunWithLeaderElection_NilChannel_LogsWithRequestID(t *testing.T) {
	// arrange
	runFunc := func(ctx context.Context, ch chan os.Signal) {}

	// action
	RunWithLeaderElection(context.Background(), LeaderElectionConf{}, runFunc, nil)

	// assert
}

func TestRunWithLeaderElection_NilClient_LogsWithRequestID(t *testing.T) {
	// arrange
	ch := make(chan os.Signal, 1)
	conf := LeaderElectionConf{
		Client: nil,
	}
	runFunc := func(ctx context.Context, ch chan os.Signal) {}

	// action
	RunWithLeaderElection(context.Background(), conf, runFunc, ch)

	// assert
	select {
	case sig := <-ch:
		assert.Equal(t, syscall.SIGINT, sig)
	case <-time.After(time.Second):
		t.Error("expected SIGINT signal on channel")
	}
}
