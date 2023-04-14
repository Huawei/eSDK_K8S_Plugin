package handle

import (
	"context"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"google.golang.org/grpc"

	"huawei-csi-driver/lib/drcsi"
	"huawei-csi-driver/lib/drcsi/rpc"
)

func initBackend() *backend {
	return &backend{}
}

func TestNewBackend(t *testing.T) {
	NewBackend(&grpc.ClientConn{})
}

func TestAddStorageBackend(t *testing.T) {
	fakeBackend := initBackend()
	clientPatch := gomonkey.ApplyFunc(drcsi.NewStorageBackendClient,
		func(cc grpc.ClientConnInterface) drcsi.StorageBackendClient {
			return drcsi.NewStorageBackendClient(fakeBackend.conn)
		})
	defer clientPatch.Reset()

	providerPatch := gomonkey.ApplyFunc(rpc.GetProviderName,
		func(ctx context.Context, conn *grpc.ClientConn) (string, error) {
			return "fake-provider", nil
		})
	defer providerPatch.Reset()

	addPatch := gomonkey.ApplyFunc(addStorageBackend,
		func(ctx context.Context, conn *grpc.ClientConn, req *drcsi.AddStorageBackendRequest) (
			*drcsi.AddStorageBackendResponse, error) {
			return &drcsi.AddStorageBackendResponse{BackendId: "fake-backend-id"}, nil
		})
	defer addPatch.Reset()

	name, backendId, err := fakeBackend.AddStorageBackend(context.TODO(),
		"fake-content", "fake-configmapMeta", "fake-secretMeta", nil)
	if name != "fake-provider" || backendId != "fake-backend-id" || err != nil {
		t.Errorf("TestAddStorageBackend failed, name: %s, backendId: %s, error: %v",
			name, backendId, err)
	}
}
