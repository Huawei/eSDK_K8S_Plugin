package fibrechannel

import (
	"connector"
	"errors"
	"fmt"
	"strings"
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

func (fc *FibreChannel) ConnectVolume(conn map[string]interface{}) (string, error) {
	fc.mutex.Lock()
	defer fc.mutex.Unlock()
	log.Infof("FC Start to connect volume ==> connect info: %v", conn)

	for i := 0; i < 3; i++ {
		device, err := tryConnectVolume(conn)
		if err != nil && strings.Contains(err.Error(), "command not found") {
			return "", err
		} else if err != nil && err.Error() == "NoFibreChannelVolumeDeviceFound" {
			time.Sleep(time.Second * 3)
			continue
		} else {
			return device, err
		}
	}

	log.Errorln("Final found no device.")
	return "", nil
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
