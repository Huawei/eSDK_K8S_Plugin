/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2025. All rights reserved.
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

package backend

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"runtime"
	"strconv"
	"strings"

	v1 "github.com/Huawei/eSDK_K8S_Plugin/v4/client/apis/xuanwu/v1"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/cache"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/model"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/plugin"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	pkgUtils "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
	fsUtils "github.com/Huawei/eSDK_K8S_Plugin/v4/storage/fusionstorage/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/k8sutils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	// Topology constant for topology filter function
	Topology = "topology"
	// supported topology key in CSI plugin configuration
	supportedTopologiesKey = "supportedTopologies"
	// NoAvailablePool message of no available poll error
	NoAvailablePool = "no storage pool meets the requirements"
)

var (
	// ValidateFilterFuncs validate filters' function map
	ValidateFilterFuncs = [][]interface{}{
		{"backend", validateBackendName},
		{"volumeType", validateVolumeType},
	}

	// PrimaryFilterFuncs primary filters' function map
	PrimaryFilterFuncs = [][]interface{}{
		{"backend", filterByBackendName},
		{"pool", filterByStoragePool},
		{"volumeType", filterByVolumeType},
		{"allocType", filterByAllocType},
		{"qos", filterByQos},
		{"hyperMetro", filterByMetro},
		{"replication", filterByReplication},
		{"applicationType", filterByApplicationType},
		{"storageQuota", filterByStorageQuota},
		{"sourceVolumeName", filterBySupportClone},
		{"sourceSnapshotName", filterBySupportClone},
		{"nfsProtocol", filterByNFSProtocol},
	}

	// SecondaryFilterFuncs secondary filters' function map
	SecondaryFilterFuncs = [][]interface{}{
		{"volumeType", filterByVolumeType},
		{"allocType", filterByAllocType},
		{"qos", filterByQos},
		{"replication", filterByReplication},
		{"applicationType", filterByApplicationType},
	}
)

// AccessibleTopology represents selected node topology
type AccessibleTopology struct {
	RequisiteTopologies []map[string]string
	PreferredTopologies []map[string]string
}

// CSIConfig holds the CSI config of backend resources
type CSIConfig struct {
	Backends map[string]interface{} `json:"backends"`
}

func analyzePools(backend *model.Backend, config map[string]interface{}) error {
	var pools []*model.StoragePool

	if backend.Storage == constants.OceanStorDtree || backend.Storage == constants.FusionDTree {
		pools = append(pools, &model.StoragePool{
			Storage:      backend.Storage,
			Name:         backend.Name,
			Parent:       backend.Name,
			Plugin:       backend.Plugin,
			Capabilities: make(map[string]bool),
			Capacities:   map[string]string{},
		})
	}

	configPools, _ := config["pools"].([]interface{})
	for _, i := range configPools {
		name, ok := i.(string)
		if !ok || name == "" {
			continue
		}

		pool := &model.StoragePool{
			Storage:      backend.Storage,
			Name:         name,
			Parent:       backend.Name,
			Plugin:       backend.Plugin,
			Capabilities: make(map[string]bool),
			Capacities:   map[string]string{},
		}

		pools = append(pools, pool)
	}

	if len(pools) == 0 {
		return fmt.Errorf("no valid pools configured for backend %s", backend.Name)
	}

	backend.Pools = pools
	return nil
}

// BuildBackend build a valid backend
func BuildBackend(ctx context.Context, content v1.StorageBackendContent) (*model.Backend, error) {
	if content.Spec.BackendClaim == "" || content.Spec.ConfigmapMeta == "" ||
		content.Spec.SecretMeta == "" {
		return nil, pkgUtils.Errorf(ctx, "valid tuple failed, tuple: %+v", content)
	}

	ns, name, err := pkgUtils.SplitMetaNamespaceKey(content.Spec.BackendClaim)
	if err != nil {
		return nil, err
	}

	config, err := GetStorageBackendInfo(ctx,
		pkgUtils.MakeMetaWithNamespace(ns, name),
		NewGetBackendInfoArgsFromContent(&content))
	if err != nil {
		return nil, err
	}

	bk, err := NewBackend(name, config)
	if err != nil {
		return nil, err
	}

	err = analyzePools(bk, config)
	if err != nil {
		return nil, err
	}

	err = addProtocolTopology(bk, app.GetGlobalConfig().DriverName)
	if err != nil {
		return nil, err
	}

	err = bk.Plugin.Init(ctx, config, bk.Parameters, true)
	if err != nil {
		return nil, err
	}

	return bk, nil
}

