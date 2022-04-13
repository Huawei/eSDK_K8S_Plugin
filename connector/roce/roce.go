package roce

import (
	"context"

	"huawei-csi-driver/connector"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

type RoCE struct {
}

const (
	intNumTwo   = 2
	intNumThree = 3
)

func init() {
	connector.RegisterConnector(connector.RoCEDriver, &RoCE{})
}

func (roce *RoCE) ConnectVolume(ctx context.Context, conn map[string]interface{}) (string, error) {
	log.AddContext(ctx).Infof("RoCE Start to connect volume ==> connect info: %v", conn)
	tgtLunGUID, exist := conn["tgtLunGuid"].(string)
	if !exist {
		return "", utils.Errorln(ctx, "key tgtLunGuid does not exist in connection properties")
	}
	return connector.ConnectVolumeCommon(ctx, conn, tgtLunGUID, connector.RoCEDriver, tryConnectVolume)
}

func (roce *RoCE) DisConnectVolume(ctx context.Context, tgtLunGuid string) error {
	log.AddContext(ctx).Infof("RoCE Start to disconnect volume ==> Volume Guid info: %v", tgtLunGuid)
	return connector.DisConnectVolumeCommon(ctx, tgtLunGuid, connector.RoCEDriver, tryDisConnectVolume)
}
