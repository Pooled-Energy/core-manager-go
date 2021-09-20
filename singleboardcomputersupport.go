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

func (sbc *SBC) GPIOInit(pin int) {
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
