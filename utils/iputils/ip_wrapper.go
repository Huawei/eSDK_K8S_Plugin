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
	pingCommand     = "ping -c 3 -i 0.001 -w 1 %s"
	pingIPv6Command = "ping -6 -c 3 -i 0.001 -w 1 %s"
)

// IPDomainWrapper is a struct that represents an IP address
type IPDomainWrapper struct {
	ip  net.IP
	raw string
}

// NewIPDomainWrapper used to create a new IPDomainWrapper instance
func NewIPDomainWrapper(rawIP string) *IPDomainWrapper {
	if rawIP == "" {
		return nil
	}

	ip := net.ParseIP(rawIP)
	return &IPDomainWrapper{ip: ip, raw: rawIP}
}

// IsIPv4 used to check if the IP address is IPv4
func (w *IPDomainWrapper) IsIPv4() bool {
	return w.ip != nil && w.ip.To4() != nil
}

// IsDN used to check if the IP address is domain name
func (w *IPDomainWrapper) IsDN() bool {
	return w.ip == nil
}

// GetPingCommand used to get ping command according to IP type
func (w *IPDomainWrapper) GetPingCommand() string {
	if w.IsDN() || w.IsIPv4() {
		return pingCommand
	}

	return pingIPv6Command
}

// GetFormatPortalIP used to get format portal ip according to IP type
func (w *IPDomainWrapper) GetFormatPortalIP() string {
	if w.IsDN() {
		return w.raw
	}

	if w.IsIPv4() {
		return w.ip.String()
	}

	return "[" + w.ip.String() + "]"
}

// String used to get ip string
func (w *IPDomainWrapper) String() string {
	if w.IsDN() {
		return w.raw
	}

	return w.ip.String()
}
