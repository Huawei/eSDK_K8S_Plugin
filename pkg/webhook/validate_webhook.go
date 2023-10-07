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

// Package webhook validate the request
package webhook

import (
	"context"

	admissionV1 "k8s.io/api/admissionregistration/v1"
	apisErrors "k8s.io/apimachinery/pkg/api/errors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"huawei-csi-driver/pkg/admission"
	"huawei-csi-driver/utils/log"
)

type AdmissionWebHookCFG struct {
	WebhookName   string
	ServiceName   string
	WebhookPath   string
	WebhookPort   int32
	AdmissionOps  []admissionV1.OperationType
	AdmissionRule AdmissionRule
}

type AdmissionRule struct {
	APIGroups   []string
	APIVersions []string
	Resources   []string
}

// CreateValidateWebhook create new webhook config if not exist already
func CreateValidateWebhook(ctx context.Context, admissionWebhook AdmissionWebHookCFG,
	caBundle []byte, ns string) error {
	sideEffect := admissionV1.SideEffectClassNoneOnDryRun
	failurePolicy := admissionV1.Fail
	matchPolicy := admissionV1.Exact
	webhook := admissionV1.ValidatingWebhook{
		Name: admissionWebhook.WebhookName,
		ClientConfig: admissionV1.WebhookClientConfig{
			Service: &admissionV1.ServiceReference{
				Name:      admissionWebhook.ServiceName,
				Namespace: ns,
				Path:      &admissionWebhook.WebhookPath,
				Port:      &admissionWebhook.WebhookPort,
			},
			CABundle: caBundle,
		},
		Rules: []admissionV1.RuleWithOperations{{
			Operations: admissionWebhook.AdmissionOps,
			Rule: admissionV1.Rule{
				APIGroups:   admissionWebhook.AdmissionRule.APIGroups,
				APIVersions: admissionWebhook.AdmissionRule.APIVersions,
				Resources:   admissionWebhook.AdmissionRule.Resources,
			},
		}},
		SideEffects:             &sideEffect,
		FailurePolicy:           &failurePolicy,
		AdmissionReviewVersions: []string{"v1", "v1beta1"},
		MatchPolicy:             &matchPolicy,
	}

	req := &admissionV1.ValidatingWebhookConfiguration{
		ObjectMeta: metaV1.ObjectMeta{
			Name: admissionWebhook.WebhookName,
		},
		Webhooks: []admissionV1.ValidatingWebhook{webhook},
	}

	_, err := admission.Instance().CreateValidatingWebhookCfg(req)
	if err != nil && !apisErrors.IsAlreadyExists(err) {
		log.AddContext(ctx).Errorf("unable to create webhook configuration: %v", err)
		return err
	}
	log.AddContext(ctx).Infof("%v webhook v1 configured", admissionWebhook.WebhookName)
	return nil
}
