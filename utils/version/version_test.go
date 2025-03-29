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

package version

import (
	"testing"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name string
		v1   string
		v2   string
		want int
	}{
		{name: "compare 6.1.9 and 6.1.10", v1: "6.1.9", v2: "6.1.10", want: -1},
		{name: "compare 6.1.10 and 6.1.10", v1: "6.1.10", v2: "6.1.10", want: 0},
		{name: "compare 6.1.10 and 6.1.9", v1: "6.1.10", v2: "6.1.9", want: 1},
		{name: "compare 6.2.0 and 6.1.9999", v1: "6.2.0", v2: "6.1.9999", want: 1},
		{name: "compare 6.1.9999 and 6.2.0", v1: "6.1.9999", v2: "6.2.0", want: -1},
		{name: "compare 7.0.0 and 6.9.9", v1: "7.0.0", v2: "6.9.9", want: 1},
		{name: "compare 6.9.9 and 7.0.0", v1: "6.9.9", v2: "7.0.0", want: -1},
		{name: "compare 6.1.8 and 6.1.8.SPH001", v1: "6.1.8", v2: "6.1.8.SPH001", want: -1},
		{name: "compare 6.1.8.SPH001 and 6.1.8", v1: "6.1.8.SPH001", v2: "6.1.8", want: 1},
		{name: "compare 6.1.8.SPH002 and 6.1.8.SPH001", v1: "6.1.8.SPH002", v2: "6.1.8.SPH001", want: 1},
		{name: "compare 6.1.8.SPH001 and 6.1.8.SPH002", v1: "6.1.8.SPH001", v2: "6.1.8.SPH002", want: -1},
		{name: "compare 6.1.8.SPH001 and 6.1.8.SPH001", v1: "6.1.8.SPH001", v2: "6.1.8.SPH001", want: 0},
		{name: "compare 7.0 and 6.9.9", v1: "7.0", v2: "6.9.9", want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CompareVersions(tt.v1, tt.v2)
			if got != tt.want {
				t.Errorf("CompareVersions() got = %v, want %v", got, tt.want)
			}
		})
	}
}
