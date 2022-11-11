FROM busybox:stable-glibc

LABEL maintainers="The Huawei CSI Team"
LABEL description="Huawei Storage CSI Driver"
LABEL version="unionpay"

COPY huawei-csi /

ENTRYPOINT ["/huawei-csi"]
