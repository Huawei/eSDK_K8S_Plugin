package attacher

import (
	"context"

	"huawei-csi-driver/connector"
	"huawei-csi-driver/storage/oceanstor/client"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

type OceanStorAttacher struct {
	Attacher
}

const (
	MULTIPATHTYPE_DEFAULT = "0"
)

func newOceanStorAttacher(
	cli *client.Client,
	protocol,
	invoker string,
	portals []string,
	alua map[string]interface{}) AttacherPlugin {
	return &OceanStorAttacher{
		Attacher: Attacher{
			cli:      cli,
			protocol: protocol,
			invoker:  invoker,
			portals:  portals,
			alua:     alua,
		},
	}
}

func (p *OceanStorAttacher) needUpdateInitiatorAlua(initiator map[string]interface{},
	hostAlua map[string]interface{}) bool {
	multiPathType, ok := hostAlua["MULTIPATHTYPE"]
	if !ok {
		return false
	}

	if multiPathType != initiator["MULTIPATHTYPE"] {
		return true
	} else if initiator["MULTIPATHTYPE"] == MULTIPATHTYPE_DEFAULT {
		return false
	}

	failoverMode, ok := hostAlua["FAILOVERMODE"]
	if ok && failoverMode != initiator["FAILOVERMODE"] {
		return true
	}

	specialModeType, ok := hostAlua["SPECIALMODETYPE"]
	if ok && specialModeType != initiator["SPECIALMODETYPE"] {
		return true
	}

	pathType, ok := hostAlua["PATHTYPE"]
	if ok && pathType != initiator["PATHTYPE"] {
		return true
	}

	return false
}

func (p *OceanStorAttacher) attachISCSI(ctx context.Context, hostID, hostName string) error {
	iscsiInitiator, err := p.Attacher.attachISCSI(ctx, hostID)
	if err != nil {
		return err
	}

	hostAlua := utils.GetAlua(ctx, p.alua, hostName)
	if hostAlua != nil && p.needUpdateInitiatorAlua(iscsiInitiator, hostAlua) {
		err = p.cli.UpdateIscsiInitiator(ctx, iscsiInitiator["ID"].(string), hostAlua)
	}

	return err
}

func (p *OceanStorAttacher) attachFC(ctx context.Context, hostID, hostName string) error {
	fcInitiators, err := p.Attacher.attachFC(ctx, hostID)
	if err != nil {
		return err
	}

	hostAlua := utils.GetAlua(ctx, p.alua, hostName)
	if hostAlua != nil {
		for _, i := range fcInitiators {
			if !p.needUpdateInitiatorAlua(i, hostAlua) {
				continue
			}

			err := p.cli.UpdateFCInitiator(ctx, i["ID"].(string), hostAlua)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (p *OceanStorAttacher) attachRoCE(ctx context.Context, hostID string) error {
	_, err := p.Attacher.attachRoCE(ctx, hostID)
	return err
}

func (p *OceanStorAttacher) ControllerAttach(ctx context.Context,
	lunName string,
	parameters map[string]interface{}) (
	map[string]interface{}, error) {
	host, err := p.getHost(ctx, parameters, true)
	if err != nil {
		log.AddContext(ctx).Errorf("Get host ID error: %v", err)
		return nil, err
	}

	hostID := host["ID"].(string)
	hostName := host["NAME"].(string)

	if p.protocol == "iscsi" {
		err = p.attachISCSI(ctx, hostID, hostName)
	} else if p.protocol == "fc" || p.protocol == "fc-nvme" {
		err = p.attachFC(ctx, hostID, hostName)
	} else if p.protocol == "roce" {
		err = p.attachRoCE(ctx, hostID)
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

// NodeStage to do storage mapping and get the connector
func (p *OceanStorAttacher) NodeStage(ctx context.Context,
	lunName string,
	parameters map[string]interface{}) (*connector.ConnectInfo, error) {
	return connectVolume(ctx, p, lunName, p.protocol, parameters)
}
