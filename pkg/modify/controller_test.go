/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2023-2024. All rights reserved.
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

// Package claim
package modify

import (
	"testing"
	"time"
)

func TestWorkerThreads(t *testing.T) {
	// arrange
	ctrl := &VolumeModifyController{}
	workThreads := 10

	// action
	WorkerThreads(workThreads)(ctrl)

	// assert
	if ctrl.workerThreads != workThreads {
		t.Errorf("TestWorkerThreads failed, want %d, but got %d", workThreads, ctrl.workerThreads)
	}
}

func TestReSyncPeriod(t *testing.T) {
	// arrange
	ctrl := &VolumeModifyController{}
	reSyncPeriod := 10 * time.Second

	// action
	ReSyncPeriod(reSyncPeriod)(ctrl)

	// assert
	if ctrl.reSyncPeriod != reSyncPeriod {
		t.Errorf("TestReSyncPeriod failed, want %v, but got %v", reSyncPeriod, ctrl.reSyncPeriod)
	}
}

func TestRetryMaxDelay(t *testing.T) {
	// arrange
	ctrl := &VolumeModifyController{}
	retryMaxDelay := 10 * time.Second

	// action
	RetryMaxDelay(retryMaxDelay)(ctrl)

	// assert
	if ctrl.retryMaxDelay != retryMaxDelay {
		t.Errorf("TestRetryMaxDelay failed, want %v, but got %v", retryMaxDelay, ctrl.retryMaxDelay)
	}
}

func TestRetryBaseDelay(t *testing.T) {
	// arrange
	ctrl := &VolumeModifyController{}
	retryBaseDelay := 10 * time.Second

	// action
	RetryBaseDelay(retryBaseDelay)(ctrl)

	// assert
	if ctrl.retryBaseDelay != retryBaseDelay {
		t.Errorf("TestRetryBaseDelay failed, want %v, but got %v", retryBaseDelay, ctrl.retryBaseDelay)
	}
}

func TestReconcileClaimStatusDelay(t *testing.T) {
	// arrange
	ctrl := &VolumeModifyController{}
	reconcileClaimStatusDelay := 10 * time.Second

	// action
	ReconcileClaimStatusDelay(reconcileClaimStatusDelay)(ctrl)

	// assert
	if ctrl.reconcileClaimStatusDelay != reconcileClaimStatusDelay {
		t.Errorf("TestReconcileClaimStatusDelay failed, want %v, but got %v",
			reconcileClaimStatusDelay, ctrl.reconcileClaimStatusDelay)
	}
}
