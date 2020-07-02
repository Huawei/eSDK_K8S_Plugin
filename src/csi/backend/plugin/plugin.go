package plugin

type Plugin interface {
	NewPlugin() Plugin
	Init(map[string]interface{}, map[string]interface{}, bool) error
	CreateVolume(string, map[string]interface{}) (string, error)
	DeleteVolume(string) error
	ExpandVolume(string, int64) (bool, error)
	AttachVolume(string, map[string]interface{}) error
	DetachVolume(string, map[string]interface{}) error
	UpdateBackendCapabilities() (map[string]interface{}, error)
	UpdatePoolCapabilities([]string) (map[string]interface{}, error)
	StageVolume(string, map[string]interface{}) error
	UnstageVolume(string, map[string]interface{}) error
	UpdateMetroRemotePlugin(Plugin)
	UpdateReplicaRemotePlugin(Plugin)
	NodeExpandVolume(string, string) error
	CreateSnapshot(string, string) (map[string]interface{}, error)
	DeleteSnapshot(string, string) error
}

var (
	plugins = map[string]Plugin{}
)

func RegPlugin(storageType string, plugin Plugin) {
	plugins[storageType] = plugin
}

func GetPlugin(storageType string) Plugin {
	if plugin, exist := plugins[storageType]; exist {
		return plugin.NewPlugin()
	}

	return nil
}

type basePlugin struct {
}

func (p *basePlugin) AttachVolume(string, map[string]interface{}) error {
	return nil
}

func (p *basePlugin) DetachVolume(string, map[string]interface{}) error {
	return nil
}

func (p *basePlugin) UpdateMetroRemotePlugin(Plugin) {
}

func (p *basePlugin) UpdateReplicaRemotePlugin(Plugin) {
}