// NewBackend constructs an object of Kubernetes backend resource
func NewBackend(backendName string, config map[string]interface{}) (*model.Backend, error) {
	// Verifying Common Parameters:
	// - storage:
	//     oceanstor-san;
	//     oceanstor-nas;
	//     oceanstor-dtree;
	//     fusionstorage-san;
	//     fusionstorage-nas;
	//     fusionstorage-dtree;
	//     oceandisk-san;
	// - parameters: must exist
	// - supportedTopologies: must valid
	// - hypermetro must valid
	storage, exist := config["storage"].(string)
	if !exist {
		return nil, errors.New("storage type must be configured for backend")
	}

	targetPlugin := plugin.GetPlugin(storage)
	if targetPlugin == nil {
		return nil, fmt.Errorf("cannot get plugin for storage: [%s]", storage)
	}

	parameters, exist := config["parameters"].(map[string]interface{})
	if !exist {
		return nil, errors.New("parameters must be configured for backend")
	}

	// Get supported topologies for backend
	supportedTopologies, err := getSupportedTopologies(config)
	if err != nil {
		return nil, err
	}

	metroDomain, _ := config["hyperMetroDomain"].(string)
	metrovStorePairID, _ := config["metrovStorePairID"].(string)
	replicaBackend, _ := config["replicaBackend"].(string)
	metroBackend, _ := config["metroBackend"].(string)
	accountName, _ := config["accountName"].(string)
	contentName, _ := config["contentName"].(string)

	// while config hyperMetro, the metroBackend must config, hyperMetroDomain or metrovStorePairID should be config
	if ((metroDomain != "" || metrovStorePairID != "") && metroBackend == "") ||
		((metroDomain == "" && metrovStorePairID == "") && metroBackend != "") {
		return nil, fmt.Errorf("hyperMetro configuration in backend %s is incorrect", backendName)
	}

	return &model.Backend{
		Name:                backendName,
		ContentName:         contentName,
		Storage:             storage,
		Available:           false,
		SupportedTopologies: supportedTopologies,
		Plugin:              targetPlugin,
		Parameters:          parameters,
		MetroDomain:         metroDomain,
		MetrovStorePairID:   metrovStorePairID,
		ReplicaBackendName:  replicaBackend,
		MetroBackendName:    metroBackend,
		AccountName:         accountName,
	}, nil
}

func getSupportedTopologies(config map[string]interface{}) ([]map[string]string, error) {
	supportedTopologies := make([]map[string]string, 0)

	topologies, exist := config[supportedTopologiesKey]
	if !exist {
		return supportedTopologies, nil
	}

	// populate configured topologies
	topologyArray, ok := topologies.([]interface{})
	if !ok {
		return nil, fmt.Errorf("configured supported topologies [%v] for backend is not list", topologies)
	}
	for _, topologyArrElem := range topologyArray {
		topologyMap, ok := topologyArrElem.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("configured supported topology [%v] for backend is not dictionary", topologyMap)
		}
		tempMap := make(map[string]string, 0)
		for topologyKey, value := range topologyMap {
			if topologyValue, ok := value.(string); ok {
				tempMap[topologyKey] = topologyValue
			}
		}
		supportedTopologies = append(supportedTopologies, tempMap)
	}

	return supportedTopologies, nil
}

