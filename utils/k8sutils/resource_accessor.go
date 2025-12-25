/*
 *
 *  Copyright (c) Huawei Technologies Co., Ltd. 2024-2024. All rights reserved.
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

// Package k8sutils provides Kubernetes utilities
package k8sutils

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

// ResourceAccessor provides the methods to get kubernetes resource.
type ResourceAccessor[T runtime.Object] struct {
	informer cache.SharedIndexInformer
}

// WithTransformer sets transformer for the resource.
func WithTransformer[T runtime.Object](transform cache.TransformFunc) func(*ResourceAccessor[T]) error {
	return func(r *ResourceAccessor[T]) error {
		if err := r.informer.SetTransform(transform); err != nil {
			return fmt.Errorf("set transformer failed: %v", err)
		}

		return nil
	}
}

// WithIndexers sets indexers for the resource.
func WithIndexers[T runtime.Object](indexers cache.Indexers) func(*ResourceAccessor[T]) error {
	return func(r *ResourceAccessor[T]) error {
		if err := r.informer.AddIndexers(indexers); err != nil {
			return fmt.Errorf("add indexers failed: %w", err)
		}

		return nil
	}
}

// WithHandler sets event handler for the resource.
func WithHandler[T runtime.Object](handler cache.ResourceEventHandlerFuncs) func(*ResourceAccessor[T]) error {
	return func(r *ResourceAccessor[T]) error {
		if _, err := r.informer.AddEventHandler(handler); err != nil {
			return fmt.Errorf("add handler failed: %w", err)
		}

		return nil
	}
}

// NewResourceAccessor returns a ResourceAccessor instance.
func NewResourceAccessor[T runtime.Object](informer cache.SharedIndexInformer,
	options ...func(*ResourceAccessor[T]) error) (*ResourceAccessor[T], error) {
	watcher := &ResourceAccessor[T]{}
	watcher.informer = informer

	for _, option := range options {
		if err := option(watcher); err != nil {
			return nil, err
		}
	}

	return watcher, nil
}

// GetByIndex gets resource by index.
// To use this method, you must add the indexer first.
func (rw *ResourceAccessor[T]) GetByIndex(indexName, indexValue string) ([]T, error) {
	value, err := rw.getByIndex(indexName, indexValue)
	if err != nil {
		if err = rw.informer.GetIndexer().Resync(); err != nil {
			return value, fmt.Errorf("resync %T resource failed: %w", value, err)
		}

		value, err = rw.getByIndex(indexName, indexValue)
		if err != nil {
			return value, fmt.Errorf("get by index failed: %w", err)
		}
	}

	return value, nil
}

func (rw *ResourceAccessor[T]) getByIndex(indexName, indexValue string) ([]T, error) {
	var value T
	items, err := rw.informer.GetIndexer().ByIndex(indexName, indexValue)
	if err != nil {
		return nil, fmt.Errorf("could not search cache for %T by index %s", value, indexValue)
	}

	res := make([]T, 0, len(items))
	var ok bool
	for _, item := range items {
		value, ok = item.(T)
		if !ok {
			return nil, fmt.Errorf("convert %v to %T error", item, value)
		}
		res = append(res, value)
	}

	if len(res) == 0 {
		return nil, fmt.Errorf("%T object not found in cache by index %s", value, indexValue)
	}

	return res, nil
}
