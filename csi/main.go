/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2025. All rights reserved.
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

package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/connector/host"
	connUtils "github.com/Huawei/eSDK_K8S_Plugin/v4/connector/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/connector/utils/lock"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/handler"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/job"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/driver"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/provider"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/lib/drcsi"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/cert"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/iputils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/notify"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/version"
)

const (
	versionFile       = "/csi/version"
	controllerLogFile = "huawei-csi-controller"
	nodeLogFile       = "huawei-csi-node"

	endpointDirPerm = 0755
)

var (
	csiVersion = constants.CSIVersion
	config     CSIConfig
	secret     CSISecret
)

// CSIConfig defines csi config
type CSIConfig struct {
	Backends []map[string]interface{} `json:"backends"`
}

// CSISecret defines csi secret
type CSISecret struct {
	Secrets map[string]interface{} `json:"secrets"`
}

func getLogFileName() string {
	if app.GetGlobalConfig().Controller {
		return controllerLogFile
	}

	return nodeLogFile
}

func ensureRuntimePanicLogging(ctx context.Context) {
	utils.RecoverPanic(ctx)

	log.Flush()
	log.Close()
}

func releaseStorageClient(ctx context.Context) {
	handler.NewCacheWrapper().Clear(ctx)
}

func runCSIController(ctx context.Context, csiDriver *driver.CsiDriver) {
	log.AddContext(ctx).Infoln("Run as huawei-csi-controller.")

	app.GetGlobalConfig().K8sUtils.Activate()

	// Clean up before exiting
	go exitClean(true)

	// Refresh backend cache
	go job.RunSyncBackendTaskInBackground()

	// register the kahu community DRCSI service
	go registerDRCSIServer()

	// expose csi controller server on k8s service
	go runCsiControllerOnService(ctx, csiDriver)

	// register the K8S community CSI service
	registerCSIServer(csiDriver)
}

func runCSINode(ctx context.Context, csiDriver *driver.CsiDriver) {
	go exitClean(false)

	// Init file lock
	err := lock.InitLock(app.GetGlobalConfig().DriverName)
	if err != nil {
		notify.Stop("Init Lock error for driver %s: %v", app.GetGlobalConfig().DriverName, err)
	}

	// Init version file on every node
	err = version.InitVersion(versionFile, csiVersion)
	if err != nil {
		log.AddContext(ctx).Warningf("Init version error: %v", err)
	}

	checkMultiPathService()

	triggerGarbageCollector()

	// Save host info to secret, such as: hostname, initiator
	go func() {
		if err := host.SaveNodeHostInfoToSecret(context.Background()); err != nil {
			notify.Stop("SaveNodeHostInfo fail ,error: [%v]", err)
		}
		log.Infof("save node info to secret success")
	}()

	// register the K8S community CSI service
	registerCSIServer(csiDriver)
}

func main() {
	// Processing Input Parameters
	if err := app.NewCommand().Execute(); err != nil {
		logrus.Fatalf("Execute app command failed. error: %v", err)
	}

	// Init logger
	err := log.InitLogging(&log.Config{
		LogName:       getLogFileName(),
		LogFileSize:   app.GetGlobalConfig().LogFileSize,
		LoggingModule: app.GetGlobalConfig().LoggingModule,
		LogLevel:      app.GetGlobalConfig().LogLevel,
		LogFileDir:    app.GetGlobalConfig().LogFileDir,
		MaxBackups:    app.GetGlobalConfig().MaxBackups,
	})
	if err != nil {
		logrus.Fatalf("Init log error: %v", err)
	}

	csiDriver := driver.NewServer(app.GetGlobalConfig().DriverName,
		csiVersion,
		app.GetGlobalConfig().K8sUtils,
		app.GetGlobalConfig().NodeName)

	// Start CSI service
	if app.GetGlobalConfig().Controller {
		runCSIController(context.Background(), csiDriver)
	} else {
		runCSINode(context.Background(), csiDriver)
	}
}

func registerDRCSIServer() {
	p := provider.NewProvider(app.GetGlobalConfig().DriverName, csiVersion)
	drListener := listenEndpoint(app.GetGlobalConfig().DrEndpoint)
	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(log.EnsureGRPCContext),
	}
	grpcServer := grpc.NewServer(opts...)
	drcsi.RegisterIdentityServer(grpcServer, p)
	drcsi.RegisterStorageBackendServer(grpcServer, p)
	drcsi.RegisterModifyVolumeInterfaceServer(grpcServer, p)

	if err := grpcServer.Serve(drListener); err != nil {
		notify.Stop("Start Huawei CSI driver error: %v", err)
	}
}

func runCsiControllerOnService(ctx context.Context, csiDriver *driver.CsiDriver) {
	if app.GetGlobalConfig().ExportCsiServerAddress == "" {
		return
	}

	ipWrapper := iputils.NewIPWrapper(app.GetGlobalConfig().ExportCsiServerAddress)
	if ipWrapper == nil {
		notify.Stop("ExportCsiServerAddress [%s] is not a valid ip", app.GetGlobalConfig().ExportCsiServerAddress)
	}

	address := fmt.Sprintf("%s:%d", ipWrapper.GetFormatPortalIP(), app.GetGlobalConfig().ExportCsiServerPort)
	listen, err := net.Listen("tcp", address)
	if err != nil {
		notify.Stop("listen on %s error: %v", address, err)
	}
	registerServerOnService(ctx, listen, csiDriver)
}

