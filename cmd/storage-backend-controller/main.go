/*
 Copyright (c) Huawei Technologies Co., Ltd. 2022-2024. All rights reserved.

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

// Package main entry point for application
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	coreV1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"

	"huawei-csi-driver/csi/app"
	clientSet "huawei-csi-driver/pkg/client/clientset/versioned"
	backendScheme "huawei-csi-driver/pkg/client/clientset/versioned/scheme"
	backendInformers "huawei-csi-driver/pkg/client/informers/externalversions"
	"huawei-csi-driver/pkg/storage-backend/controller"
	"huawei-csi-driver/pkg/utils"
	"huawei-csi-driver/pkg/webhook"
	"huawei-csi-driver/utils/log"
)

const (
	containerName        = "storage-backend-controller"
	eventComponentName   = "XuanWu-StorageBackend-Mngt"
	leaderLockObjectName = "storage-backend-controller"

	backoffDuration = 100 * time.Millisecond
	backoffFactor   = 1.5
	backoffSteps    = 10
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
		logrus.Fatalf("Init logger [%s] failed. error: [%v]", containerName, err)
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

	// start the webhook
	recorder := initRecorder(k8sClient)
	webHook := initWebhookController(recorder)
	webHookCfg, admissionWebhooks := webhook.GetStorageWebHookCfg()
	if err = webHook.Start(ctx, webHookCfg, admissionWebhooks); err != nil {
		log.AddContext(ctx).Errorf("Failed to start webhook controller: %v", err)
		return
	}

	signalChan := make(chan os.Signal, 1)
	defer close(signalChan)

	startWithLeaderElectionOnCondition(ctx, k8sClient, crdClient, recorder, signalChan)

	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGILL, syscall.SIGKILL, syscall.SIGTERM)
	stopSignal := <-signalChan
	log.AddContext(ctx).Warningf("stop main, stopSignal is [%v]", stopSignal)
}

func initWebhookController(recorder record.EventRecorder) *webhook.Controller {
	return &webhook.Controller{
		Recorder: recorder,
	}
}

func initRecorder(client kubernetes.Interface) record.EventRecorder {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&coreV1.EventSinkImpl{Interface: client.CoreV1().Events(v1.NamespaceAll)})
	return eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: fmt.Sprintf(eventComponentName)})
}

func runController(
	ctx context.Context,
	storageBackendClient *clientSet.Clientset,
	eventRecorder record.EventRecorder, ch chan os.Signal) {

	if ch == nil {
		log.AddContext(ctx).Errorln("the channel should not be nil")
		return
	}

	// Add StorageBackend types to the default Kubernetes so events can be logged for them
	if err := backendScheme.AddToScheme(scheme.Scheme); err != nil {
		log.AddContext(ctx).Errorf("Add to scheme error: %v", err)
		ch <- syscall.SIGINT
		return
	}

	if err := ensureCRDExist(ctx, storageBackendClient); err != nil {
		log.AddContext(ctx).Errorf("Exiting due to failure to ensure CRDs exist during startup: %+v", err)
		ch <- syscall.SIGINT
		return
	}

	factory := backendInformers.NewSharedInformerFactory(storageBackendClient, app.GetGlobalConfig().ReSyncPeriod)
	ctrl := controller.NewBackendController(controller.BackendControllerRequest{
		ClientSet:       storageBackendClient,
		ClaimInformer:   factory.Xuanwu().V1().StorageBackendClaims(),
		ContentInformer: factory.Xuanwu().V1().StorageBackendContents(),
		ReSyncPeriod:    app.GetGlobalConfig().ReSyncPeriod,
		EventRecorder:   eventRecorder})

	run := func(ctx context.Context) {
		// run...
		stopCh := make(chan struct{})
		factory.Start(stopCh)
		go ctrl.Run(ctx, app.GetGlobalConfig().WorkerThreads, stopCh)

		// Stop the controller when stop signals are received
		utils.WaitExitSignal(ctx, "controller")

		close(stopCh)
	}

	run(ctx)
}

func ensureCRDExist(ctx context.Context, client *clientSet.Clientset) error {
	exist := func() (bool, error) {
		_, err := utils.ListClaim(ctx, client, "")
		if err != nil {
			log.AddContext(ctx).Errorf("Failed to list StorageBackendClaims, error: %v", err)
			return false, nil
		}

		_, err = utils.ListContent(ctx, client)
		if err != nil {
			log.AddContext(ctx).Errorf("Failed to list StorageBackendContents, error: %v", err)
			return false, nil
		}

		return true, nil
	}

	backoff := wait.Backoff{
		Duration: backoffDuration,
		Factor:   backoffFactor,
		Steps:    backoffSteps,
	}
	if err := wait.ExponentialBackoff(backoff, exist); err != nil {
		return err
	}

	return nil
}

func startWithLeaderElectionOnCondition(ctx context.Context, k8sClient *kubernetes.Clientset,
	crdClient *clientSet.Clientset, recorder record.EventRecorder, ch chan os.Signal) {
	if !app.GetGlobalConfig().EnableLeaderElection {
		log.AddContext(ctx).Infoln("Start controller without leader election.")
		go runController(ctx, crdClient, recorder, ch)
	} else {
		leaderElection := utils.LeaderElectionConf{
			LeaderName:    leaderLockObjectName,
			LeaseDuration: app.GetGlobalConfig().LeaderLeaseDuration,
			RenewDeadline: app.GetGlobalConfig().LeaderRenewDeadline,
			RetryPeriod:   app.GetGlobalConfig().LeaderRetryPeriod,
		}

		runFun := func(ctx context.Context, ch chan os.Signal) {
			runController(ctx, crdClient, recorder, ch)
		}

		go utils.RunWithLeaderElection(ctx, leaderElection, k8sClient, recorder, runFun, ch)
	}
}
