package main

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"

	"github.com/imdario/mergo"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
)

type Configuration struct {
	VerboseMode                bool
	DebugMode                  bool
	APN                        string
	SBC                        string
	CheckInternetInterval      int
	SendMonitoringDataInterval int
	PingTimeout                int
	OtherPingTimeout           int
	NetworkPriority            map[string]int
	CellularInterfaces         []string
	AcceptableAPNs             map[string]struct{}
	LoggerLevel                string
	ReloadRequired             bool
	ConfigChanged              bool
	ModemConfigRequired        bool
	LogConfigRequired          bool
}

func (c *Configuration) SetDefaults() {
	c.VerboseMode = false
	c.DebugMode = false
	c.APN = "super"
	c.SBC = "rpi4"
	c.CheckInternetInterval = 60
	c.SendMonitoringDataInterval = 25
	c.PingTimeout = 9
	c.OtherPingTimeout = 3
	c.NetworkPriority = map[string]int{"eth0": 1, "wlan0": 2, "wwan0": 3, "usb0": 4}
	c.CellularInterfaces = []string{"wwan0", "usb0"}
	c.AcceptableAPNs = map[string]struct{}{"super": {}, "de1.super": {}, "sg1.super": {}}
	c.LoggerLevel = "debug" // Is this needed?
	c.ReloadRequired = false
	c.ConfigChanged = false
	c.ModemConfigRequired = false
	c.LogConfigRequired = false
}

func (c *Configuration) UpdateConfig(newConfig *Configuration) {
	c.VerboseMode = newConfig.VerboseMode
	c.DebugMode = newConfig.DebugMode
	c.APN = newConfig.APN
	c.SBC = newConfig.SBC
	c.CheckInternetInterval = newConfig.CheckInternetInterval
	c.SendMonitoringDataInterval = newConfig.SendMonitoringDataInterval
	c.PingTimeout = newConfig.PingTimeout
	c.OtherPingTimeout = newConfig.OtherPingTimeout
	c.NetworkPriority = newConfig.NetworkPriority
	c.CellularInterfaces = newConfig.CellularInterfaces
	c.AcceptableAPNs = newConfig.AcceptableAPNs
	c.LoggerLevel = newConfig.LoggerLevel // Is this needed?
	c.ReloadRequired = newConfig.ReloadRequired
	c.ConfigChanged = newConfig.ConfigChanged
	c.ModemConfigRequired = newConfig.ModemConfigRequired
	c.LogConfigRequired = newConfig.LogConfigRequired
}

var Config = Configuration{}
var oldConfig = Configuration{}

func LoadConfiguration() *Configuration {
	conf := Configuration{}
	if _, err := os.Stat("config.yaml"); err != nil {
		zap.S().Error("config file doesn't exist, using defaults")
		conf.SetDefaults()
		return &conf
	}

	configFileContent, err := os.ReadFile("config.yaml")
	if err != nil {
		zap.S().Error("an issue occured when reading config.yaml, returning defaults. error: %v", err)
		conf.SetDefaults()
		return &conf
	}

	if reflect.DeepEqual(conf, oldConfig) {
		return &conf
	}

	yaml.Unmarshal(configFileContent, &conf)
	conf.ConfigChanged = true
	oldConfig.UpdateConfig(&conf)
	Config.UpdateConfig(&conf)
}

func getRequests() []string {
	paths, err := filepath.Glob("/config_request*.yaml")
	if err != nil {
		zap.S().Error("there was an issue loading config requests, error: %v", err)
		return []string{}
	}

	sort.Strings(paths)

	return paths
}

// The idea here is we load all the config files and then we apply them directly to the configuration,
// saving it if things have changed.
func Configure() {
	// First we load everything up, so we can have something to compare against
	LoadConfiguration()

	// Load all requested changes
	configureRequests := getRequests()

	for _, configurationRequest := range configureRequests {
		update := Configuration{}
		configFileContent, err := os.ReadFile(configurationRequest)
		// We might want to remove this config, though I'm not sure how to go about this at the moment
		if err != nil {
			zap.S().Error("an issue occured when reading %s, returning defaults. error: %v", configurationRequest, err)
			continue
		}

		yaml.Unmarshal(configFileContent, &update)

		if Config.APN != update.APN {
			Config.ModemConfigRequired = true
		}

		// TODO: Actually implement the ability to change logging style with config files

		if err := mergo.Merge(&Config, configFileContent, mergo.WithOverride); err != nil {
			zap.S().Error("an issue occured when updating config with %s. error: %v", configurationRequest, err)
		}
	}

	if !reflect.DeepEqual(Config, oldConfig) {
		systemConfig, err := yaml.Marshal(&Config)
		if err != nil {
			zap.S().Error("Error parsing configuration, err: %v", err)
		}

		os.WriteFile("config.yaml", systemConfig, 0644)
	}

	if Config.ModemConfigRequired {
		conductor.SetStep(0, 2, 14, 13, 1, false, 5)
		Config.ModemConfigRequired = false
	}

	Config.ConfigChanged = false

}
