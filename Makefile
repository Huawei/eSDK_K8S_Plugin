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
	mkdir -p ./${PACKAGE}/deploy
	cp -r ./deploy/huawei-csi-node.yaml ./deploy/huawei-csi-rbac.yaml ./deploy/huawei-csi-configmap ./${PACKAGE}/deploy
	cp ./deploy/huawei-csi-controller-snapshot-v1.yaml ./${PACKAGE}/deploy/huawei-csi-controller.yaml
	cp ./deploy/huawei-csi-snapshot-crd-v1.yaml ./${PACKAGE}/deploy/huawei-csi-snapshot-crd.yaml

	mkdir -p ./${PACKAGE}/examples
	cp ./examples/* ./${PACKAGE}/examples

	mkdir -p ./${PACKAGE}/helm/esdk
	cp -r ./helm/esdk/* ./${PACKAGE}/helm/esdk

	mkdir -p ./${PACKAGE}/tools
	cp -r ./tools/imageUpload/* ./${PACKAGE}/tools

	zip -r ${PACKAGE}.zip ./${PACKAGE}
	mv ${PACKAGE} ${CLOUD_PACKAGE}
	zip -r ${CLOUD_PACKAGE}.zip ./${CLOUD_PACKAGE}
	rm -rf ./${CLOUD_PACKAGE}
