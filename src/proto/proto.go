package proto

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"strings"
	"utils"
	"utils/log"
)

func GetISCSIInitiator() (string, error) {

	initiatornameBuf, err := ioutil.ReadFile("/etc/iscsi/initiatorname.iscsi")

	var initiatorname string;
	if err != nil {
		msg := "Error reading file /etc/iscsi/initiatorname.iscsi"
		log.Errorln(msg)
		return "", errors.New(msg)
	}

	for _, line := range strings.Split(string(initiatornameBuf), "\n") {
		if strings.TrimSpace(line) != "" {
			splitValue := strings.Split(line, "=")
			if len(splitValue) == 2 {
				initiatorname = string(splitValue[1])
			} else {
				msg := "bad content for file /etc/iscsi/initiatorname.iscsi"
				log.Errorln(msg)
				return "", errors.New(msg)
			}
			break
		} else {
			msg := "empty file /etc/iscsi/initiatorname.iscsi"
			log.Errorln(msg)
			return "", errors.New(msg)
		}
	}

	return initiatorname, nil
}

func GetFCInitiator() ([]string, error) {
	output, err := utils.ExecShellCmd("cat /sys/class/fc_host/host*/port_name | awk 'BEGIN{FS=\"0x\";ORS=\" \"}{print $2}'")
	if err != nil {
		log.Errorf("Get FC initiator error: %v", output)
		return nil, err
	}

	if strings.Contains(output, "No such file or directory") {
		msg := "No FC initiator exist"
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	return strings.Fields(output), nil
}

func GetRoCEInitiator() (string, error) {
	output, err := utils.ExecShellCmd("cat /etc/nvme/hostnqn")
	if err != nil {
		if strings.Contains(output, "No such file or directory") {
			msg := "No NVME initiator exists"
			log.Errorln(msg)
			return "", errors.New(msg)
		}

		log.Errorf("Get NVME initiator error: %v", output)
		return "", err
	}

	return strings.TrimRight(output, "\n"), nil
}

func VerifyIscsiPortals(portals []interface{}) ([]string, error) {
	if len(portals) < 1 {
		return nil, errors.New("At least 1 portal must be provided for iscsi backend")
	}

	var verifiedPortals []string

	for _, i := range portals {
		portal := i.(string)
		ip := net.ParseIP(portal)
		if ip == nil {
			return nil, fmt.Errorf("%s of portals is invalid", portal)
		}

		verifiedPortals = append(verifiedPortals, portal)
	}

	return verifiedPortals, nil
}
