package main

type ModemConductor struct {
	Sub      int
	Base     int
	Success  int
	Fail     int
	Interval float32
	IsOk     bool
	Retry    int
	Counter  int
}

func (mc *ModemConductor) ClearCounter() {
	mc.Counter = 0
}

func (mc *ModemConductor) CounterTick() {
	mc.Counter++
}

func (mc *ModemConductor) SetStep(sub int, base int, success int, fail int, interval float32, isOk bool, retry int) {
	mc.Sub = sub
	mc.Base = base
	mc.Success = success
	mc.Fail = fail
	mc.Interval = interval
	mc.IsOk = isOk
	mc.Retry = retry
}
