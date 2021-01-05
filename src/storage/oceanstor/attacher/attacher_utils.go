package attacher

import (
	"errors"
	"fmt"

	"github.com/Huawei/eSDK_K8S_Plugin/src/connector"
	_ "github.com/Huawei/eSDK_K8S_Plugin/src/connector/fibrechannel"
	_ "github.com/Huawei/eSDK_K8S_Plugin/src/connector/iscsi"
	_ "github.com/Huawei/eSDK_K8S_Plugin/src/connector/nvme"
	_ "github.com/Huawei/eSDK_K8S_Plugin/src/connector/roce"
	"github.com/Huawei/eSDK_K8S_Plugin/src/utils/log"
)

func iSCSIControllerAttach(attacher AttacherPlugin, lunName string,
	parameters map[string]interface{}, tgtPortals []string) (string, error) {
	wwn, err := attacher.ControllerAttach(lunName, parameters)
	if err != nil {
		return "", err
	}

	lenPortals := len(tgtPortals)
	var tgtLunWWNs []string
	for i := 0; i < lenPortals; i++ {
		tgtLunWWNs = append(tgtLunWWNs, wwn)
	}
	connMap := map[string]interface{}{
		"tgtPortals": tgtPortals,
		"tgtLunWWNs": tgtLunWWNs,
	}

	conn := connector.GetConnector(connector.ISCSIDriver)
	devPath, err := conn.ConnectVolume(connMap)
	if err != nil {
		return "", err
	}

	return devPath, nil
}

func fcControllerAttach(attacher AttacherPlugin, lunName string,
	parameters map[string]interface{}) (string, error) {
	wwn, err := attacher.ControllerAttach(lunName, parameters)
	if err != nil {
		return "", err
	}

	connMap := map[string]interface{}{
		"tgtLunWWN": wwn,
	}

	conn := connector.GetConnector(connector.FCDriver)
	devPath, err := conn.ConnectVolume(connMap)
	if err != nil {
		return "", err
	}

	return devPath, nil
}

func roceControllerAttach(attacher AttacherPlugin, lunName string,
	parameters map[string]interface{}, tgtPortals []string) (string, error) {
	lunGuid, err := attacher.ControllerAttach(lunName, parameters)
	if err != nil {
		return "", err
	}

	lenPortals := len(tgtPortals)
	var tgtLunGuids []string
	for i := 0; i < lenPortals; i++ {
		tgtLunGuids = append(tgtLunGuids, lunGuid)
	}
	connMap := map[string]interface{}{
		"tgtPortals":  tgtPortals,
		"tgtLunGuids": tgtLunGuids,
	}

	conn := connector.GetConnector(connector.RoCEDriver)
	devPath, err := conn.ConnectVolume(connMap)
	if err != nil {
		return "", err
	}

	return devPath, nil
}

func fcNVMeControllerAttach(attacher AttacherPlugin, lunName string,
	parameters map[string]interface{}) (string, error) {
	wwn, err := attacher.ControllerAttach(lunName, parameters)
	if err != nil {
		return "", err
	}

	connMap := map[string]interface{}{
		"tgtLunGuid": wwn,
	}

	conn := connector.GetConnector(connector.FCNVMeDriver)
	devPath, err := conn.ConnectVolume(connMap)
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
		tgtPortals, err := attacher.getTargetISCSIPortals()
		if err != nil {
			return "", err
		}
		return iSCSIControllerAttach(attacher, lunName, parameters, tgtPortals)
	case "fc":
		return fcControllerAttach(attacher, lunName, parameters)
	case "roce":
		tgtPortals, err := attacher.getTargetRoCEPortals()
		if err != nil {
			return "", err
		}
		return roceControllerAttach(attacher, lunName, parameters, tgtPortals)
	case "fc-nvme":
		return fcNVMeControllerAttach(attacher, lunName, parameters)
	default:
		return "", nil
	}
}