// addProtocolTopology add up protocol specific topological support
// Note: Protocol is considered as special topological parameter.
// The protocol topology is populated internally by plugin using protocol name.
// If configured protocol for backend is "iscsi", topology.kubernetes.io/protocol.iscsi "=" csi.huawei.com
// will be added to supportedTopologies by CSI plugin internally.
//
// Now users can opt to provision volumes based on protocol by
// 1. Labeling kubernetes nodes with protocol specific label (ie topology.kubernetes.io/protocol.iscsi = csi.huawei.com)
// 2. Configure topology support in plugin
// 3. Configure protocol topology in allowedTopologies fo Storage class
// addProtocolTopology is called after backend plugin init as init takes care of protocol validation
func addProtocolTopology(backend *model.Backend, driverName string) error {
	proto, protocolAvailable := backend.Parameters["protocol"]
	protocol, isString := proto.(string)
	if !protocolAvailable || !isString {
		return errors.New("supported topology for protocol may not work as protocol is miss configured " +
			"in backend configuration")
	}

	protocolTopologyKey := k8sutils.ProtocolTopologyPrefix + protocol

	// add combination of protocol support
	if len(backend.SupportedTopologies) > 0 {
		protocolTopologyCombination := make([]map[string]string, 0)

		for _, supportedTopology := range backend.SupportedTopologies {
			copyofProtocolTopology := make(map[string]string, 0)
			for key, value := range supportedTopology {
				copyofProtocolTopology[key] = value
			}
			copyofProtocolTopology[protocolTopologyKey] = driverName
			protocolTopologyCombination = append(protocolTopologyCombination, copyofProtocolTopology)
		}
		backend.SupportedTopologies = append(backend.SupportedTopologies, protocolTopologyCombination...)
	}

	// add support for protocol topology only
	backend.SupportedTopologies = append(backend.SupportedTopologies, map[string]string{
		protocolTopologyKey: driverName,
	})

	return nil
}

// GetMetroDomain get metro domain of backend
func GetMetroDomain(backendName string) string {
	bk, exists := cache.BackendCacheProvider.Load(backendName)
	if !exists {
		return ""
	}
	return bk.MetroDomain
}

// GetMetrovStorePairID get MetrovStorePairID of backend
func GetMetrovStorePairID(backendName string) string {
	bk, exists := cache.BackendCacheProvider.Load(backendName)
	if !exists {
		return ""
	}
	return bk.MetrovStorePairID
}

// GetAccountName get account name of backend
func GetAccountName(backendName string) string {
	bk, exists := cache.BackendCacheProvider.Load(backendName)
	if !exists {
		return ""
	}
	return bk.AccountName
}

// FilterStoragePool filter storage pool by capability, topology and capacity.
func FilterStoragePool(ctx context.Context, requestSize int64, parameters map[string]interface{},
	candidatePools []*model.StoragePool, filterFuncs [][]interface{}) ([]*model.StoragePool, error) {
	// filter the storage pools by capability
	filterPools, err := FilterByCapability(ctx, parameters, candidatePools, filterFuncs)
	if err != nil {
		return nil, fmt.Errorf("failed to select pool, the capability filter failed, error: %v."+
			" please check your storage class", err)
	}

	// filter the storage by topology
	filterPools, err = FilterByTopology(parameters, filterPools)
	if err != nil {
		return nil, err
	}

	allocType, _ := parameters["allocType"].(string)
	// filter the storage pool by capacity
	filterPools = FilterByCapacity(requestSize, allocType, filterPools)
	if len(filterPools) == 0 {
		return nil, fmt.Errorf("failed to select pool, the capacity filter failed, capacity: %d", requestSize)
	}

	return filterPools, nil
}

// SelectRemotePool select the optimal remote storage pool based on the free capacity.
func SelectRemotePool(ctx context.Context, requestSize int64, parameters map[string]interface{},
	localBackendName string) (*model.StoragePool, error) {
	hyperMetro, hyperMetroOK := parameters["hyperMetro"].(string)
	replication, replicationOK := parameters["replication"].(string)

	if hyperMetroOK && utils.StrToBool(ctx, hyperMetro) &&
		replicationOK && utils.StrToBool(ctx, replication) {
		return nil, fmt.Errorf("cannot create volume with hyperMetro and replication properties: %v", parameters)
	}

	var remotePool *model.StoragePool
	var remotePools []*model.StoragePool
	var err error

	if hyperMetroOK && utils.StrToBool(ctx, hyperMetro) {
		localBackend, exists := cache.BackendCacheProvider.Load(localBackendName)
		if !exists || localBackend.MetroBackend == nil {
			return nil, fmt.Errorf("no metro backend exists for volume: %v, local backend: %s", parameters,
				localBackendName)
		}

		remotePools, err = FilterStoragePool(ctx, requestSize, parameters, localBackend.MetroBackend.Pools,
			SecondaryFilterFuncs)
	}

	if replicationOK && utils.StrToBool(ctx, replication) {
		localBackend, exists := cache.BackendCacheProvider.Load(localBackendName)
		if !exists || localBackend.ReplicaBackend == nil {
			return nil, fmt.Errorf("no replica backend exists for volume: %v, local backend: %s", parameters,
				localBackendName)
		}

		remotePools, err = FilterStoragePool(ctx, requestSize, parameters, localBackend.ReplicaBackend.Pools,
			SecondaryFilterFuncs)
	}

	if err != nil {
		return nil, fmt.Errorf("select remote pool failed: %v", err)
	}

	if len(remotePools) == 0 {
		return nil, nil
	}
	// weight the remote pool
	remotePool, err = WeightSinglePools(ctx, requestSize, parameters, remotePools)
	return remotePool, err
}

