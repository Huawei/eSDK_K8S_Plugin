FROM busybox:stable-glibc

LABEL maintainers="The Huawei CSI Team"
LABEL description="Huawei Storage CSI Driver"
LABEL version="3.1.0"

COPY huawei-csi /

ENTRYPOINT ["/huawei-csi"]
