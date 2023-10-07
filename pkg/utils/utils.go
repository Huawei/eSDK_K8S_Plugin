/*
 Copyright (c) Huawei Technologies Co., Ltd. 2022-2023. All rights reserved.

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

// Package utils to provide utils for storageBackend
package utils

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/tools/cache"

	xuanwuv1 "huawei-csi-driver/client/apis/xuanwu/v1"
	"huawei-csi-driver/csi/app"
	"huawei-csi-driver/pkg/finalizers"
	"huawei-csi-driver/utils/log"
)

var (
	keyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc
)

const (
	// ConfigMapFinalizer used when configmap as a resource to create StorageBackendContent
	ConfigMapFinalizer = "storagebackend.xuanwu.huawei.io/configmap-protection"
	// SecretFinalizer used when secret as a resource to create StorageBackendContent
	SecretFinalizer = "storagebackend.xuanwu.huawei.io/secret-protection"
	// ClaimBoundFinalizer used when storageBackendClaim bound to a storageBackendContent
	ClaimBoundFinalizer = "storagebackend.xuanwu.huawei.io/storagebackendclaim-bound-protection"
	// ContentBoundFinalizer used when storageBackendContent bound to a storageBackendClaim
	ContentBoundFinalizer = "storagebackend.xuanwu.huawei.io/storagebackendcontent-bound-protection"
)

// StoreObjectUpdate updates given cache with a new object version from Informer
func StoreObjectUpdate(ctx context.Context, store cache.Store, obj interface{}, className string) (bool, error) {
	objName, err := keyFunc(obj)
	if err != nil {
		return false, fmt.Errorf("couldn't get key for object %+v: %v", obj, err)
	}
	oldObj, found, err := store.Get(obj)
	if err != nil {
		return false, fmt.Errorf("error finding %s %q in controller cache: %v", className, objName, err)
	}

	objAccessor, err := meta.Accessor(obj)
	if err != nil {
		return false, err
	}

	if !found {
		// This is a new object
		log.AddContext(ctx).Infof("storeObjectUpdate: adding %s %q, version %s", className, objName,
			objAccessor.GetResourceVersion())
		if err = store.Add(obj); err != nil {
			return false, fmt.Errorf("error adding %s %q to controller cache: %v", className, objName, err)
		}
		return true, nil
	}

	oldObjAccessor, err := meta.Accessor(oldObj)
	if err != nil {
		return false, err
	}

	objResourceVersion, err := strconv.ParseInt(objAccessor.GetResourceVersion(), 10, 64)
	if err != nil {
		return false, fmt.Errorf("error parsing ResourceVersion %q of %s %q: %s",
			objAccessor.GetResourceVersion(), className, objName, err)
	}
	oldObjResourceVersion, err := strconv.ParseInt(oldObjAccessor.GetResourceVersion(), 10, 64)
	if err != nil {
		return false, fmt.Errorf("error parsing old ResourceVersion %q of %s %q: %s",
			oldObjAccessor.GetResourceVersion(), className, objName, err)
	}

	// Throw away only older version, let the same version pass - we do want to
	// get periodic sync events.
	if oldObjResourceVersion > objResourceVersion {
		log.AddContext(ctx).Infof("storeObjectUpdate: ignoring %s %q version %s",
			className, objName, objAccessor.GetResourceVersion())
		return false, nil
	}

	log.AddContext(ctx).Infof("storeObjectUpdate updating %s %q with version %s",
		className, objName, objAccessor.GetResourceVersion())
	if err = store.Update(obj); err != nil {
		return false, fmt.Errorf("error updating %s %q in controller cache: %v", className, objName, err)
	}
	return true, nil
}

// StorageBackendClaimKey return the uniq name of claim
func StorageBackendClaimKey(storageBackend *xuanwuv1.StorageBackendClaim) string {
	return fmt.Sprintf("%s/%s", storageBackend.Namespace, storageBackend.Name)
}

// GenDynamicContentName return the uniq name of StorageBackendContent
func GenDynamicContentName(storageBackend *xuanwuv1.StorageBackendClaim) string {
	return fmt.Sprintf("content-%s", storageBackend.UID)
}

// IsClaimBoundContent indicates whether StorageBackendClaim bound to StorageBackendContent
func IsClaimBoundContent(storageBackend *xuanwuv1.StorageBackendClaim) bool {
	return storageBackend.Status != nil && storageBackend.Status.BoundContentName != ""
}

// NeedAddClaimBoundFinalizers returns whether to add a Finalizer to claim
func NeedAddClaimBoundFinalizers(storageBackend *xuanwuv1.StorageBackendClaim) bool {
	return storageBackend.ObjectMeta.DeletionTimestamp == nil && !finalizers.ContainsFinalizer(
		storageBackend, ClaimBoundFinalizer) && IsClaimBoundContent(storageBackend)
}

// NeedRemoveClaimBoundFinalizers returns whether to add a Finalizer
func NeedRemoveClaimBoundFinalizers(storageBackend *xuanwuv1.StorageBackendClaim) bool {
	return storageBackend.ObjectMeta.DeletionTimestamp != nil && finalizers.ContainsFinalizer(
		storageBackend, ClaimBoundFinalizer)
}

// IsClaimReady returns StorageBackendClaim ready or not
func IsClaimReady(storageBackend *xuanwuv1.StorageBackendClaim) bool {
	return storageBackend.Status != nil && storageBackend.Status.Phase == xuanwuv1.BackendBound
}

// IsContentReady returns StorageBackendContent ready or not
func IsContentReady(ctx context.Context, content *xuanwuv1.StorageBackendContent) bool {
	// If all spec info is empty, no need to call provider
	if content.Spec.Parameters == nil && content.Spec.ConfigmapMeta == "" && content.Spec.SecretMeta == "" {
		log.AddContext(ctx).Warningln("The spec Parameters, ConfigmapMeta and SecretMeta all empty")
		return true
	}

	return content.Status != nil && (content.Status.ContentName != "" ||
		content.Status.ProviderVersion != "" || content.Status.VendorName != "")
}

// NeedAddContentBoundFinalizers returns whether to add a Finalizer to content
func NeedAddContentBoundFinalizers(content *xuanwuv1.StorageBackendContent) bool {
	return content.ObjectMeta.DeletionTimestamp == nil && !finalizers.ContainsFinalizer(content, ContentBoundFinalizer)
}

// NeedRemoveContentBoundFinalizers returns whether to add a Finalizer
func NeedRemoveContentBoundFinalizers(content *xuanwuv1.StorageBackendContent) bool {
	return content.ObjectMeta.DeletionTimestamp != nil && finalizers.ContainsFinalizer(
		content, ContentBoundFinalizer)
}

// SplitMetaNamespaceKey returns the namespace and name
func SplitMetaNamespaceKey(obj string) (namespace, name string, err error) {
	return cache.SplitMetaNamespaceKey(obj)
}

// MakeMetaWithNamespace used to make a backendID
func MakeMetaWithNamespace(ns, name string) string {
	return fmt.Sprintf("%s/%s", ns, name)
}

// GenObjectMetaKey return the meta namespace key
func GenObjectMetaKey(obj interface{}) (string, error) {
	if obj == nil {
		return "", nil
	}

	objAccessor, err := meta.Accessor(obj)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/%s", objAccessor.GetNamespace(), objAccessor.GetName()), nil
}

// GetConfigMap is to get configmap using storage backend claim
func GetConfigMap(configmapMeta string) (*coreV1.ConfigMap, error) {
	namespace, name, err := SplitMetaNamespaceKey(configmapMeta)
	if err != nil {
		return nil, fmt.Errorf("split configmap meta namespace failed, error: %v", err)
	}

	k8sUtils := app.GetGlobalConfig().K8sUtils
	return k8sUtils.GetConfigmap(context.TODO(), name, namespace)
}

// UpdateConfigMap is to update configmap using storage backend claim
func UpdateConfigMap(configmap *coreV1.ConfigMap) (*coreV1.ConfigMap, error) {
	k8sUtils := app.GetGlobalConfig().K8sUtils
	return k8sUtils.UpdateConfigmap(context.TODO(), configmap)
}

// GetSecret is to get secret using storage backend claim
func GetSecret(meta string) (*coreV1.Secret, error) {
	namespace, name, err := SplitMetaNamespaceKey(meta)
	if err != nil {
		return nil, fmt.Errorf("split meta namespace failed, error: %v", err)
	}

	k8sUtils := app.GetGlobalConfig().K8sUtils

	return k8sUtils.GetSecret(context.TODO(), name, namespace)
}

// Errorln used to create and print error messages.
func Errorln(ctx context.Context, msg string) error {
	log.AddContext(ctx).Errorln(msg)
	return errors.New(msg)
}

// Errorf used to create and print formatted error messages.
func Errorf(ctx context.Context, format string, a ...interface{}) error {
	msg := fmt.Sprintf(format, a...)
	log.AddContext(ctx).Errorln(msg)
	return errors.New(msg)
}

// NeedChangeContent returns need update content or not
func NeedChangeContent(storageBackend *xuanwuv1.StorageBackendClaim) bool {
	if storageBackend.Status == nil {
		return false
	}

	if storageBackend.Status.BoundContentName == "" {
		return false
	}

	if storageBackend.Status.SecretMeta != storageBackend.Spec.SecretMeta {
		return true
	}

	if storageBackend.Status.MaxClientThreads != storageBackend.Spec.MaxClientThreads {
		return true
	}

	if storageBackend.Status.UseCert != storageBackend.Spec.UseCert {
		return true
	}

	if storageBackend.Status.CertSecret != storageBackend.Spec.CertSecret {
		return true
	}

	return false
}

// GetNameSpaceFromEnv get the namespace from the env
func GetNameSpaceFromEnv(namespaceEnv, defaultNamespace string) string {
	ns := os.Getenv(namespaceEnv)
	if ns == "" {
		ns = defaultNamespace
	}

	return ns
}

// WaitExitSignal is used to wait exits signal, components e.g. webhook, controller
func WaitExitSignal(ctx context.Context, components string) {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGILL, syscall.SIGKILL, syscall.SIGTERM)
	stopSignal := <-signalChan
	log.AddContext(ctx).Warningf("stop %s, stopSignal is [%v]", components, stopSignal)
	close(signalChan)
}