// WeightSinglePools select the optimal storage pool based on the free capacity.
func WeightSinglePools(
	ctx context.Context,
	requestSize int64,
	parameters map[string]interface{},
	filterPools []*model.StoragePool) (*model.StoragePool, error) {
	// weight the storage pool by free capacity
	var selectPool *model.StoragePool
	selectPool = weightByFreeCapacity(filterPools)
	if selectPool == nil {
		return nil, fmt.Errorf("cannot select a storage pool for volume (%d, %v)", requestSize, parameters)
	}

	log.AddContext(ctx).Infof("Select storage pool %s:%s for volume (%d, %v)",
		selectPool.Parent, selectPool.Name, requestSize, parameters)
	return selectPool, nil
}

// WeightPools select the optimal local and remote storage pool based on the free capacity.
func WeightPools(ctx context.Context, requestSize int64, parameters map[string]interface{},
	localPools []*model.StoragePool, poolPairs []model.SelectPoolPair) (*model.StoragePool, *model.StoragePool, error) {
	localPool, err := WeightSinglePools(ctx, requestSize, parameters, localPools)
	if err != nil {
		return nil, nil, err
	}

	for _, pair := range poolPairs {
		if pair.Local == localPool {
			updateSelectPool(requestSize, parameters, pair.Local)
			updateSelectPool(requestSize, parameters, pair.Remote)
			return pair.Local, pair.Remote, nil
		}
	}
	return nil, nil, errors.New("weight pool failed")
}

func updateSelectPool(requestSize int64, parameters map[string]interface{}, selectPool *model.StoragePool) {
	if selectPool == nil {
		return
	}

	allocType, _ := parameters["allocType"].(string)
	// when the allocType is thin, do not change the FreeCapacity.
	if allocType == "thick" {
		freeCapacity := utils.ParseIntWithDefault(selectPool.Capacities["FreeCapacity"], 10, 64, 0)
		selectPool.Capacities["FreeCapacity"] = strconv.FormatInt(freeCapacity-requestSize, 10)
	}
}

func filterByBackendName(ctx context.Context, backendName string, candidatePools []*model.StoragePool) (
	[]*model.StoragePool, error) {
	var filterPools []*model.StoragePool

	for _, pool := range candidatePools {
		if backendName == "" || backendName == pool.Parent {
			filterPools = append(filterPools, pool)
		}
	}

	return filterPools, nil
}

func filterByStoragePool(ctx context.Context, poolName string, candidatePools []*model.StoragePool) (
	[]*model.StoragePool, error) {
	var filterPools []*model.StoragePool

	for _, pool := range candidatePools {
		if poolName == "" || poolName == pool.Name {
			filterPools = append(filterPools, pool)
		}
	}

	return filterPools, nil
}

func filterByVolumeType(ctx context.Context, volumeType string, candidatePools []*model.StoragePool) (
	[]*model.StoragePool, error) {
	var filterPools []*model.StoragePool

	for _, pool := range candidatePools {
		if volumeType == "lun" || volumeType == "" {
			if pool.Storage == constants.OceanStorSan || pool.Storage == constants.FusionSan ||
				pool.Storage == constants.OceandiskSan {
				filterPools = append(filterPools, pool)
			}
		} else if volumeType == "fs" {
			if pool.Storage == constants.OceanStorNas || pool.Storage == constants.OceanStor9000 ||
				pool.Storage == constants.FusionNas {
				filterPools = append(filterPools, pool)
			}
		} else if volumeType == "dtree" {
			if pool.Storage == constants.OceanStorDtree || pool.Storage == constants.FusionDTree {
				filterPools = append(filterPools, pool)
			}
		}
	}

	return filterPools, nil
}

