
# Huawei Storage CSI Driver For Kubernetes
## Description

Huawei Container Storage Interface (CSI) Driver is used to provision LUN, recycle LUN, 
and provide a series of disaster recovery functions of storages for Kubernetes Containers.

## Compiling the Huawei CSI Driver
This section describes the environmental requirements and steps of compiling Huawei CSI Driver

### Compiler Environment
| System | Go Version |
|---|---|
|Linux|    >=1.17|

### Compilation steps
Step 1. Download the package and **cd** into the package

Step 2. Run following command to compile the Huawei CSI Driver

    make -f Makefile RELEASE_VER=[2.5.RC1] VER=[3.0.0] PLATFORM=[X86|ARM]

Step 3. After the compilation is finished, a bin directory will be created in the current 
directory, the structure is as follows:

    - bin
      - huawei-csi
      - secretGenerate
      - secretUpdate

In addition, we also provide a way to directly download the installation package, 
click Release to obtain the corresponding version of the plug-in package

## Documentation

For details, see the user guide in the docs directory.