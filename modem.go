package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/gousb"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
)

type DiagnosticProperties struct {
	ConnInterface  bool
	ModemReachable bool
	UsbDriver      bool
	UsbInterface   bool
	ModemDriver    bool
	PDPContext     bool
	// This is how it was spelt in the original, typo?
	NetworkReqister bool
	SimReady        bool
	ModemMode       bool
	ModemApn        bool
	Timestamp       time.Time
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
		err := m.SoftModemReset()
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

func (m *Modem) SoftModemReset() error {
	zap.S().Info("resetting modem softly")
	output, err := RunModemManagerCommand(m.RebootCommand)
	if err != nil {
		return fmt.Errorf("unable to execute modem manager reboot command, error: %v", err)
	}

	if !strings.Contains(output, "OK") {
		return fmt.Errorf("reboot command unable to reach modem, error: %v", err)
	}

	err = checkModemTurnedOff(m)
	if err != nil {
		return err
	}

	err = checkModemStarted(m)
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

func (m *Modem) Diagnose(diagnosisType int) error {
	m.DiagnosticProperties = DiagnosticProperties{
		ConnInterface:   false,
		ModemReachable:  false,
		UsbDriver:       false,
		ModemDriver:     false,
		PDPContext:      false,
		NetworkReqister: false,
		SimReady:        false,
		ModemMode:       false,
		ModemApn:        false,
		UsbInterface:    false,
	}

	zap.S().Info("diagnostic is working...")
	zap.S().Info("[1] - does the connection interface exist?")
	routeData, err := RunShellCommand("route -n")
	if err != nil {
		return fmt.Errorf("error checking route information, error: %v", err)
	}
	m.DiagnosticProperties.ConnInterface = strings.Contains(routeData, m.InterfaceName)

	zap.S().Info("[2] - does the USB interface exist?")
	usbInterface, err := RunShellCommand("lsusb")
	if err != nil {
		return fmt.Errorf("error checking usb interface information, error: %v", err)
	}
	m.DiagnosticProperties.UsbInterface = strings.Contains(usbInterface, m.Vendor)

	zap.S().Info("[3] - does the USB driver exist?")
	usbDevices, err := RunShellCommand("usb-devices")
	if err != nil {
		return fmt.Errorf("error checking usb driver information, error: %v", err)
	}
	m.DiagnosticProperties.UsbDriver = strings.Count(usbDevices, "cdc_ether") >= 2

	zap.S().Info("[4] - is modem reachable?")
	response, err := RunModemManagerCommand("AT")
	if err != nil {
		return fmt.Errorf("error checking usb driver information, error: %v", err)
	}
	if strings.Contains(response, "OK") {
		m.DiagnosticProperties.ModemReachable = true
	}

	zap.S().Info("[5] - is ECM PDP context active?")
	response, err = RunModemManagerCommand(m.PDPStatusCommand)
	if err != nil {
		return fmt.Errorf("error checking ECM PDP context information, error: %v", err)
	}
	if strings.Contains(response, "1,1") {
		m.DiagnosticProperties.PDPContext = true
	}

	zap.S().Info("[6] - is the network registered?")
	err = m.CheckNetwork()
	m.DiagnosticProperties.NetworkReqister = (err == nil)

	zap.S().Info("[7] - is the APN ok?")
	expectedApn := fmt.Sprintf("\"%s\"", Config.APN)
	apn, err := RunModemManagerCommand("AT+CGDCONT?")
	if err != nil {
		return fmt.Errorf("unable to get apn from modem, err: %v", err)
	}
	m.DiagnosticProperties.ModemApn = strings.Contains(apn, expectedApn)

	zap.S().Info("[8] - is the modem mode ok?")
	mode, err := RunModemManagerCommand(m.ModeStatusCommand)
	if err != nil {
		return fmt.Errorf("unable to get modem mode from modem, err: %v", err)
	}
	m.DiagnosticProperties.ModemMode = strings.Contains(mode, m.EcmModeResponse)

	zap.S().Info("[8] - is the SIM ready?")
	simStatus, err := RunModemManagerCommand("AT+CPIN?")
	if err != nil {
		return fmt.Errorf("unable to get modem mode from modem, err: %v", err)
	}
	m.DiagnosticProperties.SimReady = strings.Contains(simStatus, "READY")

	m.DiagnosticProperties.Timestamp = time.Now()

	switch diagnosisType {
	case 0:
		zap.S().Info("creating diagnostic report called cm-diag_%s.yaml", m.DiagnosticProperties.Timestamp)
		out, err := yaml.Marshal(m.DiagnosticProperties)
		if err != nil {
			return fmt.Errorf("error occured when saving diagnosis, error: %v", err)
		}
		os.WriteFile(fmt.Sprintf("cm-diag_%s.yaml", m.DiagnosticProperties.Timestamp), out, 0666)
	case 1:
		zap.S().Info("creating diagnostic report called cm-diag_repeated.yaml", m.DiagnosticProperties.Timestamp)
		out, err := yaml.Marshal(m.DiagnosticProperties)
		if err != nil {
			return fmt.Errorf("error occured when saving diagnosis, error: %v", err)
		}
		os.WriteFile("cm-diag_repeated.yaml", out, 0666)
	}

	if config.DebugMode && config.VerboseMode {
		zap.S().Info("")
		zap.S().Info("=============================================================")
		zap.S().Info("[?] Diagnostic Report")
		zap.S().Info("---------------------------")
		zap.S().Info("%v+", m.DiagnosticProperties)
		zap.S().Info("=============================================================")
		zap.S().Info("")
	}

	return nil
}

func (m *Modem) ResetConnectionInterface() error {
	down := fmt.Sprintf("sudo ip link set dev %s down", m.InterfaceName)
	up := fmt.Sprintf("sudo ip link set dev %s up", m.InterfaceName)

	zap.S().Info("resetting connection interface...")
	_, err := RunShellCommand(down)
	if err != nil {
		return fmt.Errorf("error bringing interface down, error: %v", err)
	}
	zap.S().Info("interface %s is down", m.InterfaceName)

	time.Sleep(5 * time.Second)

	_, err = RunShellCommand(up)
	if err != nil {
		return fmt.Errorf("error bringing interface up, error: %v", err)
	}
	zap.S().Info("interface %s is up", m.InterfaceName)

	err = checkifModemInterfaceIsUp(m)
	if err != nil {
		return err
	}

	return nil
}

func checkifModemInterfaceIsUp(modem *Modem) error {
	counter := 0
	zap.S().Debug("interface name: %s", modem.InterfaceName)
	for i := 0; i < 20; i++ {
		output, err := RunShellCommand("route -n")
		if err != nil {
			zap.S().Error("error trying to get modem interface data, error: %v", err)
		}
		if strings.Contains(output, modem.InterfaceName) {
			zap.S().Info("modem interface detected")
			counter = 0
			break
		} else {
			time.Sleep(1 * time.Second)
			counter += 1
			fmt.Println(string(counter) + " attempts to get ensure modem is up")
		}
	}

	if counter != 0 {
		return fmt.Errorf("modem interface couldn't be detected")
	}

	return nil
}

func (m *Modem) ResetUsbInterface() error {
	zap.S().Info("resetting usb interface...")
	usbContext := gousb.NewContext()
	defer usbContext.Close()

	vendorId, err := strconv.Atoi(m.VendorId)
	if err != nil {
		return fmt.Errorf("issue converting vendor id, error %v", err)
	}

	productId, err := strconv.Atoi(m.ProductId)
	if err != nil {
		return fmt.Errorf("issue converting product id, error %v", err)
	}

	device, err := usbContext.OpenDeviceWithVIDPID(gousb.ID(vendorId), gousb.ID(productId))
	if err != nil {
		return fmt.Errorf("issue accessing usb device, error: %v", err)
	}
	defer device.Close()

	err = device.Reset()
	if err != nil {
		return fmt.Errorf("error resetting usb interface, error: %v", err)
	}

	zap.S().Info("usb interface reset")

	return nil
}

// TODO: revisit this. I think we can refactor it since the idea seems to be that
// we try and activate pins without throwing errors, so just recording it. That's how
// it was originally designed.
func (m *Modem) HardModemReset() error {
	zap.S().Info("physically rebooting the hardware...")
	sbc := supportedSBCs[config.SBC]
	sbc.ModemPowerDisable()
	time.Sleep(2 * time.Second)
	sbc.ModemPowerEnable()

	zap.S().Info("hard reset complete")
	return nil
}
