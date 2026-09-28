/*
 Copyright (c) Huawei Technologies Co., Ltd. 2024-2026. All rights reserved.

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

package controller

import (
	"context"
	"errors"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"

	xuanwuv1 "github.com/Huawei/eSDK_K8S_Plugin/v4/client/apis/xuanwu/v1"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/lib/drcsi"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

var contentSyncTestLogName = "content_sync_test"

func init() {
	log.MockInitLogging(contentSyncTestLogName)
}

// newTestBackendController creates a backendController with fake fields for testing.
func newTestBackendController() *backendController {
	return &backendController{
		clientSet:     nil, // will be mocked via gomonkey on utils.GetContent/UpdateContentStatus
		eventRecorder: record.NewFakeRecorder(10),
		contentStore:  cache.NewStore(cache.MetaNamespaceKeyFunc),
	}
}

// ---------------------------------------------------------------------------
// initContentStatus tests
// ---------------------------------------------------------------------------

func TestBackendController_InitContentStatus_Success(t *testing.T) {
	// arrange
	ctrl := newTestBackendController()
	ctx := context.Background()
	content := &xuanwuv1.StorageBackendContent{}
	content.Name = "test-content"

	updatedContent := &xuanwuv1.StorageBackendContent{}
	updatedContent.Name = "test-content"
	updatedContent.Status = &xuanwuv1.StorageBackendContentStatus{
		ContentName:     "",
		VendorName:      "",
		ProviderVersion: "",
		Online:          true,
		Capabilities:    make(map[string]bool),
	}

	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFuncReturn(utils.GetContent, updatedContent, nil)
	patches.ApplyFuncReturn(utils.UpdateContentStatus, updatedContent, nil)

	// action
	gotContent, gotErr := ctrl.initContentStatus(ctx, content)

	// assert
	require.NoError(t, gotErr, "initContentStatus should succeed when status is nil")
	require.NotNil(t, gotContent, "initContentStatus should return content")
	assert.Equal(t, "test-content", gotContent.Name, "returned content name should match")
	assert.NotNil(t, gotContent.Status, "content status should be initialized")
	assert.True(t, gotContent.Status.Online, "initialized status should have Online=true")
}

func TestBackendController_InitContentStatus_StatusAlreadyInitialized(t *testing.T) {
	// arrange
	ctrl := newTestBackendController()
	ctx := context.Background()
	content := &xuanwuv1.StorageBackendContent{}
	content.Name = "test-content"

	existingContent := &xuanwuv1.StorageBackendContent{}
	existingContent.Name = "test-content"
	existingContent.Status = &xuanwuv1.StorageBackendContentStatus{
		ContentName: "existing-backend",
		Online:      true,
	}

	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFuncReturn(utils.GetContent, existingContent, nil)
	// UpdateContentStatus should NOT be called; mock it to fail so we can detect it
	patches.ApplyFuncReturn(utils.UpdateContentStatus, nil, errors.New("should not be called"))

	// action
	gotContent, gotErr := ctrl.initContentStatus(ctx, content)

	// assert
	require.NoError(t, gotErr, "initContentStatus should succeed when status already initialized")
	require.NotNil(t, gotContent, "initContentStatus should return content")
	assert.Equal(t, "existing-backend", gotContent.Status.ContentName,
		"existing status should be preserved without update")
}

func TestBackendController_InitContentStatus_ConflictRetrySuccess(t *testing.T) {
	// arrange
	ctrl := newTestBackendController()
	ctx := context.Background()
	content := &xuanwuv1.StorageBackendContent{}
	content.Name = "test-content"

	conflictErr := apiErrors.NewConflict(
		schema.GroupResource{Group: "xuanwu.huawei.com", Resource: "storagebackendcontents"},
		"test-content",
		errors.New("the object has been modified; please apply your changes to the latest version"),
	)

	contentWithoutStatus := &xuanwuv1.StorageBackendContent{}
	contentWithoutStatus.Name = "test-content"
	contentWithoutStatus.Status = nil

	updatedContent := &xuanwuv1.StorageBackendContent{}
	updatedContent.Name = "test-content"
	updatedContent.Status = &xuanwuv1.StorageBackendContentStatus{
		Online:       true,
		Capabilities: make(map[string]bool),
	}

	patches := gomonkey.NewPatches()
	defer patches.Reset()

	// GetContent always succeeds
	patches.ApplyFuncReturn(utils.GetContent, contentWithoutStatus, nil)
	// UpdateContentStatus: first call returns conflict, second call succeeds
	patches.ApplyFuncSeq(utils.UpdateContentStatus, []gomonkey.OutputCell{
		{Values: gomonkey.Params{nil, conflictErr}},
		{Values: gomonkey.Params{updatedContent, nil}},
	})

	// action
	gotContent, gotErr := ctrl.initContentStatus(ctx, content)

	// assert
	require.NoError(t, gotErr, "initContentStatus should succeed after conflict retry")
	require.NotNil(t, gotContent, "initContentStatus should return content after conflict retry")
	assert.NotNil(t, gotContent.Status, "content status should be initialized after retry")
	assert.True(t, gotContent.Status.Online, "initialized status should have Online=true")
}

func TestBackendController_InitContentStatus_GetContentError(t *testing.T) {
	// arrange
	ctrl := newTestBackendController()
	ctx := context.Background()
	content := &xuanwuv1.StorageBackendContent{}
	content.Name = "test-content"

	wantErr := errors.New("connection refused")

	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFuncReturn(utils.GetContent, nil, wantErr)

	// action
	gotContent, gotErr := ctrl.initContentStatus(ctx, content)

	// assert
	require.Error(t, gotErr, "initContentStatus should fail when GetContent fails")
	assert.Nil(t, gotContent, "no content should be returned on GetContent error")
	assert.Contains(t, gotErr.Error(), "initContentStatus", "error should mention function name")
	assert.Contains(t, gotErr.Error(), "connection refused", "error should wrap original GetContent error")
}

// ---------------------------------------------------------------------------
// updateContentStatusWithEvent tests
// ---------------------------------------------------------------------------

func TestBackendController_UpdateContentStatusWithEvent_Success(t *testing.T) {
	// arrange
	ctrl := newTestBackendController()
	ctx := context.Background()
	// content.Status is the desired status already computed by the caller
	content := &xuanwuv1.StorageBackendContent{}
	content.Name = "test-content"
	content.Status = &xuanwuv1.StorageBackendContentStatus{
		ContentName: "backend-1",
		SecretMeta:  "ns/new-secret",
		Online:      true,
	}

	// fresh content from API server (retry loop fetches for resourceVersion)
	freshContent := &xuanwuv1.StorageBackendContent{}
	freshContent.Name = "test-content"
	freshContent.Status = &xuanwuv1.StorageBackendContentStatus{
		ContentName: "backend-1",
		SecretMeta:  "ns/old-secret",
		Online:      true,
	}

	updatedContent := &xuanwuv1.StorageBackendContent{}
	updatedContent.Name = "test-content"
	updatedContent.Status = &xuanwuv1.StorageBackendContentStatus{
		ContentName: "backend-1",
		SecretMeta:  "ns/new-secret",
		Online:      true,
	}

	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFuncReturn(utils.GetContent, freshContent, nil)
	patches.ApplyFuncReturn(utils.UpdateContentStatus, updatedContent, nil)
	patches.ApplyFuncReturn(utils.StoreObjectUpdate, true, nil)

	// action
	gotContent, gotErr := ctrl.updateContentStatusWithEvent(
		ctx, content, "UpdateContentStatus", "Successful update")

	// assert
	require.NoError(t, gotErr, "updateContentStatusWithEvent should succeed")
	require.NotNil(t, gotContent, "updateContentStatusWithEvent should return content")
	assert.Equal(t, "backend-1", gotContent.Status.ContentName, "returned content should have the desired status")

	// verify event was recorded
	fakeRecorder, ok := ctrl.eventRecorder.(*record.FakeRecorder)
	if !ok {
		t.Errorf("expected eventRecorder to be *record.FakeRecorder, got %T", ctrl.eventRecorder)
	}
	select {
	case event := <-fakeRecorder.Events:
		assert.Contains(t, event, "UpdateContentStatus", "event should contain the reason")
		assert.Contains(t, event, "Successful update", "event should contain the message")
	default:
		t.Error("expected an event to be recorded")
	}
}

func TestBackendController_UpdateContentStatusWithEvent_ConflictRetrySuccess(t *testing.T) {
	// arrange
	ctrl := newTestBackendController()
	ctx := context.Background()
	content := &xuanwuv1.StorageBackendContent{}
	content.Name = "test-content"
	content.Status = &xuanwuv1.StorageBackendContentStatus{
		ContentName: "backend-1",
		Online:      true,
	}

	conflictErr := apiErrors.NewConflict(
		schema.GroupResource{Group: "xuanwu.huawei.com", Resource: "storagebackendcontents"},
		"test-content",
		errors.New("the object has been modified"),
	)

	freshContent := &xuanwuv1.StorageBackendContent{}
	freshContent.Name = "test-content"
	freshContent.Status = &xuanwuv1.StorageBackendContentStatus{
		ContentName: "backend-1",
		Online:      true,
	}

	updatedContent := &xuanwuv1.StorageBackendContent{}
	updatedContent.Name = "test-content"
	updatedContent.Status = &xuanwuv1.StorageBackendContentStatus{
		ContentName: "backend-1",
		Online:      true,
	}

	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFuncReturn(utils.GetContent, freshContent, nil)
	// UpdateContentStatus: first conflict, then success
	patches.ApplyFuncSeq(utils.UpdateContentStatus, []gomonkey.OutputCell{
		{Values: gomonkey.Params{nil, conflictErr}},
		{Values: gomonkey.Params{updatedContent, nil}},
	})
	patches.ApplyFuncReturn(utils.StoreObjectUpdate, true, nil)

	// action
	gotContent, gotErr := ctrl.updateContentStatusWithEvent(
		ctx, content, "UpdateContentStatus", "Successful update")

	// assert
	require.NoError(t, gotErr, "updateContentStatusWithEvent should succeed after conflict retry")
	require.NotNil(t, gotContent, "updateContentStatusWithEvent should return content after conflict retry")
	assert.Equal(t, "backend-1", gotContent.Status.ContentName,
		"returned content should have the desired status after retry")

	// verify event was recorded after successful retry
	fakeRecorder, ok := ctrl.eventRecorder.(*record.FakeRecorder)
	if !ok {
		t.Errorf("expected eventRecorder to be *record.FakeRecorder, got %T", ctrl.eventRecorder)
	}
	select {
	case event := <-fakeRecorder.Events:
		assert.Contains(t, event, "UpdateContentStatus", "event should contain the reason after retry")
	default:
		t.Error("expected an event to be recorded after conflict retry")
	}
}

func TestBackendController_UpdateContentStatusWithEvent_GetContentError(t *testing.T) {
	// arrange
	ctrl := newTestBackendController()
	ctx := context.Background()
	content := &xuanwuv1.StorageBackendContent{}
	content.Name = "test-content"
	content.Status = &xuanwuv1.StorageBackendContentStatus{Online: true}

	wantErr := errors.New("api server unreachable")

	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFuncReturn(utils.GetContent, nil, wantErr)

	// action
	gotContent, gotErr := ctrl.updateContentStatusWithEvent(
		ctx, content, "UpdateContentStatus", "Successful update")

	// assert
	require.Error(t, gotErr, "updateContentStatusWithEvent should fail when GetContent fails")
	assert.Nil(t, gotContent, "no content should be returned on GetContent error")
	assert.Equal(t, wantErr, gotErr, "error should be the original GetContent error")
}

func TestBackendController_UpdateContentStatusWithEvent_ContentStoreUpdateFailed(t *testing.T) {
	// arrange
	ctrl := newTestBackendController()
	ctx := context.Background()
	content := &xuanwuv1.StorageBackendContent{}
	content.Name = "test-content"
	content.Status = &xuanwuv1.StorageBackendContentStatus{Online: true}

	freshContent := &xuanwuv1.StorageBackendContent{}
	freshContent.Name = "test-content"
	freshContent.Status = &xuanwuv1.StorageBackendContentStatus{Online: true}

	updatedContent := &xuanwuv1.StorageBackendContent{}
	updatedContent.Name = "test-content"
	updatedContent.Status = &xuanwuv1.StorageBackendContentStatus{Online: true}

	wantErr := errors.New("cache update failed")

	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFuncReturn(utils.GetContent, freshContent, nil)
	patches.ApplyFuncReturn(utils.UpdateContentStatus, updatedContent, nil)
	patches.ApplyFuncReturn(utils.StoreObjectUpdate, false, wantErr)

	// action
	gotContent, gotErr := ctrl.updateContentStatusWithEvent(
		ctx, content, "UpdateContentStatus", "Successful update")

	// assert
	require.Error(t, gotErr, "updateContentStatusWithEvent should fail when contentStore update fails")
	assert.Nil(t, gotContent, "no content should be returned on store update failure")
	assert.Contains(t, gotErr.Error(), "cache update failed", "error should wrap the store update error")
}

// ---------------------------------------------------------------------------
// fakeHandler implements Handler interface for testing createContent /
// getContentStats / updateContentObj flows.
// ---------------------------------------------------------------------------

type fakeHandler struct {
	createErr error
	updateErr error
	statsErr  error
	backendID string
	stats     *drcsi.GetBackendStatsResponse
}

func (f *fakeHandler) CreateStorageBackend(ctx context.Context, content *xuanwuv1.StorageBackendContent) (
	string, string, error) {
	return "test-provider", f.backendID, f.createErr
}

func (f *fakeHandler) DeleteStorageBackend(ctx context.Context, backendName string) error {
	return nil
}

func (f *fakeHandler) UpdateStorageBackend(ctx context.Context, content *xuanwuv1.StorageBackendContent) error {
	return f.updateErr
}

func (f *fakeHandler) GetStorageBackendStats(ctx context.Context, contentName, backendName string) (
	*drcsi.GetBackendStatsResponse, error) {
	return f.stats, f.statsErr
}

// newTestBackendControllerWithHandler builds a controller with a fake handler.
func newTestBackendControllerWithHandler(h Handler) *backendController {
	return &backendController{
		clientSet:     nil,
		eventRecorder: record.NewFakeRecorder(10),
		contentStore:  cache.NewStore(cache.MetaNamespaceKeyFunc),
		handler:       h,
	}
}

// ---------------------------------------------------------------------------
// createContent tests
// ---------------------------------------------------------------------------

func TestBackendController_CreateContent_Success(t *testing.T) {
	// arrange
	h := &fakeHandler{backendID: "backend-1"}
	ctrl := newTestBackendControllerWithHandler(h)
	ctx := context.Background()

	content := &xuanwuv1.StorageBackendContent{}
	content.Name = "test-content"
	content.Spec = xuanwuv1.StorageBackendContentSpec{SecretMeta: "ns/secret-1"}

	// fresh content from API server with Online already set by controller
	freshContent := &xuanwuv1.StorageBackendContent{}
	freshContent.Name = "test-content"
	freshContent.Spec = xuanwuv1.StorageBackendContentSpec{SecretMeta: "ns/secret-1"}
	freshContent.Status = &xuanwuv1.StorageBackendContentStatus{Online: true}

	updatedContent := &xuanwuv1.StorageBackendContent{}
	updatedContent.Name = "test-content"
	updatedContent.Status = &xuanwuv1.StorageBackendContentStatus{
		ContentName: "backend-1",
		SecretMeta:  "ns/secret-1",
		Online:      true,
	}

	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFuncReturn(utils.GetContent, freshContent, nil)
	patches.ApplyFuncReturn(utils.UpdateContentStatus, updatedContent, nil)
	patches.ApplyFuncReturn(utils.StoreObjectUpdate, true, nil)

	// action
	gotContent, gotErr := ctrl.createContent(ctx, content)

	// assert
	require.NoError(t, gotErr, "createContent should succeed")
	require.NotNil(t, gotContent, "createContent should return content")
	assert.Equal(t, "backend-1", gotContent.Status.ContentName, "backendId should be set on status")
}

func TestBackendController_CreateContent_GetFreshContentError(t *testing.T) {
	// arrange
	h := &fakeHandler{backendID: "backend-1"}
	ctrl := newTestBackendControllerWithHandler(h)
	ctx := context.Background()

	content := &xuanwuv1.StorageBackendContent{}
	content.Name = "test-content"

	wantErr := errors.New("api server unreachable")

	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFuncReturn(utils.GetContent, nil, wantErr)

	// action
	gotContent, gotErr := ctrl.createContent(ctx, content)

	// assert
	require.Error(t, gotErr, "createContent should fail when fresh GetContent fails")
	assert.Nil(t, gotContent, "no content should be returned on fresh fetch error")
	assert.Equal(t, wantErr, gotErr, "error should be the GetContent error")
}

// ---------------------------------------------------------------------------
// getContentStats tests
// ---------------------------------------------------------------------------

func TestBackendController_GetContentStats_Success(t *testing.T) {
	// arrange
	stats := &drcsi.GetBackendStatsResponse{
		VendorName:      "Huawei",
		ProviderVersion: "v1.0",
		Online:          true,
		Specifications:  map[string]string{"LocalDeviceSN": "SN12345"},
		Capabilities:    map[string]bool{"snapshot": true},
	}
	h := &fakeHandler{stats: stats}
	ctrl := newTestBackendControllerWithHandler(h)
	ctx := context.Background()

	content := &xuanwuv1.StorageBackendContent{}
	content.Name = "test-content"
	content.Status = &xuanwuv1.StorageBackendContentStatus{ContentName: "backend-1"}

	// fresh content: Online was set to false by controller concurrently; should be preserved
	freshContent := &xuanwuv1.StorageBackendContent{}
	freshContent.Name = "test-content"
	freshContent.Status = &xuanwuv1.StorageBackendContentStatus{ContentName: "backend-1"}

	updatedContent := &xuanwuv1.StorageBackendContent{}
	updatedContent.Name = "test-content"
	updatedContent.Status = &xuanwuv1.StorageBackendContentStatus{
		ContentName:     "backend-1",
		VendorName:      "Huawei",
		ProviderVersion: "v1.0",
		Online:          true,
		SN:              "SN12345",
		Capabilities:    map[string]bool{"snapshot": true},
		Specification:   map[string]string{"LocalDeviceSN": "SN12345"},
	}

	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFuncReturn(utils.GetContent, freshContent, nil)
	patches.ApplyFuncReturn(utils.UpdateContentStatus, updatedContent, nil)
	patches.ApplyFuncReturn(utils.StoreObjectUpdate, true, nil)

	// action
	gotContent, gotErr := ctrl.getContentStats(ctx, content)

	// assert
	require.NoError(t, gotErr, "getContentStats should succeed")
	require.NotNil(t, gotContent, "getContentStats should return content")
	assert.Equal(t, "Huawei", gotContent.Status.VendorName, "VendorName should be updated from stats")
	assert.True(t, gotContent.Status.Online, "Online should be set from stats")
	assert.Equal(t, "SN12345", gotContent.Status.SN, "SN should be set from stats")
}

func TestBackendController_GetContentStats_GetFreshContentError(t *testing.T) {
	// arrange
	stats := &drcsi.GetBackendStatsResponse{Online: true}
	h := &fakeHandler{stats: stats}
	ctrl := newTestBackendControllerWithHandler(h)
	ctx := context.Background()

	content := &xuanwuv1.StorageBackendContent{}
	content.Name = "test-content"
	content.Status = &xuanwuv1.StorageBackendContentStatus{ContentName: "backend-1"}

	wantErr := errors.New("api server unreachable")

	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFuncReturn(utils.GetContent, nil, wantErr)

	// action
	gotContent, gotErr := ctrl.getContentStats(ctx, content)

	// assert
	require.Error(t, gotErr, "getContentStats should fail when fresh GetContent fails")
	assert.Nil(t, gotContent, "no content should be returned on fresh fetch error")
	assert.Equal(t, wantErr, gotErr, "error should be the GetContent error")
}

// ---------------------------------------------------------------------------
// updateContentObj tests
// ---------------------------------------------------------------------------

func TestBackendController_UpdateContentObj_Success(t *testing.T) {
	// arrange
	h := &fakeHandler{}
	ctrl := newTestBackendControllerWithHandler(h)
	ctx := context.Background()

	content := &xuanwuv1.StorageBackendContent{}
	content.Name = "test-content"
	content.Spec = xuanwuv1.StorageBackendContentSpec{SecretMeta: "ns/new-secret"}

	// fresh content has stale SecretMeta → shouldUpdateContent returns true
	freshContent := &xuanwuv1.StorageBackendContent{}
	freshContent.Name = "test-content"
	freshContent.Spec = xuanwuv1.StorageBackendContentSpec{SecretMeta: "ns/new-secret"}
	freshContent.Status = &xuanwuv1.StorageBackendContentStatus{
		SecretMeta: "ns/old-secret",
		Online:     true,
	}

	updatedContent := &xuanwuv1.StorageBackendContent{}
	updatedContent.Name = "test-content"
	updatedContent.Status = &xuanwuv1.StorageBackendContentStatus{
		SecretMeta: "ns/new-secret",
		Online:     true,
	}

	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFuncReturn(utils.GetContent, freshContent, nil)
	patches.ApplyFuncReturn(utils.UpdateContentStatus, updatedContent, nil)
	patches.ApplyFuncReturn(utils.StoreObjectUpdate, true, nil)

	// action
	gotContent, gotErr := ctrl.updateContentObj(ctx, content)

	// assert
	require.NoError(t, gotErr, "updateContentObj should succeed")
	require.NotNil(t, gotContent, "updateContentObj should return content")
	assert.Equal(t, "ns/new-secret", gotContent.Status.SecretMeta, "SecretMeta should be updated")
}

func TestBackendController_UpdateContentObj_GetFreshContentError(t *testing.T) {
	// arrange
	h := &fakeHandler{}
	ctrl := newTestBackendControllerWithHandler(h)
	ctx := context.Background()

	content := &xuanwuv1.StorageBackendContent{}
	content.Name = "test-content"

	wantErr := errors.New("api server unreachable")

	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFuncReturn(utils.GetContent, nil, wantErr)

	// action
	gotContent, gotErr := ctrl.updateContentObj(ctx, content)

	// assert
	require.Error(t, gotErr, "updateContentObj should fail when fresh GetContent fails")
	assert.Nil(t, gotContent, "no content should be returned on fresh fetch error")
	assert.Equal(t, wantErr, gotErr, "error should be the GetContent error")
}

// ---------------------------------------------------------------------------
// shouldUpdateContent tests
// ---------------------------------------------------------------------------

func TestBackendController_ShouldUpdateContent_SpecFieldsChanged(t *testing.T) {
	// arrange
	ctrl := newTestBackendController()
	ctx := context.Background()
	content := &xuanwuv1.StorageBackendContent{}
	content.Spec = xuanwuv1.StorageBackendContentSpec{
		SecretMeta:       "ns/new-secret",
		UseCert:          true,
		CertSecret:       "ns/cert-new",
		MaxClientThreads: "100",
		ConfigmapMeta:    "ns/cm-new",
	}
	content.Status = &xuanwuv1.StorageBackendContentStatus{}

	// action
	needUpdate := ctrl.shouldUpdateContent(ctx, content, nil, "new-backend-id")

	// assert
	assert.True(t, needUpdate, "should return true when spec fields differ from status")
	assert.Equal(t, "new-backend-id", content.Status.ContentName, "ContentName should be updated")
	assert.Equal(t, "ns/new-secret", content.Status.SecretMeta, "SecretMeta should be updated")
	assert.True(t, content.Status.UseCert, "UseCert should be updated")
	assert.Equal(t, "ns/cert-new", content.Status.CertSecret, "CertSecret should be updated")
	assert.Equal(t, "100", content.Status.MaxClientThreads, "MaxClientThreads should be updated")
	assert.Equal(t, "ns/cm-new", content.Status.ConfigmapMeta, "ConfigmapMeta should be updated")
}

func TestBackendController_ShouldUpdateContent_NoChange(t *testing.T) {
	// arrange
	ctrl := newTestBackendController()
	ctx := context.Background()
	content := &xuanwuv1.StorageBackendContent{}
	content.Status = &xuanwuv1.StorageBackendContentStatus{
		ContentName:      "backend-1",
		SecretMeta:       "ns/secret-1",
		UseCert:          false,
		CertSecret:       "",
		MaxClientThreads: "",
		ConfigmapMeta:    "",
	}
	content.Spec = xuanwuv1.StorageBackendContentSpec{
		SecretMeta:       "ns/secret-1",
		UseCert:          false,
		CertSecret:       "",
		MaxClientThreads: "",
		ConfigmapMeta:    "",
	}

	// action
	needUpdate := ctrl.shouldUpdateContent(ctx, content, nil, "backend-1")

	// assert
	assert.False(t, needUpdate, "should return false when all spec fields match status and backendId matches")
}

// ---------------------------------------------------------------------------
// shouldUpdateContentStatus tests
// ---------------------------------------------------------------------------

func TestBackendController_ShouldUpdateContentStatus_UpdatesAllFields(t *testing.T) {
	// arrange
	ctrl := newTestBackendController()
	ctx := context.Background()
	content := &xuanwuv1.StorageBackendContent{}
	content.Status = &xuanwuv1.StorageBackendContentStatus{}

	status := &drcsi.GetBackendStatsResponse{
		VendorName:      "Huawei",
		ProviderVersion: "v2.0",
		Online:          true,
		Specifications:  map[string]string{"LocalDeviceSN": "SN99999"},
		Capabilities:    map[string]bool{"snapshot": true, "clone": true},
		Pools:           []*drcsi.Pool{{Name: "pool-1", Capacities: map[string]string{"total": "100GB"}}},
	}

	// action
	needUpdate := ctrl.shouldUpdateContentStatus(ctx, content, status)

	// assert
	assert.True(t, needUpdate, "shouldUpdateContentStatus always returns true")
	assert.Equal(t, "Huawei", content.Status.VendorName, "VendorName should be updated from status")
	assert.Equal(t, "v2.0", content.Status.ProviderVersion, "ProviderVersion should be updated from status")
	assert.True(t, content.Status.Online, "Online should be updated from status")
	assert.Equal(t, "SN99999", content.Status.SN, "SN should be updated from specifications")
	assert.Equal(t, map[string]bool{"snapshot": true, "clone": true},
		content.Status.Capabilities, "Capabilities should be updated from status")
	assert.Equal(t, map[string]string{"LocalDeviceSN": "SN99999"},
		content.Status.Specification, "Specification should be updated from specifications")
	require.Len(t, content.Status.Pools, 1, "Pools should have one entry")
	assert.Equal(t, "pool-1", content.Status.Pools[0].Name, "Pool name should be set")
	assert.Equal(t, map[string]string{"total": "100GB"},
		content.Status.Pools[0].Capacities, "Pool capacities should be set")
}
