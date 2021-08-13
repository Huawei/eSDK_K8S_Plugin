package attacher

import (
	"connector"
	_ "connector/fibrechannel"
	_ "connector/iscsi"
	_ "connector/nvme"
	_ "connector/roce"
	"errors"
	"fmt"
	"utils/log"
)

func iSCSIControllerAttach(attacher AttacherPlugin, lunName string, parameters map[string]interface{}) (string, error) {
	connectInfo, err := attacher.ControllerAttach(lunName, parameters)
	if err != nil {
		return "", err
	}

	connectInfo["volumeUseMultiPath"] = parameters["volumeUseMultiPath"].(bool)
	conn := connector.GetConnector(connector.ISCSIDriver)
	devPath, err := conn.ConnectVolume(connectInfo)
	if err != nil {
		return "", err
	}

	return devPath, nil
}

func fcControllerAttach(attacher AttacherPlugin, lunName string,
	parameters map[string]interface{}) (string, error) {
	connectInfo, err := attacher.ControllerAttach(lunName, parameters)
	if err != nil {
		return "", err
	}

	connectInfo["volumeUseMultiPath"] = parameters["volumeUseMultiPath"].(bool)
	conn := connector.GetConnector(connector.FCDriver)
	devPath, err := conn.ConnectVolume(connectInfo)
	if err != nil {
		return "", err
	}

	return devPath, nil
}

func roceControllerAttach(attacher AttacherPlugin, lunName string, parameters map[string]interface{}) (string, error) {
	connectInfo, err := attacher.ControllerAttach(lunName, parameters)
	if err != nil {
		return "", err
	}

	connectInfo["volumeUseMultiPath"] = parameters["volumeUseMultiPath"].(bool)
	conn := connector.GetConnector(connector.RoCEDriver)
	devPath, err := conn.ConnectVolume(connectInfo)
	if err != nil {
		return "", err
	}

	return devPath, nil
}

func fcNVMeControllerAttach(attacher AttacherPlugin, lunName string,
	parameters map[string]interface{}) (string, error) {
	connectInfo, err := attacher.ControllerAttach(lunName, parameters)
	if err != nil {
		return "", err
	}

	conn := connector.GetConnector(connector.FCNVMeDriver)
	devPath, err := conn.ConnectVolume(connectInfo)
	if err != nil {
		return "", err
	}

	return devPath, nil
}

func disConnectVolume(tgtLunWWN, protocol string) error {
	var conn connector.Connector
	switch protocol {
	case "iscsi":
		conn = connector.GetConnector(connector.ISCSIDriver)
	case "fc":
		conn = connector.GetConnector(connector.FCDriver)
	case "roce":
		conn = connector.GetConnector(connector.RoCEDriver)
	case "fc-nvme":
		conn = connector.GetConnector(connector.FCNVMeDriver)
	default:
		msg := fmt.Sprintf("the protocol %s is not valid", protocol)
		log.Errorln(msg)
		return errors.New(msg)
	}

	err := conn.DisConnectVolume(tgtLunWWN)
	if err != nil {
		log.Errorf("Delete dev %s error: %v", tgtLunWWN, err)
		return err
	}
	return nil
}

func connectVolume(attacher AttacherPlugin, lunName, protocol string, parameters map[string]interface{}) (string, error) {
	switch protocol {
	case "iscsi":
		return iSCSIControllerAttach(attacher, lunName, parameters)
	case "fc":
		return fcControllerAttach(attacher, lunName, parameters)
	case "roce":
		return roceControllerAttach(attacher, lunName, parameters)
	case "fc-nvme":
		return fcNVMeControllerAttach(attacher, lunName, parameters)
	default:
		return "", nil
	}
}

