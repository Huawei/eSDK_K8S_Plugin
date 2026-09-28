/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2023-2026. All rights reserved.
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

package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	authv1 "k8s.io/api/authentication/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	authv1client "k8s.io/client-go/kubernetes/typed/authentication/v1"
	k8stesting "k8s.io/client-go/testing"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const testAudience = "csi.huawei.com"

func TestMain(m *testing.M) {
	log.MockInitLogging("auth_test.log")
	defer log.MockStopLogging("auth_test.log")

	m.Run()
}

func newFakeTokenReviewer(reviewResponse *authv1.TokenReview, err error) authv1client.TokenReviewInterface {
	fakeClient := fake.NewSimpleClientset()
	if err != nil {
		fakeClient.Fake.PrependReactor("create", "tokenreviews",
			func(action k8stesting.Action) (bool, runtime.Object, error) {
				return true, nil, err
			})
	} else {
		fakeClient.Fake.PrependReactor("create", "tokenreviews",
			func(action k8stesting.Action) (bool, runtime.Object, error) {
				return true, reviewResponse, nil
			})
	}
	return fakeClient.AuthenticationV1().TokenReviews()
}

func handlerCalledMarker() (grpc.UnaryHandler, *bool) {
	called := false
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		called = true
		return "ok", nil
	}
	return handler, &called
}

// TestNewInterceptor_NilTokenReviewer_ReturnsError tests that
// NewInterceptor returns error when tokenReviewer is nil.
func TestNewInterceptor_NilTokenReviewer_ReturnsError(t *testing.T) {
	interceptor, gotErr := NewInterceptor(nil, testAudience)

	require.Nil(t, interceptor)
	require.Error(t, gotErr)
	st, ok := status.FromError(gotErr)
	require.True(t, ok)
	require.Equal(t, codes.Internal, st.Code())
}

// TestAuthenticate_NoMetadata_RequestRejected tests that a request without
// gRPC metadata is rejected with Unauthenticated.
func TestAuthenticate_NoMetadata_RequestRejected(t *testing.T) {
	interceptor, _ := NewInterceptor(newFakeTokenReviewer(nil, nil), testAudience)
	handler, called := handlerCalledMarker()

	_, gotErr := interceptor.ValidateToken(
		context.Background(), nil,
		&grpc.UnaryServerInfo{}, handler,
	)

	require.False(t, *called, "handler should not be called")
	require.Error(t, gotErr)
	st, ok := status.FromError(gotErr)
	require.True(t, ok)
	require.Equal(t, codes.Unauthenticated, st.Code())
	require.Equal(t, "no authorization token provided", st.Message())
}

// TestAuthenticate_NoAuthorizationHeader_RequestRejected tests that a request
// with metadata but no authorization header is rejected.
func TestAuthenticate_NoAuthorizationHeader_RequestRejected(t *testing.T) {
	interceptor, _ := NewInterceptor(newFakeTokenReviewer(nil, nil), testAudience)
	handler, called := handlerCalledMarker()
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs())

	_, gotErr := interceptor.ValidateToken(
		ctx, nil,
		&grpc.UnaryServerInfo{}, handler,
	)

	require.False(t, *called, "handler should not be called")
	require.Error(t, gotErr)
	st, ok := status.FromError(gotErr)
	require.True(t, ok)
	require.Equal(t, codes.Unauthenticated, st.Code())
	require.Equal(t, "no authorization token provided", st.Message())
}

// TestAuthenticate_InvalidFormat_RequestRejected tests that an authorization
// header without "Bearer " prefix is rejected.
func TestAuthenticate_InvalidFormat_RequestRejected(t *testing.T) {
	interceptor, _ := NewInterceptor(newFakeTokenReviewer(nil, nil), testAudience)
	handler, called := handlerCalledMarker()
	ctx := metadata.NewIncomingContext(context.Background(),
		metadata.Pairs("authorization", "Basic dXNlcjpwYXNz"))

	_, gotErr := interceptor.ValidateToken(
		ctx, nil,
		&grpc.UnaryServerInfo{}, handler,
	)

	require.False(t, *called, "handler should not be called")
	require.Error(t, gotErr)
	st, ok := status.FromError(gotErr)
	require.True(t, ok)
	require.Equal(t, codes.Unauthenticated, st.Code())
	require.Equal(t, "invalid authorization format", st.Message())
}

// TestAuthenticate_EmptyBearerToken_RequestRejected tests that an authorization
// header with "Bearer " prefix but empty token is rejected.
func TestAuthenticate_EmptyBearerToken_RequestRejected(t *testing.T) {
	interceptor, _ := NewInterceptor(newFakeTokenReviewer(nil, nil), testAudience)
	handler, called := handlerCalledMarker()
	ctx := metadata.NewIncomingContext(context.Background(),
		metadata.Pairs("authorization", "Bearer "))

	_, gotErr := interceptor.ValidateToken(
		ctx, nil,
		&grpc.UnaryServerInfo{}, handler,
	)

	require.False(t, *called, "handler should not be called")
	require.Error(t, gotErr)
	st, ok := status.FromError(gotErr)
	require.True(t, ok)
	require.Equal(t, codes.Unauthenticated, st.Code())
	require.Equal(t, "empty bearer token", st.Message())
}

