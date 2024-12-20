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

// Package main use to start huawei-csi-extender services
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/lib/drcsi"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/lib/drcsi/rpc"
	clientSet "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/client/clientset/versioned"
	crdScheme "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/client/clientset/versioned/scheme"
	crdInformers "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/client/informers/externalversions"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/modify"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	containerName        = "huawei-csi-extender"
	leaderLockObjectName = "huawei-csi-extender"
	eventComponentName   = "Volume-Modify-Mngt"
)

func main() {
	if err := app.NewCommand().Execute(); err != nil {
		logrus.Fatalf("Execute app command failed. error: %v", err)
	}
	err := log.InitLogging(&log.Config{
		LogName:       containerName,
		LogFileSize:   app.GetGlobalConfig().LogFileSize,
		LoggingModule: app.GetGlobalConfig().LoggingModule,
		LogLevel:      app.GetGlobalConfig().LogLevel,
		LogFileDir:    app.GetGlobalConfig().LogFileDir,
		MaxBackups:    app.GetGlobalConfig().MaxBackups,
	})
	if err != nil {
		logrus.Fatalf("Init logger %s failed. error: %v", containerName, err)
	}

	ctx, err := log.SetRequestInfo(context.Background())
	if err != nil {
		log.AddContext(ctx).Infof("set request id failed, error is [%v]", err)
	}

	k8sClient, crdClient, err := utils.GetK8SAndCrdClient(ctx)
	if err != nil {
		log.AddContext(ctx).Errorf("GetK8SAndCrdClient failed, error: %v", err)
		return
	}

	recorder := utils.InitRecorder(k8sClient, eventComponentName)
	signalChan := make(chan os.Signal, 1)
	defer close(signalChan)

	startWithLeaderElectionOnCondition(ctx, k8sClient, crdClient, recorder, signalChan)

	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGILL, syscall.SIGKILL, syscall.SIGTERM)
	stopSignal := <-signalChan
	log.AddContext(ctx).Warningf("stop main, stopSignal is [%v]", stopSignal)
}

func runController(ctx context.Context, crdClient *clientSet.Clientset, k8sClient *kubernetes.Clientset,
	ch chan os.Signal) {
	if ch == nil {
		log.AddContext(ctx).Errorln("the channel should not be nil")
		return
	}

	// Add types to the default Kubernetes so events can be logged for them
	if err := crdScheme.AddToScheme(scheme.Scheme); err != nil {
		log.AddContext(ctx).Errorf("Add to scheme error: %v", err)
		ch <- syscall.SIGINT
		return
	}

	conn, provider, err := rpc.ConnectProvider()
	if err != nil {
		log.AddContext(ctx).Errorf("connect provider error: %v", err)
		ch <- syscall.SIGINT
		return
	}
	modifyClient := drcsi.NewModifyVolumeInterfaceClient(conn)
	factory := crdInformers.NewSharedInformerFactory(crdClient, app.GetGlobalConfig().VolumeModifyReSyncPeriod)
	controller := modify.NewVolumeModifyController(ctx, k8sClient, crdClient, factory,
		modify.Provisioner(provider),
		modify.ClientOfModify(modifyClient),
		modify.WorkerThreads(app.GetGlobalConfig().WorkerThreads),
		modify.ReSyncPeriod(app.GetGlobalConfig().VolumeModifyReSyncPeriod),
		modify.RetryMaxDelay(app.GetGlobalConfig().VolumeModifyRetryMaxDelay),
		modify.RetryBaseDelay(app.GetGlobalConfig().VolumeModifyRetryBaseDelay),
		modify.ReconcileClaimStatusDelay(app.GetGlobalConfig().VolumeModifyReconcileDelay))

	run := func(ctx context.Context) {
		stopCh := make(chan struct{})
		factory.Start(stopCh)
		go controller.Run(ctx, stopCh)

		// Stop the controller when stop signals are received
		utils.WaitExitSignal(ctx, "controller")

		close(stopCh)
	}

	run(ctx)
}

func startWithLeaderElectionOnCondition(ctx context.Context, k8sClient *kubernetes.Clientset,
	crdClient *clientSet.Clientset, recorder record.EventRecorder, ch chan os.Signal) {
	if !app.GetGlobalConfig().EnableLeaderElection {
		log.AddContext(ctx).Infoln("Start controller without leader election.")
		go runController(ctx, crdClient, k8sClient, ch)
	} else {
		leaderElection := utils.LeaderElectionConf{
			LeaderName:    leaderLockObjectName,
			LeaseDuration: app.GetGlobalConfig().LeaderLeaseDuration,
			RenewDeadline: app.GetGlobalConfig().LeaderRenewDeadline,
			RetryPeriod:   app.GetGlobalConfig().LeaderRetryPeriod,
		}

		runFunc := func(ctx context.Context, ch chan os.Signal) {
			runController(ctx, crdClient, k8sClient, ch)
		}
		go utils.RunWithLeaderElection(ctx, leaderElection, k8sClient, recorder, runFunc, ch)
	}
}
