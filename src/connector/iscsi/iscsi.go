package iscsi

import (
	"connector"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"utils/log"
)

type iSCSI struct {
	mutex sync.Mutex
}

func init() {
	connector.RegisterConnector(connector.ISCSIDriver, &iSCSI{})
}

func (isc *iSCSI) ConnectVolume(conn map[string]interface{}) (string, error) {
	isc.mutex.Lock()
	defer isc.mutex.Unlock()
	log.Infof("iSCSI Start to connect volume ==> connect info: %v", conn)

	for i := 0; i < 3; i++ {
		device, err := tryConnectVolume(conn)
		if err != nil && strings.Contains(err.Error(), "volume device not found") {
			time.Sleep(time.Second * 3)
			continue
		} else {
			return device, err
		}
	}

	log.Errorln("final found no device.")
	return "", nil
}

func (isc *iSCSI) DisConnectVolume(tgtLunWWN string) error {
	isc.mutex.Lock()
	defer isc.mutex.Unlock()
	log.Infof("Start to disconnect volume ==> volume wwn is: %v", tgtLunWWN)
	for i := 0; i < 3; i++ {
		err := tryDisConnectVolume(tgtLunWWN)
		if err == nil {
			return nil
		}
		time.Sleep(time.Second * 2)
	}

	msg := fmt.Sprintf("failed to delete volume %s.", tgtLunWWN)
	log.Errorln(msg)
	return errors.New(msg)
}
