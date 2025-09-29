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

// Package rest provides operations for rest request
package rest

import (
	"fmt"
	"net/url"
	"strings"
)

const (
	defaultListStart = 0
	defaultListEnd   = 100
)

type listRange struct {
	start uint
	end   uint
}

// RequestPath is a struct for storage api path
type RequestPath struct {
	// Path is the base path for api
	Path string

	query     url.Values
	filter    map[string]any
	listRange *listRange
}

// NewRequestPath returns a rest request path
func NewRequestPath(p string) *RequestPath {
	return &RequestPath{
		Path:   p,
		query:  url.Values{},
		filter: map[string]any{},
	}
}

// SetQuery sets a key/value pair to query of a request path
func (p *RequestPath) SetQuery(key, value string) {
	p.query.Set(key, value)
}

// AddFilter adds key/value pair to filter of a batch query request path
func (p *RequestPath) AddFilter(key string, value any) {
	p.filter[key] = value
}

// SetListRange sets list range of a batch query request path
func (p *RequestPath) SetListRange(start, end uint) {
	if p.listRange == nil {
		p.listRange = new(listRange)
	}
	p.listRange.start = start
	p.listRange.end = end
}

// SetDefaultListRange sets list range to default value
func (p *RequestPath) SetDefaultListRange() {
	p.SetListRange(defaultListStart, defaultListEnd)
}

// Encode encodes request path to string
func (p *RequestPath) Encode() (string, error) {
	parsedURL, err := url.Parse(p.Path)
	if err != nil {
		return "", err
	}

	if len(p.filter) > 0 {
		var strs []string
		for k, v := range p.filter {
			strs = append(strs, fmt.Sprintf("%s::%s", k, v))
		}

		p.query.Set("filter", strings.Join(strs, " and "))
	}

	if p.listRange != nil {
		rangeStr := fmt.Sprintf("[%d-%d]", p.listRange.start, p.listRange.end)
		p.query.Set("range", rangeStr)
	}

	parsedURL.RawQuery = p.query.Encode()
	return parsedURL.String(), nil
}
