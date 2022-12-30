#!/bin/bash
#
#  Copyright (c) Huawei Technologies Co., Ltd. 2020-2022. All rights reserved.
#
#  Licensed under the Apache License, Version 2.0 (the "License");
#  you may not use this file except in compliance with the License.
#  You may obtain a copy of the License at
#
#       http://www.apache.org/licenses/LICENSE-2.0
#
#  Unless required by applicable law or agreed to in writing, software
#  distributed under the License is distributed on an "AS IS" BASIS,
#  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#  See the License for the specific language governing permissions and
#  limitations under the License.
#

export GOPROXY=http://mirrors.tools.huawei.com/goproxy/

make -f Makefile RELEASE_VER=$1 VER=$2 PLATFORM=$3

if [ ${PLATFORM} == X86 ];then
    wget http://10.29.160.97/busybox-x86.tar
    docker load -i busybox-x86.tar > imageidfile
    imageId=$(cat imageidfile|awk '{print $4}')
    docker tag ${imageId} busybox:stable-glibc
elif [ ${PLATFORM} == ARM ]; then
    wget http://10.29.160.97/busybox-arm.tar
    docker load -i busybox-arm.tar > imageidfile
    imageId=$(cat imageidfile|awk '{print $4}')
    docker tag ${imageId} busybox:stable-glibc
fi

package_name="eSDK_Huawei_Storage_${RELEASE_VER}_Kubernetes_CSI_Plugin_V${VER}_${PLATFORM}_64"
unzip -d k8s -q ${package_name}.zip
\cp -rf k8s/${package_name}/bin/huawei-csi .

if [ ${PLATFORM} == X86 ];then
    docker build --platform linux/amd64 -f Dockerfile -t huawei-csi:${VER} .
elif [ ${PLATFORM} == ARM ]; then
    docker build --platform linux/arm64 -f Dockerfile -t huawei-csi:${VER} .
fi

plat=$(echo ${PLATFORM}|tr 'A-Z' 'a-z')
docker save huawei-csi:${VER} -o huawei-csi-v${VER}-${plat}.tar

mkdir k8s/${package_name}/image
mv huawei-csi-v${VER}-${plat}.tar k8s/${package_name}/image

rm -rf ${package_name}.zip
cd k8s
zip -rq ../${package_name}.zip *
#签名
cd ..
mkdir sign
mv ${package_name}.zip sign
sh esdk_ci/ci/build_product_signature.sh $(pwd)/sign
mkdir cms
mv sign/*.cms .
sh esdk_ci/ci/build_product_signature_hwp7s.sh $(pwd)/sign
mv sign/* .