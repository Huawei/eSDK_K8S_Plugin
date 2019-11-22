package dev

import (
	"strings"
	"time"
	"utils"
	"utils/log"
)

func GetDev(wwn string) (string, error) {
	output, err := utils.ExecShellCmd("ls -l /dev/disk/by-id/ | grep %s", wwn)
	if err != nil {
		if strings.TrimSpace(output) == "" {
			return "", nil
		}

		return "", err
	}

	devLines := strings.Split(output, "\n")
	for _, line := range devLines {
		splits := strings.Split(line, "../../")
		if len(splits) >= 2 {
			if strings.HasPrefix(splits[1], "dm") {
				return splits[1], nil
			} else if strings.HasPrefix(splits[1], "sd") {
				log.Warningf("Only sd dev %s was found, maybe multipath isn't installed or there're not multipaths", splits[1])
				return splits[1], nil
			}
		}
	}

	return "", nil
}

func rescan(protocol string) error {
	var hostClass string

	if protocol == "iscsi" {
		hostClass = "iscsi_host"
	} else {
		hostClass = "fc_host"
	}

	output, err := utils.ExecShellCmd("for host in $(ls /sys/class/%s/); do echo \"- - -\" > /sys/class/scsi_host/${host}/scan; done", hostClass)
	if err != nil {
		log.Errorf("rescan %s error: %s", hostClass, output)
		return err
	}

	return nil
}

func deleteDMDev(dm string) error {
	output, err := utils.ExecShellCmd("for sd in $(ls /sys/block/%s/slaves/); do echo \"1\" > /sys/block/${sd}/device/delete; done", dm)
	if err != nil {
		log.Errorf("Delete DM device %s error: %v", dm, output)
		return err
	}

	time.Sleep(time.Second * 2)

	output, _ = utils.ExecShellCmd("multipath -l | grep %s", dm)
	if strings.TrimSpace(output) != "" {
		utils.ExecShellCmd("multipath -F")
	}

	return nil
}

func deleteSDDev(sd string) error {
	output, err := utils.ExecShellCmd("echo \"1\" > /sys/block/%s/device/delete", sd)
	if err != nil {
		log.Errorf("Delete SD device %s error: %v", sd, output)
		return err
	}

	return nil
}

func DeleteDev(wwn string) error {
	for {
		device, err := GetDev(wwn)
		if err != nil {
			log.Errorf("Get device of WWN %s error: %v", wwn, err)
			return err
		}

		if strings.HasPrefix(device, "dm") {
			err = deleteDMDev(device)
		} else if strings.HasPrefix(device, "sd") {
			err = deleteSDDev(device)
		} else {
			log.Warningf("Device of WWN %s to delete does not exist anymore", wwn)
			return nil
		}

		if err != nil {
			log.Errorf("Delete %s error: %v", device, err)
			return err
		}

		time.Sleep(time.Second * 5)
	}
}

func ScanDev(wwn, protocol string) string {
	rescan(protocol)
	time.Sleep(time.Second * 5)

	device, _ := GetDev(wwn)
	if device == "" {
		log.Warningf("Device of WWN %s wasn't found yet, will rescan and check again", wwn)
		output, err := utils.ExecShellCmd("multipath -F; for dev in $(ls /sys/block/); do echo 1 > /sys/block/${dev}/device/rescan; done; multipath -v2")
		if err != nil {
			log.Warningf("Rescan error: %v", output)
		}

		device, _ = GetDev(wwn)
	}

	if device == "" {
		log.Errorf("Device of WWN %s cannot be detected", wwn)
		return ""
	}

	log.Infof("Device %s was found", device)
	return device
}

func MountLunDev(dev, targetPath, fsType, flags string) error {
	output, err := utils.ExecShellCmd("blkid -o udev %s | grep ID_FS_UUID | cut -d = -f2", dev)
	if err != nil {
		log.Errorf("Query fs of %s error: %s", dev, output)
		return err
	}

	if fsType == "" {
		fsType = "ext4"
	}

	if output == "" {
		output, err = utils.ExecShellCmd("mkfs -t %s -F %s", fsType, dev)
		if err != nil {
			log.Errorf("Couldn't mkfs %s to %s: %s", dev, fsType, output)
			return err
		}
	}

	if flags != "" {
		output, err = utils.ExecShellCmd("mount %s %s -o %s", dev, targetPath, flags)
	} else {
		output, err = utils.ExecShellCmd("mount %s %s", dev, targetPath)
	}
	if err != nil {
		if strings.Contains(output, "is already mounted") {
			return nil
		}

		log.Errorf("Mount %s to %s error: %s", dev, targetPath, output)
		return err
	}

	return nil
}

func MountFsDev(exportPath, targetPath, flags string) error {
	var output string
	var err error

	if flags != "" {
		output, err = utils.ExecShellCmd("mount %s %s -o %s", exportPath, targetPath, flags)
	} else {
		output, err = utils.ExecShellCmd("mount %s %s", exportPath, targetPath)
	}

	if err != nil {
		if strings.Contains(output, "is already mounted") {
			return nil
		}

		log.Errorf("Mount %s to %s error: %s", exportPath, targetPath, output)
		return err
	}

	return nil
}

func Unmount(targetPath string) error {
	output, err := utils.ExecShellCmd("umount %s", targetPath)
	if err != nil && !strings.Contains(output, "not mounted") {
		log.Errorf("Unmount %s error: %s", targetPath, output)
		return err
	}

	return nil
}

func WaitDevOnline(devPath string) {
	var i int
	for i = 0; i < 30; i++ {
		output, _ := utils.ExecShellCmd("ls -l %s", devPath)
		if strings.Contains(output, "No such file or directory") {
			time.Sleep(time.Second * 2)
			continue
		} else if strings.Contains(output, devPath) {
			return
		}
	}

	log.Warningf("Wait dev %s online timeout", devPath)
}

func CheckMultiPathAvailable() error {
	_, err := utils.ExecShellCmd("multipath -l")
	return err
}
