/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2024-2024. All rights reserved.
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

package connector

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	fieldsLengthToGetVLun = 5
	diskNameDeleted       = "deleted"
)

const (
	idIndex = iota
	diskIndex
	nameIndex
	wwnIndex
	statusIndex
)

// reHctl is the regex to find the physical device, for example "6:0:0:2".
var reHctl = regexp.MustCompile("\\d:\\d:\\d:\\d")

// UltrapathVLun is the vLun of ultrapath.
type UltrapathVLun struct {
	ID     string
	Disk   string
	Name   string
	WWN    string
	Status string
	upType string
}

// GetUltrapathVLunByWWN gets the ultraPath vLun by the WWN of device.
func GetUltrapathVLunByWWN(ctx context.Context, upType, wwn string) (*UltrapathVLun, error) {
	res, err := GetUltraPathInfoByDevName(ctx, upType, wwn)
	if err != nil {
		if err.Error() == exitStatus1 && res == "" {
			return nil, nil
		}

		return nil, err
	}

	fields := strings.Fields(res)
	if len(fields) < fieldsLengthToGetVLun {
		return nil, fmt.Errorf("want [%d] fields to get vLun, but got [%d]", fieldsLengthToGetVLun, len(fields))
	}

	return &UltrapathVLun{
		ID:     fields[idIndex],
		Disk:   fields[diskIndex],
		Name:   fields[nameIndex],
		WWN:    fields[wwnIndex],
		Status: fields[statusIndex],
		upType: upType,
	}, nil
}

// CleanResidualPath clean residual path of vLun.
func (vLun *UltrapathVLun) CleanResidualPath(ctx context.Context) error {
	if vLun.Disk != diskNameDeleted {
		return nil
	}

	log.AddContext(ctx).Infof("the disk [%s] status is [%s], need to clean", vLun.Disk, vLun.Status)
	if err := vLun.cleanPhysicalPaths(ctx); err != nil {
		return err
	}

	log.AddContext(ctx).Infoln("device deleted, waiting 5s")
	time.Sleep(waitingDeletePeriod)

	return nil
}

func (vLun *UltrapathVLun) cleanPhysicalPaths(ctx context.Context) error {
	paths, err := vLun.getPhysicalPaths(ctx)
	if err != nil {
		log.AddContext(ctx).Warningf("get physical paths failed, error: %v", err)
		return err
	}

	for _, p := range paths {
		if err := p.clean(ctx); err != nil {
			log.AddContext(ctx).Warningf("clean path [%s] failed, error: %v", p.hctl, err)
		}
	}

	return nil
}

func (vLun *UltrapathVLun) getPhysicalPaths(ctx context.Context) ([]physicalPath, error) {
	output, err := runUpCommand(ctx, vLun.upType, "show vlun id=%s | grep -w Path", vLun.ID)
	if err != nil {
		return nil, err
	}

	var physicalPaths []physicalPath
	for _, subPath := range strings.Split(output, "\n") {
		if strings.TrimSpace(subPath) == "" {
			continue
		}

		physicalPaths = append(physicalPaths, physicalPath{
			hctl:   reHctl.FindString(subPath),
			status: getStatusFromPathResult(subPath),
		})
	}

	return physicalPaths, nil
}

type physicalPath struct {
	hctl   string
	status string
}

func (p physicalPath) clean(ctx context.Context) error {
	log.AddContext(ctx).Infof("start to delete physical device [%s], status is [%s]", p.hctl, p.status)
	if err := deletePhysicalDevice(ctx, p.hctl); err != nil {
		log.AddContext(ctx).Warningf("delete physical device [%s] failed, error is %v", p.hctl, err)
	}

	return nil
}

func getStatusFromPathResult(result string) string {
	segments := strings.Split(result, ":")
	if len(segments) > 0 {
		return strings.TrimSpace(segments[len(segments)-1])
	}

	return ""
}
