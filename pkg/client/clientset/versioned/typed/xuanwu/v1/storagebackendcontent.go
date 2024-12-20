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
// Code generated by client-gen. DO NOT EDIT.

package v1

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"

	v1 "github.com/Huawei/eSDK_K8S_Plugin/v4/client/apis/xuanwu/v1"
	scheme "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/client/clientset/versioned/scheme"
)

// StorageBackendContentsGetter has a method to return a StorageBackendContentInterface.
// A group's client should implement this interface.
type StorageBackendContentsGetter interface {
	StorageBackendContents() StorageBackendContentInterface
}

// StorageBackendContentInterface has methods to work with StorageBackendContent resources.
type StorageBackendContentInterface interface {
	Create(ctx context.Context, storageBackendContent *v1.StorageBackendContent, opts metav1.CreateOptions) (*v1.StorageBackendContent, error)
	Update(ctx context.Context, storageBackendContent *v1.StorageBackendContent, opts metav1.UpdateOptions) (*v1.StorageBackendContent, error)
	UpdateStatus(ctx context.Context, storageBackendContent *v1.StorageBackendContent, opts metav1.UpdateOptions) (*v1.StorageBackendContent, error)
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1.StorageBackendContent, error)
	List(ctx context.Context, opts metav1.ListOptions) (*v1.StorageBackendContentList, error)
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.StorageBackendContent, err error)
	StorageBackendContentExpansion
}

// storageBackendContents implements StorageBackendContentInterface
type storageBackendContents struct {
	client rest.Interface
}

// newStorageBackendContents returns a StorageBackendContents
func newStorageBackendContents(c *XuanwuV1Client) *storageBackendContents {
	return &storageBackendContents{
		client: c.RESTClient(),
	}
}

// Get takes name of the storageBackendContent, and returns the corresponding storageBackendContent object, and an error if there is any.
func (c *storageBackendContents) Get(ctx context.Context, name string, options metav1.GetOptions) (result *v1.StorageBackendContent, err error) {
	result = &v1.StorageBackendContent{}
	err = c.client.Get().
		Resource("storagebackendcontents").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of StorageBackendContents that match those selectors.
func (c *storageBackendContents) List(ctx context.Context, opts metav1.ListOptions) (result *v1.StorageBackendContentList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1.StorageBackendContentList{}
	err = c.client.Get().
		Resource("storagebackendcontents").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested storageBackendContents.
func (c *storageBackendContents) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Resource("storagebackendcontents").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a storageBackendContent and creates it.  Returns the server's representation of the storageBackendContent, and an error, if there is any.
func (c *storageBackendContents) Create(ctx context.Context, storageBackendContent *v1.StorageBackendContent, opts metav1.CreateOptions) (result *v1.StorageBackendContent, err error) {
	result = &v1.StorageBackendContent{}
	err = c.client.Post().
		Resource("storagebackendcontents").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(storageBackendContent).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a storageBackendContent and updates it. Returns the server's representation of the storageBackendContent, and an error, if there is any.
func (c *storageBackendContents) Update(ctx context.Context, storageBackendContent *v1.StorageBackendContent, opts metav1.UpdateOptions) (result *v1.StorageBackendContent, err error) {
	result = &v1.StorageBackendContent{}
	err = c.client.Put().
		Resource("storagebackendcontents").
		Name(storageBackendContent.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(storageBackendContent).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *storageBackendContents) UpdateStatus(ctx context.Context, storageBackendContent *v1.StorageBackendContent, opts metav1.UpdateOptions) (result *v1.StorageBackendContent, err error) {
	result = &v1.StorageBackendContent{}
	err = c.client.Put().
		Resource("storagebackendcontents").
		Name(storageBackendContent.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(storageBackendContent).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the storageBackendContent and deletes it. Returns an error if one occurs.
func (c *storageBackendContents) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return c.client.Delete().
		Resource("storagebackendcontents").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *storageBackendContents) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Resource("storagebackendcontents").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched storageBackendContent.
func (c *storageBackendContents) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.StorageBackendContent, err error) {
	result = &v1.StorageBackendContent{}
	err = c.client.Patch(pt).
		Resource("storagebackendcontents").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
