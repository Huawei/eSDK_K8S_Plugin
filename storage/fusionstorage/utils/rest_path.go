/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2025-2025. All rights reserved.
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

package utils

import (
	"encoding/json"
	"net/url"
)

const (
	defaultOffset = 0
	defaultLimit  = 100
)

type restRange struct {
	Offset uint `json:"offset"`
	Limit  uint `json:"limit"`
}

// FusionRestPath is a struct for fusion storage api path
type FusionRestPath struct {
	// Path is the base path for api
	Path string

	query     url.Values
	filter    map[string]any
	restRange *restRange
}

// NewFusionRestPath returns a fusion storage path
func NewFusionRestPath(p string) *FusionRestPath {
	return &FusionRestPath{
		Path:   p,
		query:  url.Values{},
		filter: map[string]any{},
	}
}

// SetQuery sets a key/value pair to query of a request path
func (fp *FusionRestPath) SetQuery(key, value string) {
	fp.query.Set(key, value)
}

// AddFilter adds key/value pair to filter of a batch query request path
func (fp *FusionRestPath) AddFilter(key string, value any) {
	fp.filter[key] = value
}

// SetRange sets range of a batch query request path
func (fp *FusionRestPath) SetRange(offset, limit uint) {
	if fp.restRange == nil {
		fp.restRange = new(restRange)
	}
	fp.restRange.Offset = offset
	fp.restRange.Limit = limit
}

// SetDefaultRange sets range to default value
func (fp *FusionRestPath) SetDefaultRange() {
	fp.SetRange(defaultOffset, defaultLimit)
}

// Encode encodes fusion path to string
func (fp *FusionRestPath) Encode() (string, error) {
	parsedURL, err := url.Parse(fp.Path)
	if err != nil {
		return "", err
	}

	if len(fp.filter) > 0 {
		rawFilter, err := json.Marshal([]any{fp.filter})
		if err != nil {
			return "", err
		}
		fp.query.Set("filter", string(rawFilter))
	}

	if fp.restRange != nil {
		rawRange, err := json.Marshal(fp.restRange)
		if err != nil {
			return "", err
		}
		fp.query.Set("range", string(rawRange))
	}

	parsedURL.RawQuery = fp.query.Encode()
	return parsedURL.String(), nil
}
