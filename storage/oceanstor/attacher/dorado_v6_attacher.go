package attacher

import (
	"context"

	"huawei-csi-driver/connector"
	"huawei-csi-driver/storage/oceanstor/client"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

type DoradoV6Attacher struct {
	Attacher
}

const (
	ACCESS_MODE_BALANCED = "0"
)

func newDoradoV6Attacher(
	cli *client.Client,
	protocol, invoker string,
	portals []string,
	alua map[string]interface{}) AttacherPlugin {
	return &DoradoV6Attacher{
		Attacher: Attacher{
			cli:      cli,
			protocol: protocol,
			invoker:  invoker,
			portals:  portals,
			alua:     alua,
		},
	}
}

func (p *DoradoV6Attacher) needUpdateHost(host map[string]interface{}, hostAlua map[string]interface{}) bool {
	accessMode, ok := hostAlua["accessMode"]
	if !ok {
		return false
	}

	if accessMode != host["accessMode"] {
		return true
	} else if host["accessMode"] == ACCESS_MODE_BALANCED {
		return false
	}

	hyperMetroPathOptimized, ok := hostAlua["hyperMetroPathOptimized"]
	if ok && hyperMetroPathOptimized != host["hyperMetroPathOptimized"] {
		return true
	}

	return false
}

func (p *DoradoV6Attacher) ControllerAttach(ctx context.Context,
	lunName string,
	parameters map[string]interface{}) (map[string]interface{}, error) {
	host, err := p.getHost(ctx, parameters, true)
	if err != nil {
		log.AddContext(ctx).Errorf("Get host ID error: %v", err)
		return nil, err
	}

	hostID := host["ID"].(string)
	hostAlua := utils.GetAlua(ctx, p.alua, host["NAME"].(string))

	if hostAlua != nil && p.needUpdateHost(host, hostAlua) {
		err := p.cli.UpdateHost(ctx, hostID, hostAlua)
		if err != nil {
			log.AddContext(ctx).Errorf("Update host %s error: %v", hostID, err)
			return nil, err
		}
	}

	if p.protocol == "iscsi" {
		_, err = p.Attacher.attachISCSI(ctx, hostID)
	} else if p.protocol == "fc" || p.protocol == "fc-nvme" {
		_, err = p.Attacher.attachFC(ctx, hostID)
	} else if p.protocol == "roce" {
		_, err = p.Attacher.attachRoCE(ctx, hostID)
	}

	if err != nil {
		log.AddContext(ctx).Errorf("Attach %s connection error: %v", p.protocol, err)
		return nil, err
	}

	wwn, hostLunId, err := p.doMapping(ctx, hostID, lunName)
	if err != nil {
		log.AddContext(ctx).Errorf("Mapping LUN %s to host %s error: %v", lunName, hostID, err)
		return nil, err
	}

	return p.getMappingProperties(ctx, wwn, hostLunId, parameters)
}

func (p *DoradoV6Attacher) NodeStage(ctx context.Context,
	lunName string,
	parameters map[string]interface{}) (*connector.ConnectInfo, error) {
	return connectVolume(ctx, p, lunName, p.protocol, parameters)
}
