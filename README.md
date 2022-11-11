# 银联容器项目--华为CSI

![GitHub](https://img.shields.io/github/license/Huawei/eSDK_K8S_Plugin)
[![Go Report Card](https://goreportcard.com/badge/github.com/huawei/esdk_k8s_plugin)](https://goreportcard.com/report/github.com/huawei/esdk_k8s_plugin)
![GitHub go.mod Go version (subdirectory of monorepo)](https://img.shields.io/github/go-mod/go-version/Huawei/eSDK_K8S_Plugin)

<img src="logo/csi.png" alt="Huawei CSI" width="100" height="100">

## 说明

本分支代码仅供银联容器项目使用。
## 编译

### 编译环境
| System | Go Version |
|---|---|
|Linux|    >=1.17|

### 编译步骤
步骤一：下载源代码，并进入到Makefile所在的目录下

步骤二. 执行编译命令

    make -f Makefile VER=unionpay PLATFORM=X86

步骤三. 使用bin目录下的huawei-csi二进制制作镜像，详细操作请参考docs文档。

    - bin
      - huawei-csi
      - secretGenerate
      - secretUpdate
## 文档

有关详细信息，请参阅docs目录中的用户指南。