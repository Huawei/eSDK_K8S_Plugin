package roce

import (
	"connector"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"utils/log"
)

type RoCE struct{
	mutex sync.Mutex
}

const (
	intNumTwo             = 2
	intNumThree           = 3
)

func init() {
	connector.RegisterConnector(connector.RoCEDriver, &RoCE{})
}

func (roce *RoCE) ConnectVolume(conn map[string]interface{}) (string, error) {
	roce.mutex.Lock()
	defer roce.mutex.Unlock()
	log.Infof("RoCE Start to connect volume ==> connect info: %v", conn)

	for i := 0; i < 3; i++ {
		dev, err := tryConnectVolume(conn)
		if err != nil && strings.Contains(err.Error(), "volume device not found") {
			time.Sleep(time.Second * 3)
			continue
		} else {
			return dev, err
		}
	}

	log.Errorln("final found no device.")
	return "", errors.New("final found no device")
}

func (roce *RoCE) DisConnectVolume(tgtLunGuid string) error {
	roce.mutex.Lock()
	defer roce.mutex.Unlock()
	log.Infof("RoCE Start to disconnect volume ==> Volume Guid info: %v", tgtLunGuid)
	for i := 0; i < 3; i++ {
		err := tryDisConnectVolume(tgtLunGuid, true)
		if err == nil {
			return nil
		}

		log.Errorf("Failed to delete device in %d time(s), err: %v", i, err)
		time.Sleep(time.Second * intNumTwo)
	}

	msg := fmt.Sprintf("Failed to delete volume %s.", tgtLunGuid)
	log.Errorln(msg)
	return errors.New(msg)
}
