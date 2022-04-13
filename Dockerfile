FROM busybox:stable-glibc

ADD ["huawei-csi", "/"]
RUN ["chmod", "+x", "/huawei-csi"]
ENTRYPOINT ["/huawei-csi"]