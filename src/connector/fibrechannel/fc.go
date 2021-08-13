package fibrechannel

import (
	"connector"
	"errors"
	"fmt"
	"sync"
	"time"
	"utils/log"
)

type FibreChannel struct {
	mutex sync.Mutex
}

func init() {
	connector.RegisterConnector(connector.FCDriver, &FibreChannel{})
}

func normalConnect(conn map[string]interface{}) (string, error) {
	tgtLunWWN, exist := conn["tgtLunWWN"].(string)
	if !exist {
		msg := "there is no Lun WWN in connect info"
		log.Errorln(msg)
		return "", errors.New(msg)
	}

	scanHost()
	var device string
	var findDeviceMap map[string]string
	for i := 0; i < 5; i++ {
		time.Sleep(time.Second * 3)
		device, _ = connector.GetDevice(findDeviceMap, tgtLunWWN, true)
		if device != "" {
			break
		}

		log.Warningf("Device of WWN %s wasn't found yet, will wait and check again", tgtLunWWN)
	}

	if device == "" {
		msg := fmt.Sprintf("Cannot detect device %s", tgtLunWWN)
		log.Errorln(msg)
		return "", errors.New(msg)
	}

	return fmt.Sprintf("/dev/%s", device), nil
}

func (fc *FibreChannel) ConnectVolume(conn map[string]interface{}) (string, error) {
	fc.mutex.Lock()
	defer fc.mutex.Unlock()
	log.Infof("FC Start to connect volume ==> connect info: %v", conn)

	for i := 0; i < 3; i++ {
		device, err := tryConnectVolume(conn)
		if err != nil && err.Error() == "command not found" {
			break
		} else if err != nil && err.Error() == "NoFibreChannelVolumeDeviceFound" {
			time.Sleep(time.Second * 3)
			continue
		} else {
			return device, err
		}
	}

	return normalConnect(conn)
}

func (fc *FibreChannel) DisConnectVolume(tgtLunWWN string) error {
	fc.mutex.Lock()
	defer fc.mutex.Unlock()
	log.Infof("FC Start to disconnect volume ==> volume wwn is: %v", tgtLunWWN)
	for i := 0; i < 3; i++ {
		err := tryDisConnectVolume(tgtLunWWN, true)
		if err == nil {
			return nil
		}
		time.Sleep(time.Second * 2)
		log.Errorf("Failed to delete device in %d time(s), err: %v", i, err)
	}

	msg := fmt.Sprintf("Failed to delete volume %s.", tgtLunWWN)
	log.Errorln(msg)
	return errors.New(msg)
}
