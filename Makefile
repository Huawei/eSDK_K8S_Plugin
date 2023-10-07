# usage: make -f Makefile VER={VER} PLATFORM={PLATFORM}

# (required) [x.y.x]
VER=VER
# (required) [X86 ARM]
PLATFORM=PLATFORM

# (Optional) [2.5.RC1 2.5.RC2 ...] eSDK Version
RELEASE_VER=RELEASE_VER
# (Optional) [TRUE FALSE] Compile Binary Only, Cancel Inline Optimization
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
# Disable inline optimization
flag = -gcflags "all=-N -l"
else
flag = -ldflags="-s" -buildmode=pie
all:PREPARE BUILD COPY_FILE PACK
endif

# Platform [X86, ARM]
ifeq (${PLATFORM}, X86)
env = CGO_ENABLED=0 GOOS=linux GOARCH=amd64
else
env = CGO_ENABLED=0 GOOS=linux GOARCH=arm64
endif

PREPARE:
	rm -rf ./${PACKAGE}
	mkdir -p ./${PACKAGE}/bin

BUILD:
# usage: [env] go build [-o output] [flags] packages
	${env} go build -o ./${PACKAGE}/bin/huawei-csi ${flag} ./csi
	${env} go build -o ./${PACKAGE}/bin/storage-backend-controller ${flag} ./cmd/storage-backend-controller
	${env} go build -o ./${PACKAGE}/bin/storage-backend-sidecar ${flag} ./cmd/storage-backend-sidecar
	${env} go build -o ./${PACKAGE}/bin/oceanctl ${flag} ./cli

COPY_FILE:
	mkdir -p ./${PACKAGE}/examples
	cp -r ./examples/* ./${PACKAGE}/examples

	mkdir -p ./${PACKAGE}/helm/
	cp -r ./helm/* ./${PACKAGE}/helm/

	mkdir -p ./${PACKAGE}/manual/
	cp -r ./manual/* ./${PACKAGE}/manual/
	cp -r ./helm/esdk/crds ./${PACKAGE}/manual/esdk/crds

	mkdir -p ./${PACKAGE}/tools
	cp -r ./tools/imageUpload/* ./${PACKAGE}/tools

PACK:
	zip -r ${PACKAGE}.zip ./${PACKAGE}
	rm -rf ./${PACKAGE}
