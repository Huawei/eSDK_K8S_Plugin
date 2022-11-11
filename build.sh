#!/bin/bash
#
#  Copyright (c) Huawei Technologies Co., Ltd. 2022-2022. All rights reserved.
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

# usage: sh build.sh unionpay X86

# [unionpay]
VER=$1
# [X86 ARM]
PLATFORM=$2

package_name="eSDK_Huawei_Storage_Kubernetes_CSI_Plugin_V${VER}_${PLATFORM}_64"

echo "Start to make with Makefile"
make -f Makefile VER=$1 PLATFORM=$2

echo "Start to pull busybox image with architecture"
if [[ "${PLATFORM}" == "ARM" ]];then
  docker pull --platform=arm64 busybox:stable-glibc
elif [[ "${PLATFORM}" == "X86" ]];then
  docker pull --platform=amd64 busybox:stable-glibc
else
  echo "Wrong PLATFORM, support [X86, ARM]"
  exit
fi

echo "Start to build image with Dockerfile"
rm -rf build_dir
rm -f ./huawei-csi
unzip -d build_dir -q ${package_name}.zip
cp build_dir/${package_name}/bin/huawei-csi ./
if [[ "${PLATFORM}" == "ARM" ]];then
  docker build --platform linux/arm64 -f Dockerfile -t huawei-csi:${VER} .
elif [[ "${PLATFORM}" == "X86" ]];then
  docker build --platform linux/amd64 -f Dockerfile -t huawei-csi:${VER} .
fi

echo "Start to save image file"
plat=$(echo ${PLATFORM}|tr 'A-Z' 'a-z')
docker save huawei-csi:${VER} -o huawei-csi-v${VER}-${plat}.tar
mkdir build_dir/${package_name}/image
mv huawei-csi-v${VER}-${plat}.tar build_dir/${package_name}/image

echo "Start to packing files"
rm -rf ${package_name}.zip
cd build_dir
zip -rq ../${package_name}.zip *

echo "Start to clear temporary files"
rm -f ./huawei-csi
rm -rf ./build_dir

echo "Build finish"
