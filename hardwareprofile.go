package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/godbus/dbus/v5"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
)

type VendorModem struct {
	Name    string
	VID     string
	Modules map[string]string
}

type Profile struct {
	ModemVendor     string
	ModemName       string
	ModemVendorId   string
	ModemProductId  string
	IMEI            string
	SoftwareVersion string
	ICCID           string
	Architecture    string
	Machine         string
	Kernel          string
	Hostname        string
	Platform        string
	Board           string
}

var modems = [...]VendorModem{
	{
		Name: "Quectel",
		VID:  "2c7c",
		Modules: map[string]string{
			"EX25-Series": "0125",
			"EC21":        "0121",
		},
	},
	{
		Name: "Telit",
		VID:  "1bc7",
		Modules: map[string]string{
			"LE910CX-Series_COMP_1": "1201",
			"LE910CX-Series_COMP_2": "1206",
			"ME910C1-WW_COMP_1":     "1101",
			"ME910C1-WW_COMP_2":     "1102",
		},
	},
}

//zap.S().Error("No system.yaml file found")
//zap.S().Error("There was an error reading the existing profile yaml, error: %v", err)
func GetHardwareProfile() (*Profile, error) {
	// First we want to load the old hardware profile if we have it saved
	hardwareProfile := Profile{}
	var oldHardwareProfile, err = loadHardwareProfile()
	if err != nil {
		zap.S().Error("unable to load system.yaml, error: %q", err)
	}

	zap.S().Info("begin system network hardware profile construction")

	zap.S().Info("modem Vendor Name:")
	err = identifyVendorName(&hardwareProfile)
	if err != nil {
		return nil, fmt.Errorf("error getting vendor name, %v", err)
	}

	zap.S().Info("Ttrning off modem echo")
	err = turnOffEcho()
	if err != nil {
		return nil, err
	}

	zap.S().Info("Get product name")
	identifyProductName(&hardwareProfile)

	zap.S().Info("Get Vendor ID and Product ID")
	identifyUsbVIDAndPID(&hardwareProfile)

	zap.S().Info("Get IEMI")
	identifyIEMI(&hardwareProfile)

	zap.S().Info("Get Firmware Version Number")
	identifyFirmwareVersion(&hardwareProfile)

	zap.S().Info("Get ICCID")
	identifyIccid(&hardwareProfile)

	zap.S().Info("Get OS information")
	identifyOS(&hardwareProfile)

	zap.S().Info("Get board information")
	identifyBoard(&hardwareProfile)

	zap.S().Info("=============================================================")
	zap.S().Info("Hardware Profile Report")
	zap.S().Info("%v+", hardwareProfile)
	zap.S().Info("=============================================================")
	zap.S().Info("")

	if hardwareProfile != oldHardwareProfile {
		zap.S().Info("system setup has changed")
	}

	return &hardwareProfile
}

func loadHardwareProfile() (Profile, error) {
	var hardwareProfile Profile
	if _, err := os.Stat("system.yaml"); err == nil {
		data, err := os.ReadFile("system.yaml")
		if err != nil {
			return Profile{}, err
		}
		err = yaml.Unmarshal(data, &hardwareProfile)
		if err != nil {
			return Profile{}, err
		}
	}

	return hardwareProfile, nil
}

func identifyVendorName(hardwareProfile *Profile) error {
	usbDevcies, err := RunShellCommand("lsusb")
	if err != nil {
		return fmt.Errorf("modem vendor could not be found, error %v", err)
	}

	for _, value := range modems {
		if strings.Contains(usbDevcies, value.VID) {
			hardwareProfile.ModemVendor = value.Name
		}
	}

	if hardwareProfile.ModemVendor == "" {
		return fmt.Errorf("modem vendor was not present")
	}
}

