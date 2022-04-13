package fibrechannel

import (
	"context"

	"huawei-csi-driver/connector"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

type FibreChannel struct {
}

func init() {
	connector.RegisterConnector(connector.FCDriver, &FibreChannel{})
}

func (fc *FibreChannel) ConnectVolume(ctx context.Context, conn map[string]interface{}) (string, error) {
	log.AddContext(ctx).Infof("FC Start to connect volume ==> connect info: %v", conn)
	tgtLunWWN, exist := conn["tgtLunWWN"].(string)
	if !exist {
		return "", utils.Errorln(ctx, "key tgtLunWWN does not exist in connection properties")
	}
	return connector.ConnectVolumeCommon(ctx, conn, tgtLunWWN, connector.FCDriver, tryConnectVolume)
}

func (fc *FibreChannel) DisConnectVolume(ctx context.Context, tgtLunWWN string) error {
	log.AddContext(ctx).Infof("FC Start to disconnect volume ==> volume wwn is: %v", tgtLunWWN)
	return connector.DisConnectVolumeCommon(ctx, tgtLunWWN, connector.FCDriver, tryDisConnectVolume)
}
