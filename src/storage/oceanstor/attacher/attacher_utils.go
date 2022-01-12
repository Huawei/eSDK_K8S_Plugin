package attacher

import (
	"errors"
	"fmt"

	"connector"
	_ "connector/fibrechannel"
	_ "connector/iscsi"
	_ "connector/nvme"
	_ "connector/roce"
	"utils/log"
)

func disConnectVolume(tgtLunWWN, protocol string) (*connector.DisConnectInfo, error) {
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
		return nil, errors.New(msg)
	}

	return &connector.DisConnectInfo{
		Conn:   conn,
		TgtLun: tgtLunWWN,
	}, nil
}

func connectVolume(attacher AttacherPlugin, lunName, protocol string, parameters map[string]interface{}) (*connector.ConnectInfo, error) {
	mappingInfo, err := attacher.ControllerAttach(lunName, parameters)
	if err != nil {
		return nil, err
	}

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
		return nil, errors.New(msg)
	}

	volumeUseMultiPath, ok := parameters["volumeUseMultiPath"].(bool)
	if !ok {
		volumeUseMultiPath = true
	}
	mappingInfo["volumeUseMultiPath"] = volumeUseMultiPath

	return &connector.ConnectInfo{
		Conn:        conn,
		MappingInfo: mappingInfo,
	}, nil
}
