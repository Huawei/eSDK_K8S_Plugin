package dev

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Huawei/eSDK_K8S_Plugin/src/connector"
	"github.com/Huawei/eSDK_K8S_Plugin/src/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/src/utils/log"
)

func GetDev(wwn string) (string, error) {
	output, err := utils.ExecShellCmd("ls -l /dev/disk/by-id/ | grep %s", wwn)
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

			if len(dev) == 0 && strings.HasPrefix(name, "sd") {
				log.Warningf("sd dev %s was found, may multipath isn't installed or there're no multipaths", name)
				dev = name
			}

			if len(dev) == 0 && strings.HasPrefix(name, "nvme") {
				log.Warningf("nvme dev %s was found, may multipath isn't installed or there're no multipaths", name)
				dev = name
			}
		}
	}

	if len(dev) != 0 {
		devPath := fmt.Sprintf("/dev/%s", dev)
		if exist, _ := utils.PathExist(devPath); !exist {
			return "", nil
		}
	}

	return dev, nil
}

func deleteDMDev(dm string) error {
	mPath, err := utils.ExecShellCmd("ls -l /dev/mapper/ | grep -w %s | awk '{print $9}'", dm)
	if err != nil {
		log.Errorf("Get DM device %s error: %v", dm, mPath)
		return err
	}

	if mPath == "" {
		log.Infof("The DM device %s does not exist, return success.", dm)
		return nil
	}

	output, err := utils.ExecShellCmd("for sd in $(ls /sys/block/%s/slaves/); do echo 1 > /sys/block/${sd}/device/delete; done", dm)
	if err != nil {
		log.Errorf("Delete DM device %s error: %v", dm, output)
		return err
	}

	time.Sleep(time.Second * 2)

	utils.ExecShellCmd("multipath -f %s", mPath)

	return nil
}

func deleteSDDev(sd string) error {
	output, err := utils.ExecShellCmd("echo 1 > /sys/block/%s/device/delete", sd)
	if err != nil {
		if strings.Contains(output, "No such file or directory") {
			return nil
		}

		log.Errorf("Delete SD device %s error: %v", sd, output)
		return err
	}

	return nil
}

func DeleteDev(wwn string) error {
	// Retry 10 times most, to avoid infinite loop
	for i := 0; i < 10; i++ {
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

		time.Sleep(time.Second * 2)
	}

	return fmt.Errorf("delete device of WWN %s timeout", wwn)
}

func MountLunDev(dev, targetPath, fsType, flags string) error {
	var resize bool
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
	} else {
		resize = true
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

	if resize {
		err = ResizeMountPath(targetPath)
		if err != nil {
			log.Errorf("Resize mount path %s err %s", targetPath, err)
			return err
		}
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

func BlockResize(wwn string) error {
	device, err := GetDev(wwn)
	if err != nil {
		log.Errorf("Get device of WWN %s error: %v", wwn, err)
		return err
	}

	if strings.HasPrefix(device, "dm") {
		err = resizeDMDev(device)
	} else if strings.HasPrefix(device, "sd") {
		err = resizeSDDev(device)
	} else if strings.HasPrefix(device, "nvme") {
		devices := []string{device}
		err = resizeNVMeDev(devices)
	} else {
		msg := fmt.Sprintf("Device of WWN %s to resize does not exist anymore", wwn)
		log.Errorln(msg)
		return errors.New(msg)
	}

	if err != nil {
		log.Errorf("Resize %s error: %v", device, err)
		return err
	}

	return nil
}

func resizeDMDev(dm string) error {
	output, err := utils.ExecShellCmd("ls /sys/block/%s/slaves/", dm)
	if strings.Contains(output, "nvme") {
		devices := strings.Split(output, "\n")
		err = resizeNVMeDev(devices)
		if err != nil {
			log.Errorf("Rescan nvme error: %v", output)
			return err
		}
	} else {
		output, err = utils.ExecShellCmd("for sd in $(ls /sys/block/%s/slaves/); do echo 1 > /sys/block/${sd}/device/rescan; done", dm)
		if err != nil {
			log.Errorf("Rescan DM device %s error: %v", dm, output)
			return err
		}
	}

	time.Sleep(time.Second * 2)
	_, err = utils.ExecShellCmd("multipathd resize map %s", dm)
	if err != nil {
		log.Errorf("Resize DM device %s error: %v", dm, err)
		return err
	}

	return nil
}

func resizeSDDev(sd string) error {
	output, err := utils.ExecShellCmd("echo 1 > /sys/block/%s/device/rescan", sd)
	if err != nil {
		log.Errorf("Rescan device %s error: %v", sd, output)
		return err
	}

	return nil
}

func resizeNVMeDev(devices []string) error {
	connector.ReScanNVMe(devices)
	return nil
}

func ResizeMountPath(volumePath string) error {
	output, err := utils.ExecShellCmd("findmnt -o source --noheadings --target %s", volumePath)
	if err != nil {
		return fmt.Errorf("findmnt volumePath: %s error: %v", volumePath, err)
	}

	devicePath := strings.TrimSpace(output)
	if len(devicePath) == 0 {
		return fmt.Errorf("could not get valid device for mount path: %s", volumePath)
	}

	fsType, err := utils.ExecShellCmd("blkid -p -s TYPE -o value %s", devicePath)
	if err != nil {
		log.Errorf("blkid %s error: %s", devicePath, err)
		return err
	}

	if fsType == "" {
		return nil
	}

	fsType = strings.Trim(fsType, "\n")
	switch fsType {
	case "ext2", "ext3", "ext4":
		return extResize(devicePath)
	}

	return fmt.Errorf("resize of format %s is not supported for device %s", fsType, devicePath)
}

func extResize(devicePath string) error {
	output, err := utils.ExecShellCmd("resize2fs %s", devicePath)
	if err != nil {
		log.Errorf("Resize %s error: %s", devicePath, output)
		return err
	}

	log.Infof("Resize success for device path : %v", devicePath)
	return nil
}
