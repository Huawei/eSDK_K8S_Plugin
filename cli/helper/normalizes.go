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

package helper

import (
	"strings"
)

const Indentation = `  `

type Normalizer struct {
	string
}

// NewNormalizer initialize an instance of Normalizer
func NewNormalizer(s string) Normalizer {
	return Normalizer{s}
}

// Examples normalizes a command's examples to follow the conventions.
func Examples(s string) string {
	if len(s) == 0 {
		return s
	}
	return NewNormalizer(s).trim().indent().string
}

// NIndent used to indent n
func (s Normalizer) NIndent(spaces int) string {
	pad := strings.Repeat(" ", spaces)
	return "\n" + pad + strings.Replace(s.string, "\n", "\n"+pad, -1)
}

func (s Normalizer) trim() Normalizer {
	s.string = strings.TrimSpace(s.string)
	return s
}

func (s Normalizer) indent() Normalizer {
	var indentedLines []string
	for _, line := range strings.Split(s.string, "\n") {
		trimmed := strings.TrimSpace(line)
		indented := Indentation + trimmed
		indentedLines = append(indentedLines, indented)
	}
	s.string = strings.Join(indentedLines, "\n")
	return s
}
