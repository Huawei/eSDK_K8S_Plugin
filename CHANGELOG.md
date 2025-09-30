# Change Log

[Releases](https://github.com/Huawei/eSDK_K8S_Plugin/releases)

## v4.9.0
- Support Kubernetes 1.33
- Support Oceanstor / Oceanstor Dorado V700R001C10
- Added support for Pacific block storage with iSCSI networking multipath optimization
- Added support for Oceanstor A-series Storage (local filesystem with NFS/DataTurbo protocol)
- Added support for nvme-cli 2.x
- Fixed NFS protocol mount failures when using domain names
- Fixed SCSI protocol mount failures during Pod restarts in StatefulSet configurations
- Reorganized User Guide structure with dedicated sections for storage and protocol content

## v4.8.0

- Added support for OpenShift 4.18
- Enabled IPv6 integration with Huawei Enterprise Storage
- Added LDAP user login support for Huawei Enterprise Storage
- Custom alert thresholds now supported for oceanstor-nas storage type
- Custom storage resource names for dynamic volume provision (Applies to OceanStor and OceanStor Dorado storage products)
- Introduced new common parameters in Helm values.yaml:
  `imagePullSecrets`, `resources`, and `affinity` configurations
- Upgraded VolumeSnapshot CRDs to align with the external-snapshot sidecar version used by Huawei CSI

## v4.7.0

- Support Kubernetes 1.32
- Support Oceanstor Pacific 8.2.1
- Support Oceanstor Pacific DTree feature
- Add the `csiDriver.enableRoCEConnect` parameter to the Helm values.yaml file to allow disabling automatic disk scanning when using the RoCE protocol

## v4.6.0

- Support Kubernetes 1.31.
- Support Openshift 4.16/4.17.
- Support OceanStor Dorado V700R001C00.
- Supports the PPC64LE CPU architecture of the IBM Power platform.
- Support for Red Hat Enterprise Linux 8.6/8.7/8.8/8.9/8.10/9.4 x86_64.
- Support NFS 4.2 on OceanStor Dorado storage 6.1.8 and later version.
- Support NFS over RDMA on OceanStor Pacific storage 8.2.0 and later version.
- Added `disableVerifyCapacity` parameter in StorageClass whether allow to disable volume capacity verification.
- Added the restriction which is 1~30 on the `maxClientThreads` parameter in the backend.
- Fixed an issue where raw volumes may be misplaced when they are powered off unexpectedly.

## v4.5.0

- The default synchronization speed of hyper metro pair is changed from the highest speed to the default speed determined by the storage.
- Fixed the semaphore timeout issue when a large number of PVs fail to be mounted.

## v4.4.0

- Support Kubernetes 1.30
- Support OceanStor 6.1.8
- Support OceanStor Dorado 6.1.8
- Support Red Hat CoreOS 4.15 x86_64
- Support OpenEuler 22.03 LTS SP1 x86_64
- The new feature Modify Volume allows a normal PV to be changed to a HyperMetro PV
- Create VolumeSnapshot and Clone Persistent Volume support HyperMetro PV

## v4.3.0

- Support UltraPath 31.2.1/NVMe over RoCE on Rocky Linux 8.6 X86_64
- Support OpenShift 4.14
- Support OceanStor Dorado 6.1.7
- Support OceanStor 6.1.7
- Support OceanStor Pacific 8.2.0
- Support Tanzu Kubernetes TKGI 1.17 and TKGI 1.18
- Support Debian 12 x86_64
- Support Ubuntu 22.04 ARM
- Support Kylin V10 SP3 x86_64
- Support Red Hat CoreOS 4.14 x86_64
- Support EulerOS V2R12 ARM/x86_64
- Support UOS V20 x86_64
- Support BC-linux ARM
- Support NFS v4 Kerberos encryption in Dorado V6
- Support NFS over RDMA
- Support configuring requests and limits of container
- The log directory of the oceanctl tool can be configured

## v4.2.0

- Support OpenShift 4.13
- Support Centos 8.4 X86_64
- Support Rocky Linux 8.6 X86_64
- Support EulerOS V2R11 X86_64
- Support k8s 1.16 and 1.27
- Support configuring the timeout for executing commands
- Support create volume snapshot for Hyper-Metro

## v4.1.0

**Enhancements**

- Support OpenShift 4.12
- Support Debian 9 x86_64
- Support EulerOS V2R9 X86_64
- Support Kylin 7.6 x86_64
- The number of path groups aggregated by DM-multipath can be configured

## v4.0.0

**Enhancements**

- CSI Controller supports multi-copy deployment.
- CSI supports volume management(Static PV enhancement).
- The oceanctl tool is added for backend management.
- CSI nodes do not depend on the storage management network.
- Node selection and taint tolerance can be configured.
- Adding CSIDriver Resources for CSI.
- CSI supports the configuration of Volume Limits.
- Support k8s 1.26
- Upgrade using go 1.18

## v3.2.0

**Enhancements**

- The PV prefix can be customized.
- The description of the filesystem can be customized.
- The reserved snapshot space can be modified.
- The Pacific namespace can be expanded.
- Support k8s 1.13 - 1.25
- Support OpenShift 4.11
- Support Ubuntu 22.04, SUSE 15 SP3
- Support EulerOS V2R10

## v3.1.0

**Enhancements**

- Support Generic ephemeral volumes
- Support Customizing Volume Directory Permissions
- Support Dorado V6 NAS 6.1.5 Clone and Snapshot Restore
- Support Dorado V6 NAS 6.1.2 System VStrore HyperMetro
- Support k8s 1.19 and 1.20
- Support Debian 11

## v3.0.0

**Enhancements**

- Support Helm
- Support ReadWriteOncePod
- Support NFS 4.0/4.1
- Support DPC
- Support Kubernets 1.24
- Support Red Hat CoreOS 4.10 x86_64
- Support Kylin V10 SP1/SP2 ARM
- Support OceanStor Dorado V6 6.1.5
- Support OceanStor V6 6.1.5
- Support OceanStor Pacific 8.1.3

## v2.2.16

**Enhancements**

- Support Raw Block
- Support Static pv
- Support NAS HyperMetro
- Support NVMe over RoCE/NVMe over FC
- Support UltraPath 31.1.0/UltraPath-NVMe 31.1.RC8
- Support Kubernets 1.22/1.23
- Support Ubuntu 18.04/20.04
- Support Centos 7.9