func filterByAllocType(ctx context.Context, allocType string, candidatePools []*model.StoragePool) (
	[]*model.StoragePool, error) {
	var filterPools []*model.StoragePool

	for _, pool := range candidatePools {
		valid := false

		if pool.Storage == constants.OceanStor9000 {
			valid = true
		} else if allocType == "thin" || allocType == "" {
			supportThin, exist := pool.Capabilities["SupportThin"]
			if !exist {
				log.AddContext(ctx).Warningf("convert supportThin to bool failed, data: %v",
					pool.Capabilities["SupportThin"])
			}
			valid = exist && supportThin
		} else if allocType == "thick" {
			supportThick, exist := pool.Capabilities["SupportThick"]
			if !exist {
				log.AddContext(ctx).Warningf("convert supportThick to bool failed, data: %v",
					pool.Capabilities["SupportThick"])
			}
			valid = exist && supportThick
		}

		if valid {
			filterPools = append(filterPools, pool)
		}
	}

	return filterPools, nil
}

func filterByQos(ctx context.Context, qos string, candidatePools []*model.StoragePool) ([]*model.StoragePool, error) {
	var filterPools []*model.StoragePool

	if qos == "" {
		return candidatePools, nil
	}

	var poolSelectionErrors []error
	for _, pool := range candidatePools {
		supportQoS, exist := pool.Capabilities["SupportQoS"]
		if exist && supportQoS {
			err := pool.Plugin.SupportQoSParameters(ctx, qos)
			if err != nil {
				poolSelectionErrors = append(poolSelectionErrors,
					fmt.Errorf("%s:%s", pool.Parent, err))
				continue
			}

			filterPools = append(filterPools, pool)
		}
	}

	if len(filterPools) == 0 {
		err := errors.New("failed to select pool with QoS parameters")
		for _, poolSelectionError := range poolSelectionErrors {
			err = fmt.Errorf("%s %s", err, poolSelectionError)
		}
		return filterPools, err
	}

	return filterPools, nil
}

func filterByMetro(ctx context.Context, hyperMetro string, candidatePools []*model.StoragePool) (
	[]*model.StoragePool, error) {
	if len(hyperMetro) == 0 || !utils.StrToBool(ctx, hyperMetro) {
		return candidatePools, nil
	}

	var filterPools []*model.StoragePool

	for _, pool := range candidatePools {
		backend, exists := cache.BackendCacheProvider.Load(pool.Parent)
		if !exists {
			continue
		}
		if backend.MetroBackend == nil {
			continue
		}

		if supportMetro, exist := pool.Capabilities["SupportMetro"]; exist && supportMetro {
			filterPools = append(filterPools, pool)
		}
	}

	return filterPools, nil
}

func filterByReplication(ctx context.Context, replication string, candidatePools []*model.StoragePool) (
	[]*model.StoragePool, error) {
	if len(replication) == 0 || !utils.StrToBool(ctx, replication) {
		return candidatePools, nil
	}

	var filterPools []*model.StoragePool

	for _, pool := range candidatePools {
		backend, exists := cache.BackendCacheProvider.Load(pool.Parent)
		if !exists || backend.ReplicaBackend == nil {
			continue
		}

		if SupportReplication, exist := pool.Capabilities["SupportReplication"]; exist && SupportReplication {
			filterPools = append(filterPools, pool)
		}
	}

	return filterPools, nil
}

