package backend

import (
	"csi/backend/plugin"
	"errors"
	"fmt"
	"regexp"
	"sync"
	"utils"
	"utils/log"
)

var (
	mutex       sync.Mutex
	csiBackends = make(map[string]*Backend)

	FilterFuncs = [][]interface{}{
		{"backend", filterByBackendName},
		{"pool", filterByStoragePool},
		{"volumeType", filterByVolumeType},
		{"allocType", filterByAllocType},
		{"qos", filterByQos},
		{"hyperMetro", filterByMetro},
	}
)

type StoragePool struct {
	Name         string
	Storage      string
	Parent       string
	Capabilities map[string]interface{}
	Plugin       plugin.Plugin
}

type Backend struct {
	Name       string
	Storage    string
	Available  bool
	Plugin     plugin.Plugin
	Pools      []*StoragePool
	Parameters map[string]interface{}

	MetroDomain  string
	MetroBackend *Backend
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

	plugin := plugin.GetPlugin(storage)
	if plugin == nil {
		return nil, fmt.Errorf("Cannot get plugin for storage %s", storage)
	}

	err := plugin.Init(config, parameters)
	if err != nil {
		return nil, err
	}

	metroDomain, _ := config["hyperMetroDomain"].(string)

	return &Backend{
		Name:        backendName,
		Storage:     storage,
		Available:   false,
		Plugin:      plugin,
		Parameters:  parameters,
		MetroDomain: metroDomain,
	}, nil
}

func analyzeBackend(config map[string]interface{}) error {
	backendName, exist := config["name"].(string)
	if !exist {
		return errors.New("Name must be configured for backend")
	}

	match, err := regexp.MatchString(`^[\w-]+$`, backendName)
	if err != nil || !match {
		return fmt.Errorf(
			"backend name %v is invalid, support upper&lower characters, numeric and [-_]", backendName)
	}

	if _, exist := csiBackends[backendName]; exist {
		return fmt.Errorf("Backend name %s is duplicated", backendName)
	}

	backend, err := newBackend(backendName, config)
	if err != nil {
		return err
	}

	err = analyzePools(backend, config)
	if err != nil {
		return err
	}

	csiBackends[backendName] = backend
	return nil
}

func updateMetroBackends() {
	for _, i := range csiBackends {
		if len(i.MetroDomain) == 0 || i.MetroBackend != nil {
			continue
		}

		for _, j := range csiBackends {
			if i.Name != j.Name && i.MetroDomain == j.MetroDomain && i.Storage == j.Storage {
				i.MetroBackend, j.MetroBackend = j, i

				i.Plugin.UpdateMetroRemotePlugin(j.Plugin)
				j.Plugin.UpdateMetroRemotePlugin(i.Plugin)
			}
		}
	}
}

func RegisterBackend(backendConfigs []map[string]interface{}) error {
	for _, i := range backendConfigs {
		err := analyzeBackend(i)
		if err != nil {
			log.Errorf("Analyze backend error: %v", err)
			return err
		}
	}

	updateMetroBackends()

	return nil
}

func GetBackend(backendName string) *Backend {
	return csiBackends[backendName]
}

func GetMetroDomain(backendName string) string {
	return csiBackends[backendName].MetroDomain
}

func selectOnePool(requestSize int64, parameters map[string]interface{}) (*StoragePool, error) {
	var selectPool *StoragePool
	var filterPools []*StoragePool

	mutex.Lock()
	defer mutex.Unlock()

	for _, backend := range csiBackends {
		if backend.Available {
			filterPools = append(filterPools, backend.Pools...)
		}
	}

	if len(filterPools) == 0 {
		msg := fmt.Sprintf("No available storage pool for volume %v", parameters)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	for _, i := range FilterFuncs {
		key, filter := i[0].(string), i[1].(func(string, []*StoragePool) []*StoragePool)
		value, _ := parameters[key].(string)
		filterPools = filter(value, filterPools)
	}

	for _, pool := range filterPools {
		if selectPool == nil {
			freeCapacity, _ := pool.Capabilities["FreeCapacity"].(int64)
			if requestSize <= freeCapacity {
				selectPool = pool
			}
		} else {
			freeCapacity, _ := selectPool.Capabilities["FreeCapacity"].(int64)
			cmpFreeCapacity, _ := pool.Capabilities["FreeCapacity"].(int64)

			if freeCapacity < cmpFreeCapacity {
				selectPool = pool
			}
		}
	}

	if selectPool == nil {
		msg := fmt.Sprintf("Cannot select a storage pool for volume (%d, %v)", requestSize, parameters)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	log.Infof("Select storage pool %s:%s for volume (%d, %v)",
		selectPool.Parent, selectPool.Name, requestSize, parameters)

	freeCapacity, _ := selectPool.Capabilities["FreeCapacity"].(int64)
	selectPool.Capabilities["FreeCapacity"] = freeCapacity - requestSize

	return selectPool, nil
}

func SelectStoragePool(requestSize int64, parameters map[string]interface{}) (*StoragePool, *StoragePool, error) {
	localPool, err := selectOnePool(requestSize, parameters)
	if err != nil {
		return nil, nil, err
	}

	var remotePool *StoragePool

	localBackend := csiBackends[localPool.Parent]
	if localBackend.MetroBackend != nil {
		parameters["backend"] = localBackend.MetroBackend.Name
		remotePool, err = selectOnePool(requestSize, parameters)
		if err != nil {
			return nil, nil, err
		}
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
			if pool.Storage == "oceanstor-nas" || pool.Storage == "oceanstor-9000" {
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
