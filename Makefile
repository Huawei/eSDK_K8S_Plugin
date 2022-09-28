# usage: make -f Makefile VER=3.1.0 PLATFORM=X86 RELEASE_VER=2.5.RC2

# (required) [3.1.0]
VER=VER
# (required) [X86 ARM]
PLATFORM=PLATFORM

# (Optional) [2.5.RC1 2.5.RC2 ...] eSDK Version
RELEASE_VER=RELEASE_VER
# (Optional) [TRUE FALSE] Compile Binary Only
ONLY_BIN=ONLY_BIN

export GO111MODULE=on
export GOPATH:=$(GOPATH):$(shell pwd)

ifeq (${RELEASE_VER}, RELEASE_VER)
       export PACKAGE=eSDK_Huawei_Storage_Kubernetes_CSI_Plugin_V${VER}_${PLATFORM}_64
else
       export PACKAGE=eSDK_Huawei_Storage_${RELEASE_VER}_Kubernetes_CSI_Plugin_V${VER}_${PLATFORM}_64
endif

# Build process
ifeq (${ONLY_BIN}, TRUE)
all:PREPARE BUILD PACK
else
all:PREPARE BUILD COPY_FILE PACK
endif


PREPARE:
	rm -rf ./${PACKAGE}
	mkdir -p ./${PACKAGE}/bin

BUILD:
# usage: go build [-o output] [build flags] [packages]
ifeq (${PLATFORM}, X86)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ./${PACKAGE}/bin/huawei-csi -ldflags="-s" -buildmode=pie ./csi
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ./${PACKAGE}/bin/secretGenerate -ldflags="-s" -buildmode=pie ./tools/secretGenerate
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ./${PACKAGE}/bin/secretUpdate -ldflags="-s" -buildmode=pie ./tools/secretUpdate
endif

ifeq (${PLATFORM}, ARM)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o ./${PACKAGE}/bin/huawei-csi -ldflags="-s" -buildmode=pie ./csi
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o ./${PACKAGE}/bin/secretGenerate -ldflags="-s" -buildmode=pie ./tools/secretGenerate
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o ./${PACKAGE}/bin/secretUpdate -ldflags="-s" -buildmode=pie ./tools/secretUpdate
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

	mkdir -p ./${PACKAGE}/docs
	-cp ./docs/eSDK* ./${PACKAGE}/docs

PACK:
	zip -r ${PACKAGE}.zip ./${PACKAGE}
	rm -rf ./${PACKAGE}
