//go:build !unix

package psxplay

import "sync/atomic"

// watchShots does nothing where there is no SIGUSR1; F12 still takes a
// screenshot.
func watchShots(shot *atomic.Bool) {}
