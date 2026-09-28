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

// Package auth provides gRPC authentication interceptor for CSI export service.
package auth

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	authv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	authv1client "k8s.io/client-go/kubernetes/typed/authentication/v1"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

// Interceptor validates Bearer tokens via Kubernetes TokenReview API.
type Interceptor struct {
	tokenReviewer authv1client.TokenReviewInterface
	audience      string
}

// NewInterceptor creates an authentication interceptor.
// Returns error if tokenReviewer is nil.
func NewInterceptor(tokenReviewer authv1client.TokenReviewInterface, audience string) (*Interceptor, error) {
	if tokenReviewer == nil {
		return nil, status.Error(codes.Internal, "tokenReviewer must not be nil")
	}
	return &Interceptor{
		tokenReviewer: tokenReviewer,
		audience:      audience,
	}, nil
}

// ValidateToken validates the Bearer token from gRPC metadata via Kubernetes TokenReview API
// and checks the audience binding. Authorization model is audience-only (no SubjectAccessReview).
func (i *Interceptor) ValidateToken(ctx context.Context, req interface{},
	info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	// 1. Extract Bearer token from gRPC metadata
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		log.AddContext(ctx).Warningf("CSI export auth failed: no metadata in request")
		return nil, status.Error(codes.Unauthenticated, "no authorization token provided")
	}

	values := md.Get("authorization")
	if len(values) == 0 {
		log.AddContext(ctx).Warningf("CSI export auth failed: no authorization header")
		return nil, status.Error(codes.Unauthenticated, "no authorization token provided")
	}

	authHeader := values[0]
	schemeEnd := strings.Index(authHeader, " ")
	if schemeEnd < 0 {
		log.AddContext(ctx).Warningf("CSI export auth failed: invalid authorization format")
		return nil, status.Error(codes.Unauthenticated, "invalid authorization format")
	}

	scheme := strings.TrimSpace(authHeader[:schemeEnd])
	if !strings.EqualFold(scheme, "bearer") {
		log.AddContext(ctx).Warningf("CSI export auth failed: invalid authorization format")
		return nil, status.Error(codes.Unauthenticated, "invalid authorization format")
	}

	token := strings.TrimSpace(authHeader[schemeEnd+1:])
	if token == "" {
		log.AddContext(ctx).Warningf("CSI export auth failed: empty bearer token")
		return nil, status.Error(codes.Unauthenticated, "empty bearer token")
	}

	// 2. Call TokenReview API
	review, err := i.tokenReviewer.Create(ctx, &authv1.TokenReview{
		Spec: authv1.TokenReviewSpec{
			Token:     token,
			Audiences: []string{i.audience},
		},
	}, metav1.CreateOptions{})

	if err != nil {
		log.AddContext(ctx).Warningf("CSI export auth failed: token review API unavailable: %v", err)
		return nil, status.Error(codes.Unavailable,
			"failed to verify token: token review API unavailable")
	}

	if !review.Status.Authenticated {
		log.AddContext(ctx).Warningf("CSI export auth failed: token not authenticated")
		return nil, status.Error(codes.Unauthenticated, "token authentication failed")
	}

	// 3. Check audience match
	audienceMatched := false
	for _, aud := range review.Status.Audiences {
		if aud == i.audience {
			audienceMatched = true
			break
		}
	}
	if !audienceMatched {
		log.AddContext(ctx).Warningf("CSI export auth failed: token audience not allowed, "+
			"expected %s, got %v", i.audience, review.Status.Audiences)
		return nil, status.Error(codes.Unauthenticated, "token audience not allowed")
	}

	// 4. Auth passed - forward to next interceptor/handler
	return handler(ctx, req)
}
