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
	"errors"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func TestQPS(t *testing.T) {
	// Arrange
	config := &rest.Config{}

	// Act
	opt := QPS(50.0)
	opt(config)

	// Assert
	assert.Equal(t, float32(50.0), config.QPS)
}

func TestBurst(t *testing.T) {
	// Arrange
	config := &rest.Config{}

	// Act
	opt := Burst(100)
	opt(config)

	// Assert
	assert.Equal(t, 100, config.Burst)
}

func TestBuildConfig_SuccessWithQPSAndBurst(t *testing.T) {
	// Arrange
	kubeConfig := ""

	patch := gomonkey.ApplyFunc(rest.InClusterConfig, func() (*rest.Config, error) {
		return &rest.Config{}, nil
	})
	defer patch.Reset()

	// Act
	config, err := BuildConfig(kubeConfig, QPS(50.0), Burst(100))

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, float32(50.0), config.QPS)
	assert.Equal(t, 100, config.Burst)
}

func TestBuildConfig_SuccessNoOptions(t *testing.T) {
	// Arrange
	kubeConfig := ""

	patch := gomonkey.ApplyFunc(rest.InClusterConfig, func() (*rest.Config, error) {
		return &rest.Config{}, nil
	})
	defer patch.Reset()

	// Act
	config, err := BuildConfig(kubeConfig)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, config)
}

func TestBuildConfig_InClusterConfigError(t *testing.T) {
	// Arrange
	kubeConfig := ""
	expectedErr := errors.New("in cluster config error")

	patch := gomonkey.ApplyFunc(rest.InClusterConfig, func() (*rest.Config, error) {
		return nil, expectedErr
	})
	defer patch.Reset()

	// Act
	config, err := BuildConfig(kubeConfig)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Nil(t, config)
}

func TestBuildConfig_BuildConfigFromFlagsError(t *testing.T) {
	// Arrange
	kubeConfig := "/invalid/path"
	expectedErr := errors.New("kubeconfig error")

	patch := gomonkey.ApplyFunc(clientcmd.BuildConfigFromFlags, func(_, _ string) (*rest.Config, error) {
		return nil, expectedErr
	})
	defer patch.Reset()

	// Act
	config, err := BuildConfig(kubeConfig)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Nil(t, config)
}

func TestBuildConfig_WithQPSOnly(t *testing.T) {
	// Arrange
	kubeConfig := ""

	patch := gomonkey.ApplyFunc(rest.InClusterConfig, func() (*rest.Config, error) {
		return &rest.Config{}, nil
	})
	defer patch.Reset()

	// Act
	config, err := BuildConfig(kubeConfig, QPS(50.0))

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, float32(50.0), config.QPS)
	assert.Equal(t, 0, config.Burst)
}

func TestBuildConfig_WithBurstOnly(t *testing.T) {
	// Arrange
	kubeConfig := ""

	patch := gomonkey.ApplyFunc(rest.InClusterConfig, func() (*rest.Config, error) {
		return &rest.Config{}, nil
	})
	defer patch.Reset()

	// Act
	config, err := BuildConfig(kubeConfig, Burst(100))

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, float32(0), config.QPS)
	assert.Equal(t, 100, config.Burst)
}

func TestNewBackendUtils_SuccessWithQPSAndBurst(t *testing.T) {
	// Arrange
	kubeConfig := ""

	patch := gomonkey.ApplyFunc(rest.InClusterConfig, func() (*rest.Config, error) {
		return &rest.Config{}, nil
	})
	defer patch.Reset()

	// Act
	clientset, err := NewBackendUtils(kubeConfig, QPS(50.0), Burst(100))

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, clientset)
}

func TestNewBackendUtils_SuccessNoOptions(t *testing.T) {
	// Arrange
	kubeConfig := ""

	patch := gomonkey.ApplyFunc(rest.InClusterConfig, func() (*rest.Config, error) {
		return &rest.Config{}, nil
	})
	defer patch.Reset()

	// Act
	clientset, err := NewBackendUtils(kubeConfig)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, clientset)
}

func TestNewBackendUtils_BuildConfigError(t *testing.T) {
	// Arrange
	kubeConfig := ""
	expectedErr := errors.New("build config error")

	patch := gomonkey.ApplyFunc(rest.InClusterConfig, func() (*rest.Config, error) {
		return nil, expectedErr
	})
	defer patch.Reset()

	// Act
	clientset, err := NewBackendUtils(kubeConfig)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Nil(t, clientset)
}
