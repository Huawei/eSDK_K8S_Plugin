FROM busybox:stable-glibc

LABEL maintainers="The Huawei CSI Team"
LABEL description="Kubernetes CSI Driver for Huawei Storage"
LABEL version="3.2.3"

COPY huawei-csi /

ENTRYPOINT ["/huawei-csi"]
