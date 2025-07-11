/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2024-2025. All rights reserved.
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

package creator

import (
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
)

const (
	// AccessKrb5ReadOnly defines ReadOnly AccessKrb5 access level
	AccessKrb5ReadOnly = "read_only"

	// AccessKrb5ReadWrite defines ReadWrite AccessKrb5 access level
	AccessKrb5ReadWrite = "read_write"

	// AccessKrb5ReadNone defines None AccessKrb5 access level
	AccessKrb5ReadNone = "none"

	// AccessKrb5ReadOnlyInt defines ReadOnly AccessKrb5 access level of int value
	AccessKrb5ReadOnlyInt = 0

	// AccessKrb5ReadWriteInt defines ReadWrite AccessKrb5 access level of int value
	AccessKrb5ReadWriteInt = 1

	// AccessKrb5ReadNoneInt defines None AccessKrb5 access level of int value
	AccessKrb5ReadNoneInt = 5

	// AccessKrb5UndefinedInt is undefined int value
	AccessKrb5UndefinedInt = -1
)

// AccessKrb is a type alias for a string that represents access levels to Kerberos.
type AccessKrb string

// Int converts the access level to an integer.
func (a AccessKrb) Int() int {
	switch a {
	case AccessKrb5ReadOnly:
		return AccessKrb5ReadOnlyInt
	case AccessKrb5ReadWrite:
		return AccessKrb5ReadWriteInt
	case AccessKrb5ReadNone:
		return AccessKrb5ReadNoneInt
	default:
		return AccessKrb5UndefinedInt
	}
}

const (
	// AllocTypeKey is the string of AllocType's key
	AllocTypeKey = "alloctype"
	// AllSquashKey is the string of AllSquash's key
	AllSquashKey = "allsquash"
	// AuthClientKey is the string of AuthClient's key
	AuthClientKey = "authclient"
	// BackendKey is the string of Backend's key
	BackendKey = "backend"
	// CapacityKey is the string of Capacity's key
	CapacityKey = "capacity"
	// DescriptionKey is the string of Description's key
	DescriptionKey = "description"
	// MetroDomainIDKey is the string of MetroDomainID's key
	MetroDomainIDKey = "metroDomainID"
	// PvcNameKey is the string of PvcName's key
	PvcNameKey = "name"
	// PoolIDKey is the string of PoolID's key
	PoolIDKey = "poolID"
	// RootSquashKey is the string of RootSquash's key
	RootSquashKey = "rootsquash"
	// StoragePoolKey is the string of StoragePool's key
	StoragePoolKey = "storagepool"
	// ActiveVStoreIDKey is the string of ActiveVStoreID's key
	ActiveVStoreIDKey = "localVStoreID"
	// StandByVStoreIDKey is the string of StandByVStoreID's key
	StandByVStoreIDKey = "remoteVStoreID"
	// CloneFromKey is the string of CloneFrom's key
	CloneFromKey = "clonefrom"
	// CloneSpeedKey is the string of CloneSpeed's key
	CloneSpeedKey = "clonespeed"
	// SourceVolumeNameKey is the string of SourceVolumeName's key
	SourceVolumeNameKey = "sourcevolumename"
	// SourceSnapshotNameKey is the string of SourceSnapshotName's key
	SourceSnapshotNameKey = "sourcesnapshotname"
	// SnapshotParentIdKey is the string of SnapshotParentId's key
	SnapshotParentIdKey = "snapshotparentid"
	// HyperMetroKey is the string of HyperMetro's key
	HyperMetroKey = "hypermetro"
	// MetroPairSyncSpeedKey is the string of MetroPairSyncSpeed's key
	MetroPairSyncSpeedKey = "metropairsyncspeed"
	// RemoteStoragePoolKey is the string of RemoteStoragePool's key
	RemoteStoragePoolKey = "remotestoragepool"
	// RemotePoolIdKey is the string of RemotePoolId's key
	RemotePoolIdKey = "remotePoolID"
	// VStorePairIdKey is the string of VStorePairId's key
	VStorePairIdKey = "vstorepairid"
	// ReplicationKey is the string of Replication's key
	ReplicationKey = "replication"
	// FsPermissionKey is the string of FsPermission's key
	FsPermissionKey = "fspermission"
	// SnapshotFromKey is the string of FromSnapshot's key
	SnapshotFromKey = "fromSnapshot"
	// IsSkipNfsShareAndQoS is the string of SkipNfsShareAndQoS's key
	IsSkipNfsShareAndQoS = "skipNfsShareAndQos"
	// QoSKey is the string of qos's key
	QoSKey = "qos"
	// WorkloadTypeIDKey is the string of WorkloadTypeID's key
	WorkloadTypeIDKey = "workloadTypeID"
	// IsShowSnapDirKey is the string of IsShowSnapDir's key
	IsShowSnapDirKey = "isshowsnapdir"
	// SnapshotReservePerKey is the string of SnapshotReservePer's key
	SnapshotReservePerKey = "reservedsnapshotspaceratio"
	// AccessKrb5Key is the string of AccessKrb5's key
	AccessKrb5Key = "accesskrb5"
	// AccessKrb5iKey is the string of AccessKrb5i's key
	AccessKrb5iKey = "accesskrb5i"
	// AccessKrb5pKey is the string of AccessKrb5p's key
	AccessKrb5pKey = "accesskrb5p"
	// FilesystemModeKey is the string of FilesystemMode's key
	FilesystemModeKey = "filesystemmode"
	// ProductKey is the string of product's key
	ProductKey = "product"
	// ModifyVolumeKey is the string of ModifyVolume's key
	ModifyVolumeKey = "ModifyVolume"
	// SnapshotIDKey is the string of SnapshotID's key
	SnapshotIDKey = "snapshotID"
	// SnapshotParentNameKey is the string of SnapshotParentName's key
	SnapshotParentNameKey = "snapshotParentName"

	// DefaultAllSquash is the default value of all squash
	DefaultAllSquash = 1
	// DefaultRootSquash is the default value of root squash
	DefaultRootSquash = 1
	// DefaultCloneSpeed is the default value of clone speed
	DefaultCloneSpeed = 3
	// DefaultAllocType is the default value of alloc type
	DefaultAllocType = 1
)

