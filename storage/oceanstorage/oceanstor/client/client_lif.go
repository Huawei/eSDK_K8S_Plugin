/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2024-2024. All rights reserved.
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
	"fmt"
	netUrl "net/url"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

// LIF defines interfaces for lif operations
type LIF interface {
	// GetLogicPort get current lif info, the storage interface is called
	GetLogicPort(ctx context.Context, addr string) (*Lif, error)
	// GetCurrentLifWwn get current lif wwn, the storage interface is not called
	GetCurrentLifWwn() string
	// GetCurrentLif get current lif
	GetCurrentLif(ctx context.Context) string
}

// GetLogicPort gets logic port information by port address
func (cli *OceanstorClient) GetLogicPort(ctx context.Context, addr string) (*Lif, error) {
	url := fmt.Sprintf("/lif?filter=IPV4ADDR:%s&range=[0-100]", addr)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}
	if err := resp.AssertErrorCode(); err != nil {
		return nil, err
	}

	var lifs []*Lif
	if err := resp.GetData(&lifs); err != nil {
		return nil, fmt.Errorf("get logic port error: %w", err)
	}

	if len(lifs) == 0 {
		// because manage lif is not exist lif list
		log.AddContext(ctx).Infof("return lis list not contains [%s], it is considered as the management LIF", addr)
		return &Lif{}, nil
	}

	return lifs[0], nil
}

// GetCurrentLifWwn used for get current lif wwn
func (cli *OceanstorClient) GetCurrentLifWwn() string {
	return cli.CurrentLifWwn
}

// GetCurrentLif used for get current lif wwn
func (cli *OceanstorClient) GetCurrentLif(ctx context.Context) string {
	u, err := netUrl.Parse(cli.Url)
	if err != nil {
		log.AddContext(ctx).Errorf("parse url failed, error: %v", err)
		return ""
	}
	return u.Hostname()
}
