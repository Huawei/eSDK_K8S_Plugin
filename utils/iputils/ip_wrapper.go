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

// Package iputils offers a series of methods for handling ip addresses
package iputils

import (
	"net"
)

const (
	pingIPv4Command = "ping -c 3 -i 0.001 -w 1 %s"
	pingIPv6Command = "ping -6 -c 3 -i 0.001 -w 1 %s"
)

// IPWrapper is a struct that represents an IP address
type IPWrapper struct {
	net.IP
}

// NewIPWrapper used to create a new IPWrapper instance
func NewIPWrapper(rawIP string) *IPWrapper {
	ip := net.ParseIP(rawIP)
	if ip == nil {
		return nil
	}

	return &IPWrapper{IP: ip}
}

// IsIPv4 used to check if the IP address is IPv4
func (ip *IPWrapper) IsIPv4() bool {
	return ip.To4() != nil
}

// GetPingCommand used to get ping command according to IP type
func (ip *IPWrapper) GetPingCommand() string {
	if ip.IsIPv4() {
		return pingIPv4Command
	}

	return pingIPv6Command
}

// GetFormatPortalIP used to get format portal ip according to IP type
func (ip *IPWrapper) GetFormatPortalIP() string {
	if ip.IsIPv4() {
		return ip.String()
	}

	return "[" + ip.String() + "]"
}
