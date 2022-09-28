/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2022. All rights reserved.
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

package utils

import (
	"testing"
)

func same(v1, v2 *Version) bool {
	if len(v1.components) != len(v2.components) {
		return false
	}

	for i := range v1.components {
		if v1.components[i] != v2.components[i] {
			return false
		}
	}

	if v1.semver != v2.semver || v1.datever != v2.datever || v1.preRelease != v2.preRelease || v1.buildMetadata != v2.buildMetadata {
		return false
	}

	return true
}

type parseTest struct {
	str     string
	version *Version
	wantErr bool
}

var parseGenericTests = []parseTest{
	{"", nil, true},
	{"v1", nil, true},
	{"v01.1", nil, true},
	{"v1.*", &Version{components: []uint{1, 4294967295}}, false},
	{"v1.2", &Version{components: []uint{1, 2}}, false},
	{"v1.02", &Version{components: []uint{1, 2}}, false},
}

var parseSemanticTests = []parseTest{
	{"", nil, true},
	{"v1.02", nil, true},
	{"v1.2", nil, true},
	{"v1.2-1.3", nil, true},
	{"v1.2.1-1.3+data", &Version{components: []uint{1, 2, 1}, semver: true, datever: false, preRelease: "1.3", buildMetadata: "data"}, false},
}

func parseTests(t *testing.T, f func(str string) (*Version, error), funcName string, testCases []parseTest) {
	for _, test := range testCases {
		actual, err := f(test.str)

		if (err != nil) != test.wantErr {
			t.Errorf("%s(%v) = Error(%v); wantErr Error(%v)", funcName, test.str, err, test.wantErr)
		}

		if actual == nil && actual != test.version {
			t.Errorf("%s(%v) = Version(%v); wantErr Version(%v)", funcName, test.str, err, test.wantErr)
		}

		if actual != nil && !same(actual, test.version) {
			t.Errorf("%s(%v) = Version(%v); want Version(%v)", funcName, test.str, actual, test.version)
		}
	}
}

func TestMustParseGeneric(t *testing.T) {
	parseTests(t, ParseGeneric, "MustParseGeneric", parseGenericTests)
}

func TestMustParseSemantic(t *testing.T) {
	parseTests(t, ParseSemantic, "MustParseSemantic", parseSemanticTests)
}

var testVersion = &Version{
	components:    []uint{1, 2},
	semver:        true,
	datever:       false,
	preRelease:    "1.3",
	buildMetadata: "data",
}

type thanTest struct {
	result bool
	v      *Version
}

var lessThanTests = []thanTest{
	{false, &Version{components: []uint{1, 1}}},
	{true, &Version{components: []uint{1, 3}}},
	{false, &Version{components: []uint{1}}},
	{false, &Version{components: []uint{1, 2}, semver: false, datever: false}},
	{true, &Version{components: []uint{1, 2}, semver: true, datever: false, preRelease: ""}},
	{true, &Version{components: []uint{1, 2}, semver: true, datever: false, preRelease: "1.4"}},
	{false, &Version{components: []uint{1, 2}, semver: true, datever: false, preRelease: "1.2"}},
	{false, testVersion},
}

func TestLessThan(t *testing.T) {
	for _, tt := range lessThanTests {
		if got := testVersion.LessThan(tt.v); got != tt.result {
			t.Errorf("Version(%v).LessThan(Version(%v)) = %v, want %v", testVersion, tt.v, got, tt.result)
		}
	}
}

var greaterThanTests = []thanTest{
	{true, &Version{components: []uint{1, 1}}},
	{false, &Version{components: []uint{1, 3}}},
	{true, &Version{components: []uint{1}}},
	{false, testVersion},
}

func TestGreaterThan(t *testing.T) {
	for _, tt := range greaterThanTests {
		if got := testVersion.GreaterThan(tt.v); got != tt.result {
			t.Errorf("Version(%v).GreaterThan(Version(%v)) = %v, want %v", testVersion, tt.v, got, tt.result)
		}
	}
}

type stringTest struct {
	version string
	v       *Version
}

var shortStringTests = []stringTest{
	{"", &Version{}},
	{"1.2", &Version{components: []uint{1, 2}}},
	{"1.02", &Version{components: []uint{1, 2}, datever: true}},
}

func TestShortString(t *testing.T) {
	for _, tt := range shortStringTests {
		if got := tt.v.ShortString(); got != tt.version {
			t.Errorf("Version(%v).ShortString() = %v, want %v", tt.v, got, tt.version)
		}
	}
}

var stringTests = []stringTest{
	{"", &Version{}},
	{"1.2", &Version{components: []uint{1, 2}}},
	{"1.02", &Version{components: []uint{1, 2}, datever: true}},
	{"1.2-1.3", &Version{components: []uint{1, 2}, preRelease: "1.3"}},
	{"1.2-1.3+data", testVersion},
}

func TestString(t *testing.T) {
	for _, tt := range stringTests {
		if got := tt.v.String(); got != tt.version {
			t.Errorf("Version(%v).String() = %v, want %v", tt.v, got, tt.version)
		}
	}
}
