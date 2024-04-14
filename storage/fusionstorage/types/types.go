/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2024. All rights reserved.
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

// Package types defines converged qoS request params
package types

const (
	// MaxIopsOfConvergedQoS defines max iops of converged qos
	MaxIopsOfConvergedQoS = 1073741824000

	// MaxMbpsOfConvergedQoS defines max mbps of converged qos
	MaxMbpsOfConvergedQoS = 1073741824

	// QosScaleNamespace defines namespace scale of qos
	QosScaleNamespace = 0

	// QosScaleClient defines client scale of qos
	QosScaleClient = 1

	// QosScaleAccount defines account scale of qos
	QosScaleAccount = 2

	// QosModeManual defines manual mode of qos
	QosModeManual = 3

	// NoQoSPolicyId defines qos policy id which is not found
	NoQoSPolicyId = -1

	// DefaultAccountName defines default account name
	DefaultAccountName = "system"

	// DefaultAccountId defines default account id
	DefaultAccountId = 0
)

// CreateConvergedQoSReq used to CreateConvergedQoS request
type CreateConvergedQoSReq struct {
	// (Mandatory) Upper limit control dimension.
	// The value can be:
	// 0:"NAMESPACE": namespace.
	// 1:"CLIENT": client.
	// 2:"ACCOUNT": account.
	QosScale int
	// (Mandatory) Name of a QoS policy.
	// When "qos_scale" is set to "NAMESPACE" or "CLIENT", the value is a string of 1 to 63 characters, including
	// digits, letters, hyphens (-), and underscores (_), and must start with a letter or digit.
	// When "qos_scale" is set to "ACCOUNT", the value is an account ID and is an integer ranging from 0 to 4294967293.
	Name string
	// (Mandatory) QoS mode.
	// When "qos_scale" is set to "NAMESPACE", the value can be "1" (by_usage), "2" (by_package), or "3" (manual).
	// When "qos_scale" is set to "CLIENT" or "ACCOUNT", the value can be "3" (manual).
	QosMode int
	// (Conditionally Mandatory) Bandwidth upper limit.
	// This parameter is mandatory when "qos_mode" is set to "manual".
	// The value is an integer ranging from 0 to 1073741824000(0 indicates no limit), in Mbit/s.
	MaxMbps int
	// (Conditionally Mandatory) OPS upper limit.
	// This parameter is mandatory when "qos_mode" is set to "manual".
	// The value is an integer ranging from 0 to 1073741824000(0 indicates no limit).
	MaxIops int
}

// AssociateConvergedQoSWithVolumeReq used to AssociateConvergedQoSWithVolume request
type AssociateConvergedQoSWithVolumeReq struct {
	// (Mandatory) qos_scale, Upper limit control dimension.
	// The value can be:
	// 0:"NAMESPACE": namespace.
	// 1:"CLIENT": client.
	// 2:"ACCOUNT": account.
	// 3:"USER": user.
	// 5:"HIDDEN_FS": hidden namespace.
	QosScale int

	// (Mandatory) object_name, Name of the associated object.
	// while qos_scale is NAMESPACE:
	// The associated object is a namespace. The value is a string of 1 to 255 characters. Only digits, letters,
	// underscores (_), periods (.), and hyphens (-) are supported.
	ObjectName string

	// (Mandatory) qos_policy_id, QoS policy ID.
	// The value is an integer ranging from 1 to 2147483647.
	QoSPolicyID int
}
