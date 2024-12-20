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
// Code generated by informer-gen. DO NOT EDIT.

package v1

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"

	xuanwuv1 "github.com/Huawei/eSDK_K8S_Plugin/v4/client/apis/xuanwu/v1"
	versioned "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/client/clientset/versioned"
	internalinterfaces "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/client/informers/externalversions/internalinterfaces"
	v1 "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/client/listers/xuanwu/v1"
)

// StorageBackendClaimInformer provides access to a shared informer and lister for
// StorageBackendClaims.
type StorageBackendClaimInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1.StorageBackendClaimLister
}

type storageBackendClaimInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewStorageBackendClaimInformer constructs a new informer for StorageBackendClaim type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewStorageBackendClaimInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredStorageBackendClaimInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredStorageBackendClaimInformer constructs a new informer for StorageBackendClaim type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredStorageBackendClaimInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.XuanwuV1().StorageBackendClaims(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.XuanwuV1().StorageBackendClaims(namespace).Watch(context.TODO(), options)
			},
		},
		&xuanwuv1.StorageBackendClaim{},
		resyncPeriod,
		indexers,
	)
}

func (f *storageBackendClaimInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredStorageBackendClaimInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *storageBackendClaimInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&xuanwuv1.StorageBackendClaim{}, f.defaultInformer)
}

func (f *storageBackendClaimInformer) Lister() v1.StorageBackendClaimLister {
	return v1.NewStorageBackendClaimLister(f.Informer().GetIndexer())
}
