package proto

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"utils"
	"utils/log"
)

func GetISCSIInitiator() (string, error) {
	output, err := utils.ExecShellCmd("awk 'BEGIN{FS=\"=\";ORS=\"\"}/^InitiatorName=/{print $2}' /etc/iscsi/initiatorname.iscsi")
	if err != nil {
		if strings.Contains(output, "cannot open file") {
			msg := "No ISCSI initiator exist"
			log.Errorln(msg)
			return "", errors.New(msg)
		}

		log.Errorf("Get ISCSI initiator error: %v", output)
		return "", err
	}

	return output, nil
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
