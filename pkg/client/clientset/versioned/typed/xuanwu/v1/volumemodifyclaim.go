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
	v1 "huawei-csi-driver/client/apis/xuanwu/v1"
	scheme "huawei-csi-driver/pkg/client/clientset/versioned/scheme"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// VolumeModifyClaimsGetter has a method to return a VolumeModifyClaimInterface.
// A group's client should implement this interface.
type VolumeModifyClaimsGetter interface {
	VolumeModifyClaims() VolumeModifyClaimInterface
}

// VolumeModifyClaimInterface has methods to work with VolumeModifyClaim resources.
type VolumeModifyClaimInterface interface {
	Create(ctx context.Context, volumeModifyClaim *v1.VolumeModifyClaim, opts metav1.CreateOptions) (*v1.VolumeModifyClaim, error)
	Update(ctx context.Context, volumeModifyClaim *v1.VolumeModifyClaim, opts metav1.UpdateOptions) (*v1.VolumeModifyClaim, error)
	UpdateStatus(ctx context.Context, volumeModifyClaim *v1.VolumeModifyClaim, opts metav1.UpdateOptions) (*v1.VolumeModifyClaim, error)
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1.VolumeModifyClaim, error)
	List(ctx context.Context, opts metav1.ListOptions) (*v1.VolumeModifyClaimList, error)
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.VolumeModifyClaim, err error)
	VolumeModifyClaimExpansion
}

// volumeModifyClaims implements VolumeModifyClaimInterface
type volumeModifyClaims struct {
	client rest.Interface
}

// newVolumeModifyClaims returns a VolumeModifyClaims
func newVolumeModifyClaims(c *XuanwuV1Client) *volumeModifyClaims {
	return &volumeModifyClaims{
		client: c.RESTClient(),
	}
}

// Get takes name of the volumeModifyClaim, and returns the corresponding volumeModifyClaim object, and an error if there is any.
func (c *volumeModifyClaims) Get(ctx context.Context, name string, options metav1.GetOptions) (result *v1.VolumeModifyClaim, err error) {
	result = &v1.VolumeModifyClaim{}
	err = c.client.Get().
		Resource("volumemodifyclaims").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of VolumeModifyClaims that match those selectors.
func (c *volumeModifyClaims) List(ctx context.Context, opts metav1.ListOptions) (result *v1.VolumeModifyClaimList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1.VolumeModifyClaimList{}
	err = c.client.Get().
		Resource("volumemodifyclaims").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested volumeModifyClaims.
func (c *volumeModifyClaims) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Resource("volumemodifyclaims").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a volumeModifyClaim and creates it.  Returns the server's representation of the volumeModifyClaim, and an error, if there is any.
func (c *volumeModifyClaims) Create(ctx context.Context, volumeModifyClaim *v1.VolumeModifyClaim, opts metav1.CreateOptions) (result *v1.VolumeModifyClaim, err error) {
	result = &v1.VolumeModifyClaim{}
	err = c.client.Post().
		Resource("volumemodifyclaims").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(volumeModifyClaim).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a volumeModifyClaim and updates it. Returns the server's representation of the volumeModifyClaim, and an error, if there is any.
func (c *volumeModifyClaims) Update(ctx context.Context, volumeModifyClaim *v1.VolumeModifyClaim, opts metav1.UpdateOptions) (result *v1.VolumeModifyClaim, err error) {
	result = &v1.VolumeModifyClaim{}
	err = c.client.Put().
		Resource("volumemodifyclaims").
		Name(volumeModifyClaim.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(volumeModifyClaim).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *volumeModifyClaims) UpdateStatus(ctx context.Context, volumeModifyClaim *v1.VolumeModifyClaim, opts metav1.UpdateOptions) (result *v1.VolumeModifyClaim, err error) {
	result = &v1.VolumeModifyClaim{}
	err = c.client.Put().
		Resource("volumemodifyclaims").
		Name(volumeModifyClaim.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(volumeModifyClaim).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the volumeModifyClaim and deletes it. Returns an error if one occurs.
func (c *volumeModifyClaims) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return c.client.Delete().
		Resource("volumemodifyclaims").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *volumeModifyClaims) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Resource("volumemodifyclaims").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched volumeModifyClaim.
func (c *volumeModifyClaims) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.VolumeModifyClaim, err error) {
	result = &v1.VolumeModifyClaim{}
	err = c.client.Patch(pt).
		Resource("volumemodifyclaims").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
