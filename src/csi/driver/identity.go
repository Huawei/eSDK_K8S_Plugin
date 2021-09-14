package driver

import (
	"context"
	"fmt"
	"utils"
	"utils/log"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/protobuf/ptypes/wrappers"
)

func (d *Driver) GetPluginInfo(ctx context.Context, req *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	log.Infof("Get plugin info %v", *d)
	return &csi.GetPluginInfoResponse{
		Name:          d.name,
		VendorVersion: d.version,
	}, nil
}

func (d *Driver) GetPluginCapabilities(ctx context.Context, req *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
	log.Infof("Get plugin capabilities of %v", *d)

	return &csi.GetPluginCapabilitiesResponse{
		Capabilities: []*csi.PluginCapability{
			&csi.PluginCapability{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{
						Type: csi.PluginCapability_Service_CONTROLLER_SERVICE,
					},
				},
			},
		},
	}, nil
}

func (d *Driver) Probe(ctx context.Context, req *csi.ProbeRequest) (*csi.ProbeResponse, error) {

	log.Infof("Probe the csi-driver plugin %v", *d)
	bootstrap := utils.GetBootStrap()

	if !bootstrap {
		return &csi.ProbeResponse{}, fmt.Errorf("Bootstarp is false")
	}
	resp := &csi.ProbeResponse{
		Ready: &wrappers.BoolValue{Value: bootstrap},
	}

	return resp, nil

}
