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

package backend

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"runtime"
	"strings"
	"sync"

	xuanwuv1 "huawei-csi-driver/client/apis/xuanwu/v1"
	"huawei-csi-driver/csi/app"
	"huawei-csi-driver/csi/backend/plugin"
	"huawei-csi-driver/pkg/finalizers"
	pkgUtils "huawei-csi-driver/pkg/utils"
	fsUtils "huawei-csi-driver/storage/fusionstorage/utils"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/k8sutils"
	"huawei-csi-driver/utils/log"
)

const (
	// Topology constant for topology filter function
	Topology = "topology"
	// supported topology key in CSI plugin configuration
	supportedTopologiesKey = "supportedTopologies"

	NoAvailablePool = "no storage pool meets the requirements"
)

var (
	mutex       sync.Mutex
	csiBackends = make(map[string]*Backend)

	validateFilterFuncs = [][]interface{}{
		{"backend", validateBackendName},
		{"volumeType", validateVolumeType},
	}

	primaryFilterFuncs = [][]interface{}{
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

	secondaryFilterFuncs = [][]interface{}{
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

type StoragePool struct {
	Name         string
	Storage      string
	Parent       string
	Capabilities map[string]interface{}
	Plugin       plugin.Plugin
}

type Backend struct {
	Name                string
	Storage             string
	Available           bool
	Plugin              plugin.Plugin
	Pools               []*StoragePool
	Parameters          map[string]interface{}
	SupportedTopologies []map[string]string
	AccountName         string

	MetroDomain       string
	MetrovStorePairID string
	MetroBackendName  string
	MetroBackend      *Backend

	ReplicaBackendName string
	ReplicaBackend     *Backend
}

type SelectPoolPair struct {
	local  *StoragePool
	remote *StoragePool
}

type CSIConfig struct {
	Backends map[string]interface{} `json:"backends"`
}

func analyzePools(backend *Backend, config map[string]interface{}) error {
	var pools []*StoragePool

	if backend.Storage == plugin.DTreeStorage {
		pools = append(pools, &StoragePool{
			Storage:      backend.Storage,
			Name:         backend.Name,
			Parent:       backend.Name,
			Plugin:       backend.Plugin,
			Capabilities: make(map[string]interface{}),
		})
	}

	configPools, _ := config["pools"].([]interface{})
	for _, i := range configPools {
		name, ok := i.(string)
		if !ok || name == "" {
			continue
		}

		pool := &StoragePool{
			Storage:      backend.Storage,
			Name:         name,
			Parent:       backend.Name,
			Plugin:       backend.Plugin,
			Capabilities: make(map[string]interface{}),
		}

		pools = append(pools, pool)
	}

	if len(pools) == 0 {
		return fmt.Errorf("no valid pools configured for backend %s", backend.Name)
	}

	backend.Pools = pools
	return nil
}

func NewBackend(backendName string, config map[string]interface{}) (*Backend, error) {
	// Verifying Common Parameters:
	// - storage: oceanstor-san; oceanstor-nas; oceanstor-dtree; fusionstorage-san; fusionstorage-nas;
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

	// while config hyperMetro, the metroBackend must config, hyperMetroDomain or metrovStorePairID should be config
	if ((metroDomain != "" || metrovStorePairID != "") && metroBackend == "") ||
		((metroDomain == "" && metrovStorePairID == "") && metroBackend != "") {
		return nil, fmt.Errorf("hyperMetro configuration in backend %s is incorrect", backendName)
	}

	return &Backend{
		Name:                backendName,
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
// If configured protocol for backend is "iscsi", CSI plugin internally add
// topology.kubernetes.io/protocol.iscsi = csi.huawei.com in supportedTopologies.
//
// Now users can opt to provision volumes based on protocol by
// 1. Labeling kubernetes nodes with protocol specific label (ie topology.kubernetes.io/protocol.iscsi = csi.huawei.com)
// 2. Configure topology support in plugin
// 3. Configure protocol topology in allowedTopologies fo Storage class
// addProtocolTopology is called after backend plugin init as init takes care of protocol validation
func addProtocolTopology(backend *Backend, driverName string) error {
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

func updateMetroBackends() {
	for _, i := range csiBackends {
		if (i.MetroDomain == "" && i.MetrovStorePairID == "") || i.MetroBackend != nil {
			continue
		}

		for _, j := range csiBackends {
			if i.Name == j.Name || i.Storage != j.Storage {
				continue
			}

			if ((i.MetroDomain != "" && i.MetroDomain == j.MetroDomain) ||
				(i.MetrovStorePairID != "" && i.MetrovStorePairID == j.MetrovStorePairID)) &&
				(i.MetroBackendName == j.Name && j.MetroBackendName == i.Name) {
				i.MetroBackend, j.MetroBackend = j, i
				i.Plugin.UpdateMetroRemotePlugin(j.Plugin)
				j.Plugin.UpdateMetroRemotePlugin(i.Plugin)
			}
		}
	}
}

func GetMetroDomain(backendName string) string {
	return csiBackends[backendName].MetroDomain
}

func GetMetrovStorePairID(backendName string) string {
	return csiBackends[backendName].MetrovStorePairID
}

func GetAccountName(backendName string) string {
	if _, ok := csiBackends[backendName]; !ok {
		return ""
	}
	return csiBackends[backendName].AccountName
}

func selectOnePool(ctx context.Context, requestSize int64, parameters map[string]interface{},
	candidatePools []*StoragePool, filterFuncs [][]interface{}) ([]*StoragePool, error) {

	var filterPools []*StoragePool
	if len(candidatePools) == 0 {
		for _, backend := range csiBackends {
			if backend.Available {
				filterPools = append(filterPools, backend.Pools...)
			}
		}
	} else {
		filterPools = append(filterPools, candidatePools...)
	}

	if len(filterPools) == 0 {
		regErr := RegisterAllBackend(ctx)
		if regErr != nil {
			return nil, fmt.Errorf("RegisterAllBackend failed, error: [%v]", regErr)
		}
		return nil, fmt.Errorf("no available storage pool for volume %v", parameters)
	}

	// filter the storage pools by capability
	filterPools, err := filterByCapabilityWithRetry(ctx, parameters, filterPools, filterFuncs)
	if err != nil {
		return nil, fmt.Errorf("failed to select pool, the capability filter failed, error: %v."+
			" please check your storage class", err)
	}

	// filter the storage by topology
	filterPools, err = filterByTopology(parameters, filterPools)
	if err != nil {
		return nil, err
	}

	allocType, _ := parameters["allocType"].(string)
	// filter the storage pool by capacity
	filterPools = filterByCapacity(requestSize, allocType, filterPools)
	if len(filterPools) == 0 {
		return nil, fmt.Errorf("failed to select pool, the capacity filter failed, capacity: %d", requestSize)
	}

	return filterPools, nil
}

func selectRemotePool(ctx context.Context,
	requestSize int64,
	parameters map[string]interface{},
	localBackendName string) (*StoragePool, error) {
	hyperMetro, hyperMetroOK := parameters["hyperMetro"].(string)
	replication, replicationOK := parameters["replication"].(string)

	if hyperMetroOK && utils.StrToBool(ctx, hyperMetro) &&
		replicationOK && utils.StrToBool(ctx, replication) {
		return nil, fmt.Errorf("cannot create volume with hyperMetro and replication properties: %v", parameters)
	}

	var remotePool *StoragePool
	var remotePools []*StoragePool
	var err error

	if hyperMetroOK && utils.StrToBool(ctx, hyperMetro) {
		localBackend := csiBackends[localBackendName]
		if localBackend.MetroBackend == nil {
			return nil, fmt.Errorf("no metro backend exists for volume: %v", parameters)
		}

		remotePools, err = selectOnePool(ctx, requestSize, parameters, localBackend.MetroBackend.Pools,
			secondaryFilterFuncs)
	}

	if replicationOK && utils.StrToBool(ctx, replication) {
		localBackend := csiBackends[localBackendName]
		if localBackend.ReplicaBackend == nil {
			return nil, fmt.Errorf("no replica backend exists for volume: %v", parameters)
		}

		remotePools, err = selectOnePool(ctx, requestSize, parameters, localBackend.ReplicaBackend.Pools,
			secondaryFilterFuncs)
	}

	if err != nil {
		return nil, fmt.Errorf("select remote pool failed: %v", err)
	}

	if len(remotePools) == 0 {
		return nil, nil
	}
	// weight the remote pool
	remotePool, err = weightSinglePools(ctx, requestSize, parameters, remotePools)
	return remotePool, err
}

func weightSinglePools(
	ctx context.Context,
	requestSize int64,
	parameters map[string]interface{},
	filterPools []*StoragePool) (*StoragePool, error) {
	// weight the storage pool by free capacity
	var selectPool *StoragePool
	selectPool = weightByFreeCapacity(filterPools)
	if selectPool == nil {
		return nil, fmt.Errorf("cannot select a storage pool for volume (%d, %v)", requestSize, parameters)
	}

	log.AddContext(ctx).Infof("Select storage pool %s:%s for volume (%d, %v)",
		selectPool.Parent, selectPool.Name, requestSize, parameters)
	return selectPool, nil
}

var SelectStoragePool = func(ctx context.Context, requestSize int64, parameters map[string]interface{}) (*StoragePool, *StoragePool, error) {
	localPools, err := selectOnePool(ctx, requestSize, parameters, nil, primaryFilterFuncs)
	if err != nil {
		return nil, nil, err
	}
	log.AddContext(ctx).Debugf("Select local pools are %v.", localPools)

	var poolPairs []SelectPoolPair
	for _, localPool := range localPools {
		remotePool, err := selectRemotePool(ctx, requestSize, parameters, localPool.Parent)
		if err != nil {
			return nil, nil, err
		}
		log.AddContext(ctx).Debugf("Select remote pool is %v.", remotePool)
		poolPairs = append(poolPairs, SelectPoolPair{local: localPool, remote: remotePool})
	}

	return weightPools(ctx, requestSize, parameters, localPools, poolPairs)
}

func weightPools(ctx context.Context,
	requestSize int64,
	parameters map[string]interface{}, localPools []*StoragePool,
	poolPairs []SelectPoolPair) (*StoragePool, *StoragePool, error) {
	localPool, err := weightSinglePools(ctx, requestSize, parameters, localPools)
	if err != nil {
		return nil, nil, err
	}

	for _, pair := range poolPairs {
		if pair.local == localPool {
			updateSelectPool(requestSize, parameters, pair.local)
			updateSelectPool(requestSize, parameters, pair.remote)
			return pair.local, pair.remote, nil
		}
	}
	return nil, nil, errors.New("weight pool failed")
}

func updateSelectPool(requestSize int64, parameters map[string]interface{}, selectPool *StoragePool) {
	if selectPool == nil {
		return
	}

	allocType, _ := parameters["allocType"].(string)
	// when the allocType is thin, do not change the FreeCapacity.
	if allocType == "thick" {
		freeCapacity, _ := selectPool.Capabilities["FreeCapacity"].(int64)
		selectPool.Capabilities["FreeCapacity"] = freeCapacity - requestSize
	}
}

func filterByBackendName(ctx context.Context, backendName string, candidatePools []*StoragePool) ([]*StoragePool,
	error) {
	var filterPools []*StoragePool

	for _, pool := range candidatePools {
		if backendName == "" || backendName == pool.Parent {
			filterPools = append(filterPools, pool)
		}
	}

	return filterPools, nil
}

func filterByStoragePool(ctx context.Context, poolName string, candidatePools []*StoragePool) ([]*StoragePool, error) {
	var filterPools []*StoragePool

	for _, pool := range candidatePools {
		if poolName == "" || poolName == pool.Name {
			filterPools = append(filterPools, pool)
		}
	}

	return filterPools, nil
}

func filterByVolumeType(ctx context.Context, volumeType string, candidatePools []*StoragePool) ([]*StoragePool, error) {
	var filterPools []*StoragePool

	for _, pool := range candidatePools {
		if volumeType == "lun" || volumeType == "" {
			if pool.Storage == "oceanstor-san" || pool.Storage == "fusionstorage-san" {
				filterPools = append(filterPools, pool)
			}
		} else if volumeType == "fs" {
			if pool.Storage == "oceanstor-nas" || pool.Storage == "oceanstor-9000" || pool.Storage == "fusionstorage-nas" {
				filterPools = append(filterPools, pool)
			}
		} else if volumeType == "dtree" {
			if pool.Storage == "oceanstor-dtree" {
				filterPools = append(filterPools, pool)
			}
		}
	}

	return filterPools, nil
}

func filterByAllocType(ctx context.Context, allocType string, candidatePools []*StoragePool) ([]*StoragePool, error) {
	var filterPools []*StoragePool

	for _, pool := range candidatePools {
		valid := false

		if pool.Storage == "oceanstor-9000" {
			valid = true
		} else if allocType == "thin" || allocType == "" {
			supportThin, exist := pool.Capabilities["SupportThin"].(bool)
			if !exist {
				log.AddContext(ctx).Warningf("convert supportThin to bool failed, data: %v", pool.Capabilities["SupportThin"])
			}
			valid = exist && supportThin
		} else if allocType == "thick" {
			supportThick, exist := pool.Capabilities["SupportThick"].(bool)
			if !exist {
				log.AddContext(ctx).Warningf("convert supportThick to bool failed, data: %v", pool.Capabilities["SupportThick"])
			}
			valid = exist && supportThick
		}

		if valid {
			filterPools = append(filterPools, pool)
		}
	}

	return filterPools, nil
}

func filterByQos(ctx context.Context, qos string, candidatePools []*StoragePool) ([]*StoragePool, error) {
	var filterPools []*StoragePool

	if qos == "" {
		return candidatePools, nil
	}

	var poolSelectionErrors []error
	for _, pool := range candidatePools {
		supportQoS, exist := pool.Capabilities["SupportQoS"].(bool)
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

func filterByMetro(ctx context.Context, hyperMetro string, candidatePools []*StoragePool) ([]*StoragePool, error) {
	if len(hyperMetro) == 0 || !utils.StrToBool(ctx, hyperMetro) {
		return candidatePools, nil
	}

	var filterPools []*StoragePool

	for _, pool := range candidatePools {
		backend, exist := csiBackends[pool.Parent]
		if !exist || backend.MetroBackend == nil {
			continue
		}

		if supportMetro, exist := pool.Capabilities["SupportMetro"].(bool); exist && supportMetro {
			filterPools = append(filterPools, pool)
		}
	}

	return filterPools, nil
}

func filterByReplication(ctx context.Context, replication string, candidatePools []*StoragePool) ([]*StoragePool,
	error) {
	if len(replication) == 0 || !utils.StrToBool(ctx, replication) {
		return candidatePools, nil
	}

	var filterPools []*StoragePool

	for _, pool := range candidatePools {
		backend, exist := csiBackends[pool.Parent]
		if !exist || backend.ReplicaBackend == nil {
			continue
		}

		if SupportReplication, exist := pool.Capabilities["SupportReplication"].(bool); exist && SupportReplication {
			filterPools = append(filterPools, pool)
		}
	}

	return filterPools, nil
}

// filterByTopology returns a subset of the provided pools that can support any of the topology requirement.
func filterByTopology(parameters map[string]interface{}, candidatePools []*StoragePool) ([]*StoragePool, error) {
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
func isTopologySupportedByBackend(backend *Backend, topology map[string]string) bool {
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
func filterPoolsOnTopology(candidatePools []*StoragePool, requisiteTopologies []map[string]string) []*StoragePool {
	filteredPools := make([]*StoragePool, 0)

	if len(requisiteTopologies) == 0 {
		return candidatePools
	}

	for _, pool := range candidatePools {
		// mutex lock acquired in pool selection
		backend, exist := csiBackends[pool.Parent]
		if !exist {
			continue
		}

		// when backend is not configured with supported topology
		if len(backend.SupportedTopologies) == 0 {
			filteredPools = append(filteredPools, pool)
			continue
		}

		for _, topology := range requisiteTopologies {
			if isTopologySupportedByBackend(backend, topology) {
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
func sortPoolsByPreferredTopologies(candidatePools []*StoragePool, preferredTopologies []map[string]string) []*StoragePool {
	remainingPools := make([]*StoragePool, len(candidatePools))
	copy(remainingPools, candidatePools)
	orderedPools := make([]*StoragePool, 0)

	for _, preferred := range preferredTopologies {
		newRemainingPools := make([]*StoragePool, 0)
		poolBucket := make([]*StoragePool, 0)

		for _, pool := range remainingPools {
			backend, exist := csiBackends[pool.Parent]
			if !exist {
				continue
			}
			// If it supports topology, pop it and add to bucket. Otherwise, add it to newRemaining pools to be
			// addressed in future loop iterations.
			if isTopologySupportedByBackend(backend, preferred) {
				poolBucket = append(poolBucket, pool)
			} else {
				newRemainingPools = append(newRemainingPools, pool)
			}
		}

		// make new list of remaining pools
		remainingPools = make([]*StoragePool, len(newRemainingPools))
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

func filterByCapabilityWithRetry(ctx context.Context, parameters map[string]interface{}, candidatePools []*StoragePool,
	filterFuncs [][]interface{}) ([]*StoragePool, error) {

	filterPools, err := filterByCapability(ctx, parameters, candidatePools, filterFuncs)
	if err == nil {
		return filterPools, nil
	} else if !strings.Contains(err.Error(), NoAvailablePool) {
		return nil, fmt.Errorf("failed to select pool, the capability filter failed, error: %v."+
			" please check your storage class", err)
	}

	regErr := RegisterAllBackend(ctx)
	if regErr != nil {
		return nil, fmt.Errorf("filterByCapabilityWithRetry failed, RegisterAllBackend failed, error: [%v]", regErr)
	}

	return filterByCapability(ctx, parameters, filterPools, filterFuncs)
}

func filterByCapability(ctx context.Context, parameters map[string]interface{}, candidatePools []*StoragePool,
	filterFuncs [][]interface{}) ([]*StoragePool, error) {

	var err error
	for _, i := range filterFuncs {
		key, filter := i[0].(string), i[1].(func(context.Context, string, []*StoragePool) ([]*StoragePool, error))
		value, _ := parameters[key].(string)
		candidatePools, err = filter(ctx, value, candidatePools)
		if err != nil {
			msg := fmt.Sprintf("Filter pool by capability failed, filter field: [%s], fileter function: [%s], paramters: [%v], error: [%v].",
				value, runtime.FuncForPC(reflect.ValueOf(filter).Pointer()).Name(), parameters, err)
			log.AddContext(ctx).Errorln(msg)
			return nil, errors.New(msg)
		}
		if len(candidatePools) == 0 {
			msg := fmt.Sprintf("%s. the final filter field: %s, filter function: %s, parameters %v.",
				NoAvailablePool, value, runtime.FuncForPC(reflect.ValueOf(filter).Pointer()).Name(), parameters)
			log.AddContext(ctx).Errorln(msg)
			return nil, errors.New(msg)
		}
	}

	return candidatePools, nil
}

func filterByNFSProtocol(ctx context.Context, nfsProtocol string, candidatePools []*StoragePool) ([]*StoragePool,
	error) {
	if nfsProtocol == "" {
		return candidatePools, nil
	}

	var filterPools []*StoragePool
	for _, pool := range candidatePools {
		if nfsProtocol == "nfs3" &&
			pool.Capabilities["SupportNFS3"] != nil && pool.Capabilities["SupportNFS3"].(bool) {
			filterPools = append(filterPools, pool)
		} else if nfsProtocol == "nfs4" &&
			pool.Capabilities["SupportNFS4"] != nil && pool.Capabilities["SupportNFS4"].(bool) {
			filterPools = append(filterPools, pool)
		} else if nfsProtocol == "nfs41" &&
			pool.Capabilities["SupportNFS41"] != nil && pool.Capabilities["SupportNFS41"].(bool) {
			filterPools = append(filterPools, pool)
		}
	}

	return filterPools, nil
}

func filterBySupportClone(ctx context.Context, cloneSource string, candidatePools []*StoragePool) ([]*StoragePool,
	error) {
	if cloneSource == "" {
		return candidatePools, nil
	}
	var filterPools []*StoragePool
	for _, pool := range candidatePools {
		if pool.Capabilities["SupportClone"].(bool) {
			filterPools = append(filterPools, pool)
		}
	}
	return filterPools, nil
}

func filterByCapacity(requestSize int64, allocType string, candidatePools []*StoragePool) []*StoragePool {
	var filterPools []*StoragePool
	for _, pool := range candidatePools {
		supportThin, thinExist := pool.Capabilities["SupportThin"].(bool)
		if !thinExist {
			log.Warningf("convert supportThin to bool failed, data: %v", pool.Capabilities["SupportThin"])
		}
		supportThick, thickExist := pool.Capabilities["SupportThick"].(bool)
		if !thickExist {
			log.Warningf("convert supportThick to bool failed, data: %v", pool.Capabilities["SupportThick"])
		}
		if (allocType == "thin" || allocType == "") && thinExist && supportThin {
			filterPools = append(filterPools, pool)
		} else if allocType == "thick" && thickExist && supportThick {
			freeCapacity, _ := pool.Capabilities["FreeCapacity"].(int64)
			if requestSize <= freeCapacity {
				filterPools = append(filterPools, pool)
			}
		}
	}

	return filterPools
}

func weightByFreeCapacity(candidatePools []*StoragePool) *StoragePool {
	var selectPool *StoragePool

	for _, pool := range candidatePools {
		if selectPool == nil {
			selectPool = pool
		} else {
			selectCapacity, _ := selectPool.Capabilities["FreeCapacity"].(int64)
			curFreeCapacity, _ := pool.Capabilities["FreeCapacity"].(int64)
			if selectCapacity < curFreeCapacity {
				selectPool = pool
			}
		}
	}
	return selectPool
}

// GetSupportedTopologies return configured supported topology by pool
func (pool *StoragePool) GetSupportedTopologies(ctx context.Context) []map[string]string {
	mutex.Lock()
	defer mutex.Unlock()
	backend, exist := csiBackends[pool.Parent]
	if !exist {
		log.AddContext(ctx).Warningf("Backend [%v] does not exist in CSI backend pool", pool.Parent)
		return make([]map[string]string, 0)
	}

	return backend.SupportedTopologies
}

func filterByApplicationType(ctx context.Context, appType string, candidatePools []*StoragePool) ([]*StoragePool,
	error) {
	var filterPools []*StoragePool
	for _, pool := range candidatePools {
		if appType != "" {
			supportAppType, ok := pool.Capabilities["SupportApplicationType"].(bool)
			if ok && supportAppType {
				filterPools = append(filterPools, pool)
			}
		} else {
			filterPools = append(filterPools, pool)
		}
	}
	return filterPools, nil
}

func filterByStorageQuota(ctx context.Context, storageQuota string, candidatePools []*StoragePool) ([]*StoragePool,
	error) {
	var filterPools []*StoragePool
	if storageQuota == "" {
		return candidatePools, nil
	}

	for _, pool := range candidatePools {
		supportStorageQuota, ok := pool.Capabilities["SupportQuota"].(bool)
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
var ValidateBackend = func(ctx context.Context, selectBackend *Backend, parameters map[string]interface{}) error {
	for _, i := range validateFilterFuncs {
		key, validator := i[0].(string), i[1].(func(context.Context, string, *Backend) error)
		value, _ := parameters[key].(string)
		if err := validator(ctx, value, selectBackend); err != nil {
			return fmt.Errorf("validate backend error for manage Volume. "+
				"the final validator field: %s, validator function: %s, parameters %v. Reason: %v",
				key, runtime.FuncForPC(reflect.ValueOf(validator).Pointer()).Name(), parameters, err)
		}
	}

	return nil
}

func validateBackendName(ctx context.Context, backendName string, selectBackend *Backend) error {
	if backendName != "" && selectBackend.Name != backendName {
		return utils.Errorf(ctx, "the backend name between StorageClass(%s) and PVC annotation(%s) "+
			"is different", backendName, selectBackend.Name)
	}

	return nil
}

func validateVolumeType(ctx context.Context, volumeType string, selectBackend *Backend) error {
	if filterPools, err := filterByVolumeType(ctx, volumeType, selectBackend.Pools); len(filterPools) == 0 {
		if err != nil {
			return err
		}
		return utils.Errorf(ctx, "the volumeType between StorageClass(%s) and PVC annotation(%s) "+
			"is different, err: filterPools is empty", volumeType, selectBackend.Name)
	}
	return nil
}

// RegisterOneBackend used to register a backend to plugin
func RegisterOneBackend(ctx context.Context, backendID, configmapMeta, secretMeta, certSecret string,
	useCert bool) (string, error) {
	mutex.Lock()
	defer mutex.Unlock()

	log.AddContext(ctx).Infof("Register backend: [%s], configmap: [%s], meta: [%s]",
		backendID, configmapMeta, secretMeta)

	// If the backend exists, the registration is not repeated.
	_, backendName, err := pkgUtils.SplitMetaNamespaceKey(backendID)
	if _, exist := csiBackends[backendName]; exist {
		log.AddContext(ctx).Infof("Backend %s already exist, ignore current request.", backendName)
		return backendName, nil
	}

	storageInfo, err := GetStorageBackendInfo(ctx, backendID, configmapMeta, secretMeta, certSecret, useCert)
	if err != nil {
		return "", err
	}

	bk, err := NewBackend(backendName, storageInfo)
	if err != nil {
		return "", err
	}

	err = analyzePools(bk, storageInfo)
	if err != nil {
		return "", err
	}

	err = bk.Plugin.Init(storageInfo, bk.Parameters, true)
	if err != nil {
		log.Errorf("Init backend plugin error: %v", err)
		return "", err
	}

	err = addProtocolTopology(bk, app.GetGlobalConfig().DriverName)
	if err != nil {
		log.Errorf("Add protocol topology error: %v", err)
		return "", err
	}

	csiBackends[backendName] = bk

	updateMetroBackends()

	log.AddContext(ctx).Infof("The backend: [%s] is registered successfully. Registered: [%+v]",
		backendName, csiBackends)
	return backendName, nil
}

// RegisterAllBackend used to synchronize all backend from storageBackendContent to driver
func RegisterAllBackend(ctx context.Context) error {
	log.AddContext(ctx).Infoln("Synchronize backend from online storageBackendContent.")
	defer log.AddContext(ctx).Infoln("Finish synchronize backend from online storageBackendContent.")

	// Obtains all storageBackendClaims in the CSI namespace.
	claims, err := pkgUtils.ListClaim(ctx, app.GetGlobalConfig().BackendUtils, app.GetGlobalConfig().Namespace)
	if err != nil {
		msg := fmt.Sprintf("List storageBackendClaim failed, error: [%v].", err)
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}

	// Get all online storageBackendContent
	var onlineContents []*xuanwuv1.StorageBackendContent
	for _, claim := range claims.Items {
		content, err := pkgUtils.GetContent(ctx, app.GetGlobalConfig().BackendUtils, claim.Status.BoundContentName)
		if err != nil {
			log.AddContext(ctx).Warningf("Get storageBackendContent failed, error: [%v]", err)
			continue
		}
		if content == nil || content.Status == nil {
			log.AddContext(ctx).Warningf("Get empty storageBackendContent, content: %+v", content)
			continue
		}
		if !content.Status.Online {
			log.AddContext(ctx).Warningf("StorageBackendContent: [%s] is offline(online: false), "+
				"will not register.", content.Name)
			continue
		}
		onlineContents = append(onlineContents, content)
	}

	// If the backend name is not in csiBackends, invoke RegisterOneBackend for registration.
	for _, content := range onlineContents {
		configmapMeta, secretMeta, err := pkgUtils.GetConfigMeta(ctx, content.Spec.BackendClaim)
		if err != nil {
			log.AddContext(ctx).Warningf("Get storageBackendClaim: [%s] ConfigMeta failed, storageBackendContent: "+
				"[%s], error: [%v]", content.Spec.BackendClaim, content.Name, err)
			continue
		}

		useCert, certSecret, err := pkgUtils.GetCertMeta(ctx, content.Spec.BackendClaim)
		if err != nil {
			log.AddContext(ctx).Warningf("Get storageBackendClaim: [%s] CertMeta failed, storageBackendContent: "+
				"[%s], error: [%v]", content.Spec.BackendClaim, content.Name, err)
			continue
		}

		_, err = RegisterOneBackend(ctx, content.Spec.BackendClaim, configmapMeta, secretMeta, certSecret, useCert)
		if err != nil {
			log.AddContext(ctx).Warningf("RegisterOneBackend failed, meta: [%s %s %s], error: %v",
				content.Spec.BackendClaim, configmapMeta, secretMeta, err)
		}
	}

	return nil
}

// RemoveOneBackend remove a storage backend from plugin
func RemoveOneBackend(ctx context.Context, storageBackendId string) {
	mutex.Lock()
	defer mutex.Unlock()

	if _, exist := csiBackends[storageBackendId]; exist {
		delete(csiBackends, storageBackendId)
	}
	log.AddContext(ctx).Infof("storageBackends: Successful remove backend %s. csiBackends: [%v] ",
		storageBackendId, csiBackends)

	finalizers.RemoveStorageBackendMutex(ctx, storageBackendId)
	return
}

func IsBackendRegistered(backendName string) bool {
	return csiBackends[backendName] != nil
}
