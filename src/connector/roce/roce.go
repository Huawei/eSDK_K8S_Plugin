package roce

import (
	"strings"
	"time"

	"github.com/Huawei/eSDK_K8S_Plugin/src/connector"
	"github.com/Huawei/eSDK_K8S_Plugin/src/utils/log"
)

type RoCE struct{}

func init() {
	connector.RegisterConnector(connector.RoCEDriver, &RoCE{})
}

func (roce *RoCE) ConnectVolume(conn map[string]interface{}) (string, error) {
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
	return "", nil
}

func (roce *RoCE) DisConnectVolume(tgtLunGuid string) error {
	log.Infof("RoCE Start to disconnect volume ==> Volume Guid info: %v", tgtLunGuid)
	return connector.DeleteDevice(tgtLunGuid)
}
