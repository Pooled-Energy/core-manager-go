package main

import (
	"fmt"
	"strconv"
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
	CellularConnection bool
	CellularLatency    int
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
	output, err := RunModemManagerCommand(m.EcmModeSetterCommand)
	if err != nil {
		return fmt.Errorf("an issue occured when setting ECM mode")
	}

	if !strings.Contains(output, "OK") {
		return fmt.Errorf("error occured while setting mode configuration, output: %s", output)
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

func (m *Modem) CheckSimReady() error {
	zap.S().Info("checking the SIM is ready...")
	output, err := RunModemManagerCommand("AT+CPIN?")
	if err != nil {
		return fmt.Errorf("an error occured when checking SIM status, error: %v", err)
	}

	if !strings.Contains(output, "CPIN: READY") {
		return fmt.Errorf("SIM not ready")
	}

	zap.S().Info("SIM is ready!")
	return nil
}

func (m *Modem) CheckNetwork() error {
	zap.S().Info("checking the network is ready...")

	output, err := RunModemManagerCommand("AT+CREG?")
	if err != nil {
		return fmt.Errorf("an error occured when checking network status, error: %v", err)
	}

	if !strings.Contains(output, "OK") {
		return fmt.Errorf("modem error, output %s", output)
	}

	if !strings.Contains(output, "+CREG: 0,1") || !strings.Contains(output, "+CREG: 0,5") {
		return fmt.Errorf("network registration failed, output %s", output)
	}

	zap.S().Info("network is registered")

	return nil
}

func (m *Modem) InitiateECM() error {
	zap.S().Info("checking the ECM initialization...")
	output, err := RunModemManagerCommand(m.PDPStatusCommand)
	if err != nil {
		return fmt.Errorf("an error occured when checking ecm status, error: %v", err)
	}

	if !strings.Contains(output, "OK") {
		return fmt.Errorf("error occured when checking pdp status, output: %s", output)
	}

	if strings.Contains(output, "0,1") || strings.Contains(output, "1,1") {
		zap.S().Info("ECM is already initiated")
		time.Sleep(10 * time.Second)
		return nil
	}

	zap.S().Info("ECM connection is initiating...")
	output, err = RunModemManagerCommand(m.PDPActivateCommand)
	if err != nil {
		return fmt.Errorf("an error occured when initiating ecm connection, error: %v", err)
	}

	if !strings.Contains(output, "OK") {
		return fmt.Errorf("ecm initiation failed, output: %s", output)
	}

	for i := 0; i < 60; i++ {
		output, err := RunModemManagerCommand(m.PDPStatusCommand)
		if err != nil {
			return fmt.Errorf("an error occured when checking ecm status, error: %v", err)
		}

		if !strings.Contains(output, "OK") {
			time.Sleep(1 * time.Second)
			continue
		}

		if strings.Contains(output, "0,1") || strings.Contains(output, "1,1") {
			zap.S().Info("ECM is already initiated")
			time.Sleep(10 * time.Second)
			return nil
		} else {
			time.Sleep(1 * time.Second)
		}
	}

	return fmt.Errorf("ECM initiation timeout")
}

func (m *Modem) CheckInternet() error {
	latency, err := checkInterfaceHealth(m.InterfaceName, config.PingTimeout)
	if err != nil {
		m.MonitoringProperties.CellularConnection = false
		m.MonitoringProperties.CellularLatency = 0
		return fmt.Errorf("error checking internet connection, error: %v", err)
	}

	m.MonitoringProperties.CellularConnection = true
	m.MonitoringProperties.CellularLatency = latency
	return nil

}

func checkInterfaceHealth(interfaceName string, pingTimeout int) (int, error) {
	pingResult, err := RunShellCommand(fmt.Sprintf("ping -1 -c 1 -s 8 -w %s -I %s 8.8.8.8", string(pingTimeout), interfaceName))
	if err != nil {
		return 0, fmt.Errorf("no internet, error: %v", err)
	}

	pingLatencies := parsePingOutput(pingResult, "min/avg/max/mdev =", "ms")
	latenciesValues, err := strconv.ParseFloat(strings.Split(pingLatencies, "/")[0], 32)
	if err != nil {
		return 0, fmt.Errorf("issue converting ping data to numeric, error: %v", err)
	}
	return int(latenciesValues), nil
}

func parsePingOutput(output, header, end string) string {
	header += " "
	headerSize := len(header)
	indexOfData := strings.Index(output, header) + headerSize
	endOfData := indexOfData + strings.Index(output[indexOfData:], end)
	return output[indexOfData:endOfData]
}
