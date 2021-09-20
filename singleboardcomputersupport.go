package main

import (
	"fmt"
	"time"

	"go.uber.org/zap"
)

// SBC = Single Board Computer
type SBC struct {
	Name       string
	OS         string
	DisablePin int
}

func (sbc *SBC) GPIOPinInit(pin int) {
	pinName := "gpio" + string(pin)

	_, err := RunShellCommand(fmt.Sprintf("ls /sys/class/gpio/%d", pinName))
	if err != nil {
		activatePin := fmt.Sprintf("echo %d > /sys/class/gpio/export", pin)
		_, err := RunShellCommand(activatePin)
		if err != nil {
			zap.S().Error("error exporting GPIO pin, error: %v", err)
		}

		time.Sleep(2 * time.Second)
	}

	command := fmt.Sprintf("echo out > /sys/class/gpio/gpio%d/direction", pin)
	_, err = RunShellCommand(command)
	if err != nil {
		zap.S().Error("error initializing GPIO pin, error: %v", err)
	}

	time.Sleep(1 * time.Second)

}

func (sbc *SBC) GPIOPinUnexport() {
	command := fmt.Sprintf("echo %d > /sys/class/gpio/unexport", sbc.DisablePin)
	_, err := RunShellCommand(command)
	if err != nil {
		zap.S().Error("error unexporting GPIO pin, error: %v", err)
	}
}

func (sbc *SBC) ModemPowerEnable() {
	sbc.GPIOPinInit(sbc.DisablePin)

	command := fmt.Sprintf("echo 0 > /sys/class/gpio/gpio%d/value", sbc.DisablePin)
	_, err := RunShellCommand(command)
	if err != nil {
		zap.S().Error("error enabling modem power, error: %v", err)
	}
}

func (sbc *SBC) ModemPowerDisable() {
	sbc.GPIOPinInit(sbc.DisablePin)

	command := fmt.Sprintf("echo 1 > /sys/class/gpio/gpio%d/value", sbc.DisablePin)
	_, err := RunShellCommand(command)
	if err != nil {
		zap.S().Error("error disabling modem power, error: %v", err)
	}
}
