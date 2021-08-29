package nvme

import (
	"connector"
	"time"
	"utils/log"
)

func tryDisConnectVolume(tgtLunWWN string, checkDeviceAvailable bool) error {
	device, err := connector.GetDevice(nil, tgtLunWWN, checkDeviceAvailable)
	if err != nil {
		log.Warningf("Get device of WWN %s error: %v", tgtLunWWN, err)
		return err
	}

	multiPathName, err := connector.RemoveNvmeFcDevice(device)
	if err != nil {
		log.Errorf("Remove device %s error: %v", device, err)
		return err
	}

	if multiPathName != "" {
		time.Sleep(time.Second * intNumThree)
		err = connector.FlushDMDevice(device)
		if err != nil {
			return err
		}
	}

	return nil
}
