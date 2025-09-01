# eg: docker build --target huawei-csi-driver --platform linux/amd64 --build-arg VERSION=${VER} -f Dockerfile -t huawei-csi:${VER} .
ARG VERSION

FROM alpine

LABEL version="${VERSION}"
LABEL maintainers="Huawei eSDK CSI development team"
LABEL description="Kubernetes CSI Driver for Huawei Storage: $VERSION"

RUN apk update && apk add --no-cache xfsprogs xfsprogs-extra findmnt blkid
ARG binary=./huawei-csi
COPY ${binary} huawei-csi
ENTRYPOINT ["/huawei-csi"]


# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/base:latest as storage-backend-controller
LABEL version="${VERSION}"
LABEL maintainers="Huawei eSDK CSI development team"
LABEL description="Storage Backend Controller"

ARG binary=./storage-backend-controller
COPY ${binary} storage-backend-controller
ENTRYPOINT ["/storage-backend-controller"]


FROM gcr.io/distroless/base:latest as storage-backend-sidecar
LABEL version="${VERSION}"
LABEL maintainers="Huawei eSDK CSI development team"
LABEL description="Storage Backend Sidecar"

ARG binary=./storage-backend-sidecar
COPY ${binary} storage-backend-sidecar
ENTRYPOINT ["/storage-backend-sidecar"]


FROM gcr.io/distroless/base:latest as huawei-csi-extender
LABEL version="${VERSION}"
LABEL maintainers="Huawei eSDK CSI development team"
LABEL description="Huawei CSI Extender"

ARG binary=./huawei-csi-extender
COPY ${binary} huawei-csi-extender
ENTRYPOINT ["/huawei-csi-extender"]
