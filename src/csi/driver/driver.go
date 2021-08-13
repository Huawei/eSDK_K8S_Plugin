package driver

type Driver struct {
	name            string
	version         string
	useMultiPath    bool
	isNeedMultiPath bool
}

func NewDriver(name, version string, useMultiPath, isNeedMultiPath bool) *Driver {
	return &Driver{
		name:            name,
		version:         version,
		useMultiPath:    useMultiPath,
		isNeedMultiPath: isNeedMultiPath,
	}
}
