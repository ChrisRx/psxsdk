//go:build unix

package psxplay

import (
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
)

// watchShots sets shot each time the process is sent SIGUSR1, which asks
// for a screenshot of a running window from outside it.
func watchShots(shot *atomic.Bool) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGUSR1)
	go func() {
		for range ch {
			shot.Store(true)
		}
	}()
}
