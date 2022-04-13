PLATFORM=PLATFORM
RELEASE_VER=RELEASE_VER
VER=VER
export GO111MODULE=on
export GOPATH:=$(GOPATH):$(shell pwd)
export PACKAGE=eSDK_Enterprise_Storage_${RELEASE_VER}_Kubernetes_CSI_Plugin_V${VER}_${PLATFORM}_64
export CLOUD_PACKAGE=eSDK_Cloud_Storage_${RELEASE_VER}_Kubernetes_CSI_Plugin_V${VER}_${PLATFORM}_64

all:COMMON_1 DIFF COMMON_2

COMMON_1:
	rm -rf ./${PACKAGE} ./${CLOUD_PACKAGE}
	rm -rf ./src/vendor

	mkdir -p ./${PACKAGE}/bin

DIFF:
ifeq (${PLATFORM}, X86)
	go build -o ./${PACKAGE}/bin/huawei-csi	./csi
	go build -o ./${PACKAGE}/bin/secretGenerate  ./tools/secretGenerate
	go build -o ./${PACKAGE}/bin/secretUpdate ./tools/secretUpdate
endif

ifeq (${PLATFORM}, ARM)
	GOOS=linux GOARCH=arm64 go build -o ./${PACKAGE}/bin/huawei-csi	./csi
	GOOS=linux GOARCH=arm64 go build -o ./${PACKAGE}/bin/secretGenerate ./tools/secretGenerate
	GOOS=linux GOARCH=arm64 go build -o ./${PACKAGE}/bin/secretUpdate ./tools/secretUpdate
endif

COMMON_2:
	mkdir -p ./${PACKAGE}/deploy/v1.18-v1.19
	mkdir -p ./${PACKAGE}/deploy/v1.20-v1.23

	cp -r ./deploy/huawei-csi-node.yaml ./deploy/huawei-csi-rbac.yaml ./deploy/huawei-csi-configmap ./${PACKAGE}/deploy/v1.18-v1.19
	cp -r ./deploy/huawei-csi-node.yaml ./deploy/huawei-csi-rbac.yaml ./deploy/huawei-csi-configmap ./${PACKAGE}/deploy/v1.20-v1.23
	cp ./deploy/huawei-csi-controller-snapshot-v1beta1.yaml ./${PACKAGE}/deploy/v1.18-v1.19/huawei-csi-controller.yaml
	cp ./deploy/huawei-csi-snapshot-crd-v1beta1.yaml ./${PACKAGE}/deploy/v1.18-v1.19/huawei-csi-snapshot-crd.yaml
	cp ./deploy/huawei-csi-controller-snapshot-v1.yaml ./${PACKAGE}/deploy/v1.20-v1.23/huawei-csi-controller.yaml
	cp ./deploy/huawei-csi-snapshot-crd-v1.yaml ./${PACKAGE}/deploy/v1.20-v1.23/huawei-csi-snapshot-crd.yaml

	mkdir -p ./${PACKAGE}/examples/v1.18-v1.19
	mkdir -p ./${PACKAGE}/examples/v1.20-v1.23
	cp ./examples/* ./${PACKAGE}/examples/v1.18-v1.19
	cp ./examples/* ./${PACKAGE}/examples/v1.20-v1.23
	rm -rf ./${PACKAGE}/examples/v1.18-v1.19/*-v1.yaml
	rm -rf ./${PACKAGE}/examples/v1.20-v1.23/*-v1beta1.yaml
	mv ./${PACKAGE}/examples/v1.18-v1.19/snapshot-v1beta1.yaml ./${PACKAGE}/examples/v1.18-v1.19/volume-snapshot.yaml
	mv ./${PACKAGE}/examples/v1.18-v1.19/snapshotclass-v1beta1.yaml ./${PACKAGE}/examples/v1.18-v1.19/volume-snapshot-class.yaml
	mv ./${PACKAGE}/examples/v1.20-v1.23/snapshot-v1.yaml ./${PACKAGE}/examples/v1.20-v1.23/volume-snapshot.yaml
	mv ./${PACKAGE}/examples/v1.20-v1.23/snapshotclass-v1.yaml ./${PACKAGE}/examples/v1.20-v1.23/volume-snapshot-class.yaml

	zip -r ${PACKAGE}.zip ./${PACKAGE}
	mv ${PACKAGE} ${CLOUD_PACKAGE}
	zip -r ${CLOUD_PACKAGE}.zip ./${CLOUD_PACKAGE}
	rm -rf ./${CLOUD_PACKAGE}
