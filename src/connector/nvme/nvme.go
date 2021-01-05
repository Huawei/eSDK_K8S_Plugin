package nvme

import (
	"errors"
	"fmt"
	"time"

	"github.com/Huawei/eSDK_K8S_Plugin/src/connector"
	"github.com/Huawei/eSDK_K8S_Plugin/src/utils/log"
)

type FCNVMe struct{}

func init() {
	connector.RegisterConnector(connector.FCNVMeDriver, &FCNVMe{})
}

func (fc *FCNVMe) ConnectVolume(conn map[string]interface{}) (string, error) {
	log.Infof("FC-NVMe Start to connect volume ==> connect info: %v", conn)
	tgtLunGuid, exist := conn["tgtLunGuid"].(string)
	if !exist {
		msg := "there is no Lun GUID in connect info"
		log.Errorln(msg)
		return "", errors.New(msg)
	}
	connectInfo := map[string]interface{}{
		"protocol": "fc",
	}
	connector.ScanNVMe(connectInfo)
	var device string
	var findDeviceMap map[string]string

	for i := 0; i < 5; i++ {
		time.Sleep(time.Second * 3)
		device, _ = connector.GetDevice(findDeviceMap, tgtLunGuid)
		if device != "" {
			break
		}
		log.Warningf("Device of GUID %s wasn't found yet, will wait and check again", tgtLunGuid)
	}

	if device == "" {
		msg := fmt.Sprintf("Cannot detect device %s", tgtLunGuid)
		log.Errorln(msg)
		return "", errors.New(msg)
	}

	return fmt.Sprintf("/dev/%s", device), nil
}

func (fc *FCNVMe) DisConnectVolume(tgtLunGuid string) error {
	log.Infof("FC-NVMe Start to disconnect volume ==> Volume Guid info: %v", tgtLunGuid)
	return connector.DeleteDevice(tgtLunGuid)
}
