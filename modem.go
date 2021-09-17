package main

import (
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
)

type DiagnosticProperties struct {
	ConnInterface  bool
	ModemReachable bool
	UsbDriver      bool
	ModemDriver    bool
	PDPContext     bool
	// This is how it was spelt in the original, typo?
	NetworkReqister bool
	SimReady        bool
	ModemMode       bool
	ModemApn        bool
}

func (d *DiagnosticProperties) SetDefaults() {
	d.ConnInterface = true
	d.ModemReachable = true
	d.UsbDriver = true
	d.ModemDriver = true
	d.PDPContext = true
	d.NetworkReqister = true
	d.SimReady = true
	d.ModemMode = true
	d.ModemApn = true
}

type MonitoringProperties struct {
	CellularConnection string
	CellularLatency    string
	FixedIncident      int
}

type Modem struct {
	Vendor               string
	VendorId             string
	Model                string
	ProductId            string
	IMEI                 string
	ICCID                string
	SoftwareVersion      string
	MonitoringProperties MonitoringProperties
	InterfaceName        string
	ModeStatusCommand    string
	EcmModeResponse      string
	EcmModeSetterCommand string
	RebootCommand        string
	PDPActivateCommand   string
	PDPStatusCommand     string
	IncidentFlag         bool
	DiagnosticProperties DiagnosticProperties
}

func (m *Modem) Initialize() {
	diagnosticProperties := DiagnosticProperties{}
	diagnosticProperties.SetDefaults()
	m.DiagnosticProperties = diagnosticProperties
}

func (m *Modem) Update(vendor, model, imei, iccid, softwareVersion, vendorId, productId string) {
	m.Vendor = vendor
	m.Model = model
	m.IMEI = imei
	m.ICCID = iccid
	m.SoftwareVersion = softwareVersion
	m.VendorId = vendorId
	m.ProductId = productId

	updateModemCommands(vendor, model, m)

}

func updateModemCommands(vendor, model string, modem *Modem) {
	// Might not need interface name here since we are using Modem Manager
	if vendor == "Quectel" {
		modem.InterfaceName = "usb0"
		modem.ModeStatusCommand = "AT+QCFG=\"usbnet\""
		modem.RebootCommand = "AT+CFUN=1,1"
		modem.PDPActivateCommand = "AT"
		modem.PDPStatusCommand = "AT+CGACT?"
		modem.EcmModeSetterCommand = "AT+QCFG=\"usbnet\",1"
		modem.EcmModeResponse = "\"usbnet\",1"
	} else if vendor == "Telit" {
		modem.InterfaceName = "wwan0"
		modem.ModeStatusCommand = "AT#USBCFG?"
		modem.RebootCommand = "AT#REBOOT"
		modem.PDPActivateCommand = "AT#ECM=1,0"
		modem.PDPStatusCommand = "AT#ECM?"

		if model == "ME910C1-WW" {
			modem.EcmModeSetterCommand = "AT#USBCFG=3"
			modem.EcmModeResponse = "3"
		} else {
			modem.EcmModeSetterCommand = "AT#USBCFG=4"
			modem.EcmModeResponse = "4"
		}
	}
}

func (m *Modem) DetectModem() (string, error) {
	output, err := RunShellCommand("lsusb")
	if err != nil {
		zap.S().Error("There was an issue detecting modem: %v", err)
	}

	if strings.Contains(output, m.Vendor) {
		return m.Vendor, nil
	}

	return "", fmt.Errorf("no modem detected")
}

func (m *Modem) ConfigureApn() error {
	expectedApn := fmt.Sprintf("\"%s\"", Config.APN)
	apn, err := RunModemManagerCommand("AT+CGDCONT?")
	if err != nil {
		return fmt.Errorf("unable to get apn from modem, err: %v", err)
	}

	if strings.Contains(apn, expectedApn) {
		zap.S().Info("apn is up-to-date")
	} else {
		output, err := RunModemManagerCommand("AT+CGDCONT=1,\"IPV4V6\",\"" + config.APN + "\"")
		if err != nil {
			return fmt.Errorf("unable to update apn on modem, err: %v", err)
		}
		zap.S().Info("apn updated with %s", output)
	}

	return nil
}

