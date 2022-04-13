package nvme

import (
	"context"
	"sync"

	"huawei-csi-driver/connector"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

type FCNVMe struct {
	mutex sync.Mutex
}

func init() {
	connector.RegisterConnector(connector.FCNVMeDriver, &FCNVMe{})
}

func (fc *FCNVMe) ConnectVolume(ctx context.Context, conn map[string]interface{}) (string, error) {
	log.AddContext(ctx).Infof("FC-NVMe Start to connect volume ==> connect info: %v", conn)
	tgtLunGuid, exist := conn["tgtLunGuid"].(string)
	if !exist {
		return "", utils.Errorln(ctx, "there is no Lun GUID in connect info")
	}

	return connector.ConnectVolumeCommon(ctx, conn, tgtLunGuid, connector.FCNVMeDriver, tryConnectVolume)
}

func (fc *FCNVMe) DisConnectVolume(ctx context.Context, tgtLunGuid string) error {
	log.AddContext(ctx).Infof("FC-NVMe Start to disconnect volume ==> Volume Guid info: %v", tgtLunGuid)
	return connector.DisConnectVolumeCommon(ctx, tgtLunGuid, connector.FCNVMeDriver, tryDisConnectVolume)
}
