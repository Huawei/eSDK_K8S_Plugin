/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2022-2023. All rights reserved.
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

package plugin

import (
	"errors"
	"fmt"
	"net"

	pkgUtils "huawei-csi-driver/pkg/utils"
)

// verifyProtocolAndPortals verifyProtocolAndPortals
func verifyProtocolAndPortals(parameters map[string]interface{}) (string, []string, error) {
	protocol, exist := parameters["protocol"].(string)
	if !exist || protocol != ProtocolNfs && protocol != ProtocolNfsPlus {
		return "", []string{}, errors.New(fmt.Sprintf("protocol must be provided and be %s or %s for oceanstor-nas backend",
			ProtocolNfs, ProtocolNfsPlus))
	}
	portals, exist := parameters["portals"].([]interface{})
	if !exist || len(portals) == 0 {
		return "", []string{}, errors.New("portals must be provided for oceanstor-nas backend")
	}
	portalsStrs := pkgUtils.ConvertToStringSlice(portals)
	if protocol == ProtocolNfs && len(portalsStrs) != 1 {
		return "", []string{}, errors.New("portals just support one portal for oceanstor-nas backend nfs")
	}
	if protocol == ProtocolNfsPlus && !checkNfsPlusPortalsFormat(portalsStrs) {
		return "", []string{}, errors.New("portals must be ip or domain and can't both exist")
	}

	return protocol, portalsStrs, nil
}

func checkNfsPlusPortalsFormat(portals []string) bool {
	var portalsTypeIP bool
	var portalsTypeDomain bool

	for _, portal := range portals {
		ip := net.ParseIP(portal)
		if ip != nil {
			portalsTypeIP = true
			if portalsTypeDomain {
				return false
			}
		} else {
			portalsTypeDomain = true
			if portalsTypeIP {
				return false
			}
		}
	}

	return true
}
