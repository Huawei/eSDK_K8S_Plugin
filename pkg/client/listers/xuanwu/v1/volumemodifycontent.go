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
// Code generated by lister-gen. DO NOT EDIT.

package v1

import (
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	v1 "github.com/Huawei/eSDK_K8S_Plugin/v4/client/apis/xuanwu/v1"
)

// VolumeModifyContentLister helps list VolumeModifyContents.
// All objects returned here must be treated as read-only.
type VolumeModifyContentLister interface {
	// List lists all VolumeModifyContents in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1.VolumeModifyContent, err error)
	// Get retrieves the VolumeModifyContent from the index for a given name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1.VolumeModifyContent, error)
	VolumeModifyContentListerExpansion
}

// volumeModifyContentLister implements the VolumeModifyContentLister interface.
type volumeModifyContentLister struct {
	indexer cache.Indexer
}

// NewVolumeModifyContentLister returns a new VolumeModifyContentLister.
func NewVolumeModifyContentLister(indexer cache.Indexer) VolumeModifyContentLister {
	return &volumeModifyContentLister{indexer: indexer}
}

// List lists all VolumeModifyContents in the indexer.
func (s *volumeModifyContentLister) List(selector labels.Selector) (ret []*v1.VolumeModifyContent, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1.VolumeModifyContent))
	})
	return ret, err
}

// Get retrieves the VolumeModifyContent from the index for a given name.
func (s *volumeModifyContentLister) Get(name string) (*v1.VolumeModifyContent, error) {
	obj, exists, err := s.indexer.GetByKey(name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1.Resource("volumemodifycontent"), name)
	}
	return obj.(*v1.VolumeModifyContent), nil
}