// Parameter wraps the parameters from the PVC creation request, make each of parameters can be type-checked,
// and make it is easier to read and use.
type Parameter struct {
	params map[string]any
}

// NewParameter returns a new instance of Parameter.
func NewParameter(params map[string]any) *Parameter { return &Parameter{params: params} }

// PvcName gets the PvcName value of the params map.
func (p *Parameter) PvcName() string { return utils.GetValueOrFallback(p.params, PvcNameKey, "") }

// AllocType gets the AllocType value of the params map.
func (p *Parameter) AllocType() int {
	return utils.GetValueOrFallback(p.params, AllocTypeKey, DefaultAllocType)
}

// AllSquash gets the AllSquash value of the params map.
func (p *Parameter) AllSquash() int {
	return utils.GetValueOrFallback(p.params, AllSquashKey, DefaultAllSquash)
}

// AuthClient gets the AuthClient value of the params map.
func (p *Parameter) AuthClient() string { return utils.GetValueOrFallback(p.params, AuthClientKey, "") }

// Backend gets the Backend value of the params map.
func (p *Parameter) Backend() string { return utils.GetValueOrFallback(p.params, BackendKey, "") }

// Capacity gets the Capacity value of the params map.
func (p *Parameter) Capacity() int64 {
	return utils.GetValueOrFallback(p.params, CapacityKey, int64(0))
}

// Description gets the Description value of the params map.
func (p *Parameter) Description() string {
	return utils.GetValueOrFallback(p.params, DescriptionKey, "")
}

// MetroDomainID gets the MetroDomainID value of the params map.
func (p *Parameter) MetroDomainID() string {
	return utils.GetValueOrFallback(p.params, MetroDomainIDKey, "")
}

// PoolID gets the PoolID value of the params map.
func (p *Parameter) PoolID() string { return utils.GetValueOrFallback(p.params, PoolIDKey, "") }

// RootSquash gets the RootSquash value of the params map.
func (p *Parameter) RootSquash() int {
	return utils.GetValueOrFallback(p.params, RootSquashKey, DefaultRootSquash)
}

// StoragePool gets the StoragePool value of the params map.
func (p *Parameter) StoragePool() string {
	return utils.GetValueOrFallback(p.params, StoragePoolKey, "")
}

// ActiveVStoreID gets the ActiveVStoreID value of the params map.
func (p *Parameter) ActiveVStoreID() string {
	return utils.GetValueOrFallback(p.params, ActiveVStoreIDKey, "")
}

// StandByVStoreID gets the ActiveVStoreID value of the params map.
func (p *Parameter) StandByVStoreID() string {
	return utils.GetValueOrFallback(p.params, StandByVStoreIDKey, "")
}

// CloneFrom gets the CloneFrom value of the params map.
func (p *Parameter) CloneFrom() string { return utils.GetValueOrFallback(p.params, CloneFromKey, "") }

// CloneSpeed gets the CloneSpeed value of the params map.
func (p *Parameter) CloneSpeed() int {
	return utils.GetValueOrFallback(p.params, CloneSpeedKey, DefaultCloneSpeed)
}

// SnapshotParentId gets the SnapshotParentId value of the params map.
func (p *Parameter) SnapshotParentId() string {
	return utils.GetValueOrFallback(p.params, SnapshotParentIdKey, "")
}

// VStorePairId gets the VStorePairId value of the params map.
func (p *Parameter) VStorePairId() string {
	return utils.GetValueOrFallback(p.params, VStorePairIdKey, "")
}

// IsHyperMetro gets the HyperMetro value of the params map.
func (p *Parameter) IsHyperMetro() bool {
	return utils.GetValueOrFallback(p.params, HyperMetroKey, false)
}

// SyncMetroPairSpeed gets the SyncMetroPairSpeed value of the params map.
func (p *Parameter) SyncMetroPairSpeed() int {
	return utils.GetValueOrFallback(p.params, MetroPairSyncSpeedKey, 0)
}

