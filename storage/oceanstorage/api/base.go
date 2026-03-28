/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2024-2025. All rights reserved.
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

// Package api provides all kinds of oceanstor storages restful urls definition
package api

// Base interface urls
const (
	// GetAllQos is the query path for getting all qos
	GetAllQos = "/ioclass"
)

const (
	// GetRoCENVMeInitiatorByID get roce-nvme initiator filter by initiator id
	GetRoCENVMeInitiatorByID = "/NVMe_over_RoCE_initiator/%s"

	// GetTcpNVMeInitiatorByID get tcp-nvme initiator filter by initiator id
	GetTcpNVMeInitiatorByID = "/nvme_over_tcp_initiator/%s"

	// CreateRoCENVMeInitiator create roce-nvme initiator
	CreateRoCENVMeInitiator = "/NVMe_over_RoCE_initiator"

	// CreateTcpNVMeInitiator create tcp-nvme initiator
	CreateTcpNVMeInitiator = "/nvme_over_tcp_initiator"

	// AddRoCENVMeInitiatorToHost add roce-nvme initiator to host
	AddRoCENVMeInitiatorToHost = "/host/create_associate"

	// AddTcpNVMeInitiatorToHost add tcp-nvme initiator to host
	AddTcpNVMeInitiatorToHost = "/nvme_over_tcp_initiator/create_associate"

	// GetIPV4Lif get logical ports of ipv4
	GetIPV4Lif = "/lif?filter=IPV4ADDR::%s"

	// GetIPV6Lif get logical ports of ipv6
	GetIPV6Lif = "/lif?filter=IPV6ADDR::%s"
)