func registerServerOnService(ctx context.Context, listener net.Listener, d *driver.CsiDriver) {
	cred, err := cert.GetGrpcCredential(ctx)
	if err != nil {
		notify.Stop("start Huawei CSI driver on service error: %v", err)
	}
	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(log.EnsureGRPCContext),
		grpc.Creds(cred),
	}
	server := grpc.NewServer(opts...)

	csi.RegisterIdentityServer(server, d)
	csi.RegisterControllerServer(server, d)
	csi.RegisterNodeServer(server, d)

	log.Infof("starting Huawei CSI driver on service, listening on %s", listener.Addr().String())
	if err := server.Serve(listener); err != nil {
		notify.Stop("start Huawei CSI driver on service error: %v", err)
	}
}

func registerCSIServer(csiDriver *driver.CsiDriver) {
	listener := listenEndpoint(app.GetGlobalConfig().Endpoint)
	registerServer(listener, csiDriver)
}

func listenEndpoint(endpoint string) net.Listener {
	endpointDir := filepath.Dir(endpoint)
	_, err := os.Stat(endpointDir)
	if err != nil && os.IsNotExist(err) {
		err = os.Mkdir(endpointDir, endpointDirPerm)
		if err != nil {
			notify.Stop("Error creating directory %s. error: %v", endpoint, err)
		}
	} else {
		_, err = os.Stat(endpoint)
		if err == nil {
			log.Infof("Gonna remove old sock file %s", endpoint)
			err = os.Remove(endpoint)
			if err != nil {
				notify.Stop("Error removing directory %s. error: %v", endpoint, err)
			}
		}
	}
	listener, err := net.Listen("unix", endpoint)
	if err != nil {
		notify.Stop("Listen on %s error: %v", endpoint, err)
	}
	return listener
}

func registerServer(listener net.Listener, d *driver.CsiDriver) {
	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(log.EnsureGRPCContext),
	}
	server := grpc.NewServer(opts...)

	csi.RegisterIdentityServer(server, d)
	csi.RegisterControllerServer(server, d)
	csi.RegisterNodeServer(server, d)

	log.Infof("Starting Huawei CSI driver, listening on %s", app.GetGlobalConfig().Endpoint)
	if err := server.Serve(listener); err != nil {
		notify.Stop("Start Huawei CSI driver error: %v", err)
	}
}

func checkMultiPathService() {
	multipathConfig := map[string]interface{}{
		"SCSIMultipathType":  app.GetGlobalConfig().ScsiMultiPathType,
		"NVMeMultipathType":  app.GetGlobalConfig().NvmeMultiPathType,
		"volumeUseMultiPath": app.GetGlobalConfig().VolumeUseMultiPath,
	}

	requiredServices, err := utils.GetRequiredMultipath(context.Background(),
		multipathConfig, config.Backends)
	if err != nil {
		notify.Stop("Get required multipath services failed. Error: %v", err)
	}

	err = connUtils.VerifyMultipathService(requiredServices,
		utils.GetForbiddenMultipath(context.Background(), multipathConfig, config.Backends))
	if err != nil {
		notify.Stop("Check multipath service failed. error:%v", err)
	}
	log.Infof("Check multipath service success.")
}

func triggerGarbageCollector() {
	// Trigger stale device clean up and exit after cleanup completion or during timeout
	log.Debugf("Enter func triggerGarbageCollector")
	cleanupReport := make(chan error, 1)
	go func(ch chan error) {
		res := nodeStaleDeviceCleanup(context.Background(),
			app.GetGlobalConfig().K8sUtils,
			app.GetGlobalConfig().KubeletRootDir,
			app.GetGlobalConfig().DriverName,
			app.GetGlobalConfig().NodeName)
		ch <- res
		close(ch)
	}(cleanupReport)
	timeoutInterval := time.Second * time.Duration(app.GetGlobalConfig().DeviceCleanupTimeout)
	select {
	case report := <-cleanupReport:
		if report == nil {
			log.Infof("Successfully completed stale device garbage collection")
		} else {
			log.Errorf("Stale device garbage collection exited with error %s", report)
		}
	case <-time.After(timeoutInterval):
		log.Warningf("Stale device garbage collection incomplete, exited due to timeout")
	}
	return
}

func exitClean(isController bool) {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	stopChan := notify.GetStopChan()
	defer close(signalChan)
	defer close(stopChan)

	select {
	case sign := <-signalChan:
		log.Infof("Receive exit signal %v", sign)
		clean(isController)
	case <-stopChan:
		log.Infoln("Receive stop event")
		clean(isController)
		return
	}
}

func clean(isController bool) {
	ctx := context.TODO()
	// flush log
	ensureRuntimePanicLogging(ctx)
	if isController {
		// release client
		releaseStorageClient(ctx)
		app.GetGlobalConfig().K8sUtils.Deactivate()
	} else {
		// clean version file
		err := version.ClearVersion(versionFile)
		if err != nil {
			logrus.Warningf("clean version file error: %v", err)
		}
	}
}
