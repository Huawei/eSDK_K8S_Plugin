package iscsi

import (
	"context"

	"huawei-csi-driver/connector"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

type iSCSI struct {
}

func init() {
	connector.RegisterConnector(connector.ISCSIDriver, &iSCSI{})
}

func (isc *iSCSI) ConnectVolume(ctx context.Context, conn map[string]interface{}) (string, error) {
	log.AddContext(ctx).Infof("iSCSI Start to connect volume ==> connect info: %v", conn)
	tgtLunWWN, exist := conn["tgtLunWWN"].(string)
	if !exist {
		return "", utils.Errorln(ctx, "key tgtLunWWN does not exist in connection properties")
	}
	return connector.ConnectVolumeCommon(ctx, conn, tgtLunWWN, connector.ISCSIDriver, tryConnectVolume)
}

func (isc *iSCSI) DisConnectVolume(ctx context.Context, tgtLunWWN string) error {
	log.AddContext(ctx).Infof("iSCSI Start to disconnect volume ==> volume wwn is: %v", tgtLunWWN)
	return connector.DisConnectVolumeCommon(ctx, tgtLunWWN, connector.ISCSIDriver, tryDisConnectVolume)
}
