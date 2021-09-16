package main

import (
	"fmt"
	"strings"

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

func (m *Modem) ConfigureApn() {

}
