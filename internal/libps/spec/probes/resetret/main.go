//go:build yaroze

// Command resetret is a probe for spec 37: what ResetGraph returns, by level and by
// how often it has been called, and after the calls that might move it. The bus trace
// (PROBE_BUS=START) shows what each level tells the GPU.
package main

import "go.chrisrx.dev/psx/yaroze"

func r(what string, level int32) {
	v := yaroze.ResetGraph(level)
	println(what, "level", level, "=", v)
}

func main() {
	r("first in main", 0)
	r("again", 0)
	r("third", 0)
	r("then", 1)
	r("then", 3)
	r("then", 2)
	r("then", 4)
	r("then", -1)
	println("START")
	for _, l := range [...]int32{5, 6, 7, 8, 9, 10, 11, 12, 16, 17, 19, 256, 257, 259, 0x10000, 0x10001, -8, -7, -5} {
		r("then", l)
	}
	r("then", 0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	r("after GsInitGraph", 0)
	r("after GsInitGraph", 1)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.GsSwapDispBuff()
	r("after a swap", 0)
	yaroze.VSync(0)
	r("after VSync", 0)
	yaroze.SetDispMask(1)
	r("after SetDispMask", 0)
	yaroze.SetVideoMode(1)
	r("in PAL", 0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	r("in PAL after GsInitGraph", 0)
	r("in PAL after GsInitGraph", 1)
	yaroze.SetVideoMode(0)
	yaroze.GsInitGraph(640, 480, yaroze.GsOFSGPU, 0, 0)
	r("640 by 480", 0)
	yaroze.GsInitGraph(384, 240, yaroze.GsOFSGPU, 0, 0)
	r("384 by 240", 0)
	println("END")
	for {
	}
}
