package driver

import (
	"strings"

	"huawei-csi-driver/utils/k8sutils"
)

type Driver struct {
	name              string
	version           string
	useMultiPath      bool
	scsiMultiPathType string
	nvmeMultiPathType string
	k8sUtils          k8sutils.Interface
	nodeName          string
}

func NewDriver(name, version string, useMultiPath bool, scsiMultiPathType, nvmeMultiPathType string,
	k8sUtils k8sutils.Interface, nodeName string) *Driver {
	return &Driver{
		name:              name,
		version:           version,
		useMultiPath:      useMultiPath,
		scsiMultiPathType: scsiMultiPathType,
		nvmeMultiPathType: nvmeMultiPathType,
		k8sUtils:          k8sUtils,
		nodeName:          strings.TrimSpace(nodeName),
	}
}
