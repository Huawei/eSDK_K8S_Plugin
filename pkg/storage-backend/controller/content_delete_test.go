package controller

import (
	"context"
	"fmt"
	"testing"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	k8sFake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	coreV1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"

	"huawei-csi-driver/utils/log"

	xuanwuv1 "huawei-csi-driver/client/apis/xuanwu/v1"
	"huawei-csi-driver/pkg/client/clientset/versioned/fake"
	backendInformers "huawei-csi-driver/pkg/client/informers/externalversions"
)

const (
	logName string = "test"
)

// TestMain used for setup and teardown
func TestMain(m *testing.M) {
	log.MockInitLogging(logName)
	defer log.MockStopLogging(logName)

	m.Run()
}

func initRecorder(client kubernetes.Interface) record.EventRecorder {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&coreV1.EventSinkImpl{Interface: client.CoreV1().Events(v1.NamespaceAll)})
	return eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: fmt.Sprintf("fake-controller")})
}

func initController() *BackendController {
	storageBackendClient := fake.NewSimpleClientset()
	factory := backendInformers.NewSharedInformerFactory(storageBackendClient, 10)
	request := BackendControllerRequest{
		ClientSet:       storageBackendClient,
		ClaimInformer:   factory.Xuanwu().V1().StorageBackendClaims(),
		ContentInformer: factory.Xuanwu().V1().StorageBackendContents(),
		ReSyncPeriod:    10,
		EventRecorder:   initRecorder(k8sFake.NewSimpleClientset())}

	return NewBackendController(request)
}

func newContent(spec xuanwuv1.StorageBackendContentSpec) *xuanwuv1.StorageBackendContent {
	return &xuanwuv1.StorageBackendContent{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fake-name",
		},
		Spec: spec,
	}
}

func newClaim(spec xuanwuv1.StorageBackendClaimSpec) *xuanwuv1.StorageBackendClaim {
	return &xuanwuv1.StorageBackendClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fake-name",
		},
		Spec: spec,
	}
}

func TestDeleteStorageBackendContentWithoutClaim(t *testing.T) {
	err := initController().deleteStorageBackendContent(context.TODO(), newContent(
		xuanwuv1.StorageBackendContentSpec{Provider: "fake-provider"}))
	if err != nil {
		t.Errorf("TestDeleteStorageBackendContentWithoutClaim failed, error %v", err)
	}
}

func TestDeleteStorageBackendContent(t *testing.T) {
	ctrl := initController()
	err := ctrl.deleteStorageBackendContent(context.TODO(), newContent(
		xuanwuv1.StorageBackendContentSpec{
			Provider:     "fake-provider",
			BackendClaim: "fake-claim"}))
	if err != nil {
		t.Errorf("TestDeleteStorageBackendContent failed, error %v", err)
	}
}
