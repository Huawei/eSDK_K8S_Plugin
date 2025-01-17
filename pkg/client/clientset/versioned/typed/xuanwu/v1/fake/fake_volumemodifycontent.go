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

package fake

import (
	"context"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"

	xuanwuv1 "github.com/Huawei/eSDK_K8S_Plugin/v4/client/apis/xuanwu/v1"
)

// FakeVolumeModifyContents implements VolumeModifyContentInterface
type FakeVolumeModifyContents struct {
	Fake *FakeXuanwuV1
}

var volumemodifycontentsResource = schema.GroupVersionResource{Group: "xuanwu.huawei.io", Version: "v1", Resource: "volumemodifycontents"}

var volumemodifycontentsKind = schema.GroupVersionKind{Group: "xuanwu.huawei.io", Version: "v1", Kind: "VolumeModifyContent"}

// Get takes name of the volumeModifyContent, and returns the corresponding volumeModifyContent object, and an error if there is any.
func (c *FakeVolumeModifyContents) Get(ctx context.Context, name string, options v1.GetOptions) (result *xuanwuv1.VolumeModifyContent, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(volumemodifycontentsResource, name), &xuanwuv1.VolumeModifyContent{})
	if obj == nil {
		return nil, err
	}
	return obj.(*xuanwuv1.VolumeModifyContent), err
}

// List takes label and field selectors, and returns the list of VolumeModifyContents that match those selectors.
func (c *FakeVolumeModifyContents) List(ctx context.Context, opts v1.ListOptions) (result *xuanwuv1.VolumeModifyContentList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(volumemodifycontentsResource, volumemodifycontentsKind, opts), &xuanwuv1.VolumeModifyContentList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &xuanwuv1.VolumeModifyContentList{ListMeta: obj.(*xuanwuv1.VolumeModifyContentList).ListMeta}
	for _, item := range obj.(*xuanwuv1.VolumeModifyContentList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested volumeModifyContents.
func (c *FakeVolumeModifyContents) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(volumemodifycontentsResource, opts))
}

// Create takes the representation of a volumeModifyContent and creates it.  Returns the server's representation of the volumeModifyContent, and an error, if there is any.
func (c *FakeVolumeModifyContents) Create(ctx context.Context, volumeModifyContent *xuanwuv1.VolumeModifyContent, opts v1.CreateOptions) (result *xuanwuv1.VolumeModifyContent, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(volumemodifycontentsResource, volumeModifyContent), &xuanwuv1.VolumeModifyContent{})
	if obj == nil {
		return nil, err
	}
	return obj.(*xuanwuv1.VolumeModifyContent), err
}

// Update takes the representation of a volumeModifyContent and updates it. Returns the server's representation of the volumeModifyContent, and an error, if there is any.
func (c *FakeVolumeModifyContents) Update(ctx context.Context, volumeModifyContent *xuanwuv1.VolumeModifyContent, opts v1.UpdateOptions) (result *xuanwuv1.VolumeModifyContent, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(volumemodifycontentsResource, volumeModifyContent), &xuanwuv1.VolumeModifyContent{})
	if obj == nil {
		return nil, err
	}
	return obj.(*xuanwuv1.VolumeModifyContent), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeVolumeModifyContents) UpdateStatus(ctx context.Context, volumeModifyContent *xuanwuv1.VolumeModifyContent, opts v1.UpdateOptions) (*xuanwuv1.VolumeModifyContent, error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateSubresourceAction(volumemodifycontentsResource, "status", volumeModifyContent), &xuanwuv1.VolumeModifyContent{})
	if obj == nil {
		return nil, err
	}
	return obj.(*xuanwuv1.VolumeModifyContent), err
}

// Delete takes name of the volumeModifyContent and deletes it. Returns an error if one occurs.
func (c *FakeVolumeModifyContents) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteActionWithOptions(volumemodifycontentsResource, name, opts), &xuanwuv1.VolumeModifyContent{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeVolumeModifyContents) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(volumemodifycontentsResource, listOpts)

	_, err := c.Fake.Invokes(action, &xuanwuv1.VolumeModifyContentList{})
	return err
}

// Patch applies the patch and returns the patched volumeModifyContent.
func (c *FakeVolumeModifyContents) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *xuanwuv1.VolumeModifyContent, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(volumemodifycontentsResource, name, pt, data, subresources...), &xuanwuv1.VolumeModifyContent{})
	if obj == nil {
		return nil, err
	}
	return obj.(*xuanwuv1.VolumeModifyContent), err
}
