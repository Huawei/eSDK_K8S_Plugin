package connector

import (
	"fmt"
	"utils/log"
)

const (
	FCDriver               = "fibreChannel"
	FCNVMeDriver           = "FC-NVMe"
	ISCSIDriver            = "iSCSI"
	RoCEDriver             = "RoCE"
	LocalDriver            = "Local"
	NFSDriver              = "NFS"
	MountFSType            = "fs"
	MountBlockType         = "block"
	flushMultiPathInternal = 20
	intNumFour             = 4
)

var connectors = map[string]Connector{}

type Connector interface {
	ConnectVolume(map[string]interface{}) (string, error)
	DisConnectVolume(string) error
}

func GetConnector(cType string) Connector {
	if cnt, exist := connectors[cType]; exist {
		return cnt
	}

	log.Errorf("%s is not registered to connector", cType)
	return nil
}

func RegisterConnector(cType string, cnt Connector) error {
	if _, exist := connectors[cType]; exist {
		return fmt.Errorf("connector %s already exists", cType)
	}

	connectors[cType] = cnt
	return nil
}
