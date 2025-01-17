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

// VolumeModifyContentsGetter has a method to return a VolumeModifyContentInterface.
// A group's client should implement this interface.
type VolumeModifyContentsGetter interface {
	VolumeModifyContents() VolumeModifyContentInterface
}

// VolumeModifyContentInterface has methods to work with VolumeModifyContent resources.
type VolumeModifyContentInterface interface {
	Create(ctx context.Context, volumeModifyContent *v1.VolumeModifyContent, opts metav1.CreateOptions) (*v1.VolumeModifyContent, error)
	Update(ctx context.Context, volumeModifyContent *v1.VolumeModifyContent, opts metav1.UpdateOptions) (*v1.VolumeModifyContent, error)
	UpdateStatus(ctx context.Context, volumeModifyContent *v1.VolumeModifyContent, opts metav1.UpdateOptions) (*v1.VolumeModifyContent, error)
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1.VolumeModifyContent, error)
	List(ctx context.Context, opts metav1.ListOptions) (*v1.VolumeModifyContentList, error)
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.VolumeModifyContent, err error)
	VolumeModifyContentExpansion
}

// volumeModifyContents implements VolumeModifyContentInterface
type volumeModifyContents struct {
	client rest.Interface
}

// newVolumeModifyContents returns a VolumeModifyContents
func newVolumeModifyContents(c *XuanwuV1Client) *volumeModifyContents {
	return &volumeModifyContents{
		client: c.RESTClient(),
	}
}

// Get takes name of the volumeModifyContent, and returns the corresponding volumeModifyContent object, and an error if there is any.
func (c *volumeModifyContents) Get(ctx context.Context, name string, options metav1.GetOptions) (result *v1.VolumeModifyContent, err error) {
	result = &v1.VolumeModifyContent{}
	err = c.client.Get().
		Resource("volumemodifycontents").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of VolumeModifyContents that match those selectors.
func (c *volumeModifyContents) List(ctx context.Context, opts metav1.ListOptions) (result *v1.VolumeModifyContentList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1.VolumeModifyContentList{}
	err = c.client.Get().
		Resource("volumemodifycontents").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested volumeModifyContents.
func (c *volumeModifyContents) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Resource("volumemodifycontents").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a volumeModifyContent and creates it.  Returns the server's representation of the volumeModifyContent, and an error, if there is any.
func (c *volumeModifyContents) Create(ctx context.Context, volumeModifyContent *v1.VolumeModifyContent, opts metav1.CreateOptions) (result *v1.VolumeModifyContent, err error) {
	result = &v1.VolumeModifyContent{}
	err = c.client.Post().
		Resource("volumemodifycontents").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(volumeModifyContent).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a volumeModifyContent and updates it. Returns the server's representation of the volumeModifyContent, and an error, if there is any.
func (c *volumeModifyContents) Update(ctx context.Context, volumeModifyContent *v1.VolumeModifyContent, opts metav1.UpdateOptions) (result *v1.VolumeModifyContent, err error) {
	result = &v1.VolumeModifyContent{}
	err = c.client.Put().
		Resource("volumemodifycontents").
		Name(volumeModifyContent.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(volumeModifyContent).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *volumeModifyContents) UpdateStatus(ctx context.Context, volumeModifyContent *v1.VolumeModifyContent, opts metav1.UpdateOptions) (result *v1.VolumeModifyContent, err error) {
	result = &v1.VolumeModifyContent{}
	err = c.client.Put().
		Resource("volumemodifycontents").
		Name(volumeModifyContent.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(volumeModifyContent).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the volumeModifyContent and deletes it. Returns an error if one occurs.
func (c *volumeModifyContents) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return c.client.Delete().
		Resource("volumemodifycontents").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *volumeModifyContents) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Resource("volumemodifycontents").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched volumeModifyContent.
func (c *volumeModifyContents) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.VolumeModifyContent, err error) {
	result = &v1.VolumeModifyContent{}
	err = c.client.Patch(pt).
		Resource("volumemodifycontents").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
