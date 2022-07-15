#!/bin/bash

DEBUG="false"
DEFAULT_IMAGE_LOAD_COMMAND="ctr -n k8s.io i import"
WORKER_LIST_FILE=""
IMAGE_FILE=""
USERNAME=""
PASSWORD=""

# verificationSupported checks whether necessary tools exist.
function verifySupported() {
  local hasExpect="$(type "expect" &>/dev/null && echo true || echo false)"
  if [ "${hasExpect}" != "true" ]; then
    echo "expect is not installed"
    exit 1
  fi

  hasScp="$(type "scp" &>/dev/null && echo true || echo false)"
  if [ "${hasScp}" != "true" ]; then
    echo "scp is not installed"
    exit 1
  fi

  hasSshpass="$(type "sshpass" &>/dev/null && echo true || echo false)"
  if [ "${hasSshpass}" != "true" ]; then
    echo "sshpass is not installed"
    exit 1
  fi
}

function tryLogin() {
  ip=$1

  /usr/bin/expect <<EOD
set result failed
set timeout -1
spawn ssh -oStrictHostKeyChecking=no -oCheckHostIP=no $USERNAME@$ip "echo successful"
expect {
  "yes/no" {send "yes\n"; exp_continue}
  "*assword" {send "${PASSWORD}\n";exp_continue}
  "successful" {
    set ::result successful
  }
}

if { "$::result" == "successful" } {
  exit 0
} else {
  exit 2
}

wait
EOD

  return $?
}

function uploadImage() {
  ip=$1

  sshpass -p $PASSWORD scp $IMAGE_FILE $USERNAME@$ip:~
  export SSHPASS=$PASSWORD
  result=$(sshpass -e ssh $USERNAME@$ip "${DEFAULT_IMAGE_LOAD_COMMAND} ${IMAGE_FILE};if [ \$? == 0 ];then echo successful; else echo failed; fi;rm -rf ${IMAGE_FILE}")
  echo $result
  if [[ $result =~ "successful" ]]; then
    return 0
  else
    return 2
  fi
}

function startTask() {
  failed_worker_array=()
  worker_list=($(cat $WORKER_LIST_FILE | grep -Ev '^$|#'))
  for i in $(seq 0 $((${#worker_list[*]} - 1))); do
    line=${worker_list[i]}

    tryLogin $line
    if [ $? != 0 ]; then
      echo -e "login failed. $line"
      failed_worker_array[${#failed_worker_array[*]}]=$line
      continue
    fi

    uploadImage $line
    if [ $? != 0 ]; then
      echo -e "The image is uploaded failed. $line"
      failed_worker_array[${#failed_worker_array[*]}]=$line
      continue
    fi
    echo -e "The image is uploaded successfully. $line"
  done

  if [ ${#failed_worker_array[*]} == 0 ]; then
    echo -e "All images are uploaded successfully"
  else
    echo "List of nodes to which the image fails to be imported:"
    for ((i = 0; i < ${#failed_worker_array[@]}; i++)); do
      echo -e "\t${failed_worker_array[$i]}"
    done
  fi
}

# help provides possible cli installation arguments
function help() {
  echo "NAME:"
  echo -e "\tScript for automatically uploading huawei csi driver images\n"
  echo "USAGE:"
  echo -e "\t$0 worker-list.txt huawei-csi.tar"
}

# Parsing input arguments (if any)
function parseArgs() {
  if [ $# != 2 ] || [ "$1" == "" ] || [ "$2" == "" ]; then
    help
    exit 0
  fi

  WORKER_LIST_FILE="${1}"
  IMAGE_FILE="${2}"

  echo "Please enter username:"
  read USERNAME
  echo "Please enter password:"
  read -s PASSWORD
}

# Execution

# Set debug if desired
if [ "${DEBUG}" == "true" ]; then
  set -x
fi

verifySupported
parseArgs "$@"
startTask