func turnOffEcho() error {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	var result string
	dbusObject := conn.Object("org.freedesktop.ModemManager1", "/org/freedesktop/ModemManager1/Modem/0")
	err = dbusObject.Call("org.freedesktop.ModemManager1.Modem.Command", 0, "ATE0", uint32(30)).Store(result)
	if err != nil {
		return fmt.Errorf("issue contacting the modem via modem manager, error: %v", err)
	}

	return nil
}

// TODO: We can combine anything which involves a lsusb call into one
func identifyProductName(hardwareProfile *Profile) {
	usbDevcies := runShellCommand("lsusb")

	for _, value := range modems {
		for modemName, id := range value.Modules {
			if strings.Contains(usbDevcies, id) {
				hardwareProfile.ModemName = strings.Split(modemName, "_")[0]
			}
		}
	}

	if hardwareProfile.ModemVendor == "" {
		zap.S().Warn("Modem name could not be found")
	}
}

func identifyUsbVIDAndPID(hardwareProfile *Profile) {
	usbDevices := runShellCommand("lsusb")

	for _, value := range modems {
		if strings.Contains(usbDevices, value.VID) {
			hardwareProfile.ModemVendorId = value.VID
		}
	}

	for _, value := range modems {
		for key, val := range value.Modules {
			if strings.Contains(usbDevices, key) {
				hardwareProfile.ModemProductId = val
			}
		}
	}

	if hardwareProfile.ModemVendorId == "" {
		zap.S().Warn("ModemVendorId could not be found")
	}

	if hardwareProfile.ModemProductId == "" {
		zap.S().Warn("ModemProductId could not be found")
	}
}

func identifyIEMI(hardwareProfile *Profile) {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	var result string
	dbusObject := conn.Object("org.freedesktop.ModemManager1", "/org/freedesktop/ModemManager1/Modem/0")
	dBusMethodCallResult := dbusObject.Call("org.freedesktop.ModemManager1.Modem.Command", 0, "AT+CGSN", uint32(30)).Store(result)
	if dBusMethodCallResult != nil {
		zap.S().Warn("Unable to get IEMI from modem, response: %s", result)
	}

	hardwareProfile.IMEI = result
}

func identifyFirmwareVersion(hardwareProfile *Profile) {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	var result string
	dbusObject := conn.Object("org.freedesktop.ModemManager1", "/org/freedesktop/ModemManager1/Modem/0")
	dBusMethodCallResult := dbusObject.Call("org.freedesktop.ModemManager1.Modem.Command", 0, "AT+CGMR", uint32(30)).Store(result)
	if dBusMethodCallResult != nil {
		zap.S().Warn("Unable to get firmware version from modem, response: %s", result)
	}

	hardwareProfile.SoftwareVersion = result
}

func identifyIccid(hardwareProfile *Profile) {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	var result string
	dbusObject := conn.Object("org.freedesktop.ModemManager1", "/org/freedesktop/ModemManager1/Modem/0")
	dBusMethodCallResult := dbusObject.Call("org.freedesktop.ModemManager1.Modem.Command", 0, "AT+ICCID", uint32(30)).Store(result)
	if dBusMethodCallResult != nil {
		zap.S().Warn("Unable to get ICCID from modem, response: %s", result)
	}

	hardwareProfile.ICCID = result
}

func identifyOS(hardwareProfile *Profile) {
	// TODO

	hardwareProfile.Architecture = runtime.GOARCH

}

func identifyBoard(hardwareProfile *Profile) {
	board := runShellCommand("cat /sys/firmware/devicetree/base/model")
	hardwareProfile.Board = board

	if board == "" {
		zap.S().Warn("ModemProductId could not be found")
	}
}

func saveHardwareProfile(hardwareProfile *Profile) {
	systemConfig, err := yaml.Marshal(&hardwareProfile)
	if err != nil {
		zap.S().Error("Error parsing hardware profile, err: %v", err)
	}

	os.WriteFile("system.yaml", systemConfig, 0644)
}
