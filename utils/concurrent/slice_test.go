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
package concurrent

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSlice_ConcurrentAppend(t *testing.T) {
	// arrange
	s := Slice[struct{}]{}
	empty := struct{}{}
	expectedLen := 10000
	var wg sync.WaitGroup

	// action
	for range expectedLen {
		wg.Add(1)
		go func() {
			s.Append(empty)
			wg.Done()
		}()
	}
	wg.Wait()

	// assert
	require.Equal(t, expectedLen, s.Len())
}

func TestSlice_Reset(t *testing.T) {
	// arrange
	s := Slice[int]{slice: []int{2, 3}}

	// action
	s.Reset()
	s.Append(1)

	// assert
	require.Equal(t, 1, s.Len())
	require.Equal(t, 1, s.Get(0))
	require.Equal(t, []int{1}, s.Values())
}

func TestSlice_Cut(t *testing.T) {
	// arrange
	s := []int{1, 2}
	getSlice := func() Slice[int] { return Slice[int]{slice: s} }
	cases := []struct {
		name           string
		l              int
		h              int
		expectedValues []int
		expectedErr    bool
	}{
		{name: "l is 0, h is 0", l: 0, h: 0, expectedValues: s[0:0], expectedErr: false},
		{name: "l is 0, h is 1", l: 0, h: 1, expectedValues: s[0:1], expectedErr: false},
		{name: "l is 0, h is 2", l: 0, h: 2, expectedValues: s[0:], expectedErr: false},
		{name: "l is 1, h is 1", l: 1, h: 1, expectedValues: s[1:1], expectedErr: false},
		{name: "l is 1, h is 2", l: 1, h: 2, expectedValues: s[1:], expectedErr: false},
		{name: "l is 2, h is 2", l: 2, h: 2, expectedValues: s[2:], expectedErr: false},
		{name: "l is 1, h is 0", l: 1, h: 0, expectedValues: s, expectedErr: true},
		{name: "l is 0, h is 3", l: 0, h: 3, expectedValues: s, expectedErr: true},
		{name: "l is 1, h is 3", l: 1, h: 3, expectedValues: s, expectedErr: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := getSlice()

			// action
			err := s.Cut(c.l, c.h)

			// assert
			require.Equal(t, c.expectedErr, err != nil)
			if err == nil {
				require.Equal(t, c.expectedValues, s.Values())
			}
		})
	}
}

func TestSlice_Get(t *testing.T) {
	// arrange
	s := Slice[int]{slice: []int{1, 2}}

	// action
	v1 := s.Get(0)
	v2 := s.Get(1)
	v3 := s.Get(2)

	// assert
	require.Equal(t, 1, v1)
	require.Equal(t, 2, v2)
	require.Equal(t, 0, v3)
}
