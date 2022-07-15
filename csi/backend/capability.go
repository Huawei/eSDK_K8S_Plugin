package backend

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"runtime/debug"
	"sync"

	"huawei-csi-driver/utils/log"
)

func updateBackendCapabilities(backend *Backend, sync bool) error {
	backendCapabilities, err := backend.Plugin.UpdateBackendCapabilities()
	if err != nil {
		log.Errorf("Cannot update backend %s capabilities: %v", backend.Name, err)
		return err
	}

	var poolNames []string
	for _, pool := range backend.Pools {
		poolNames = append(poolNames, pool.Name)
	}

	poolCapabilities, err := backend.Plugin.UpdatePoolCapabilities(poolNames)
	if err != nil {
		log.Errorf("Cannot update pool capabilities of backend %s: %v", backend.Name, err)
		return err
	}

	if sync && len(poolCapabilities) < len(poolNames) {
		msg := fmt.Sprintf("There're pools not available for backend %s", backend.Name)
		log.Errorln(msg)
		return errors.New(msg)
	}

	for _, pool := range backend.Pools {
		for k, v := range backendCapabilities {
			if cur, exist := pool.Capabilities[k]; !exist || cur != v {
				log.Infof("Update backend capability [%s] of pool [%s] of backend [%s] from %v to %v",
					k, pool.Name, pool.Parent, cur, v)
				pool.Capabilities[k] = v
			}
		}

		capabilities, exist := poolCapabilities[pool.Name].(map[string]interface{})
		if exist {
			for k, v := range capabilities {
				if cur, exist := pool.Capabilities[k]; !exist || !reflect.DeepEqual(cur, v) {
					log.Infof("Update pool capability [%s] of pool [%s] of backend [%s] from %v to %v",
						k, pool.Name, pool.Parent, cur, v)
					pool.Capabilities[k] = v
				}
			}
		} else {
			log.Warningf("Pool %s of backend %s does not exist, set it unavailable", pool.Name, pool.Parent)
			pool.Capabilities["FreeCapacity"] = 0
		}
	}

	return nil
}

func SyncUpdateCapabilities() error {
	for _, backend := range csiBackends {
		err := updateBackendCapabilities(backend, true)
		if err != nil {
			return err
		}

		backend.Available = true
	}

	return nil
}

func AsyncUpdateCapabilities(controllerFlagFile string) {
	var wait sync.WaitGroup

	mutex.Lock()
	defer mutex.Unlock()

	for _, backend := range csiBackends {
		if len(controllerFlagFile) > 0 {
			if _, err := os.Stat(controllerFlagFile); err != nil {
				backend.Available = false
				continue
			}
		}

		wait.Add(1)

		go func(b *Backend) {
			defer func() {
				wait.Done()

				if r := recover(); r != nil {
					log.Errorf("Runtime error caught in loop routine: %v", r)
					log.Errorf("%s", debug.Stack())
				}

				log.Flush()
			}()

			err := updateBackendCapabilities(b, false)
			if err != nil {
				log.Warningf("Update %s capabilities error, set it unavailable", b.Name)
				b.Available = false
			} else {
				b.Available = true
			}
		}(backend)
	}

	wait.Wait()
}

// LogoutBackend is to logout all storage backend
func LogoutBackend() {
	for _, backend := range csiBackends {
		log.Infof("Start to logout the backend %s", backend.Name)
		backend.Plugin.Logout(context.Background())
		backend.Available = false
	}
}
