package main

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

var lock sync.Mutex

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
	interval := 0
	for {
		lock.Lock()
		interval = ManageConnection()
		lock.Unlock()

		time.Sleep(time.Duration(interval))
	}
}
