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
	"reflect"

	admissionV1 "k8s.io/api/admissionregistration/v1"
	apisErrors "k8s.io/apimachinery/pkg/api/errors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"huawei-csi-driver/pkg/admission"
	"huawei-csi-driver/utils/log"
)

// AdmissionWebHookCFG defines cfg of admission webhook
type AdmissionWebHookCFG struct {
	WebhookName   string
	ServiceName   string
	WebhookPath   string
	WebhookPort   int32
	AdmissionOps  []admissionV1.OperationType
	AdmissionRule AdmissionRule
}

// AdmissionRule includes admission rules
type AdmissionRule struct {
	APIGroups   []string
	APIVersions []string
	Resources   []string
}

// CreateValidateWebhook create new webhook config if not exist already
func CreateValidateWebhook(ctx context.Context, webHookCfg AdmissionWebHookCFG, caBundle []byte, ns string) error {
	webhook := newValidateWebhook(webHookCfg, caBundle, ns)
	req := &admissionV1.ValidatingWebhookConfiguration{
		ObjectMeta: metaV1.ObjectMeta{Name: webHookCfg.WebhookName},
		Webhooks:   []admissionV1.ValidatingWebhook{webhook},
	}

	foundWebhookCfg, err := admission.Instance().GetValidatingWebhookCfg(req.Name)
	if err != nil {
		if !apisErrors.IsNotFound(err) {
			log.AddContext(ctx).Errorf("get webhook configuration [%s] failed: %v", req.Name, err)
			return err
		}

		// no webhook configuration in k8s cluster, we need to create a new one.
		if _, err := admission.Instance().CreateValidatingWebhookCfg(req); err != nil {
			log.AddContext(ctx).Errorf("create webhook configuration [%s] failed: %v", req.Name, err)
			return err
		}
		log.AddContext(ctx).Infof("webhook configuration [%s] has been created", req.Name)
		return nil
	}

	if reflect.DeepEqual(foundWebhookCfg.Webhooks, req.Webhooks) {
		return nil
	}

	// webhook configuration has changed, we need to update it.
	foundWebhookCfg.Webhooks = req.Webhooks
	if _, err := admission.Instance().UpdateValidatingWebhookCfg(foundWebhookCfg); err != nil {
		log.AddContext(ctx).Errorf("update webhook configuration failed: %v", err)
		return err
	}

	log.AddContext(ctx).Infof("webhook [%s] has been updated", req.Name)

	return nil
}

func newValidateWebhook(webhookCfg AdmissionWebHookCFG, caBundle []byte, ns string) admissionV1.ValidatingWebhook {
	sideEffect := admissionV1.SideEffectClassNoneOnDryRun
	failurePolicy := admissionV1.Fail
	matchPolicy := admissionV1.Exact
	return admissionV1.ValidatingWebhook{
		Name: webhookCfg.WebhookName,
		ClientConfig: admissionV1.WebhookClientConfig{
			Service: &admissionV1.ServiceReference{
				Name:      webhookCfg.ServiceName,
				Namespace: ns,
				Path:      &webhookCfg.WebhookPath,
				Port:      &webhookCfg.WebhookPort,
			},
			CABundle: caBundle,
		},
		Rules: []admissionV1.RuleWithOperations{{
			Operations: webhookCfg.AdmissionOps,
			Rule: admissionV1.Rule{
				APIGroups:   webhookCfg.AdmissionRule.APIGroups,
				APIVersions: webhookCfg.AdmissionRule.APIVersions,
				Resources:   webhookCfg.AdmissionRule.Resources,
			},
		}},
		SideEffects:             &sideEffect,
		FailurePolicy:           &failurePolicy,
		AdmissionReviewVersions: []string{"v1", "v1beta1"},
		MatchPolicy:             &matchPolicy,
	}
}
