package driver

import (
	"utils"
)

type Driver struct {
	name    string
	version string
}

func NewDriver() *Driver {
	return &Driver{
		name:    "huawei.csi.driver",
		version: utils.GetCSIVersion(),
	}
}
