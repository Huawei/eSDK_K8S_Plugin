package plugin

import (
	"dev"
	"errors"
	"fmt"
	"net"
	"proto"
	"storage/fusionstorage/attacher"
	"storage/fusionstorage/client"
	"storage/fusionstorage/volume"
	"strings"
	"utils/log"
	"utils/pwd"

	volUtil "k8s.io/kubernetes/pkg/volume/util"
)

const (
	CAPACITY_UNIT int64 = 1024 * 1024
)

type FusionStorageSanPlugin struct {
	cli      *client.Client
	hosts    map[string]string
	protocol string
	portals  []string
}

func init() {
	RegPlugin("fusionstorage-san", &FusionStorageSanPlugin{})
}

func (p *FusionStorageSanPlugin) NewPlugin() Plugin {
	return &FusionStorageSanPlugin{
		hosts: make(map[string]string),
	}
}

func (p *FusionStorageSanPlugin) Init(config, parameters map[string]interface{}) error {
	scsi, scsiExist := parameters["SCSI"].(map[string]interface{})
	iscsi, iscsiExist := parameters["ISCSI"].([]interface{})
	if !scsiExist && !iscsiExist {
		return errors.New("SCSI or ISCSI must be provided for fusionstorage-san")
	} else if scsiExist && iscsiExist {
		return errors.New("Provide only one of SCSI and ISCSI for fusionstorage-san")
	} else if scsiExist {
		for k, v := range scsi {
			manageIP := v.(string)
			ip := net.ParseIP(manageIP)
			if ip == nil {
				return fmt.Errorf("Manage IP %s of host %s is invalid", manageIP, k)
			}

			p.hosts[k] = manageIP
		}

		p.protocol = "scsi"
	} else {
		portals, err := proto.VerifyIscsiPortals(iscsi)
		if err != nil {
			return err
		}

		p.portals = portals
		p.protocol = "iscsi"
	}

	url, exist := config["url"].(string)
	if !exist {
		return errors.New("url must be provided")
	}

	user, exist := config["user"].(string)
	if !exist {
		return errors.New("user must be provided")
	}

	password, exist := config["password"].(string)
	if !exist {
		return errors.New("password must be provided")
	}

	decrypted, err := pwd.Decrypt(password)
	if err != nil {
		return err
	}

	cli := client.NewClient(url, user, decrypted)
	err = cli.Login()
	if err != nil {
		return err
	}

	p.cli = cli
	return nil
}

func (p *FusionStorageSanPlugin) getParams(name string, parameters map[string]interface{}) (map[string]interface{}, error) {
	params := map[string]interface{}{
		"name":     name,
		"capacity": volUtil.RoundUpSize(parameters["size"].(int64), CAPACITY_UNIT),
	}

	paramKeys := []string{
		"storagepool",
		"cloneFrom",
	}

	for _, key := range paramKeys {
		if v, exist := parameters[key].(string); exist && v != "" {
			params[strings.ToLower(key)] = v
		}
	}

	return params, nil
}

func (p *FusionStorageSanPlugin) CreateVolume(name string, parameters map[string]interface{}) (string, error) {
	params, err := p.getParams(name, parameters)
	if err != nil {
		return "", err
	}

	san := volume.NewSAN(p.cli)
	err = san.Create(params)
	if err != nil {
		return "", err
	}

	return params["name"].(string), nil
}

func (p *FusionStorageSanPlugin) DeleteVolume(name string) error {
	san := volume.NewSAN(p.cli)
	return san.Delete(name)
}

func (p *FusionStorageSanPlugin) AttachVolume(name string, parameters map[string]interface{}) error {
	return nil
}

func (p *FusionStorageSanPlugin) DetachVolume(name string, parameters map[string]interface{}) error {
	localAttacher := attacher.NewAttacher(p.cli, p.protocol, "csi", p.portals, p.hosts)
	_, err := localAttacher.ControllerDetach(name, parameters)
	if err != nil {
		log.Errorf("Detach volume %s error: %v", name, err)
		return err
	}

	return nil
}

func (p *FusionStorageSanPlugin) StageVolume(name string, parameters map[string]interface{}) error {
	localAttacher := attacher.NewAttacher(p.cli, p.protocol, "csi", p.portals, p.hosts)
	devPath, err := localAttacher.NodeStage(name, parameters)
	if err != nil {
		log.Errorf("Stage volume %s error: %v", name, err)
		return err
	}

	targetPath := parameters["targetPath"].(string)
	fsType := parameters["fsType"].(string)
	mountFlags := parameters["mountFlags"].(string)

	err = dev.MountLunDev(devPath, targetPath, fsType, mountFlags)
	if err != nil {
		log.Errorf("Mount device %s to %s error: %v", devPath, targetPath, err)
		return err
	}

	return nil
}

func (p *FusionStorageSanPlugin) UnstageVolume(name string, parameters map[string]interface{}) error {
	targetPath := parameters["targetPath"].(string)
	err := dev.Unmount(targetPath)
	if err != nil {
		log.Errorf("Cannot unmount %s error: %v", targetPath, err)
		return err
	}

	localAttacher := attacher.NewAttacher(p.cli, p.protocol, "csi", p.portals, p.hosts)
	err = localAttacher.NodeUnstage(name, parameters)
	if err != nil {
		log.Errorf("Unstage volume %s error: %v", name, err)
		return err
	}

	return nil
}

func (p *FusionStorageSanPlugin) UpdateBackendCapabilities() (map[string]interface{}, error) {
	capabilities := map[string]interface{}{
		"SupportThin":  true,
		"SupportThick": false,
		"SupportQoS":   false,
	}

	return capabilities, nil
}

func (p *FusionStorageSanPlugin) UpdatePoolCapabilities(poolNames []string) (map[string]interface{}, error) {
	// To keep connection token alive
	p.cli.KeepAlive()

	pools, err := p.cli.GetAllPools()
	if err != nil {
		log.Errorf("Get fusionstorage pools error: %v", err)
		return nil, err
	}

	log.Debugf("Get pools: %v", pools)

	capabilities := make(map[string]interface{})

	for _, name := range poolNames {
		if i, exist := pools[name]; exist {
			pool := i.(map[string]interface{})

			totalCapacity := int64(pool["totalCapacity"].(float64))
			usedCapacity := int64(pool["usedCapacity"].(float64))

			freeCapacity := (totalCapacity - usedCapacity) * CAPACITY_UNIT
			capabilities[name] = map[string]interface{}{
				"FreeCapacity": freeCapacity,
			}
		}
	}

	return capabilities, nil
}

func (p *FusionStorageSanPlugin) UpdateMetroRemotePlugin(remote Plugin) {
}
