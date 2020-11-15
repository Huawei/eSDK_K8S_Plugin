
# Huawei Storage CSI Driver For Kubernetes
## Overview
Huawei Container Storage Interface (CSI) Driver is used to provision LUN, recycle LUN, 
and provide a series of disaster recovery functions of storages for Kubernetes Containers.

## Compatibility Matrix
| Features | 1.13|1.14|1.15|1.16|1.17|1.17|
|---|---|---|---|---|---|---|
|Create PVC|√|√|√|√|√|√|
|Delete PVC|√|√|√|√|√|√|
|Create POD|√|√|√|√|√|√|
|Delete POD|√|√|√|√|√|√|
|Offline Resize|x|x|x|√|√|√|
|Online Resize|x|x|x|√|√|√|
|Create Snapshot|x|x|x|x|√|√|
|Delete Snapshot|x|x|x|x|√|√|
|Restore|x|x|x|x|√|√|
|Clone|x|x|x|x|√|√|

## Compiling the Huawei CSI Driver
This section describes the environmental requirements and steps of compiling Huawei CSI Driver

### Compiler Environment
| System | Go Version |
|---|---|
|Linux|    >=1.10|

### Compilation steps
Step 1. Download the package and **cd** into the package, the structure is as follows:

    - docs
      - en
      - zh
    - src
      - csi
      - dev
      - proto
      - storage
      - tools
      - utils
      - vendor
    - yamls
      - deploy
      - example

step 2. Change the [keyText](https://github.com/Huawei/eSDK_K8S_Plugin/blob/master/src/utils/pwd/pwd.go#L11) 
to a private value to prevent hacking. The length of the keyText is 32 characters. Such as:

    keyText  = []byte("astaxie1279dkjajzmknm.ahkjkljl;k")

Step 3. Run following command to compile the Huawei CSI Driver

    make -f Makefile-CSI PLATFORM=[X86|ARM]
 
Step 4. After the compilation is finished, a bin directory will be created in the current 
directory, the structure is as follows:

    - bin
      - huawei-csi
      - passwdEncrypt
 
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

