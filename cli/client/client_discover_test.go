/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2023-2023. All rights reserved.
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

package client

import "testing"

func Test_nodeSupportedCliErrorMsg(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "Zero supported CLI",
			args: []string{},
			want: "Could not find any supported CLI",
		},
		{
			name: "One supported CLI",
			args: []string{"kubectl"},
			want: "Could not find any supported CLI, e.g. kubectl, details see /test",
		},
		{
			name: "Two supported CLI",
			args: []string{"kubectl", "oc"},
			want: "Could not find any supported CLI, e.g. kubectl or oc, details see /test",
		},
		{
			name: "Three supported CLI",
			args: []string{"kubectl", "oc", "other"},
			want: "Could not find any supported CLI, e.g. kubectl, oc or other, details see /test",
		},
		{
			name: "Great than three supported CLI",
			args: []string{"kubectl", "oc", "other1", "other2"},
			want: "Could not find any supported CLI, e.g. kubectl, oc, other1 or other2, details see /test",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noneSupportedCliErrorMsg(tt.args, "/test"); got != tt.want {
				t.Errorf("noneSupportedCliErrorMsg() = %v, want %v", got, tt.want)
			}
		})
	}
}
