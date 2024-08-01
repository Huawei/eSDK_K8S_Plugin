# Kubernetes CSI Driver for Huawei Storage

![GitHub](https://img.shields.io/github/license/Huawei/eSDK_K8S_Plugin)
[![Go Report Card](https://goreportcard.com/badge/github.com/huawei/esdk_k8s_plugin)](https://goreportcard.com/report/github.com/huawei/esdk_k8s_plugin)
![GitHub go.mod Go version (subdirectory of monorepo)](https://img.shields.io/github/go-mod/go-version/Huawei/eSDK_K8S_Plugin)
![GitHub Release Date](https://img.shields.io/github/release-date/Huawei/eSDK_K8S_Plugin)
![GitHub release (latest by date)](https://img.shields.io/github/downloads/Huawei/eSDK_K8S_Plugin/latest/total)

<img src="logo/csi.png" alt="Huawei CSI" width="100" height="100">

## Description

Huawei Container Storage Interface (CSI) Driver is used to provision LUN, recycle LUN, 
and provide a series of disaster recovery functions of storages for Kubernetes Containers.

## Compiling
This section describes the environmental requirements and steps of compiling Huawei CSI Driver

### Compiler Environment
| System | Go Version |
|---|---|
|Linux|    >=1.17|

### Compilation steps
Step 1. Download the package and **cd** into the package

Step 2. Run following command to compile the Huawei CSI Driver

    // PLATFORM support [X86|ARM]
    make -f Makefile VER=3.2.3 PLATFORM=X86

Step 3. After the compilation is finished, a bin directory will be created in the current 
directory, the structure is as follows:

    - bin
      - huawei-csi
      - secretGenerate
      - secretUpdate

In addition, we also provide a way to directly download the installation package, 
click [Release](https://github.com/Huawei/eSDK_K8S_Plugin/releases) to obtain the corresponding version of the plug-in package

## Documentation

For details, see the user guide in the docs directory.