package config

import (
	"encoding/json"
	"io/ioutil"
	"net"
	"proto"
	"regexp"
	"utils/pwd"

	"github.com/golang/glog"
)

type Backend struct {
	Urls             []string
	User             string
	Password         string
	VstoreName       string
	Options          map[string]interface{}
	HyperMetroDomain string
}

type FlexVolumeConfig struct {
	Backends       []map[string]interface{} `json:"backends"`
	Proto          string                   `json:"proto"`
	LogFilePrefix  string                   `json:"logFilePrefix"`
	MaxLogFileSize string                   `json:"maxLogFileSize"`
	LogDir         string                   `json:"logDir"`
}

var (
	Config   FlexVolumeConfig
	Backends = make(map[string]*Backend)
)

func ParseConfig(path string) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		glog.Fatalf("Read config file error: %v", err)
	}

	err = json.Unmarshal(data, &Config)
	if err != nil {
		glog.Fatalf("Parse config file error: %v", err)
	}

	if len(Config.Backends) <= 0 {
		glog.Fatalln("Must config at least one backend")
	}

	for _, backend := range Config.Backends {
		name, exist := backend["name"].(string)
		if !exist {
			glog.Fatalln("Backend config must have a name")
		} else {
			match, err := regexp.MatchString("^[a-z0-9]+$", name)
			if err != nil || !match {
				glog.Fatalf("Backend name %v is invalid, only support consisted by lower characters and numeric", name)
			}
		}

		user, exist := backend["user"].(string)
		if !exist || user == "" {
			glog.Fatalf("Backend config %s must have a user", name)
		}

		password, exist := backend["password"].(string)
		if !exist || password == "" {
			glog.Fatalf("Backend config %s must have a password", name)
		}
		decoded, err := pwd.Decrypt(password)
		if err != nil {
			glog.Fatalf("Decrypt password error: %v", err)
		}

		configUrls, exist := backend["urls"].([]interface{})
		if !exist || len(configUrls) <= 0 {
			glog.Fatalf("Backend config %s doesn't have valid urls", name)
		}

		var urls []string
		for _, url := range configUrls {
			urls = append(urls, url.(string))
		}

		vstoreName, _ := backend["vstoreName"].(string)

		hyperMetroDomain, _ := backend["hyperMetroDomain"].(string)

		Backends[name] = &Backend{
			Urls:             urls,
			User:             user,
			Password:         decoded,
			Options:          make(map[string]interface{}),
			HyperMetroDomain: hyperMetroDomain,
			VstoreName:       vstoreName,
		}
	}
}

func ParseNasConfig(path string) {
	ParseConfig(path)

	for _, backend := range Config.Backends {
		name := backend["name"].(string)

		options, exist := backend["options"].(map[string]interface{})
		if !exist {
			glog.Fatalf("Backend %s doesn't have options config", name)
		}

		portal, exist := options["portal"].(string)
		if !exist {
			glog.Fatalf("portal is required for NAS backend %s", name)
		}

		ip := net.ParseIP(portal)
		if ip == nil {
			glog.Fatalf("NFS portal %s is not a valid IP", portal)
		}

		Backends[name].Options["portal"] = portal
	}
}

func ParseSanConfig(path string) {
	ParseConfig(path)

	if Config.Proto != "iscsi" && Config.Proto != "fc" {
		glog.Fatalf("Proto must be 'iscsi' or 'fc'")
	}

	if Config.Proto == "iscsi" {
		for _, backend := range Config.Backends {
			name := backend["name"].(string)

			options, exist := backend["options"].(map[string]interface{})
			if !exist {
				glog.Fatalf("Backend %s doesn't have valid options config", name)
			}

			portals, exist := options["portals"].([]interface{})
			if !exist {
				glog.Fatalf("portals are required for ISCSI backend %s", name)
			}

			IPs, err := proto.VerifyIscsiPortals(portals)
			if err != nil {
				glog.Fatalf("ISCSI portals config of backend %s is not valid: %v", name, err)
			}

			Backends[name].Options["portals"] = IPs
		}
	}
}
