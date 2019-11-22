package backend

import (
	"csi/backend/plugin"
	"errors"
	"fmt"
	"regexp"
	"sync"
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
}

func analyzePools(backend *Backend, config map[string]interface{}) error {
	configPools, exist := config["pools"].([]interface{})
	if !exist || len(configPools) <= 0 {
		return fmt.Errorf("pools must be configured for backend %s", backend.Name)
	}

	var pools []*StoragePool

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

	if len(pools) <= 0 {
		return fmt.Errorf("No valid pools configured for backend %s", backend.Name)
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

	return &Backend{
		Name:       backendName,
		Storage:    storage,
		Available:  false,
		Plugin:     plugin,
		Parameters: parameters,
	}, nil
}

func RegisterBackend(backendConfigs []map[string]interface{}) error {
	for _, config := range backendConfigs {
		backendName, exist := config["name"].(string)
		if !exist {
			msg := "Name must be configured for backend"
			log.Errorln(msg)
			return errors.New(msg)
		} else {
			match, err := regexp.MatchString(`^[A-Za-z-0-9]+$`, backendName)
			if err != nil || !match {
				msg := fmt.Sprintf("backend name %v is invalid, only support consisted by upper&lower characters, numeric and [-]", backendName)
				log.Errorln(msg)
				return errors.New(msg)
			}
		}

		if _, exist := csiBackends[backendName]; exist {
			msg := fmt.Sprintf("Backend name %s is duplicated", backendName)
			log.Errorln(msg)
			return errors.New(msg)
		}

		backend, err := newBackend(backendName, config)
		if err != nil {
			log.Errorf("New backend %s error: %v", backendName, err)
			return err
		}

		err = analyzePools(backend, config)
		if err != nil {
			log.Errorf("Analyze pools config of backend %s error: %v", backendName, err)
			return err
		}

		csiBackends[backendName] = backend
	}

	return nil
}

func GetBackend(backendName string) *Backend {
	return csiBackends[backendName]
}

func SelectStoragePool(requestSize int64, parameters map[string]string) (*StoragePool, error) {
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
		value, _ := parameters[key]
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

	log.Infof("Select storage pool %s:%s for volume (%d, %v)", selectPool.Parent, selectPool.Name, requestSize, parameters)
	freeCapacity, _ := selectPool.Capabilities["FreeCapacity"].(int64)
	selectPool.Capabilities["FreeCapacity"] = freeCapacity - requestSize

	return selectPool, nil
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
			if pool.Storage == "oceanstor-nas" {
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

		if allocType == "thin" || allocType == "" {
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
