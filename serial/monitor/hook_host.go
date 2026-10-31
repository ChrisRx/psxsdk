//go:build !yaroze

package main

// On a host the monitor only compiles, for go vet: there is no kernel to
// hook or program to run.

func flushCache()              {}
func run(pc, gp, sp uint32)    {}
func installHook()             {}
func installVector()           {}
func resume()                  {}
func currentSR() uint32        { return 0 }
func funcAddr[F any](F) uint32 { return 0 }
