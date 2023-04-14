/*
 Copyright (c) Huawei Technologies Co., Ltd. 2023-2023. All rights reserved.

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

// Package utils is leader election related utils
package utils

import (
	"context"
	"os"
	"syscall"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"

	"huawei-csi-driver/csi/app"
	clientSet "huawei-csi-driver/pkg/client/clientset/versioned"
	"huawei-csi-driver/utils/log"
)

// LeaderElectionConf include the configuration of leader election
type LeaderElectionConf struct {
	LeaderName    string
	LeaseDuration time.Duration
	RenewDeadline time.Duration
	RetryPeriod   time.Duration
}

// RunWithLeaderElection run the function with leader election
func RunWithLeaderElection(ctx context.Context, leaderElection LeaderElectionConf,
	k8sClient *kubernetes.Clientset, storageBackendClient *clientSet.Clientset, recorder record.EventRecorder,
	runFunc func(ctx context.Context, storageBackendClient *clientSet.Clientset,
		recorder record.EventRecorder, ch chan os.Signal), ch chan os.Signal) {

	if ch == nil {
		log.Errorln("the channel should not be nil")
		return
	}

	id, err := os.Hostname()
	if err != nil {
		log.AddContext(ctx).Errorf("Error getting hostname: %v", err)
		ch <- syscall.SIGINT
		return

	}

	lockConfig := resourcelock.ResourceLockConfig{
		Identity:      id,
		EventRecorder: recorder,
	}

	resourceLock, err := resourcelock.New(
		resourcelock.ConfigMapsLeasesResourceLock,
		app.GetGlobalConfig().Namespace,
		leaderElection.LeaderName,
		k8sClient.CoreV1(),
		k8sClient.CoordinationV1(),
		lockConfig)
	if err != nil {
		log.AddContext(ctx).Errorf("Error creating resource lock: %v", err)
		ch <- syscall.SIGINT
		return
	}

	leaderElectionConfig := leaderelection.LeaderElectionConfig{
		Lock:          resourceLock,
		LeaseDuration: leaderElection.LeaseDuration,
		RenewDeadline: leaderElection.RenewDeadline,
		RetryPeriod:   leaderElection.RetryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				go runFunc(ctx, storageBackendClient, recorder, ch)
			},
			OnStoppedLeading: func() {
				log.AddContext(ctx).Errorf("Controller manager lost master")
				ch <- syscall.SIGINT
			},
			OnNewLeader: func(identity string) {
				log.AddContext(ctx).Infof("New leader elected. Current leader %s", identity)
			},
		},
	}

	leaderElector, err := leaderelection.NewLeaderElector(leaderElectionConfig)
	if err != nil {
		log.AddContext(ctx).Errorf("Error creating leader elector: %v", err)
		ch <- syscall.SIGINT
		return
	}
	leaderElector.Run(ctx)
}