// FilterByTopology returns a subset of the provided pools that can support any of the topology requirement.
func FilterByTopology(parameters map[string]interface{}, candidatePools []*model.StoragePool) ([]*model.StoragePool,
	error) {
	iTopology, topologyAvailable := parameters[Topology]
	if !topologyAvailable {
		// ignore topology filter
		return candidatePools, nil
	}

	topology, ok := iTopology.(AccessibleTopology)
	if !ok {
		return nil, errors.New("AccessibleTopology type is expected in topology parameters")
	}

	if len(topology.RequisiteTopologies) == 0 {
		return candidatePools, nil
	}

	filterPools := filterPoolsOnTopology(candidatePools, topology.RequisiteTopologies)
	if len(filterPools) == 0 {
		// filter out candidate pools info
		logCandidatePool := make([]string, 0)
		for _, pool := range candidatePools {
			logCandidatePool = append(logCandidatePool, pool.Parent+":"+pool.Name)
		}
		return nil, fmt.Errorf("no pool support by requisite topologies [%v] from candidate pools [%v]",
			topology.RequisiteTopologies, logCandidatePool)
	}
	return sortPoolsByPreferredTopologies(filterPools, topology.PreferredTopologies), nil
}

// isTopologySupportedByBackend returns whether the specific backend can create volumes accessible by the given topology
func isTopologySupportedByBackend(backend *model.Backend, topology map[string]string) bool {
	requisiteFound := false

	// extract protocol
	protocolTopology := make(map[string]string, 0)
	topology = extractProtocolTopology(topology, protocolTopology)

	// check for each topology key in backend supported topologies except protocol
	// The check is an "and" operation on each topology key and value
	for _, supported := range backend.SupportedTopologies {
		eachFound := true

		if len(protocolTopology) != 0 {
			// check for protocol support
			found := checkProtocolSupport(supported, protocolTopology)
			if !found {
				continue // if not found check next supported topology
			}
		}

		for k, v := range topology {
			if sup, ok := supported[k]; !ok || (sup != v) {
				eachFound = false
				break
			}
		}
		if eachFound {
			requisiteFound = true
			break
		}
	}

	return requisiteFound
}

func extractProtocolTopology(topology, protocolTopology map[string]string) map[string]string {
	remainingTopology := make(map[string]string, 0)

	for key, value := range topology {
		if strings.HasPrefix(key, k8sutils.ProtocolTopologyPrefix) {
			protocolTopology[key] = value
			continue
		}
		remainingTopology[key] = value
	}

	return remainingTopology
}

func checkProtocolSupport(supportedTopology, protocols map[string]string) bool {
	for key, value := range supportedTopology {
		if strings.HasPrefix(key, k8sutils.ProtocolTopologyPrefix) {
			if v, ok := protocols[key]; ok && value == v {
				return true
			}
		}
	}
	return false
}

// filterPoolsOnTopology returns a subset of the provided pools that can support any of the requisiteTopologies.
func filterPoolsOnTopology(candidatePools []*model.StoragePool,
	requisiteTopologies []map[string]string) []*model.StoragePool {
	filteredPools := make([]*model.StoragePool, 0)

	if len(requisiteTopologies) == 0 {
		return candidatePools
	}

	for _, pool := range candidatePools {
		// mutex lock acquired in pool selection
		backend, exists := cache.BackendCacheProvider.Load(pool.Parent)
		if !exists {
			continue
		}

		// when backend is not configured with supported topology
		if len(backend.SupportedTopologies) == 0 {
			filteredPools = append(filteredPools, pool)
			continue
		}

		for _, topology := range requisiteTopologies {
			if isTopologySupportedByBackend(&backend, topology) {
				filteredPools = append(filteredPools, pool)
				break
			}
		}
	}

	return filteredPools
}

