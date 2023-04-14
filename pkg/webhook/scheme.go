/*
Copyright (c) Huawei Technologies Co., Ltd. 2022-2022. All rights reserved.

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
	admissionRegistrationV1 "k8s.io/api/admissionregistration/v1"
	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilRuntime "k8s.io/apimachinery/pkg/util/runtime"
)

var scheme = runtime.NewScheme()
// Codecs means a code factory
var Codecs = serializer.NewCodecFactory(scheme)

func init() {
	addToScheme(scheme)
}

func addToScheme(scheme *runtime.Scheme) {
	utilRuntime.Must(coreV1.AddToScheme(scheme))
	utilRuntime.Must(admissionV1.AddToScheme(scheme))
	utilRuntime.Must(admissionRegistrationV1.AddToScheme(scheme))
}
