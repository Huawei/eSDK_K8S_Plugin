package plugin

type Plugin interface {
	NewPlugin() Plugin
	Init(map[string]interface{}, map[string]interface{}) error
	CreateVolume(string, map[string]interface{}) (string, error)
	DeleteVolume(string) error
	AttachVolume(string, map[string]interface{}) error
	DetachVolume(string, map[string]interface{}) error
	UpdateBackendCapabilities() (map[string]interface{}, error)
	UpdatePoolCapabilities([]string) (map[string]interface{}, error)
	StageVolume(string, map[string]interface{}) error
	UnstageVolume(string, map[string]interface{}) error
	UpdateMetroRemotePlugin(remote Plugin)
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