// IsReplication gets the Replication value of the params map.
func (p *Parameter) IsReplication() bool {
	return utils.GetValueOrFallback(p.params, ReplicationKey, false)
}

// FsPermission gets the FsPermission value of the params map.
func (p *Parameter) FsPermission() string {
	return utils.GetValueOrFallback(p.params, FsPermissionKey, "")
}

// SourceVolumeName gets the SourceVolumeName value of the params map.
func (p *Parameter) SourceVolumeName() string {
	return utils.GetValueOrFallback(p.params, SourceVolumeNameKey, "")
}

// SourceSnapshotName gets the SourceSnapshotName value of the params map.
func (p *Parameter) SourceSnapshotName() string {
	return utils.GetValueOrFallback(p.params, SourceSnapshotNameKey, "")
}

// RemoteStoragePool gets the RemoteStoragePool value of the params map.
func (p *Parameter) RemoteStoragePool() string {
	return utils.GetValueOrFallback(p.params, RemoteStoragePoolKey, "")
}

// RemotePoolId gets the RemotePoolID value of the params map.
func (p *Parameter) RemotePoolId() string {
	return utils.GetValueOrFallback(p.params, RemotePoolIdKey, "")
}

// QoS gets the QoS value of the params map.
func (p *Parameter) QoS() map[string]int {
	return utils.GetValueOrFallback[map[string]int](p.params, QoSKey, nil)
}

// WorkloadTypeID gets the WorkloadTypeID value of the params map.
func (p *Parameter) WorkloadTypeID() string {
	return utils.GetValueOrFallback(p.params, WorkloadTypeIDKey, "")
}

// IsShowSnapDir gets the IsShowSnapDir value of the params map.
func (p *Parameter) IsShowSnapDir() (bool, bool) {
	return utils.GetValue[bool](p.params, IsShowSnapDirKey)
}

// SnapshotReservePer gets the SnapshotReservePer value of the params map.
func (p *Parameter) SnapshotReservePer() (int, bool) {
	return utils.GetValue[int](p.params, SnapshotReservePerKey)
}

// AccessKrb5 gets the AccessKrb5 value of the params map.
func (p *Parameter) AccessKrb5() int {
	val := AccessKrb(utils.GetValueOrFallback(p.params, AccessKrb5Key, ""))
	return val.Int()
}

// AccessKrb5i gets the AccessKrb5i value of the params map.
func (p *Parameter) AccessKrb5i() int {
	val := AccessKrb(utils.GetValueOrFallback(p.params, AccessKrb5iKey, ""))
	return val.Int()
}

// AccessKrb5p gets the AccessKrb5p value of the params map.
func (p *Parameter) AccessKrb5p() int {
	val := AccessKrb(utils.GetValueOrFallback(p.params, AccessKrb5pKey, ""))
	return val.Int()
}

// FilesystemMode gets the FilesystemMode value of the params map.
func (p *Parameter) FilesystemMode() string {
	return utils.GetValueOrFallback(p.params, FilesystemModeKey, "")
}

// Product gets the Product value of the params map.
func (p *Parameter) Product() constants.OceanstorVersion {
	return constants.OceanstorVersion(utils.GetValueOrFallback(p.params, ProductKey, ""))
}

// IsModifyVolume gets the ModifyVolume value of the params map.
func (p *Parameter) IsModifyVolume() bool {
	return utils.GetValueOrFallback(p.params, ModifyVolumeKey, false)
}

// SnapshotID gets the SnapshotID value of the params map.
func (p *Parameter) SnapshotID() string { return utils.GetValueOrFallback(p.params, SnapshotIDKey, "") }

// SnapshotParentName gets the SnapshotParentName value of the params map.
func (p *Parameter) SnapshotParentName() string {
	return utils.GetValueOrFallback(p.params, SnapshotParentNameKey, "")
}

// AdvancedOptions gets the AdvancedOptions value of the params map.
func (p *Parameter) AdvancedOptions() string {
	return utils.GetValueOrFallback(p.params, constants.AdvancedOptionsKey, "")
}

// IsClone returns true if a clone filesystem needs to be created, or returns false.
func (p *Parameter) IsClone() bool {
	_, exists := p.params[CloneFromKey]
	return exists
}

// IsSnapshot returns true if a snapshot filesystem needs to be created, or returns false.
func (p *Parameter) IsSnapshot() bool {
	_, exists := p.params[SnapshotFromKey]
	return exists
}

// IsSkipNfsShareAndQos returns true if the filesystem didn't need to create nfs share and QoS, or return false.
func (p *Parameter) IsSkipNfsShareAndQos() bool {
	return utils.GetValueOrFallback(p.params, IsSkipNfsShareAndQoS, false)
}

// SetIsSkipNfsShare sets the value of isSkipNfsShare
func (p *Parameter) SetIsSkipNfsShare(isSkip bool) {
	p.params[IsSkipNfsShareAndQoS] = isSkip
}

// SetQos sets the value of qos
func (p *Parameter) SetQos(qos map[string]int) {
	p.params[QoSKey] = qos
}