func (m *Modem) ConfigureModem() error {
	forceReset := 0
	zap.S().Info("modem configuration started")

	err := m.ConfigureApn()
	if err != nil {
		return err
	}

	zap.S().Info("checking modem mode...")
	ecmMode, err := RunModemManagerCommand(m.ModeStatusCommand)
	if err != nil {
		return fmt.Errorf("unable to get modem mode, error: %v", err)
	}

	if strings.Contains(ecmMode, m.EcmModeResponse) {
		zap.S().Info("ecm mode already set, skipping...")
		return nil
	}

	zap.S().Info("modem mode not set. ECM mode will be activated")
	_, err = RunModemManagerCommand(m.EcmModeSetterCommand)
	if err != nil {
		return fmt.Errorf("an issue occured when setting ECM mode")
	}

	zap.S().Info("ECM mode set, modem will reboot to apply changes")
	time.Sleep(20 * time.Second)
	err = checkModemStarted(m)
	if err != nil {
		zap.S().Error("issue restarting modem, error: %v", err)
		forceReset = 1
	}

	if forceReset == 1 {
		forceReset = 0
		err := softModemReset(m)
		if err != nil {
			return fmt.Errorf("an issue occured when configuring the modem, error: %v", err)
		}
	}

	return nil
}

func checkModemStarted(modem *Modem) error {
	result := 0
	counter := 0

	for i := 0; i < 120; i++ {
		output, err := RunShellCommand("lsusb")
		if err != nil {
			zap.S().Error("error trying to get modem information, error: %v", err)
		}
		if strings.Contains(output, modem.Vendor) {
			zap.S().Info("modem detected")
			counter = 0
			result += 1
			break
		} else {
			time.Sleep(1 * time.Second)
			counter += 1
			fmt.Println(string(counter) + " attempts to get modem name")
		}
	}

	for i := 0; i < 10; i++ {
		output, err := RunModemManagerCommand("AT")
		if err != nil {
			zap.S().Error("error trying to get modem information, error: %v", err)
		}
		if strings.Contains(output, "OK") {
			zap.S().Info("modem AT FW is working")
			counter = 0
			result += 1
			break
		} else {
			time.Sleep(1 * time.Second)
			counter += 1
			fmt.Println(string(counter) + " attempts to get contact modem")
		}
	}

	for i := 0; i < 20; i++ {
		output, err := RunShellCommand("route -n")
		if err != nil {
			zap.S().Error("error trying to get modem information, error: %v", err)
		}
		if strings.Contains(output, modem.InterfaceName) {
			zap.S().Info("modem started")
			counter = 0
			result += 1
			break
		} else {
			time.Sleep(1 * time.Second)
			counter += 1
			fmt.Println(string(counter) + " attempts to get ensure modem is up")
		}
	}

	if result != 3 {
		return fmt.Errorf("modem could not be started")
	}

	return nil

}

func checkModemTurnedOff(modem *Modem) error {
	counter := 0
	for i := 0; i < 20; i++ {
		output, err := RunShellCommand("lsusb")
		if err != nil {
			zap.S().Error("error trying to get modem information, error: %v", err)
		}

		if strings.Contains(output, modem.Vendor) {
			time.Sleep(1 * time.Second)
			counter++
			fmt.Println(string(counter) + " attempts to check modem is off")
		} else {
			zap.S().Info("modem turned off")
			counter = 0
			return nil
		}
	}

	return fmt.Errorf("modem didn't turn off as expected")
}

func softModemReset(modem *Modem) error {
	zap.S().Info("resetting modem softly")
	output, err := RunModemManagerCommand(modem.RebootCommand)
	if err != nil {
		return fmt.Errorf("unable to execute modem manager reboot command, error: %v", err)
	}

	if !strings.Contains(output, "OK") {
		return fmt.Errorf("reboot command unable to reach modem, error: %v", err)
	}

	err = checkModemTurnedOff(modem)
	if err != nil {
		return err
	}

	err = checkModemStarted(modem)
	if err != nil {
		return err
	}

	return nil
}
