/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2025-2025. All rights reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *       http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

// Package utils
package utils

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

func fsInfo(path string) (*unixFsInfo, error) {
	statfs := &unix.Statfs_t{}
	err := unix.Statfs(path, statfs)
	if err != nil {
		return nil, err
	}

	capacity := int64(statfs.Blocks) * int64(statfs.Bsize)
	available := int64(statfs.Bavail) * int64(statfs.Bsize)
	used := (int64(statfs.Blocks) - int64(statfs.Bfree)) * int64(statfs.Bsize)

	inodes := int64(statfs.Files)
	inodesFree := int64(statfs.Ffree)
	inodesUsed := inodes - inodesFree

	return &unixFsInfo{
		inodes:     inodes,
		inodesFree: inodesFree,
		inodesUsed: inodesUsed,
		available:  available,
		capacity:   capacity,
		usage:      used,
	}, nil
}

func execShellCmd(ctx context.Context, format string, logFilter bool, args ...any) (string, bool, error) {
	cmd := fmt.Sprintf(format, args...)
	log.AddContext(ctx).Infof(`Gonna run shell cmd %s.`, MaskSensitiveInfo(cmd))

	shCmd := exec.Command("nsenter")
	shCmd.Args = append(shCmd.Args,
		"-i/proc/1/ns/ipc", "-m/proc/1/ns/mnt", "-n/proc/1/ns/net", "-u/proc/1/ns/uts", "/bin/sh", "-c", cmd)

	killProcess, killProcessAndSubprocess := true, false
	timeoutDuration := time.Duration(app.GetGlobalConfig().ExecCommandTimeout) * time.Second
	// Processes are not killed when formatting or capacity expansion commands time out.
	if strings.Contains(cmd, "mkfs") || strings.Contains(cmd, "resize2fs") || strings.Contains(cmd, "xfs_growfs") {
		timeoutDuration = longTimeout * time.Second
		killProcess = false
	} else if strings.Contains(cmd, "mount") {
		killProcessAndSubprocess = true
		shCmd.SysProcAttr = &syscall.SysProcAttr{}
		shCmd.SysProcAttr.Setpgid = true
	}

	var timeout, commandComplete bool
	var output []byte
	var err error
	time.AfterFunc(timeoutDuration, func() {
		timeout = true
		if !killProcess {
			return
		}

		if killProcessAndSubprocess {
			if !commandComplete && len(output) == 0 && err == nil {
				log.AddContext(ctx).Warningf(
					"Exec mount command: [%s] time out, try to kill this processes and subprocesses. Pid: [%d].",
					cmd, shCmd.Process.Pid)
				errKill := syscall.Kill(-shCmd.Process.Pid, syscall.SIGKILL)
				log.AddContext(ctx).Infof("Kill result: [%v]", errKill)
			}
			return
		}

		if !commandComplete {
			err = shCmd.Process.Kill()
		}
	})

	output, err = shCmd.CombinedOutput()
	commandComplete = true
	if err != nil {
		log.AddContext(ctx).Warningf(`Run shell cmd %s output: [%s], error: [%v]`, MaskSensitiveInfo(cmd),
			MaskSensitiveInfo(output), MaskSensitiveInfo(err))
		return string(output), timeout, err
	}

	if !logFilter {
		log.AddContext(ctx).Infof(`Shell cmd %s result:\n%s`, MaskSensitiveInfo(cmd), MaskSensitiveInfo(output))
	}

	return string(output), timeout, nil
}
