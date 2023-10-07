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
	"testing"

	admissionV1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestTransformV1beta1AdmitFuncToV1AdmitFunc(t *testing.T) {
	v1beta1Func := transformV1beta1AdmitFuncToV1AdmitFunc(admitStorageBackendClaim)
	req := admissionV1beta1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{},
		Request: &admissionV1beta1.AdmissionRequest{
			Operation: admissionV1beta1.Create,
			Object:    runtime.RawExtension{},
			OldObject: runtime.RawExtension{},
		},
		Response: nil,
	}

	res := v1beta1Func(req)
	if res.Allowed {
		t.Errorf("res [%v] error, should be not allowed", res)
	}
}
