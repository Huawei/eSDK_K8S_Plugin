package plugin

import (
	"connector"
	// init the nfs connector
	_ "connector/nfs"
	"errors"
	"utils"
	"utils/log"
)

type Plugin interface {
	NewPlugin() Plugin
	Init(map[string]interface{}, map[string]interface{}, bool) error
	CreateVolume(string, map[string]interface{}) (utils.Volume, error)
	DeleteVolume(string) error
	ExpandVolume(string, int64) (bool, error)
	AttachVolume(string, map[string]interface{}) error
	DetachVolume(string, map[string]interface{}) error
	UpdateBackendCapabilities() (map[string]interface{}, error)
	UpdatePoolCapabilities([]string) (map[string]interface{}, error)
	StageVolume(string, map[string]interface{}) error
	UnstageVolume(string, map[string]interface{}) error
	UnstageVolumeWithWWN(string) error
	UpdateMetroRemotePlugin(Plugin)
	UpdateReplicaRemotePlugin(Plugin)
	NodeExpandVolume(string, string) error
	CreateSnapshot(string, string) (map[string]interface{}, error)
	DeleteSnapshot(string, string) error
	Logout()

	SmartXQoSQuery
}

// SmartXQoSQuery provides Quality of Service(QoS) Query operations
type SmartXQoSQuery interface {
	// SupportQoSParameters checks requested QoS parameters support by Plugin
	SupportQoSParameters(qos string) error
}

var (
	plugins = map[string]Plugin{}
)

const (
	// SectorSize means Sector size
	SectorSize int64 = 512
)

func RegPlugin(storageType string, plugin Plugin) {
	plugins[storageType] = plugin
}

func GetPlugin(storageType string) Plugin {
	if plugin, exist := plugins[storageType]; exist {
		return plugin.NewPlugin()
	}

	return nil
}

type basePlugin struct {
}

func (p *basePlugin) AttachVolume(string, map[string]interface{}) error {
	return nil
}

func (p *basePlugin) DetachVolume(string, map[string]interface{}) error {
	return nil
}

func (p *basePlugin) UpdateMetroRemotePlugin(Plugin) {
}

func (p *basePlugin) UpdateReplicaRemotePlugin(Plugin) {
}

func (p *basePlugin) stageVolume(connectInfo map[string]interface{}) error {
	conn := connector.GetConnector(connector.NFSDriver)
	_, err := conn.ConnectVolume(connectInfo)
	if err != nil {
		log.Errorf("Mount share %s to %s error: %v", connectInfo["sourcePath"].(string),
			connectInfo["targetPath"].(string), err)
		return err
	}

	return nil
}

func (p *basePlugin) fsStageVolume(name, portal string, parameters map[string]interface{}) error {
	fsName := utils.GetFileSystemName(name)
	connectInfo := map[string]interface{}{
		"srcType":    connector.MountFSType,
		"sourcePath": portal + ":/" + fsName,
		"targetPath": parameters["targetPath"].(string),
		"mountFlags": parameters["mountFlags"].(string),
	}

	return p.stageVolume(connectInfo)
}

func (p *basePlugin) unstageVolume(name string, parameters map[string]interface{}) error {
	targetPath, exist := parameters["targetPath"].(string)
	if !exist {
		return errors.New("unstageVolume parameter targetPath does not exist")
	}

	conn := connector.GetConnector(connector.NFSDriver)
	err := conn.DisConnectVolume(targetPath)
	if err != nil {
		log.Errorf("Cannot unmount %s error: %v", name, err)
		return err
	}

	return nil
}

func (p *basePlugin) lunStageVolume(name, devPath string, parameters map[string]interface{}) error {
	connectInfo := map[string]interface{}{
		"fsType":     parameters["fsType"].(string),
		"srcType":    connector.MountBlockType,
		"sourcePath": devPath,
		"targetPath": parameters["targetPath"].(string),
		"mountFlags": parameters["mountFlags"].(string),
	}

	return p.stageVolume(connectInfo)
}

func (p *basePlugin) lunConnectVolume(connectInfo *connector.ConnectInfo) (string, error) {
	device, err := connectInfo.Conn.ConnectVolume(connectInfo.MappingInfo)
	if err != nil {
		log.Errorf("connect volume %s error: %v", connectInfo.MappingInfo, err)
	}
	return device, err
}

func (p *basePlugin) lunDisconnectVolume(disconnectInfo *connector.DisConnectInfo) error {
	err := disconnectInfo.Conn.DisConnectVolume(disconnectInfo.TgtLun)
	if err != nil {
		log.Errorf("disconnect volume %s error: %v", disconnectInfo.TgtLun, err)
	}
	return err
}

// UnstageVolumeWithWWN does node side volume cleanup using lun WWN
func (p *basePlugin) UnstageVolumeWithWWN(wwn string) error {
	return nil
}
