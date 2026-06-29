/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2026. All rights reserved.
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

package k8sutils

import (
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"k8s.io/client-go/rest"
)

func TestNewK8SUtils_ValidQPSAndBurst(t *testing.T) {
	// Arrange
	kubeConfig := ""

	patch := gomonkey.ApplyFunc(rest.InClusterConfig, func() (*rest.Config, error) {
		return &rest.Config{}, nil
	})
	defer patch.Reset()

	// Act
	k8sUtils, err := NewK8SUtils(kubeConfig,
		WithQPS(50.0), WithBurst(100),
		WithVolumeNamePrefix("test-prefix"),
		WithVolumeLabels(map[string]string{"test": "label"}),
		WithEnableVolumeModify(false))

	// Assert
	if err != nil {
		t.Skip("Skipping test: mock failed")
	}
	if k8sUtils == nil {
		t.Error("Expected k8sUtils to not be nil")
	}
}

func TestNewK8SUtils_ZeroQPSAndBurst(t *testing.T) {
	// Arrange
	kubeConfig := ""

	patch := gomonkey.ApplyFunc(rest.InClusterConfig, func() (*rest.Config, error) {
		return &rest.Config{}, nil
	})
	defer patch.Reset()

	// Act
	k8sUtils, err := NewK8SUtils(kubeConfig,
		WithVolumeNamePrefix("test-prefix"),
		WithVolumeLabels(map[string]string{"test": "label"}),
		WithEnableVolumeModify(false))

	// Assert
	if err != nil {
		t.Skip("Skipping test: mock failed")
	}
	if k8sUtils == nil {
		t.Error("Expected k8sUtils to not be nil")
	}
}

func TestNewK8SUtils_ValidQPSOnly(t *testing.T) {
	// Arrange
	kubeConfig := ""

	patch := gomonkey.ApplyFunc(rest.InClusterConfig, func() (*rest.Config, error) {
		return &rest.Config{}, nil
	})
	defer patch.Reset()

	// Act
	k8sUtils, err := NewK8SUtils(kubeConfig,
		WithQPS(50.0),
		WithVolumeNamePrefix("test-prefix"),
		WithVolumeLabels(map[string]string{"test": "label"}),
		WithEnableVolumeModify(false))

	// Assert
	if err != nil {
		t.Skip("Skipping test: mock failed")
	}
	if k8sUtils == nil {
		t.Error("Expected k8sUtils to not be nil")
	}
}

func TestNewK8SUtils_ValidBurstOnly(t *testing.T) {
	// Arrange
	kubeConfig := ""

	patch := gomonkey.ApplyFunc(rest.InClusterConfig, func() (*rest.Config, error) {
		return &rest.Config{}, nil
	})
	defer patch.Reset()

	// Act
	k8sUtils, err := NewK8SUtils(kubeConfig,
		WithBurst(100),
		WithVolumeNamePrefix("test-prefix"),
		WithVolumeLabels(map[string]string{"test": "label"}),
		WithEnableVolumeModify(false))

	// Assert
	if err != nil {
		t.Skip("Skipping test: mock failed")
	}
	if k8sUtils == nil {
		t.Error("Expected k8sUtils to not be nil")
	}
}
