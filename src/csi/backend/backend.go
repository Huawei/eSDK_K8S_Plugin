package backend

import (
	"csi/backend/plugin"
	"errors"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"sync"
	"utils"
	"utils/k8sutils"
	"utils/log"
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

	MetroDomain       string
	MetrovStorePairID string
	MetroBackendName  string
	MetroBackend      *Backend

	ReplicaBackendName string
	ReplicaBackend     *Backend
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

			if ((i.MetroDomain != "" && i.MetroDomain == j.MetroDomain) || (i.MetrovStorePairID != "" && i.MetrovStorePairID == j.MetrovStorePairID)) && (i.MetroBackendName == j.Name && j.MetroBackendName == i.Name) {
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

		// Note: Protocol is considered as special topological parameter. The
		// protocol topology is populated internally by plugin using protocol name.
		// If configured protocol for backend is "iscsi", CSI plugin internally add topology.kubernetes.io/protocol.iscsi = csi.huawei.com in supportedTopologies.
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

func selectOnePool(requestSize int64,
	parameters map[string]interface{},
	candidatePools []*StoragePool,
	filterFuncs [][]interface{}) (*StoragePool, error) {
	var selectPool *StoragePool
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
	filterPools, err := filterByCapability(parameters, filterPools, filterFuncs)
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

	// weight the storage pool by free capacity
	selectPool = weightByFreeCapacity(filterPools)
	if selectPool == nil {
		return nil, fmt.Errorf("cannot select a storage pool for volume (%d, %v)", requestSize, parameters)
	}

	log.Infof("Select storage pool %s:%s for volume (%d, %v)",
		selectPool.Parent, selectPool.Name, requestSize, parameters)

	// when the allocType is thin, do not change the FreeCapacity.
	if allocType == "thick" {
		freeCapacity, _ := selectPool.Capabilities["FreeCapacity"].(int64)
		selectPool.Capabilities["FreeCapacity"] = freeCapacity - requestSize
	}

	return selectPool, nil
}

func selectRemotePool(requestSize int64, parameters map[string]interface{}, localBackendName string) (*StoragePool, error) {
	hyperMetro, hyperMetroOK := parameters["hyperMetro"].(string)
	replication, replicationOK := parameters["replication"].(string)

	if hyperMetroOK && utils.StrToBool(hyperMetro) &&
		replicationOK && utils.StrToBool(replication) {
		return nil, fmt.Errorf("cannot create volume with hyperMetro and replication properties: %v", parameters)
	}

	var remotePool *StoragePool
	var err error

	if hyperMetroOK && utils.StrToBool(hyperMetro) {
		localBackend := csiBackends[localBackendName]
		if localBackend.MetroBackend == nil {
			return nil, fmt.Errorf("no metro backend exists for volume: %v", parameters)
		}

		remotePool, err = selectOnePool(requestSize, parameters, localBackend.MetroBackend.Pools, secondaryFilterFuncs)
	}

	if replicationOK && utils.StrToBool(replication) {
		localBackend := csiBackends[localBackendName]
		if localBackend.ReplicaBackend == nil {
			return nil, fmt.Errorf("no replica backend exists for volume: %v", parameters)
		}

		remotePool, err = selectOnePool(requestSize, parameters, localBackend.ReplicaBackend.Pools, secondaryFilterFuncs)
	}

	return remotePool, err
}

func SelectStoragePool(requestSize int64, parameters map[string]interface{}) (*StoragePool, *StoragePool, error) {
	localPool, err := selectOnePool(requestSize, parameters, nil, primaryFilterFuncs)
	if err != nil {
		return nil, nil, err
	}

	remotePool, err := selectRemotePool(requestSize, parameters, localPool.Parent)
	if err != nil {
		return nil, nil, err
	}

	return localPool, remotePool, nil
}

func filterByBackendName(backendName string, candidatePools []*StoragePool) []*StoragePool {
	var filterPools []*StoragePool

	for _, pool := range candidatePools {
		if backendName == "" || backendName == pool.Parent {
			filterPools = append(filterPools, pool)
		}
	}

	return filterPools
}

func filterByStoragePool(poolName string, candidatePools []*StoragePool) []*StoragePool {
	var filterPools []*StoragePool

	for _, pool := range candidatePools {
		if poolName == "" || poolName == pool.Name {
			filterPools = append(filterPools, pool)
		}
	}

	return filterPools
}

func filterByVolumeType(volumeType string, candidatePools []*StoragePool) []*StoragePool {
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

	return filterPools
}

func filterByAllocType(allocType string, candidatePools []*StoragePool) []*StoragePool {
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

	return filterPools
}

func filterByQos(qos string, candidatePools []*StoragePool) []*StoragePool {
	var filterPools []*StoragePool

	for _, pool := range candidatePools {
		if qos != "" {
			supportQoS, exist := pool.Capabilities["SupportQoS"].(bool)
			if exist && supportQoS {
				filterPools = append(filterPools, pool)
			}
		} else {
			filterPools = append(filterPools, pool)
		}
	}

	return filterPools
}

func filterByMetro(hyperMetro string, candidatePools []*StoragePool) []*StoragePool {
	if len(hyperMetro) == 0 || !utils.StrToBool(hyperMetro) {
		return candidatePools
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

	return filterPools
}

func filterByReplication(replication string, candidatePools []*StoragePool) []*StoragePool {
	if len(replication) == 0 || !utils.StrToBool(replication) {
		return candidatePools
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

	return filterPools
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
	parameters map[string]interface{},
	candidatePools []*StoragePool,
	filterFuncs [][]interface{}) ([]*StoragePool, error) {
	for _, i := range filterFuncs {
		key, filter := i[0].(string), i[1].(func(string, []*StoragePool) []*StoragePool)
		value, _ := parameters[key].(string)
		candidatePools = filter(value, candidatePools)
		if len(candidatePools) == 0 {
			return nil, fmt.Errorf("failed to select pool, the final filter field: %s, parameters %v", key, parameters)
		}
	}

	return candidatePools, nil
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
func (pool *StoragePool) GetSupportedTopologies() []map[string]string {
	mutex.Lock()
	defer mutex.Unlock()
	backend, exist := csiBackends[pool.Parent]
	if !exist {
		log.Warningf("Backend [%v] does not exist in CSI backend pool", pool.Parent)
		return make([]map[string]string, 0)
	}

	return backend.SupportedTopologies
}

func filterByApplicationType(appType string, candidatePools []*StoragePool) []*StoragePool {
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
	return filterPools
}
