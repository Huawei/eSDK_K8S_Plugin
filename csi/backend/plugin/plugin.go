package plugin

import (
	"context"
	"errors"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"huawei-csi-driver/connector"
	// init the nfs connector
	_ "huawei-csi-driver/connector/nfs"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

type Plugin interface {
	NewPlugin() Plugin
	Init(map[string]interface{}, map[string]interface{}, bool) error
	CreateVolume(context.Context, string, map[string]interface{}) (utils.Volume, error)
	DeleteVolume(context.Context, string) error
	ExpandVolume(context.Context, string, int64) (bool, error)
	AttachVolume(context.Context, string, map[string]interface{}) error
	DetachVolume(context.Context, string, map[string]interface{}) error
	UpdateBackendCapabilities() (map[string]interface{}, error)
	UpdatePoolCapabilities([]string) (map[string]interface{}, error)
	StageVolume(context.Context, string, map[string]interface{}) error
	UnstageVolume(context.Context, string, map[string]interface{}) error
	UnstageVolumeWithWWN(context.Context, string) error
	UpdateMetroRemotePlugin(Plugin)
	UpdateReplicaRemotePlugin(Plugin)
	NodeExpandVolume(context.Context, string, string, bool, int64) error
	CreateSnapshot(context.Context, string, string) (map[string]interface{}, error)
	DeleteSnapshot(context.Context, string, string) error
	SmartXQoSQuery
	Logout(context.Context)
}

// SmartXQoSQuery provides Quality of Service(QoS) Query operations
type SmartXQoSQuery interface {
	// SupportQoSParameters checks requested QoS parameters support by Plugin
	SupportQoSParameters(ctx context.Context, qos string) error
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

func (p *basePlugin) AttachVolume(context.Context, string, map[string]interface{}) error {
	return nil
}

func (p *basePlugin) DetachVolume(context.Context, string, map[string]interface{}) error {
	return nil
}

func (p *basePlugin) UpdateMetroRemotePlugin(Plugin) {
}

func (p *basePlugin) UpdateReplicaRemotePlugin(Plugin) {
}

func (p *basePlugin) stageVolume(ctx context.Context, connectInfo map[string]interface{}) error {
	conn := connector.GetConnector(ctx, connector.NFSDriver)
	_, err := conn.ConnectVolume(ctx, connectInfo)
	if err != nil {
		log.AddContext(ctx).Errorf("Mount share %s to %s error: %v", connectInfo["sourcePath"].(string),
			connectInfo["targetPath"].(string), err)
		return err
	}

	return nil
}

func (p *basePlugin) fsStageVolume(ctx context.Context,
	name, portal string,
	parameters map[string]interface{}) error {
	connectInfo := map[string]interface{}{
		"srcType":    connector.MountFSType,
		"sourcePath": portal + ":/" + name,
		"targetPath": parameters["targetPath"].(string),
		"mountFlags": parameters["mountFlags"].(string),
	}

	return p.stageVolume(ctx, connectInfo)
}

func (p *basePlugin) unstageVolume(ctx context.Context,
	name string,
	parameters map[string]interface{}) error {
	targetPath, exist := parameters["targetPath"].(string)
	if !exist {
		return errors.New("unstageVolume parameter targetPath does not exist")
	}

	conn := connector.GetConnector(ctx, connector.NFSDriver)
	err := conn.DisConnectVolume(ctx, targetPath)
	if err != nil {
		log.AddContext(ctx).Errorf("Cannot unmount %s error: %v", name, err)
		return err
	}

	return nil
}

func (p *basePlugin) lunStageVolume(ctx context.Context,
	name, devPath string,
	parameters map[string]interface{}) error {

	// If the request to stage is for volumeDevice of type Block and the devicePath
	// is provided then do not format and create FS and mount it. Simply create a
	// symlink to the devpath on the staging area
	if volMode, ok := parameters["volumeMode"].(string); ok && volMode == "Block" {
		log.AddContext(ctx).Infoln("The request to stage raw block device")
		mountpoint, ok := parameters["stagingPath"].(string)
		if !ok {
			errMsg := "Error in getting staging path"
			log.AddContext(ctx).Errorln(errMsg)
			return errors.New(errMsg)
		}
		err := utils.CreateSymlink(ctx, devPath, mountpoint)
		if err != nil {
			log.AddContext(ctx).Errorln("Error in staging device")
			return err
		}
		return nil
	}

	connectInfo := map[string]interface{}{
		"fsType":     parameters["fsType"].(string),
		"srcType":    connector.MountBlockType,
		"sourcePath": devPath,
		"targetPath": parameters["targetPath"].(string),
		"mountFlags": parameters["mountFlags"].(string),
		"accessMode": parameters["accessMode"].(csi.VolumeCapability_AccessMode_Mode),
	}

	return p.stageVolume(ctx, connectInfo)
}

func (p *basePlugin) lunConnectVolume(ctx context.Context,
	connectInfo *connector.ConnectInfo) (string, error) {
	device, err := connectInfo.Conn.ConnectVolume(ctx, connectInfo.MappingInfo)
	if err != nil {
		log.Errorf("connect volume %s error: %v", connectInfo.MappingInfo, err)
	}
	return device, err
}

func (p *basePlugin) lunDisconnectVolume(ctx context.Context,
	disconnectInfo *connector.DisConnectInfo) error {
	err := disconnectInfo.Conn.DisConnectVolume(ctx, disconnectInfo.TgtLun)
	if err != nil {
		log.Errorf("disconnect volume %s error: %v", disconnectInfo.TgtLun, err)
	}
	return err
}

// UnstageVolumeWithWWN does node side volume cleanup using lun WWN
func (p *basePlugin) UnstageVolumeWithWWN(ctx context.Context, wwn string) error {
	return nil
}
