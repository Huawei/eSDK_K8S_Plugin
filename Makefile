# usage: make -f Makefile VER={VER} PLATFORM={PLATFORM}

# (required) [x.y.x]
VER=VER
# (required) [X86 ARM]
PLATFORM=PLATFORM

# (Optional) [2.5.RC1 2.5.RC2 ...] eSDK Version
RELEASE_VER=RELEASE_VER

export GO111MODULE=on

ifeq (${RELEASE_VER}, RELEASE_VER)
	export PACKAGE=eSDK_Storage_CSI_V${VER}_${PLATFORM}_64
else
	export PACKAGE=eSDK_Storage_${RELEASE_VER}_CSI_V${VER}_${PLATFORM}_64
endif

# Platform [X86, ARM, PPC64LE], default value is [X86]
ifeq (${PLATFORM}, PPC64LE)
arch=ppc64le
else ifeq (${PLATFORM}, ARM)
arch=arm64
else
arch=amd64
endif
env = CGO_ENABLED=0 GOOS=linux GOARCH=${arch}
BUILD_VERSION=github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants.CSIVersion
flag = -ldflags="-s -bindnow -X '${BUILD_VERSION}=${VER}'" -buildmode=pie

all:PREPARE BUILD COPY_FILE PACK

PREPARE:
	rm -rf ./${PACKAGE}
	mkdir -p ./${PACKAGE}/bin
	find . -name "*.yaml" -exec sed -i "s/{{csi-version}}/${VER}/g" {} +

BUILD:
# usage: [env] go build [-o output] [flags] packages
	go mod tidy
	${env} go build -o ./${PACKAGE}/bin/huawei-csi ${flag} ./csi
	${env} go build -o ./${PACKAGE}/bin/storage-backend-controller ${flag} ./cmd/storage-backend-controller
	${env} go build -o ./${PACKAGE}/bin/storage-backend-sidecar ${flag} ./cmd/storage-backend-sidecar
	${env} go build -o ./${PACKAGE}/bin/huawei-csi-extender ${flag} ./cmd/huawei-csi-extender
	${env} go build -o ./${PACKAGE}/bin/oceanctl ${flag} ./cli

COPY_FILE:
	mkdir -p ./${PACKAGE}/examples
	cp -r ./examples/* ./${PACKAGE}/examples

	mkdir -p ./${PACKAGE}/helm/
	cp -r ./helm/* ./${PACKAGE}/helm/

	mkdir -p ./${PACKAGE}/manual/
	cp -r ./manual/* ./${PACKAGE}/manual/
	cp -r ./helm/esdk/crds ./${PACKAGE}/manual/esdk/crds

PACK:
	zip -r ${PACKAGE}.zip ./${PACKAGE}
	rm -rf ./${PACKAGE}

GENERATE:
	go install go.uber.org/mock/mockgen@v0.5.0
	mockgen -source ./storage/oceanstorage/oceanstor/client/client.go -package mock_client \
	 -destination ./test/mocks/mock_client/oceanstor.go OceanstorClientInterface
	mockgen -source ./storage/fusionstorage/client/client.go -package mock_client \
	 -destination ./test/mocks/mock_client/fusionstorage.go IRestClient
	mockgen -source ./storage/oceanstorage/oceandisk/client/client.go -package mock_client \
	 -destination ./test/mocks/mock_client/oceandisk.go OceandiskClientInterface
	mockgen -source ./storage/oceanstorage/aseries/client/client.go -package mock_client \
     -destination ./test/mocks/mock_client/aseries.go OceanASeriesClientInterface
	mockgen -source ./storage/dme/aseries/client/client.go -package mock_client \
	  -destination ./test/mocks/mock_client/dme.go DMEASeriesClientInterface
