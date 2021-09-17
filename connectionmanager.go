package main

import "go.uber.org/zap"

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
		zap.S().Error("error checking sim status, error: %v", err)
		return
	}

	conductor.IsOk = true
}

var actions = [...]func(){organizer, identifySetup, configureModem}

func ExecuteStep(step int) {
	actions[step]()
}

func ManageConnection() int {
	ExecuteStep(conductor.Sub)
	return conductor.Interval
}