// sortPoolsByPreferredTopologies returns a list of pools ordered by the pools supportedTopologies field against
// the provided list of preferredTopologies. If 2 or more pools can support a given preferredTopology, they are shuffled
// randomly within that segment of the list, in order to prevent hotspots.
func sortPoolsByPreferredTopologies(candidatePools []*model.StoragePool,
	preferredTopologies []map[string]string) []*model.StoragePool {
	remainingPools := make([]*model.StoragePool, len(candidatePools))
	copy(remainingPools, candidatePools)
	orderedPools := make([]*model.StoragePool, 0)

	for _, preferred := range preferredTopologies {
		newRemainingPools := make([]*model.StoragePool, 0)
		poolBucket := make([]*model.StoragePool, 0)

		for _, pool := range remainingPools {
			backend, exists := cache.BackendCacheProvider.Load(pool.Parent)
			if !exists {
				continue
			}
			// If it supports topology, pop it and add to bucket. Otherwise, add it to newRemaining pools to be
			// addressed in future loop iterations.
			if isTopologySupportedByBackend(&backend, preferred) {
				poolBucket = append(poolBucket, pool)
			} else {
				newRemainingPools = append(newRemainingPools, pool)
			}
		}

		// make new list of remaining pools
		remainingPools = make([]*model.StoragePool, len(newRemainingPools))
		copy(remainingPools, newRemainingPools)

		// shuffle bucket
		rand.Shuffle(len(poolBucket), func(i, j int) {
			poolBucket[i], poolBucket[j] = poolBucket[j], poolBucket[i]
		})

		// add all in bucket to final list
		orderedPools = append(orderedPools, poolBucket...)
	}

	// shuffle and add leftover pools the did not match any preference
	rand.Shuffle(len(remainingPools), func(i, j int) {
		remainingPools[i], remainingPools[j] = remainingPools[j], remainingPools[i]
	})
	return append(orderedPools, remainingPools...)
}

// FilterByCapability filter backend by capability
func FilterByCapability(ctx context.Context, parameters map[string]interface{}, candidatePools []*model.StoragePool,
	filterFuncs [][]interface{}) ([]*model.StoragePool, error) {

	var err error
	for _, i := range filterFuncs {
		key, filter := i[0].(string), i[1].(func(context.Context, string, []*model.StoragePool) ([]*model.StoragePool,
			error))
		value, _ := parameters[key].(string)
		candidatePools, err = filter(ctx, value, candidatePools)
		if err != nil {
			msg := fmt.Sprintf("Filter pool by capability failed, filter field: [%s], fileter function: [%s], "+
				"paramters: [%v], error: [%v].",
				value, runtime.FuncForPC(reflect.ValueOf(filter).Pointer()).Name(), parameters, err)
			return nil, errors.New(msg)
		}
		if len(candidatePools) == 0 {
			msg := fmt.Sprintf("%s. Please check the storage class. The final filter field: %s, "+
				"filter function: %s, parameters %v.", NoAvailablePool, value,
				runtime.FuncForPC(reflect.ValueOf(filter).Pointer()).Name(), parameters)
			return nil, errors.New(msg)
		}
	}

	return candidatePools, nil
}

func filterByNFSProtocol(ctx context.Context, nfsProtocol string, candidatePools []*model.StoragePool) (
	[]*model.StoragePool, error) {
	if nfsProtocol == "" {
		return candidatePools, nil
	}

	var filterPools []*model.StoragePool
	for _, pool := range candidatePools {
		if nfsProtocol == "nfs3" && pool.Capabilities["SupportNFS3"] {
			filterPools = append(filterPools, pool)
		} else if nfsProtocol == "nfs4" && pool.Capabilities["SupportNFS4"] {
			filterPools = append(filterPools, pool)
		} else if nfsProtocol == "nfs41" && pool.Capabilities["SupportNFS41"] {
			filterPools = append(filterPools, pool)
		} else if nfsProtocol == "nfs42" && pool.Capabilities["SupportNFS42"] {
			filterPools = append(filterPools, pool)
		}
	}

	return filterPools, nil
}

func filterBySupportClone(ctx context.Context, cloneSource string, candidatePools []*model.StoragePool) (
	[]*model.StoragePool, error) {
	if cloneSource == "" {
		return candidatePools, nil
	}
	var filterPools []*model.StoragePool
	for _, pool := range candidatePools {
		if pool.Capabilities["SupportClone"] {
			filterPools = append(filterPools, pool)
		}
	}
	return filterPools, nil
}

