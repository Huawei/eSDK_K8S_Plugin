/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2023-2023. All rights reserved.
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
	"errors"
)

var clientSet = map[string]KubernetesClient{}

// RegisterClient used to register a client into the clientSet
func RegisterClient(name string, client KubernetesClient) {
	clientSet[name] = client
}

// LoadSupportedClient used to load supported client. Return a client of type KubernetesClient and nil error
// if a client with the specified testName exists. If not exists, return an error with not supported.
func LoadSupportedClient(name string) (KubernetesClient, error) {
	if client, ok := clientSet[name]; ok {
		return client, nil
	}
	return nil, errors.New("not supported CLI")
}
