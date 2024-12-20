/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2023. All rights reserved.
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

import "errors"

// Volume interface is a perform operations on volume object
type Volume interface {
	GetVolumeName() string
	GetLunWWN() (string, error)
	SetLunWWN(string)
	SetSize(int64)
	GetSize() int64
	SetDTreeParentName(string)
	GetDTreeParentName() string
	GetFilesystemMode() string
	SetFilesystemMode(string)
	GetID() string
	SetID(string)
}

type volume struct {
	id              string
	name            string
	lunWWN          string
	size            int64
	dTreeParentName string
	filesystemMode  string
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

// SetSize sets volume size in volume object
func (vol *volume) SetSize(size int64) {
	vol.size = size
}

// GetSize gets volume size in volume object
func (vol *volume) GetSize() int64 {
	return vol.size
}

func (vol *volume) SetDTreeParentName(dTreeParentName string) {
	vol.dTreeParentName = dTreeParentName
}

func (vol *volume) GetDTreeParentName() string {
	return vol.dTreeParentName
}

func (vol *volume) GetFilesystemMode() string {
	return vol.filesystemMode
}

func (vol *volume) SetFilesystemMode(filesystemMode string) {
	vol.filesystemMode = filesystemMode
}

// GetID Get storage ID of volume object
func (vol *volume) GetID() string {
	return vol.id
}

// SetID Get storage ID of volume object
func (vol *volume) SetID(id string) {
	vol.id = id
}
