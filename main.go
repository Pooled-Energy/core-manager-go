package main

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

var lock sync.Mutex

// Watch these, i have a feeling i might have screwed up visibility
var networkModem Modem
var conductor ModemConductor
var config Configuration

func init() {
	networkModem.Initialize()
}

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	undo := zap.ReplaceGlobals(logger)
	defer undo()

}

func manageConnections() {
	var interval float32
	for {
		lock.Lock()
		interval = ManageConnection()
		lock.Unlock()

		time.Sleep(time.Duration(interval))
	}
}
