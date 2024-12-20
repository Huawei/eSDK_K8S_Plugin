/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2024-2024. All rights reserved.
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

package volume

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_isHyperMetroFromParams(t *testing.T) {
	tests := []struct {
		name    string
		params  map[string]any
		want    bool
		wantErr error
	}{
		{name: "not exists", params: map[string]any{"hyperMetro": true}, want: false, wantErr: nil},
		{name: "exists true", params: map[string]any{"hypermetro": true}, want: true, wantErr: nil},
		{name: "exists false", params: map[string]any{"hypermetro": false}, want: false, wantErr: nil},
		{name: "exists not bool type", params: map[string]any{"hypermetro": "true"}, want: false,
			wantErr: fmt.Errorf("parameter hyperMetro [%v] in sc must be bool", "true")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := isHyperMetroFromParams(tt.params)
			require.Equal(t, tt.wantErr, err)
			require.Equal(t, tt.want, got)
		})
	}
}