// FilterByCapacity filter backend by capacity
func FilterByCapacity(requestSize int64, allocType string, candidatePools []*model.StoragePool) []*model.StoragePool {
	var filterPools []*model.StoragePool
	for _, pool := range candidatePools {
		supportThin, thinExist := pool.Capabilities["SupportThin"]
		if !thinExist {
			log.Warningf("convert supportThin to bool failed, data: %v", pool.Capabilities["SupportThin"])
		}
		supportThick, thickExist := pool.Capabilities["SupportThick"]
		if !thickExist {
			log.Warningf("convert supportThick to bool failed, data: %v", pool.Capabilities["SupportThick"])
		}
		if (allocType == "thin" || allocType == "") && thinExist && supportThin {
			filterPools = append(filterPools, pool)
		} else if allocType == "thick" && thickExist && supportThick {
			freeCapacity := utils.ParseIntWithDefault(pool.GetCapacities()["FreeCapacity"], 10, 64, 0)
			if requestSize <= freeCapacity {
				filterPools = append(filterPools, pool)
			}
		}
	}

	return filterPools
}

func weightByFreeCapacity(candidatePools []*model.StoragePool) *model.StoragePool {
	var selectPool *model.StoragePool

	for _, pool := range candidatePools {
		if selectPool == nil {
			selectPool = pool
		} else {
			selectCapacity := utils.ParseIntWithDefault(selectPool.GetCapacities()["FreeCapacity"], 10, 64, 0)
			curFreeCapacity := utils.ParseIntWithDefault(pool.GetCapacities()["FreeCapacity"], 10, 64, 0)
			if selectCapacity < curFreeCapacity {
				selectPool = pool
			}
		}
	}
	return selectPool
}

func filterByApplicationType(ctx context.Context, appType string, candidatePools []*model.StoragePool) (
	[]*model.StoragePool, error) {
	var filterPools []*model.StoragePool
	for _, pool := range candidatePools {
		if appType != "" {
			supportAppType, ok := pool.Capabilities["SupportApplicationType"]
			if ok && supportAppType {
				filterPools = append(filterPools, pool)
			}
		} else {
			filterPools = append(filterPools, pool)
		}
	}
	return filterPools, nil
}

func filterByStorageQuota(ctx context.Context, storageQuota string, candidatePools []*model.StoragePool) (
	[]*model.StoragePool, error) {
	var filterPools []*model.StoragePool
	if storageQuota == "" {
		return candidatePools, nil
	}

	for _, pool := range candidatePools {
		supportStorageQuota, ok := pool.Capabilities["SupportQuota"]
		if ok && supportStorageQuota {
			err := fsUtils.IsStorageQuotaAvailable(ctx, storageQuota)
			if err != nil {
				return nil, err
			}
			filterPools = append(filterPools, pool)
		}
	}

	return filterPools, nil
}

// ValidateBackend valid the backend basic info, such as: volumeType(authClient if nfs)
var ValidateBackend = func(ctx context.Context, selectBackend *model.Backend, parameters map[string]interface{}) error {
	for _, i := range ValidateFilterFuncs {
		key, validator := i[0].(string), i[1].(func(context.Context, string, *model.Backend) error)
		value, _ := parameters[key].(string)
		if err := validator(ctx, value, selectBackend); err != nil {
			return fmt.Errorf("validate backend %s error for manage Volume. "+
				"the final validator field: %s, validator function: %s, parameters %v. Reason: %v",
				selectBackend.Name, key, runtime.FuncForPC(reflect.ValueOf(validator).Pointer()).Name(), parameters,
				err)
		}
	}

	return nil
}

func validateBackendName(ctx context.Context, backendName string, selectBackend *model.Backend) error {
	if backendName != "" && selectBackend.Name != backendName {
		return utils.Errorf(ctx, "the backend name between StorageClass(%s) and PVC annotation(%s) "+
			"is different", backendName, selectBackend.Name)
	}

	return nil
}

func validateVolumeType(ctx context.Context, volumeType string, selectBackend *model.Backend) error {
	if filterPools, err := filterByVolumeType(ctx, volumeType, selectBackend.Pools); len(filterPools) == 0 {
		if err != nil {
			return err
		}
		return utils.Errorf(ctx, "the volumeType between StorageClass(%s) and PVC annotation(%s) "+
			"is different, err: filterPools is empty", volumeType, selectBackend.Name)
	}
	return nil
}

// RemoveOneBackend remove a storage backend from plugin
func RemoveOneBackend(ctx context.Context, storageBackendId string) {
	cache.BackendCacheProvider.Delete(ctx, storageBackendId)
	log.AddContext(ctx).Infof("storageBackends: Successful remove backend %s.",
		storageBackendId)
}
