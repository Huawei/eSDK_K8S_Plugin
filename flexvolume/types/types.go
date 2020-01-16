package types

type CmdOptions struct {
	VolumeName string `json:"kubernetes.io/pvOrVolumeName"`
	ReadWrite  string `json:"kubernetes.io/readwrite"`
	FsType     string `json:"kubernetes.io/fsType"`
}

type Result struct {
	Status       string          `json:"status"`
	Message      string          `json:"message,omitempty"`
	Device       string          `json:"device,omitempty"`
	VolumeName   string          `json:"volumeName,omitempty"`
	Attached     bool            `json:"attached,omitempty"`
	Capabilities map[string]bool `json:"capabilities,omitempty"`
}
