package plugin

import (
	"dev"
	"errors"
	"fmt"
	"net"
	"proto"
	osAttacher "storage/fusionstorage/attacher"
	"storage/fusionstorage/client"
	fsVol "storage/fusionstorage/volume"
	"utils"
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
	scsi := parameters["SCSI"]
	iscsi := parameters["ISCSI"]
	if scsi == nil && iscsi == nil {
		return fmt.Errorf("SCSI or ISCSI need to provide for fusionstorage-san")
	}
	if scsi != nil && iscsi != nil {
		return fmt.Errorf("SCSI,SCISI exsits at the same time ,only one is required")
	}
	if scsi != nil {
		for k, v := range scsi.(map[string]interface{}) {
			manageIP := v.(string)
			ip := net.ParseIP(manageIP)
			if ip == nil {
				return fmt.Errorf("Manage IP %s of host %s is invalid", manageIP, k)
			}
			p.hosts[k] = manageIP
		}
		p.protocol = "scsi"
	} else {
		IPs, err := proto.VerifyIscsiPortals(iscsi.([]interface{}))
		if err != nil {
			return err
		}
		p.portals = IPs
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


func (p *FusionStorageSanPlugin) getParams(name string, size int64, poolName string, parameters map[string]string) (map[string]interface{}, error) {
	name = utils.GetFusionStorageLunName(name)
	params := map[string]interface{}{
		"name":     name,
		"capacity": volUtil.RoundUpSize(size, CAPACITY_UNIT),
	}

	pool, err := p.cli.GetPoolByName(poolName)
	if err != nil {
		return nil, err
	}
	if pool == nil {
		return nil, fmt.Errorf("Storage pool %s doesn't exist", pool)
	}

	params["poolId"] = int64(pool["poolId"].(float64))

	if cloneFrom, exist := parameters["cloneFrom"]; exist && cloneFrom != "" {
		params["cloneFrom"] = cloneFrom
	}
	return params, nil
}

func (p *FusionStorageSanPlugin) CreateVolume(name string, size int64, poolName string, parameters map[string]string) (string,error) {
	params, err := p.getParams(name, size, poolName, parameters)
	if err != nil {
		return " " ,err
	}
	volumeName := params["name"].(string)

	san := fsVol.NewSAN(p.cli)
	log.Infof("Start to create volume %s", volumeName)
	return volumeName ,san.Create(params)
}

func (p *FusionStorageSanPlugin) DeleteVolume(name string) error {
	san := fsVol.NewSAN(p.cli)
	return san.Delete(name)
}

func (p *FusionStorageSanPlugin) AttachVolume(name string, parameters map[string]interface{}) error {
	return nil
}

func (p *FusionStorageSanPlugin) getHostManageIP(hostname string) (string, error) {
	manageIP, exist := p.hosts[hostname]
	if !exist {
		msg := fmt.Sprintf("There is no manage IP configured for host %s", hostname)
		log.Errorln(msg)
		return "", errors.New(msg)
	}

	return manageIP, nil
}

func (p *FusionStorageSanPlugin) DetachVolume(volumeName string, parameters map[string]interface{}) error {

	var err error
	attacher := osAttacher.NewAttacher(p.cli, p.protocol, "csi")
	if p.protocol == "scsi" {
		hostname := parameters["HostName"].(string)
		manageIP, err := p.getHostManageIP(hostname)
		if err != nil {
			return err
		}
		err = attacher.DetachVolumeBySCSI(volumeName,manageIP)
	}else {
		hostname := parameters["HostName"].(string)
		err = attacher.DetachVolumeByISCSI(volumeName,hostname)
	}
	return err
}

func (p *FusionStorageSanPlugin) StageVolume(name string, parameters map[string]interface{}) error {
	var err error
	parameters["portals"] = p.portals
	parameters["hosts"] = p.hosts
	attacher := osAttacher.NewAttacher(p.cli, p.protocol, "csi")
	if p.protocol == "scsi" {
		err = attacher.NodeStageBySCSI(name, parameters)
	} else {
		err = attacher.NodeStageByISCSI(name, parameters)
	}
	if err != nil {
		log.Errorf("Stage volume %s error: %v", name, err)
		return err
	}
	return nil

}


func (p *FusionStorageSanPlugin) UnstageVolume(name string, parameters map[string]interface{}) error {
	targetPath := parameters["targetPath"].(string)
	err := dev.Unmount(targetPath)
	if err != nil {
		log.Errorf("Cannot unmount volume %s error: %v", name, err)
		return err
	}
	lunName := utils.GetFusionStorageLunName(name)
	lun,err := p.cli.GetVolumeByName(lunName)
	if err != nil {
		return err
	}
	if lun == nil {
		return nil
	}
	err = p.DetachVolume(lunName,parameters)
	if err != nil {
		return err
	}
	wwn := lun["wwn"].(string)
	err = dev.DeleteDev(wwn)
	if err != nil {
		log.Errorf("Delete dev %s error: %v", wwn, err)
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
