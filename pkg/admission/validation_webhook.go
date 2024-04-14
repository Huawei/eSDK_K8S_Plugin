/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2024. All rights reserved.
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

package admission

import (
	"context"

	"k8s.io/api/admissionregistration/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ValidatingWebhookCfgOps is interface to perform CRUD ops on validating webhook controller
type ValidatingWebhookCfgOps interface {
	// CreateValidatingWebhookCfg creates given ValidatingWebhookConfiguration
	CreateValidatingWebhookCfg(req *v1.ValidatingWebhookConfiguration) (
		*v1.ValidatingWebhookConfiguration, error)
	// UpdateValidatingWebhookCfg updates given ValidatingWebhookConfiguration
	UpdateValidatingWebhookCfg(req *v1.ValidatingWebhookConfiguration) (
		*v1.ValidatingWebhookConfiguration, error)
	// DeleteValidatingWebhookCfg deletes given ValidatingWebhookConfiguration
	DeleteValidatingWebhookCfg(name string) error
	// GetValidatingWebhookCfg get WebhookConfiguration by name
	GetValidatingWebhookCfg(name string) (*v1.ValidatingWebhookConfiguration, error)
}

// CreateValidatingWebhookCfg creates given ValidatingWebhookConfiguration
func (c *Client) CreateValidatingWebhookCfg(cfg *v1.ValidatingWebhookConfiguration) (
	*v1.ValidatingWebhookConfiguration, error) {
	if err := c.initClient(); err != nil {
		return nil, err
	}
	return c.admission.ValidatingWebhookConfigurations().Create(context.TODO(), cfg, metaV1.CreateOptions{})
}

// DeleteValidatingWebhookCfg deletes given ValidatingWebhookConfiguration
func (c *Client) DeleteValidatingWebhookCfg(name string) error {
	if err := c.initClient(); err != nil {
		return err
	}
	return c.admission.ValidatingWebhookConfigurations().Delete(context.TODO(), name, metaV1.DeleteOptions{})
}

// UpdateValidatingWebhookCfg updates given ValidatingWebhookConfiguration
func (c *Client) UpdateValidatingWebhookCfg(cfg *v1.ValidatingWebhookConfiguration) (
	*v1.ValidatingWebhookConfiguration, error) {
	if err := c.initClient(); err != nil {
		return nil, err
	}
	return c.admission.ValidatingWebhookConfigurations().Update(context.TODO(), cfg, metaV1.UpdateOptions{})
}

// GetValidatingWebhookCfg get WebhookConfiguration by name
func (c *Client) GetValidatingWebhookCfg(webhookName string) (
	*v1.ValidatingWebhookConfiguration, error) {
	if err := c.initClient(); err != nil {
		return nil, err
	}
	return c.admission.ValidatingWebhookConfigurations().Get(context.TODO(), webhookName, metaV1.GetOptions{})
}
