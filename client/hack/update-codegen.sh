#!/bin/bash

# Copyright (c) Huawei Technologies Co., Ltd. 2022-2022. All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#      http://www.apache.org/licenses/LICENSE-2.0
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY GROUP, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

# generate-groups.sh parameter introduction
# <generators>        the generators comma separated to run (deepcopy,defaulter,client,lister,informer) or "all".
# <output-package>    the output package name (e.g. github.com/example/project/pkg/generated).
# <apis-package>      the external types dir (e.g. github.com/example/apis or github.com/example/project/pkg/apis).
# <groups-versions>   the groups and their versions in the format "groupA:v1,v2 groupB:v1 groupC:v2", relative
#                      to <apis-package>.

# parameters that need to be modified
# go mod name
MODULE=huawei-csi-driver
# crd group name
GROUP=xuanwu
# code folder location, modify according to your project location
OUTPUT_BASE=/root/CSI

# clear the old file before executing script
if  [ -d "${OUTPUT_BASE}/${MODULE}" ];
  then
    rm -rf ${OUTPUT_BASE}/${MODULE}
    echo "Delete the old generated file ${OUTPUT_BASE}/${MODULE}"
  else
    echo "Generate file ${OUTPUT_BASE}/${MODULE} by code-generator"
fi

# execute script
# make sure the directory structure is correct
bash ./vendor/k8s.io/code-generator/generate-groups.sh \
      "deepcopy,client,informer,lister" \
      ${MODULE}/pkg/client \
      ${MODULE}/client/apis \
      ${GROUP}:v1 \
      --go-header-file ./client/hack/boilerplate.go.txt \
      --output-base ${OUTPUT_BASE}

# copy the newly generated zz_generated.deepcopy.go to the source directory,
# or replace the old zz_generated.deepcopy.go if the old one exists
cp ${OUTPUT_BASE}/${MODULE}/client/apis/${GROUP}/v1/zz_generated.deepcopy.go  ${OUTPUT_BASE}/client/apis/${GROUP}/v1
echo "Copy the newly generated zz_generated.deepcopy.go from ./${MODULE}/client/apis/${GROUP}/v1 to the ./client/apis/${GROUP}/v1"

# delete the temporary directory of the newly generated /client/apis/ file
rm -rf ${OUTPUT_BASE}/${MODULE}/client/apis/

# delete the old listers,informers and clientset before copy new ons to the ./client/
files=("clientset" "informers" "listers")
for file in ${files[@]}
do
  if  [ -d "${OUTPUT_BASE}/pkg/client/${file}"  ];
    then
    rm -rf ${OUTPUT_BASE}/pkg/client/${file}
    echo "Delete old file ${OUTPUT_BASE}/client/${file}"
  fi
done

# copy the generated lister informer clientset
cp -rf ${OUTPUT_BASE}/${MODULE}/pkg/client/*  ${OUTPUT_BASE}/pkg/client/
echo "Copy the newly generated listers,informers and clientset from ./${MODULE}/pkg/client/ to ./pkg/client/"

# delete the newly generated file
rm -rf ${OUTPUT_BASE}/github.com/
echo "Delete the newly generated directory ${OUTPUT_BASE}/github.com/"
