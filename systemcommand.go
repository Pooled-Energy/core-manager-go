package main

import (
	"bytes"
	"fmt"
	"os/exec"

	"github.com/godbus/dbus/v5"
)

func RunShellCommand(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", err
	}

	return out.String(), nil
}

func RunModemManagerCommand(command string) (string, error) {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	var result string
	dbusObject := conn.Object("org.freedesktop.ModemManager1", "/org/freedesktop/ModemManager1/Modem/0")
	err = dbusObject.Call("org.freedesktop.ModemManager1.Modem.Command", 0, command, uint32(30)).Store(result)
	if err != nil {
		return result, fmt.Errorf("unable to get response from modem for command %s, error: %v", command, err)
	}

	return result, nil
}
