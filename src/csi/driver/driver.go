package driver

type Driver struct {
	name    string
	version string
}

func NewDriver(name, version string) *Driver {
	return &Driver{
		name:    name,
		version: version,
	}
}