// TestAuthenticate_LowercaseBearerScheme_RequestAllowed tests that
// a lowercase "bearer" scheme is accepted (RFC 6750 compliance).
func TestAuthenticate_LowercaseBearerScheme_RequestAllowed(t *testing.T) {
	reviewResponse := &authv1.TokenReview{
		Status: authv1.TokenReviewStatus{
			Authenticated: true,
			Audiences:     []string{"csi.huawei.com"},
		},
	}
	interceptor, _ := NewInterceptor(newFakeTokenReviewer(reviewResponse, nil), testAudience)
	handler, called := handlerCalledMarker()
	ctx := metadata.NewIncomingContext(context.Background(),
		metadata.Pairs("authorization", "bearer valid-token"))

	resp, gotErr := interceptor.ValidateToken(
		ctx, nil,
		&grpc.UnaryServerInfo{}, handler,
	)

	require.NoError(t, gotErr)
	require.True(t, *called, "handler should be called")
	require.Equal(t, "ok", resp)
}

// TestAuthenticate_TokenReviewAPIUnavailable_RequestRejected tests that
// when TokenReview API is unreachable, the request is rejected with
// Unavailable (fail-closed).
func TestAuthenticate_TokenReviewAPIUnavailable_RequestRejected(t *testing.T) {
	interceptor, _ := NewInterceptor(
		newFakeTokenReviewer(nil, errors.New("api server unreachable")),
		testAudience,
	)
	handler, called := handlerCalledMarker()
	ctx := metadata.NewIncomingContext(context.Background(),
		metadata.Pairs("authorization", "Bearer some-token"))

	_, gotErr := interceptor.ValidateToken(
		ctx, nil,
		&grpc.UnaryServerInfo{}, handler,
	)

	require.False(t, *called, "handler should not be called")
	require.Error(t, gotErr)
	st, ok := status.FromError(gotErr)
	require.True(t, ok)
	require.Equal(t, codes.Unavailable, st.Code())
	require.Contains(t, st.Message(), "token review API unavailable")
}

// TestAuthenticate_TokenNotAuthenticated_RequestRejected tests that
// when TokenReview returns authenticated=false, the request is rejected.
func TestAuthenticate_TokenNotAuthenticated_RequestRejected(t *testing.T) {
	reviewResponse := &authv1.TokenReview{
		Status: authv1.TokenReviewStatus{
			Authenticated: false,
		},
	}
	interceptor, _ := NewInterceptor(newFakeTokenReviewer(reviewResponse, nil), testAudience)
	handler, called := handlerCalledMarker()
	ctx := metadata.NewIncomingContext(context.Background(),
		metadata.Pairs("authorization", "Bearer bad-token"))

	_, gotErr := interceptor.ValidateToken(
		ctx, nil,
		&grpc.UnaryServerInfo{}, handler,
	)

	require.False(t, *called, "handler should not be called")
	require.Error(t, gotErr)
	st, ok := status.FromError(gotErr)
	require.True(t, ok)
	require.Equal(t, codes.Unauthenticated, st.Code())
	require.Equal(t, "token authentication failed", st.Message())
}

// TestAuthenticate_AudienceNotMatched_RequestRejected tests that
// when the token's audience does not match the expected audience,
// the request is rejected.
func TestAuthenticate_AudienceNotMatched_RequestRejected(t *testing.T) {
	reviewResponse := &authv1.TokenReview{
		Status: authv1.TokenReviewStatus{
			Authenticated: true,
			Audiences:     []string{"api-server"},
		},
	}
	interceptor, _ := NewInterceptor(newFakeTokenReviewer(reviewResponse, nil), testAudience)
	handler, called := handlerCalledMarker()
	ctx := metadata.NewIncomingContext(context.Background(),
		metadata.Pairs("authorization", "Bearer valid-but-wrong-audience-token"))

	_, gotErr := interceptor.ValidateToken(
		ctx, nil,
		&grpc.UnaryServerInfo{}, handler,
	)

	require.False(t, *called, "handler should not be called")
	require.Error(t, gotErr)
	st, ok := status.FromError(gotErr)
	require.True(t, ok)
	require.Equal(t, codes.Unauthenticated, st.Code())
	require.Equal(t, "token audience not allowed", st.Message())
}

// TestAuthenticate_ValidTokenWithMatchingAudience_RequestAllowed tests that
// a valid token with matching audience is allowed through to the handler.
func TestAuthenticate_ValidTokenWithMatchingAudience_RequestAllowed(t *testing.T) {
	reviewResponse := &authv1.TokenReview{
		Status: authv1.TokenReviewStatus{
			Authenticated: true,
			Audiences:     []string{"csi.huawei.com"},
		},
	}
	interceptor, _ := NewInterceptor(newFakeTokenReviewer(reviewResponse, nil), testAudience)
	handler, called := handlerCalledMarker()
	ctx := metadata.NewIncomingContext(context.Background(),
		metadata.Pairs("authorization", "Bearer valid-token"))

	resp, gotErr := interceptor.ValidateToken(
		ctx, nil,
		&grpc.UnaryServerInfo{}, handler,
	)

	require.NoError(t, gotErr)
	require.True(t, *called, "handler should be called")
	require.Equal(t, "ok", resp)
}

// TestAuthenticate_CustomAudience_RequestAllowed tests that
// a custom audience value is used when configured.
func TestAuthenticate_CustomAudience_RequestAllowed(t *testing.T) {
	customAudience := "custom.csi.example.com"
	reviewResponse := &authv1.TokenReview{
		Status: authv1.TokenReviewStatus{
			Authenticated: true,
			Audiences:     []string{customAudience},
		},
	}
	interceptor, _ := NewInterceptor(newFakeTokenReviewer(reviewResponse, nil), customAudience)
	handler, called := handlerCalledMarker()
	ctx := metadata.NewIncomingContext(context.Background(),
		metadata.Pairs("authorization", "Bearer valid-token"))

	resp, gotErr := interceptor.ValidateToken(
		ctx, nil,
		&grpc.UnaryServerInfo{}, handler,
	)

	require.NoError(t, gotErr)
	require.True(t, *called, "handler should be called")
	require.Equal(t, "ok", resp)
}
