package types

const (
	MaxIopsOfConvergedQoS = 1073741824000
	MaxMbpsOfConvergedQoS = 1073741824

	QosScaleNamespace = 0
	QosScaleClient    = 1
	QosScaleAccount   = 2

	QosModeManual = 3

	NoQoSPolicyId = -1

	DefaultAccountName = "system"
	DefaultAccountId   = 0
)

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
