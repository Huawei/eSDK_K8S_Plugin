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

package backend

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"huawei-csi-driver/csi/backend/plugin"
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
)

var (
	mutex       sync.Mutex
	csiBackends = make(map[string]*Backend)

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

func analyzePools(backend *Backend, config map[string]interface{}) error {
	var pools []*StoragePool

	if backend.Storage != "OceanStor-9000" {
		configPools, _ := config["pools"].([]interface{})
		for _, i := range configPools {
			name := i.(string)
			if name == "" {
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
			return fmt.Errorf("No valid pools configured for backend %s", backend.Name)
		}
	} else {
		pool := &StoragePool{
			Storage:      backend.Storage,
			Name:         backend.Name,
			Parent:       backend.Name,
			Plugin:       backend.Plugin,
			Capabilities: make(map[string]interface{}),
		}

		pools = append(pools, pool)
	}

	backend.Pools = pools
	return nil
}

func newBackend(backendName string, config map[string]interface{}) (*Backend, error) {
	storage, exist := config["storage"].(string)
	if !exist {
		return nil, errors.New("storage type must be configured for backend")
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

	plugin := plugin.GetPlugin(storage)
	if plugin == nil {
		return nil, fmt.Errorf("Cannot get plugin for storage %s", storage)
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
		Plugin:              plugin,
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

func analyzeBackend(config map[string]interface{}) (*Backend, error) {
	backendName, exist := config["name"].(string)
	if !exist {
		return nil, errors.New("Name must be configured for backend")
	}

	match, err := regexp.MatchString(`^[\w-]+$`, backendName)
	if err != nil || !match {
		return nil, fmt.Errorf("backend name %v is invalid, support upper&lower characters, numeric and [-_]", backendName)
	}

	if _, exist := csiBackends[backendName]; exist {
		return nil, fmt.Errorf("Backend name %s is duplicated", backendName)
	}

	backend, err := newBackend(backendName, config)
	if err != nil {
		return nil, err
	}

	err = analyzePools(backend, config)
	if err != nil {
		return nil, err
	}

	return backend, nil
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

func updateReplicaBackends() {
	for _, i := range csiBackends {
		if i.ReplicaBackend != nil {
			continue
		}

		for _, j := range csiBackends {
			if i.Name == j.Name || i.Storage != j.Storage || j.ReplicaBackend != nil {
				continue
			}

			if i.ReplicaBackendName == j.Name && j.ReplicaBackendName == i.Name {
				i.ReplicaBackend, j.ReplicaBackend = j, i

				i.Plugin.UpdateReplicaRemotePlugin(j.Plugin)
				j.Plugin.UpdateReplicaRemotePlugin(i.Plugin)
			}
		}
	}
}

func RegisterBackend(backendConfigs []map[string]interface{}, keepLogin bool, driverName string) error {
	for _, i := range backendConfigs {
		backend, err := analyzeBackend(i)
		if err != nil {
			log.Errorf("Analyze backend error: %v", err)
			return err
		}

		err = backend.Plugin.Init(i, backend.Parameters, keepLogin)
		if err != nil {
			log.Errorf("Init backend plugin error: %v", err)
			return err
		}

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
		err = addProtocolTopology(backend, driverName)
		if err != nil {
			log.Errorf("Add protocol topology error: %v", err)
			return err
		}

		csiBackends[backend.Name] = backend
	}

	updateMetroBackends()
	updateReplicaBackends()

	return nil
}

func GetBackend(backendName string) *Backend {
	return csiBackends[backendName]
}

func GetMetroDomain(backendName string) string {
	return csiBackends[backendName].MetroDomain
}

func GetMetrovStorePairID(backendName string) string {
	return csiBackends[backendName].MetrovStorePairID
}

func GetAccountName(backendName string) string {
	return csiBackends[backendName].AccountName
}

func selectOnePool(ctx context.Context,
	requestSize int64,
	parameters map[string]interface{},
	candidatePools []*StoragePool,
	filterFuncs [][]interface{}) ([]*StoragePool, error) {
	var filterPools []*StoragePool

	mutex.Lock()
	defer mutex.Unlock()

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
		return nil, fmt.Errorf("no available storage pool for volume %v", parameters)
	}

	// filter the storage pools by capability
	filterPools, err := filterByCapability(ctx, parameters, filterPools, filterFuncs)
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

	if remotePools == nil {
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

func SelectStoragePool(ctx context.Context, requestSize int64, parameters map[string]interface{}) (*StoragePool, *StoragePool, error) {
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

func filterByVolumeType(ctx context.Context, volumeType string, candidatePools []*StoragePool) ([]*StoragePool,
	error) {
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
			valid = exist && supportThin
		} else if allocType == "thick" {
			supportThick, exist := pool.Capabilities["SupportThick"].(bool)
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
func filterByTopology(parameters map[string]interface{},
	candidatePools []*StoragePool) ([]*StoragePool, error) {

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

func filterByCapability(
	ctx context.Context,
	parameters map[string]interface{},
	candidatePools []*StoragePool,
	filterFuncs [][]interface{}) ([]*StoragePool, error) {
	var err error
	for _, i := range filterFuncs {
		key, filter := i[0].(string), i[1].(func(context.Context, string, []*StoragePool) ([]*StoragePool, error))
		value, _ := parameters[key].(string)
		candidatePools, err = filter(ctx, value, candidatePools)
		if err != nil || len(candidatePools) == 0 {
			return nil, fmt.Errorf("no storage pool meets the requirements. "+
				"the final filter field: %s, filter function: %s, parameters %v. Reason: %v",
				key, runtime.FuncForPC(reflect.ValueOf(filter).Pointer()).Name(), parameters, err)
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
		if nfsProtocol == "nfs3" && pool.Capabilities["SupportNFS3"].(bool) {
			filterPools = append(filterPools, pool)
		} else if nfsProtocol == "nfs4" && pool.Capabilities["SupportNFS4"].(bool) {
			filterPools = append(filterPools, pool)
		} else if nfsProtocol == "nfs41" && pool.Capabilities["SupportNFS41"].(bool) {
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
		supportThick, thickExist := pool.Capabilities["SupportThick"].(bool)
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
