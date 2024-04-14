/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2022-2023. All rights reserved.
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

package client

import (
	"context"
	"errors"
	"fmt"
	URL "net/url"
)

// ApplicationType defines interfaces for application type operations
type ApplicationType interface {
	// GetApplicationTypeByName used for get application type
	GetApplicationTypeByName(ctx context.Context, appType string) (string, error)
}

// GetApplicationTypeByName function to get the Application type ID to set the I/O size
// while creating Volume
func (cli *BaseClient) GetApplicationTypeByName(ctx context.Context, appType string) (string, error) {
	result := ""
	appType = URL.QueryEscape(appType)
	url := fmt.Sprintf("/workload_type?filter=NAME::%s", appType)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return result, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return result, fmt.Errorf("Get application types returned error: %d", code)
	}

	if resp.Data == nil {
		return result, nil
	}
	respData, ok := resp.Data.([]interface{})
	if !ok {
		return result, errors.New("application types response is not valid")
	}
	// This should be just one elem. But the return is an array with single value
	for _, i := range respData {
		applicationTypes, ok := i.(map[string]interface{})
		if !ok {
			return result, errors.New("Data in response is not valid")
		}
		// From the map we need the application type ID
		// This will be used for param to create LUN
		val, ok := applicationTypes["ID"].(string)
		if !ok {
			return result, errors.New("application type is not valid")
		}
		result = val
	}
	return result, nil
}
