#!/bin/bash
#
#  Copyright (c) Huawei Technologies Co., Ltd. 2022-2023. All rights reserved.
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

# usage: bash build.sh {VER} {PLATFORM}

# [x.y.z]
VER=$1
# [X86 ARM]
PLATFORM=$2

package_name="eSDK_Storage_CSI_V${VER}_${PLATFORM}_64"

echo "Start to make with Makefile"
make -f Makefile VER=$1 PLATFORM=$2

echo "Platform confirmation"
if [[ "${PLATFORM}" == "ARM" ]];then
  PULL_FLAG="--platform=arm64"
  BUILD_FLAG="--platform linux/arm64"
  GO_PLATFORM="arm64"
elif [[ "${PLATFORM}" == "X86" ]];then
  PULL_FLAG="--platform=amd64"
  BUILD_FLAG="--platform linux/amd64"
  GO_PLATFORM="amd64"
elif [[ "${PLATFORM}" == "PPC64LE" ]];then
  PULL_FLAG="--platform=ppc64le"
  BUILD_FLAG="--platform linux/ppc64le"
  GO_PLATFORM="ppc64le"
else
  echo "Wrong PLATFORM, support [X86, ARM, PPC64LE]"
  exit
fi

echo "Start to pull busybox image with architecture ${PULL_FLAG}"
docker pull ${PULL_FLAG} busybox:stable-glibc
docker pull ${PULL_FLAG} gcr.io/distroless/base:latest

echo "Start to build image with Dockerfile"
rm -rf build_dir
rm -f ./huawei-csi
rm -f ./storage-backend-controller
rm -f ./storage-backend-sidecar
rm -f ./huawei-csi-extender
unzip -d build_dir -q ${package_name}.zip
mv build_dir/${package_name}/bin/huawei-csi ./
mv build_dir/${package_name}/bin/storage-backend-controller ./
mv build_dir/${package_name}/bin/storage-backend-sidecar ./
mv build_dir/${package_name}/bin/huawei-csi-extender ./

docker build ${BUILD_FLAG} --build-arg VERSION=${VER} --target huawei-csi-driver -f Dockerfile -t huawei-csi:${VER} .
docker build ${BUILD_FLAG} --build-arg VERSION=${VER} --target storage-backend-controller -f Dockerfile -t storage-backend-controller:${VER} .
docker build ${BUILD_FLAG} --build-arg VERSION=${VER} --target storage-backend-sidecar -f Dockerfile -t storage-backend-sidecar:${VER} .
docker build ${BUILD_FLAG} --build-arg VERSION=${VER} --target huawei-csi-extender -f Dockerfile -t huawei-csi-extender:${VER} .

echo "Start to save image file"
plat=$(echo ${PLATFORM}|tr 'A-Z' 'a-z')
docker save huawei-csi:${VER} -o huawei-csi-v${VER}-${plat}.tar
docker save storage-backend-controller:${VER} -o storage-backend-controller-v${VER}-${plat}.tar
docker save storage-backend-sidecar:${VER} -o storage-backend-sidecar-v${VER}-${plat}.tar
docker save huawei-csi-extender:${VER} -o huawei-csi-extender-v${VER}-${plat}.tar

echo "Start to move image file"
mkdir build_dir/${package_name}/image
mv huawei-csi-v${VER}-${plat}.tar build_dir/${package_name}/image
mv storage-backend-controller-v${VER}-${plat}.tar build_dir/${package_name}/image
mv storage-backend-sidecar-v${VER}-${plat}.tar build_dir/${package_name}/image
mv huawei-csi-extender-v${VER}-${plat}.tar build_dir/${package_name}/image

echo "Start to packing files"
rm -rf ${package_name}.zip
cd build_dir
zip -rq ../${package_name}.zip *
cd ..

echo "Start to clear temporary files"
rm -f ./huawei-csi
rm -f ./huawei-csi-extender
rm -f ./storage-backend-controller
rm -f ./storage-backend-sidecar
rm -rf ./build_dir

echo "Build finish"
