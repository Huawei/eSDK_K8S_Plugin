#!/bin/bash
#
#  Copyright (c) Huawei Technologies Co., Ltd. 2020-2023. All rights reserved.
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

if [ ${PLATFORM} == X86 ];then
    wget http://10.29.160.97/busybox-x86.tar
    wget http://10.29.160.97/gcr-x86.tar
    docker load -i busybox-x86.tar > imageidfile
    docker load -i gcr-x86.tar > imageidfile1
    imageId=$(cat imageidfile|awk '{print $3}')
    imageId1=$(cat imageidfile1|awk '{print $3}')
    docker tag busybox:1.36.1 busybox:stable-glibc
    docker tag ${imageId1} gcr.io/distroless/base:latest
elif [ ${PLATFORM} == ARM ]; then
    wget http://10.29.160.97/busybox-arm.tar
    wget http://10.29.160.97/gcr-arm.tar
    docker load -i busybox-arm.tar > imageidfile
    docker load -i gcr-arm.tar > imageidfile1
    imageId=$(cat imageidfile|awk '{print $3}')
    imageId1=$(cat imageidfile1|awk '{print $3}')
    docker tag busybox:1.36.1 busybox:stable-glibc
    docker tag ${imageId1} gcr.io/distroless/base:latest
fi

package_name="eSDK_Huawei_Storage_${RELEASE_VER}_Kubernetes_CSI_Plugin_V${VER}_${PLATFORM}_64"

cd eSDK_Enterprise_Storage_Kubernetes
make -f Makefile RELEASE_VER=$1 VER=$2 PLATFORM=$3

# -------------------------------------------------------------------------------
echo "Platform confirmation"
if [[ "${PLATFORM}" == "ARM" ]];then
  PULL_FLAG="--platform=arm64"
  BUILD_FLAG="--platform linux/arm64"
elif [[ "${PLATFORM}" == "X86" ]];then
  PULL_FLAG="--platform=amd64"
  BUILD_FLAG="--platform linux/amd64"
else
  echo "Wrong PLATFORM, support [X86, ARM]"
  exit
fi

rm -rf build_dir
rm -f ./huawei-csi
rm -f ./storage-backend-controller
rm -f ./storage-backend-sidecar
unzip -d build_dir -q ${package_name}.zip
cp build_dir/${package_name}/bin/huawei-csi ./
cp build_dir/${package_name}/bin/storage-backend-controller ./
cp build_dir/${package_name}/bin/storage-backend-sidecar ./

docker build ${BUILD_FLAG} --build-arg VERSION=${VER} --target huawei-csi-driver -f Dockerfile -t huawei-csi:${VER} .
docker build ${BUILD_FLAG} --build-arg VERSION=${VER} --target storage-backend-controller -f Dockerfile -t storage-backend-controller:${VER} .
docker build ${BUILD_FLAG} --build-arg VERSION=${VER} --target storage-backend-sidecar -f Dockerfile -t storage-backend-sidecar:${VER} .


plat=$(echo ${PLATFORM}|tr 'A-Z' 'a-z')
docker save huawei-csi:${VER} -o huawei-csi-v${VER}-${plat}.tar
docker save storage-backend-controller:${VER} -o storage-backend-controller-v${VER}-${plat}.tar
docker save storage-backend-sidecar:${VER} -o storage-backend-sidecar-v${VER}-${plat}.tar


mkdir build_dir/${package_name}/image
mv huawei-csi-v${VER}-${plat}.tar build_dir/${package_name}/image
mv storage-backend-controller-v${VER}-${plat}.tar build_dir/${package_name}/image
mv storage-backend-sidecar-v${VER}-${plat}.tar build_dir/${package_name}/image
# -------------------------------------------------------------------------------

rm -rf ${package_name}.zip
cd build_dir
zip -rq ../${package_name}.zip *
# 签名
cd ..
mkdir sign
mv ${package_name}.zip sign
sh esdk_ci/ci/build_product_signature.sh $(pwd)/sign
mkdir cms
mv sign/*.cms .
sh esdk_ci/ci/build_product_signature_hwp7s.sh $(pwd)/sign
mv sign/* .