package connector

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
	"utils"
	"utils/log"
)

func GetDevice(findDeviceMap map[string]string, tgtLunGuid string) (string, error) {
	output, err := utils.ExecShellCmd("ls -l /dev/disk/by-id/ | grep %s", tgtLunGuid)
	if err != nil {
		if strings.TrimSpace(output) == "" || strings.Contains(output, "No such file or directory") {
			return "", nil
		}

		return "", err
	}

	var dev string
	devLines := strings.Split(output, "\n")
	for _, line := range devLines {
		splits := strings.Split(line, "../../")
		if len(splits) >= 2 {
			name := splits[1]

			if strings.HasPrefix(name, "dm") {
				dev = name
				break
			}

			if _, exist := findDeviceMap[name]; !exist && strings.HasPrefix(name, "nvme") {
				dev = name
				break
			}

			if _, exist := findDeviceMap[name]; !exist && strings.HasPrefix(name, "sd") {
				dev = name
				break
			}
		}
	}

	if dev != "" {
		devPath := fmt.Sprintf("/dev/%s", dev)
		if exist, _ := utils.PathExist(devPath); !exist {
			return "", nil
		}
	}

	return dev, nil
}

func DeleteDMDev(dm string) error {
	output, err := utils.ExecShellCmd("ls /sys/block/%s/slaves/", dm)
	devices := strings.Split(output, "\n")
	for _, device := range devices {
		err = DeleteNVMEDev(device)
		if err != nil {
			log.Errorf("Delete nvme error: %v", device)
			return err
		}
	}

	time.Sleep(time.Second * 2)
	output, _ = utils.ExecShellCmd("multipath -l | grep %s", dm)
	if strings.TrimSpace(output) != "" {
		utils.ExecShellCmd("multipath -f %s", dm)
	}

	return nil
}

func DeleteNVMEDev(nvme string) error {
	output, err := utils.ExecShellCmd("echo 1 > /sys/block/%s/device/rescan_controller", nvme)
	if err != nil {
		if strings.Contains(output, "No such file or directory") {
			return nil
		}

		log.Errorf("Delete NVME device %s error: %v", nvme, output)
		return err
	}

	return nil
}

func DeleteDevice(tgtLunGuid string) error {
	var findDeviceMap map[string]string

	for i := 0; i < 10; i++ {
		device, err := GetDevice(findDeviceMap, tgtLunGuid)
		if err != nil {
			log.Errorf("Get device of GUID %s error: %v", tgtLunGuid, err)
			return err
		}

		if strings.HasPrefix(device, "dm") {
			err = DeleteDMDev(device)
		} else if match, _ := regexp.MatchString(`nvme[0-9]+n[0-9]+`, device); match {
			err = DeleteNVMEDev(device)
		} else {
			log.Warningf("Device of Guid %s to delete does not exist anymore", tgtLunGuid)
			return nil
		}

		if err != nil {
			log.Errorf("Delete %s error: %v", device, err)
			return err
		}

		time.Sleep(time.Second * 2)
	}

	return fmt.Errorf("delete device of Guid %s timeout", tgtLunGuid)
}

func ReScanNVMe(devices []string) {
	for _, device := range devices {
		if match, _ := regexp.MatchString(`nvme[0-9]+n[0-9]+`, device); match {
			output, err := utils.ExecShellCmd("echo 1 > /sys/block/%s/device/rescan_controller", device)
			if err != nil {
				log.Warningf("rescan nvme path error: %s", output)
			}
		} else if match, _ := regexp.MatchString(`nvme[0-9]+$`, device); match {
			output, err := utils.ExecShellCmd("nvme ns-rescan /dev/%s", device)
			if err != nil {
				log.Warningf("rescan nvme path error: %s", output)
			}
		}
	}
}

func ScanNVMe(connectInfo map[string]interface{}) {
	protocol := connectInfo["protocol"].(string)
	var devices []string
	if protocol == "iscsi" {
		output, err := utils.ExecShellCmd("nvme list-subsys -o json")
		if err != nil {
			log.Errorf("get exist nvme connect port error: %s", err)
			return
		}

		var nvmeConnectInfo map[string]interface{}
		if err = json.Unmarshal([]byte(output), &nvmeConnectInfo); err != nil {
			log.Errorf("Failed to unmarshal input %s", output)
			return
		}

		subSystems := nvmeConnectInfo["Subsystems"].([]interface{})
		var allSubPaths []interface{}
		for _, s := range subSystems {
			subSystem := s.(map[string]interface{})
			if strings.Contains(subSystem["NQN"].(string), connectInfo["targetNqn"].(string)) {
				allSubPaths = subSystem["Paths"].([]interface{})
				break
			}
		}

		for _, p := range allSubPaths {
			path := p.(map[string]interface{})
			devices = append(devices, path["Name"].(string))
		}
	} else {
		output, err := utils.ExecShellCmd("ls /dev | grep nvme")
		if err != nil {
			log.Errorf("get nvme path error: %s", output)
			return
		}

		devices = strings.Split(output, "\n")
	}

	ReScanNVMe(devices)
}
