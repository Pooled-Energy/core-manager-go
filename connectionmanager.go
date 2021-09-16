package main

import "fmt"

var conductor ModemConductor

func connectionManagerTest() {
	fmt.Println("Hello, Ru!")
}

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
	
}

var actions = [...]func(){organizer}

func ExecuteStep(step int) {
	actions[step]()
}

func ManageConnection() int {
	ExecuteStep(conductor.Sub)
	return conductor.Interval
}
