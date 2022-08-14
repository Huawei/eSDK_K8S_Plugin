# usage: make -f Makefile VER=3.0.0 PLATFORM=X86 RELEASE_VER=2.5.RC2

# [3.0.0]
VER=VER
# [X86 ARM]
PLATFORM=PLATFORM
# eSDK Version: [2.5.RC1 2.5.RC2 ...]
RELEASE_VER=RELEASE_VER

export GO111MODULE=on
export GOPATH:=$(GOPATH):$(shell pwd)
ifeq (${RELEASE_VER}, RELEASE_VER)
       export PACKAGE=eSDK_Enterprise_Storage_Kubernetes_CSI_Plugin_V${VER}_${PLATFORM}_64
       export CLOUD_PACKAGE=eSDK_Cloud_Storage_Kubernetes_CSI_Plugin_V${VER}_${PLATFORM}_64
else
       export PACKAGE=eSDK_Enterprise_Storage_${RELEASE_VER}_Kubernetes_CSI_Plugin_V${VER}_${PLATFORM}_64
       export CLOUD_PACKAGE=eSDK_Cloud_Storage_${RELEASE_VER}_Kubernetes_CSI_Plugin_V${VER}_${PLATFORM}_64
endif


all:PREPARE BUILD COPY_FILE PACK

PREPARE:
	rm -rf ./${PACKAGE} ./${CLOUD_PACKAGE}
	rm -rf ./src/vendor
	mkdir -p ./${PACKAGE}/bin

BUILD:
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

COPY_FILE:
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

PACK:
	zip -r ${PACKAGE}.zip ./${PACKAGE}
	mv ${PACKAGE} ${CLOUD_PACKAGE}
	zip -r ${CLOUD_PACKAGE}.zip ./${CLOUD_PACKAGE}
	rm -rf ./${CLOUD_PACKAGE}
