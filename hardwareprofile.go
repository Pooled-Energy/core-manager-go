package main

import (
	"fmt"
	"math/bits"
	"os"
	"runtime"
	"strings"

	"github.com/godbus/dbus/v5"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
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

	zap.S().Info("[?] begin system network hardware profile construction")

	zap.S().Info("[+] modem vendor vame:")
	err = identifyVendorName(&hardwareProfile)
	if err != nil {
		return nil, fmt.Errorf("error getting vendor name, %v", err)
	}

	zap.S().Info("[+] turning off modem echo")
	err = turnOffEcho()
	if err != nil {
		return nil, err
	}

	zap.S().Info("[+] get product name")
	err = identifyProductName(&hardwareProfile)
	if err != nil {
		return nil, err
	}

	zap.S().Info("[+] get vendor id and product id")
	err = identifyUsbVendorAndProductId(&hardwareProfile)
	if err != nil {
		return nil, err
	}

	zap.S().Info("[+] get IEMI")
	err = identifyIEMI(&hardwareProfile)
	if err != nil {
		return nil, err
	}

	zap.S().Info("[+] get firmware version number")
	err = identifyFirmwareVersion(&hardwareProfile)
	if err != nil {
		return nil, err
	}

	zap.S().Info("[+] get ICCID")
	err = identifyIccid(&hardwareProfile)
	if err != nil {
		return nil, err
	}

	zap.S().Info("[+] get OS information")
	err = identifyOS(&hardwareProfile)
	if err != nil {
		return nil, err
	}

	zap.S().Info("[+] get board information")
	err = identifyBoard(&hardwareProfile)
	if err != nil {
		return nil, err
	}

	zap.S().Info("")
	zap.S().Info("=============================================================")
	zap.S().Info("[?] Hardware Profile Report")
	zap.S().Info("---------------------------")
	zap.S().Info("%v+", hardwareProfile)
	zap.S().Info("=============================================================")
	zap.S().Info("")

	if hardwareProfile != oldHardwareProfile {
		zap.S().Info("system setup has changed")
	}

	return &hardwareProfile, nil
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
	usbDevices, err := RunShellCommand("lsusb")
	if err != nil {
		return fmt.Errorf("modem vendor could not be found, error %v", err)
	}

	for _, value := range modems {
		if strings.Contains(usbDevices, value.VID) {
			hardwareProfile.ModemVendor = value.Name
		}
	}

	if hardwareProfile.ModemVendor == "" {
		return fmt.Errorf("modem vendor was not present")
	}

	return nil
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
func identifyProductName(hardwareProfile *Profile) error {
	usbDevices, err := RunShellCommand("lsusb")
	if err != nil {
		return fmt.Errorf("product name could not be found, error %v", err)
	}

	for _, value := range modems {
		for modemName, id := range value.Modules {
			if strings.Contains(usbDevices, id) {
				hardwareProfile.ModemName = strings.Split(modemName, "_")[0]
			}
		}
	}

	deviceNumber, err := RunModemManagerCommand("AT+GMM")
	if err != nil {
		return fmt.Errorf("product name could not be found, error %v", err)
	}

	hardwareProfile.ModemName = fmt.Sprintf("%s %s", hardwareProfile.ModemName, deviceNumber)

	if hardwareProfile.ModemVendor == "" {
		return fmt.Errorf("product name could not be found")
	}

	return nil
}

func identifyUsbVendorAndProductId(hardwareProfile *Profile) error {
	usbDevices, err := RunShellCommand("lsusb")
	if err != nil {
		return fmt.Errorf("vendor or product id could not be found, error %v", err)
	}

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
		return fmt.Errorf("modem vendor id could not be found")
	}

	if hardwareProfile.ModemProductId == "" {
		return fmt.Errorf("modem product id could not be found")
	}

	return nil
}

func identifyIEMI(hardwareProfile *Profile) error {
	iemi, err := RunModemManagerCommand("AT+CGSN")
	if err != nil {
		return fmt.Errorf("iemi could not be found, error %v", err)
	}

	hardwareProfile.IMEI = iemi

	if hardwareProfile.IMEI == "" {
		return fmt.Errorf("iemi could not be found, error %v", err)
	}

	return nil
}

func identifyFirmwareVersion(hardwareProfile *Profile) error {
	softwareVersion, err := RunModemManagerCommand("AT+CGMR")
	if err != nil {
		return fmt.Errorf("software version could not be found, error %v", err)
	}

	hardwareProfile.SoftwareVersion = softwareVersion

	if hardwareProfile.SoftwareVersion == "" {
		return fmt.Errorf("software version could not be found, error %v", err)
	}

	return nil
}

func identifyIccid(hardwareProfile *Profile) error {
	iccid, err := RunModemManagerCommand("AT+ICCID")
	if err != nil {
		return fmt.Errorf("iccid could not be found, error %v", err)
	}

	hardwareProfile.ICCID = iccid

	if hardwareProfile.ICCID == "" {
		return fmt.Errorf("iccid could not be found, error %v", err)
	}

	return nil
}

func identifyOS(hardwareProfile *Profile) error {
	utsname := unix.Utsname{}
	if err := unix.Uname(utsname); err != nil {
		return fmt.Errorf("could not gather OS information, error %v", err)
	}

	hardwareProfile.Architecture = string(bits.UintSize)
	hardwareProfile.Machine = string(utsname.Machine)
	hardwareProfile.Kernel = string(utsname.Release)
	hardwareProfile.Hostname = string(utsname.Nodename)
	hardwareProfile.Platform = runtime.GOOS

	return nil
}

func identifyBoard(hardwareProfile *Profile) error {
	board, err := RunShellCommand("cat /sys/firmware/devicetree/base/model")
	if err != nil {
		return fmt.Errorf("board could not be found, error %v", err)
	}

	hardwareProfile.Board = board

	if board == "" {
		return fmt.Errorf("board could not be found, error %v", err)
	}

	return nil
}

func saveHardwareProfile(hardwareProfile *Profile) error {
	systemConfig, err := yaml.Marshal(&hardwareProfile)
	if err != nil {
		return fmt.Errorf("Error parsing hardware profile, err: %v", err)
	}

	os.WriteFile("system.yaml", systemConfig, 0644)

	return nil
}
