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

	"github.com/kubernetes-csi/csi-lib-utils/metrics"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	coreV1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/lib/drcsi/connection"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/lib/drcsi/rpc"
	clientSet "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/client/clientset/versioned"
	backendScheme "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/client/clientset/versioned/scheme"
	backendInformers "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/client/informers/externalversions"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/sidecar/controller"
	storageBackend "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/storage-backend/handle"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	containerName      = "storage-backend-sidecar"
	eventComponentName = "XuanWu-StorageBackend-Mngt"

	leaderLockObjectName = "sb-sidecar-"
	backoffDuration      = 100 * time.Millisecond
)

var (
	connect      *grpc.ClientConn
	providerName string
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
		log.Errorf("Init logger [%s] failed. error: [%v]", containerName, err)
		return
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

	// init the recorder
	recorder := initRecorder(k8sClient)
	connect, providerName = initProvider()

	signalChan := make(chan os.Signal, 1)
	defer close(signalChan)

	if !app.GetGlobalConfig().EnableLeaderElection {
		go runController(ctx, crdClient, recorder, signalChan)
	} else {
		leaderElection := utils.LeaderElectionConf{
			LeaderName:    leaderLockObjectName + providerName,
			LeaseDuration: app.GetGlobalConfig().LeaderLeaseDuration,
			RenewDeadline: app.GetGlobalConfig().LeaderRenewDeadline,
			RetryPeriod:   app.GetGlobalConfig().LeaderRetryPeriod,
		}

		runFun := func(ctx context.Context, ch chan os.Signal) {
			runController(ctx, crdClient, recorder, ch)
		}

		go utils.RunWithLeaderElection(ctx, leaderElection, k8sClient, recorder,
			runFun, signalChan)
	}

	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGILL, syscall.SIGKILL, syscall.SIGTERM)
	stopSignal := <-signalChan
	log.AddContext(ctx).Warningf("Stop main, stopSignal is [%v]", stopSignal)
}

func initRecorder(client kubernetes.Interface) record.EventRecorder {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&coreV1.EventSinkImpl{Interface: client.CoreV1().Events(v1.NamespaceAll)})
	return eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: fmt.Sprintf(eventComponentName)})
}

func runController(ctx context.Context, crdClient *clientSet.Clientset,
	eventRecorder record.EventRecorder, ch chan os.Signal) {
	if ch == nil {
		log.Errorln("the channel should not be nil")
		return
	}

	// Add StorageBackend types to the default Kubernetes so events can be logged for them
	if err := backendScheme.AddToScheme(scheme.Scheme); err != nil {
		log.AddContext(ctx).Errorf("Add to scheme error: %v", err)
		ch <- syscall.SIGINT
		return
	}

	if err := ensureCRDExist(ctx, crdClient); err != nil {
		log.AddContext(ctx).Errorf("Exiting due to failure to ensure CRDs exist during startup: %+v", err)
		ch <- syscall.SIGINT
		return
	}

	backend := storageBackend.NewBackend(connect)
	factory := backendInformers.NewSharedInformerFactory(crdClient,
		time.Second*time.Duration(app.GetGlobalConfig().BackendUpdateInterval))
	ctrl := controller.NewSideCarBackendController(controller.BackendControllerRequest{
		ProviderName:    providerName,
		ClientSet:       crdClient,
		Backend:         backend,
		TimeOut:         app.GetGlobalConfig().Timeout,
		ContentInformer: factory.Xuanwu().V1().StorageBackendContents(),
		ReSyncPeriod:    time.Second * time.Duration(app.GetGlobalConfig().BackendUpdateInterval),
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

func initProvider() (*grpc.ClientConn, string) {
	ctx, cancel := context.WithTimeout(context.Background(), app.GetGlobalConfig().Timeout)
	defer cancel()

	metricsManager := metrics.NewCSIMetricsManager("" /* driverName */)
	conn, err := connection.Connect(ctx, app.GetGlobalConfig().DrEndpoint, metricsManager)
	if err != nil {
		log.AddContext(ctx).Fatalf("Failed to connect to DR CSI provider: %v", err)
	}

	name, err := rpc.GetProviderName(ctx, conn)
	if err != nil {
		log.AddContext(ctx).Fatalf("Failed to get DR-CSI provider name: %+v", err)
	}
	log.AddContext(ctx).Infof("DR-CSI provider name: %s", name)

	return conn, name
}

func ensureCRDExist(ctx context.Context, client *clientSet.Clientset) error {
	exist := func() (bool, error) {
		_, err := utils.ListContent(ctx, client)
		if err != nil {
			log.AddContext(ctx).Errorf("Failed to list StorageBackendContents, error: %v", err)
			return false, nil
		}

		return true, nil
	}

	backoff := wait.Backoff{
		Duration: backoffDuration,
		Factor:   1.5,
		Steps:    10,
	}
	if err := wait.ExponentialBackoff(backoff, exist); err != nil {
		return err
	}

	return nil
}
