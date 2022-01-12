
# Huawei Storage CSI Driver For Kubernetes
## Overview
Huawei Container Storage Interface (CSI) Driver is used to provision LUN, recycle LUN, 
and provide a series of disaster recovery functions of storages for Kubernetes Containers.

---
**NOTE:**

Users are informed that versions earlier than 2.2.RC4 are **NOT AVAILABLE FOR DOWNLOAD**. Versions earlier than 2.2.RC4 have the following problem: After a node on the Kubernetes platform is powered off unexpectedly, residual drive letters occur.After the node is powered on, the residual drive letters will cause disk scanning exceptions, which may affect service running.

This has been fixed with V2.2.13.

---

## Compatibility Matrix
| Features |1.16|1.17|1.18|1.19|1.20|1.21|
|---|---|---|---|---|---|---|
|Create PVC|√|√|√|√|√|√|
|Delete PVC|√|√|√|√|√|√|
|Create POD|√|√|√|√|√|√|
|Delete POD|√|√|√|√|√|√|
|Offline Resize|√|√|√|√|√|√|
|Online Resize|√|√|√|√|√|√|
|Create Snapshot|x|√|√|√|√|√|
|Delete Snapshot|x|√|√|√|√|√|
|Restore|x|√|√|√|√|√|
|Clone|x|√|√|√|√|√|

More details [release doc](https://github.com/Huawei/eSDK_K8S_Plugin/blob/V2.2.15/RELEASE.md)

## Compiling the Huawei CSI Driver
This section describes the environmental requirements and steps of compiling Huawei CSI Driver

### Compiler Environment
| System | Go Version |
|---|---|
|Linux|    >=1.14|

### Compilation steps
Step 1. Download the package and **cd** into the package, the structure is as follows:

    - docs
      - en
      - zh
    - src
      - csi
      - connector
      - proto
      - storage
      - tools
      - utils
      - vendor
    - yamls
      - deploy
      - example

Step 2. Run following command to compile the Huawei CSI Driver

    make -f Makefile-CSI PLATFORM=[X86|ARM]
 
Step 3. After the compilation is finished, a bin directory will be created in the current 
directory, the structure is as follows:

    - bin
      - huawei-csi
      - secretGenerate
      - secretUpdate

In addition, we also provide a way to directly download the installation package, 
click Release to obtain the 
corresponding version of the plug-in package
 
## Deployment and Examples
### Create Dockerfile and Make Huawei CSI Image
Ensure the docker has been installed on the host and host can access the networks in 
order to obtain some dependent software, then you can refer to the [documentation](
https://github.com/Huawei/eSDK_K8S_Plugin/tree/master/docs) and 
create your own Dockerfile.

    docker build -f Dockerfile -t huawei-csi:*.*.* 

Export and import the image

    docker save huawei-csi:*.*.* -o huawei-csi.tar

    docker load -i huawei-csi.tar

### Deploy The huawei-csi-rbac
Fill in the appropriate mirror version in [huawei-csi-rbac.yaml](https://github.com/Huawei/eSDK_K8S_Plugin/blob/master/yamls/deploy/huawei-csi-rbac.yaml),
 and then create the Huawei CSI Rbac.

    kubectl create -f huawei-csi-rbac.yaml

### Deploy the huawei-csi-configmap
Fill in the appropriate mirror version in [huawei-csi-configmap.yaml](https://github.com/Huawei/eSDK_K8S_Plugin/blob/master/yamls/deploy/huawei-csi-configmap-oceanstor-iscsi.yaml), and then create the Huawei CSI Configmap

    kubectl create -f huawei-csi-configmap.yaml

### Input the user and password of the storage device
    chmod +x secretGenerate
	./secretGenerate
   
### Deploy the huawei-csi-controller
Fill in the appropriate mirror version in [huawei-csi-controller.yaml](https://github.com/Huawei/eSDK_K8S_Plugin/blob/master/yamls/deploy/huawei-csi-controller.yaml), and then create the Huawei CSI Controller service

    kubectl create -f huawei-csi-controller.yaml

### Deploy the huawei-csi-node
Fill in the appropriate mirror version in [huawei-csi-node.yaml](https://github.com/Huawei/eSDK_K8S_Plugin/blob/master/yamls/deploy/huawei-csi-node.yaml),
 and then create the Huawei CSI Node service

    kubectl create -f huawei-csi-node.yaml

### Verify CSI Driver is running

    kubectl get pods -n kube-system

Also, we support our own logging service for the Huawei CSI Driver, the default directory is */var/log/huawei*. 
When you encounter some driver issues, you can get some help from these logs.

