/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2022-2025. All rights reserved.
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

package utils

import (
	"context"
	"fmt"
	"net"
	"os"
	"regexp"
	"slices"
	"strconv"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

// ChmodFsPermission used for change target path permission
func ChmodFsPermission(ctx context.Context, targetPath, fsPermission string) {
	reg := regexp.MustCompile(`^[0-7][0-7][0-7]$`)
	match := reg.FindStringSubmatch(fsPermission)
	if match == nil {
		log.AddContext(ctx).Errorf("fsPermission [%s] in storageClass.yaml format must be [0-7][0-7][0-7]. "+
			"Change directory [%v] fsPermission failed.", fsPermission, targetPath)
		return
	}

	// chmod need convert fsPermission to octal(8)
	mode, err := strconv.ParseInt(fsPermission, 8, 0)
	if err != nil {
		log.AddContext(ctx).Errorf("convert %s to int(octal) failed. error: %v", fsPermission, err)
		return
	}

	err = os.Chmod(targetPath, os.FileMode(mode))
	if err != nil {
		log.AddContext(ctx).Errorf("Failed to modify the directory permission. "+
			"targetPath: [%v], fsPermission: [%s]", targetPath, fsPermission)
		return
	}
	log.AddContext(ctx).Infof("Change directory [%s] to [%s] permission success.", targetPath, fsPermission)
}

// GetHostName retrieves the hostname of the system.
func GetHostName(ctx context.Context) (string, error) {
	hostname, err := ExecShellCmd(ctx, "hostname | xargs echo -n")
	if err != nil {
		return "", err
	}

	return hostname, nil
}

// GetHostIPs retrieves all non-loopback IP addresses of the system.
func GetHostIPs(ctx context.Context) ([]string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("cannot list interfaces: %w", err)
	}

	var result []string

	for _, iface := range ifaces {
		// skip loop back address
		if iface.Flags&net.FlagLoopback != 0 {
			log.AddContext(ctx).Infof("skip loop back interface %v", iface.Name)
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			return nil, fmt.Errorf("cannot get addrs for %s: %w", iface.Name, err)
		}

		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			default:
				log.AddContext(ctx).Warningf("unknown type %T", v)
			}
			if ip == nil {
				continue
			}
			if !ip.IsGlobalUnicast() {
				log.AddContext(ctx).Infof("skip none-global unicast interface %v", iface.Name)
				continue
			}

			result = append(result, ip.String())
		}
	}

	return result, nil
}

// FilterIPsByCIDRs returns the list of IPs that are contained by one or more of the CIDRs
func FilterIPsByCIDRs(ctx context.Context, ips, cidrs []string) ([]string, error) {
	var filteredIPs []string

	if len(cidrs) == 0 {
		return ips, nil
	}

	ipNets := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			return filteredIPs, fmt.Errorf("failed to parse cidr %s: %w", cidr, err)
		}
		ipNets = append(ipNets, ipNet)
	}

	for _, ip := range ips {
		filterFunc := func(ipNet *net.IPNet) bool {
			return ipNet.Contains(net.ParseIP(ip))
		}
		if !slices.ContainsFunc(ipNets, filterFunc) {
			log.AddContext(ctx).Infof("filtered out ip %s", ip)
			continue
		}

		filteredIPs = append(filteredIPs, ip)
	}

	return filteredIPs, nil
}
