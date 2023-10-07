/*
Copyright (c) Huawei Technologies Co., Ltd. 2023-2023. All rights reserved.

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
	admissionV1 "k8s.io/api/admission/v1"
	admissionV1beta1 "k8s.io/api/admission/v1beta1"
)

func transformV1beta1AdmitFuncToV1AdmitFunc(f admitV1Func) admitV1beta1Func {
	return func(review admissionV1beta1.AdmissionReview) *admissionV1beta1.AdmissionResponse {
		v1Review := admissionV1.AdmissionReview{
			Request: &admissionV1.AdmissionRequest{
				Kind:               review.Request.Kind,
				Namespace:          review.Request.Namespace,
				Name:               review.Request.Name,
				Object:             review.Request.Object,
				Resource:           review.Request.Resource,
				Operation:          admissionV1.Operation(review.Request.Operation),
				UID:                review.Request.UID,
				DryRun:             review.Request.DryRun,
				OldObject:          review.Request.OldObject,
				Options:            review.Request.Options,
				RequestKind:        review.Request.RequestKind,
				RequestResource:    review.Request.RequestResource,
				RequestSubResource: review.Request.RequestSubResource,
				SubResource:        review.Request.SubResource,
				UserInfo:           review.Request.UserInfo,
			}}
		v1Response := f(v1Review)

		var pt *admissionV1beta1.PatchType
		if v1Response.PatchType != nil {
			t := admissionV1beta1.PatchType(*v1Response.PatchType)
			pt = &t
		}

		return &admissionV1beta1.AdmissionResponse{
			UID:              v1Response.UID,
			Allowed:          v1Response.Allowed,
			AuditAnnotations: v1Response.AuditAnnotations,
			Patch:            v1Response.Patch,
			PatchType:        pt,
			Result:           v1Response.Result,
			Warnings:         v1Response.Warnings,
		}
	}
}
