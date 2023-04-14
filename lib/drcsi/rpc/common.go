package rpc

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	"huawei-csi-driver/lib/drcsi"
)

// GetProviderName returns name of DR-CSI driver.
func GetProviderName(ctx context.Context, conn *grpc.ClientConn) (string, error) {
	client := drcsi.NewIdentityClient(conn)

	req := drcsi.GetProviderInfoRequest{}
	rsp, err := client.GetProviderInfo(ctx, &req)
	if err != nil {
		return "", err
	}
	name := rsp.GetProvider()
	if name == "" {
		return "", fmt.Errorf("drcsi name is empty")
	}
	return name, nil
}

// PluginCapabilitySet is set of DR-CSI plugin capabilities. Only supported capabilities are in the map.
type PluginCapabilitySet map[drcsi.ProviderCapability_Service_Type]bool

// GetPluginCapabilities returns set of supported capabilities of DR-CSI driver.
func GetPluginCapabilities(ctx context.Context, conn *grpc.ClientConn) (PluginCapabilitySet, error) {
	client := drcsi.NewIdentityClient(conn)
	req := drcsi.GetProviderCapabilitiesRequest{}
	rsp, err := client.GetProviderCapabilities(ctx, &req)
	if err != nil {
		return nil, err
	}
	caps := PluginCapabilitySet{}
	for _, capability := range rsp.GetCapabilities() {
		if capability == nil {
			continue
		}
		srv := capability.GetService()
		if srv == nil {
			continue
		}
		t := srv.GetType()
		caps[t] = true
	}
	return caps, nil
}
