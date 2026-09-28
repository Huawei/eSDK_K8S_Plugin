/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2025-2025. All rights reserved.
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

// Package volume defines operations of volumes
package volume

import (
	"context"
)

// Deleter is used to delete a volume via ModeHandler
type Deleter struct {
	ctx     context.Context
	handler ModeHandler
}

// NewDeleter inits a new volume deleter
func NewDeleter(ctx context.Context, handler ModeHandler) *Deleter {
	return &Deleter{
		ctx:     ctx,
		handler: handler,
	}
}

// Delete deletes a volume resource from storage
func (d *Deleter) Delete() error {
	return d.handler.Delete(d.ctx)
}
