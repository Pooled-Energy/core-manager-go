package main

import (
	"go.uber.org/zap"
)

func organizer() {
	if conductor.Base == 0 {
		conductor.Sub = 1
	} else {
		if conductor.IsOk {
			conductor.Sub = conductor.Success
			conductor.IsOk = false
		} else {
			if conductor.Counter >= conductor.Retry {
				conductor.Sub = conductor.Fail
				conductor.ClearCounter()
			} else {
				conductor.Sub = conductor.Base
				conductor.CounterTick()
			}
		}
	}
}

func identifySetup() {
	conductor.SetStep(0, 1, 2, 15, 2, false, 20)

	newId, err := GetHardwareProfile()
	if err != nil {
		zap.S().Error("issue occured when identifying setup, error: %v", err)
	}

	if newId != nil {
		networkModem.Update(newId.ModemVendor, newId.ModemName, newId.IMEI,
			newId.ICCID, newId.SoftwareVersion, newId.ModemVendorId, newId.ModemProductId)

		conductor.IsOk = true

		if config.DebugMode && config.VerboseMode {
			zap.S().Info("")
			zap.S().Info("=============================================================")
			zap.S().Info("[?] Modem Report")
			zap.S().Info("---------------------------")
			zap.S().Info("%v+", networkModem)
			zap.S().Info("=============================================================")
			zap.S().Info("")
		}
	}
}

func configureModem() {
	conductor.SetStep(0, 2, 14, 13, 1, false, 5)

	err := networkModem.ConfigureModem()
	if err != nil {
		conductor.IsOk = false
		zap.S().Error("error configuring modem, error: %v", err)
		return
	}

	conductor.IsOk = true

}

func checkSimReady() {
	conductor.SetStep(0, 14, 3, 13, 1, false, 5)

	err := networkModem.CheckSimReady()
	if err != nil {
		conductor.IsOk = false
		zap.S().Error("error checking SIM status, error: %v", err)
		return
	}

	conductor.IsOk = true
}

func checkNetwork() {
	conductor.SetStep(0, 3, 4, 13, 5, false, 120)

	err := networkModem.CheckNetwork()
	if err != nil {
		conductor.IsOk = false
		zap.S().Error("error checking network status, error: %v", err)
		return
	}

	conductor.IsOk = true
}

func initiateECM() {
	conductor.SetStep(0, 4, 5, 13, 0.1, false, 5)

	err := networkModem.InitiateECM()
	if err != nil {
		conductor.IsOk = false
		zap.S().Error("error initiating ecm, error: %v", err)
		return
	}

	conductor.IsOk = true
}

func checkInternet() {
	switch conductor.Sub {
	case 5:
		conductor.SetStep(0, 5, 5, 6, float32(config.CheckInternetInterval), false, 1)
	case 8:
		conductor.SetStep(0, 8, 5, 9, 10, false, 0)
	case 10:
		conductor.SetStep(0, 10, 5, 11, 10, false, 0)
	}

	err := networkModem.CheckInternet()
	if err != nil {
		zap.S().Error("error occured when checking internet, error: %v", err)
		conductor.IsOk = false
	}

	if networkModem.IncidentFlag == true {
		networkModem.MonitoringProperties.FixedIncident++
		networkModem.IncidentFlag = false
	}

	conductor.IsOk = true
}

func diagnose() {
	networkModem.MonitoringProperties.CellularConnection = false
	networkModem.IncidentFlag = true
	diagnosisType := 0

	switch conductor.Sub {
	case 6:
		conductor.SetStep(0, 6, 7, 7, 0.1, false, 5)
		diagnosisType = 0
	case 8:
		conductor.SetStep(0, 13, 7, 7, 0.1, false, 5)
		diagnosisType = 1
	case 10:
		conductor.SetStep(0, 15, 12, 12, 0.1, false, 5)
		diagnosisType = 1
	}

	err := networkModem.Diagnose(diagnosisType)
	if err != nil {
		zap.S().Error("error occured during diagnosis, error: %v", err)
		conductor.IsOk = false
	}

	conductor.IsOk = true
}

func resetConnectionInterface() {
	conductor.SetStep(0, 7, 8, 9, 1, false, 2)

	err := networkModem.ResetConnectionInterface()
	if err != nil {
		zap.S().Error("error occured during connection interface reset, error: %v", err)
		conductor.IsOk = false
	}

	conductor.IsOk = true
}

func resetUsbInterface() {
	conductor.SetStep(0, 9, 10, 11, 1, false, 2)

	err := networkModem.ResetUsbInterface()
	if err != nil {
		zap.S().Error("error occured during usb device reset, error: %v", err)
		conductor.IsOk = false
	}

	conductor.IsOk = true
}

func resetModemSoftly() {
	conductor.SetStep(0, 11, 1, 12, 1, false, 1)

	err := networkModem.SoftModemReset()
	if err != nil {
		zap.S().Error("an issue occured when soft rebooting the modem, error: %v", err)
		conductor.IsOk = false
	}

	conductor.IsOk = true
}

func resetModemHardly() {
	conductor.SetStep(0, 12, 1, 1, 1, false, 1)

	err := networkModem.HardModemReset()
	if err != nil {
		zap.S().Error("an issue occured when soft rebooting the modem, error: %v", err)
		conductor.IsOk = false
	}

	conductor.IsOk = true
}

var actions = [...]func(){
	organizer,
	identifySetup,
	configureModem,
	checkNetwork,
	initiateECM,
	checkInternet,
	diagnose,
	resetConnectionInterface,
	checkInternet,
	resetUsbInterface,
	checkInternet,
	resetModemSoftly,
	resetModemHardly,
	diagnose,
	checkSimReady,
	diagnose,
}

func ExecuteStep(step int) {
	actions[step]()
}

func ManageConnection() float32 {
	ExecuteStep(conductor.Sub)
	return conductor.Interval
}
