package main

import (
	"os"
	"reflect"

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

func LoadConfiguration() {
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
