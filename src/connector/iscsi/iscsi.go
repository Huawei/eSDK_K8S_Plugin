package iscsi

import (
	"connector"
	"dev"
	"strings"
	"time"
	"utils/log"
)

type iSCSI struct{}

func init() {
	connector.RegisterConnector(connector.ISCSIDriver, &iSCSI{})
}

func (isc *iSCSI) ConnectVolume(conn map[string]interface{}) (string, error) {
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
	log.Infof("Start to disconnect volume ==> volume wwn is: %v", tgtLunWWN)
	return dev.DeleteDev(tgtLunWWN)
}
