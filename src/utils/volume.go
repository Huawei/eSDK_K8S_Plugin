/*
 Copyright (c) Huawei Technologies Co., Ltd. 2021-2021. All rights reserved.

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

package utils

import "errors"

// Volume interface is a perform operations on volume object
type Volume interface {
	GetVolumeName() string
	GetLunWWN() (string, error)
	SetLunWWN(string)
}

type volume struct {
	name        string
	lunWWN      string
}

// NewVolume creates volume object for the name
func NewVolume(name string) Volume {
	return &volume{
		name: name,
	}
}

// SetLunWWN sets lun WWN in volume object
func (vol *volume) SetLunWWN(wwn string) {
	vol.lunWWN = wwn
}

// GetVolumeName gets volume name from volume object
func (vol *volume) GetVolumeName() string {
	return vol.name
}

// GetLunWWN gets lun WWN from volume object
func (vol *volume) GetLunWWN() (string, error) {
	if "" == vol.lunWWN {
		return "", errors.New("empty WWN")
	}
	return vol.lunWWN, nil
}
