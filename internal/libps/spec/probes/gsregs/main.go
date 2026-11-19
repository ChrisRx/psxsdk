//go:build yaroze

// Command gsregs is a probe for spec 19: which GTE registers the coordinate calls
// change. Each case fills the GTE's registers with a pattern, makes the call, and
// lists the registers that changed, as d (data) or c (control) with the number,
// old and new value.
package main

import (
	"device"

	"go.chrisrx.dev/psx/yaroze"
)

//go:extern PSDCNT
var psdCNT uint32

var gp0 uint32

func getGP() uint32 {
	return uint32(device.AsmFull(".set push\n.set noat\nmove {}, $$28\n.set pop", nil))
}

func setGP(v uint32) {
	device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.set pop", map[string]interface{}{"v": v})
}

func readData(i int) (v uint32) {
	switch i {
	case 0:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x481c0000\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 1:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x481c0800\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 2:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x481c1000\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 3:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x481c1800\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 4:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x481c2000\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 5:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x481c2800\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 6:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x481c3000\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 7:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x481c3800\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 8:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x481c4000\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 9:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x481c4800\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 10:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x481c5000\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 11:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x481c5800\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 12:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x481c6000\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 13:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x481c6800\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 14:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x481c7000\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 15:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x481c7800\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 16:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x481c8000\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 17:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x481c8800\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 18:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x481c9000\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 19:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x481c9800\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 20:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x481ca000\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 21:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x481ca800\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 22:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x481cb000\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 23:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x481cb800\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 24:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x481cc000\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 25:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x481cc800\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 26:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x481cd000\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 27:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x481cd800\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 28:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x481ce000\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 29:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x481ce800\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 30:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x481cf000\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 31:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x481cf800\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	}
	setGP(gp0)
	return v
}

func readCtrl(i int) (v uint32) {
	switch i {
	case 0:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x485c0000\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 1:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x485c0800\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 2:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x485c1000\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 3:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x485c1800\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 4:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x485c2000\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 5:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x485c2800\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 6:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x485c3000\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 7:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x485c3800\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 8:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x485c4000\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 9:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x485c4800\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 10:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x485c5000\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 11:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x485c5800\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 12:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x485c6000\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 13:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x485c6800\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 14:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x485c7000\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 15:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x485c7800\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 16:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x485c8000\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 17:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x485c8800\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 18:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x485c9000\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 19:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x485c9800\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 20:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x485ca000\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 21:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x485ca800\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 22:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x485cb000\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 23:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x485cb800\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 24:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x485cc000\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 25:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x485cc800\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 26:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x485cd000\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 27:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x485cd800\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 28:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x485ce000\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 29:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x485ce800\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 30:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x485cf000\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	case 31:
		v = uint32(device.AsmFull(".set push\n.set noat\n.word 0x485cf800\nnop\nnop\nmove {}, $$28\n.set pop", nil))
	}
	setGP(gp0)
	return v
}

func writeData(i int, v uint32) {
	switch i {
	case 0:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x489c0000\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 1:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x489c0800\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 2:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x489c1000\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 3:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x489c1800\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 4:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x489c2000\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 5:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x489c2800\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 6:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x489c3000\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 7:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x489c3800\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 8:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x489c4000\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 9:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x489c4800\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 10:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x489c5000\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 11:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x489c5800\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 12:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x489c6000\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 13:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x489c6800\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 14:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x489c7000\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 15:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x489c7800\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 16:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x489c8000\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 17:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x489c8800\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 18:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x489c9000\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 19:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x489c9800\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 20:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x489ca000\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 21:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x489ca800\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 22:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x489cb000\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 23:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x489cb800\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 24:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x489cc000\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 25:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x489cc800\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 26:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x489cd000\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 27:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x489cd800\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 28:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x489ce000\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 29:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x489ce800\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 30:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x489cf000\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 31:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x489cf800\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	}
	setGP(gp0)
}

func writeCtrl(i int, v uint32) {
	switch i {
	case 0:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x48dc0000\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 1:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x48dc0800\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 2:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x48dc1000\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 3:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x48dc1800\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 4:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x48dc2000\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 5:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x48dc2800\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 6:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x48dc3000\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 7:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x48dc3800\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 8:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x48dc4000\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 9:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x48dc4800\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 10:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x48dc5000\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 11:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x48dc5800\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 12:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x48dc6000\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 13:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x48dc6800\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 14:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x48dc7000\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 15:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x48dc7800\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 16:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x48dc8000\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 17:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x48dc8800\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 18:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x48dc9000\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 19:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x48dc9800\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 20:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x48dca000\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 21:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x48dca800\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 22:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x48dcb000\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 23:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x48dcb800\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 24:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x48dcc000\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 25:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x48dcc800\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 26:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x48dcd000\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 27:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x48dcd800\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 28:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x48dce000\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 29:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x48dce800\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 30:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x48dcf000\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	case 31:
		device.AsmFull(".set push\n.set noat\nmove $$28, {v}\n.word 0x48dcf800\nnop\nnop\n.set pop", map[string]interface{}{"v": v})
	}
	setGP(gp0)
}

var (
	line  [512]byte
	nline int
)

func putc(c byte) {
	if nline < len(line) {
		line[nline] = c
		nline++
	}
	if c == '\n' {
		yaroze.Write(1, &line[0], int32(nline))
		nline = 0
	}
}

func ps(s string) {
	for i := 0; i < len(s); i++ {
		putc(s[i])
	}
}

func pi(v int32) {
	u := uint32(v)
	if v < 0 {
		putc('-')
		u = -u
	}
	var d [10]byte
	k := 0
	for {
		d[k] = byte('0' + u%10)
		k++
		u /= 10
		if u == 0 {
			break
		}
	}
	for k > 0 {
		k--
		putc(d[k])
	}
}

func hex(v uint32) {
	for i := range 8 {
		putc("0123456789abcdef"[v>>(28-4*i)&15])
	}
}

var data0, ctrl0 [32]uint32

// fill gives every register that can hold one a pattern, and notes what each
// reads back as.
func fill() {
	for i := range 32 {
		switch i {
		case 15, 28, 29, 31: // SXYP pushes, IRGB and ORGB and LZCR follow others
			continue
		}
		writeData(i, 0x01010101*uint32(i+1)&0x0fff0fff|0x10)
	}
	for i := range 31 {
		writeCtrl(i, 0x01020304*uint32(i+1)&0x0fff0fff|0x20)
	}
	for i := range 32 {
		data0[i] = readData(i)
		ctrl0[i] = readCtrl(i)
	}
}

// changed prints every register that differs from fill's.
func changed() {
	for i := range 32 {
		if v := readData(i); v != data0[i] {
			ps(" d")
			pi(int32(i))
			putc(':')
			hex(data0[i])
			putc('>')
			hex(v)
		}
	}
	for i := range 32 {
		if v := readCtrl(i); v != ctrl0[i] {
			ps(" c")
			pi(int32(i))
			putc(':')
			hex(ctrl0[i])
			putc('>')
			hex(v)
		}
	}
	ps("\n")
}

var (
	chain [3]yaroze.GsCOORDINATE2
	lw    yaroze.MATRIX
	ls    yaroze.MATRIX
	comp  yaroze.MATRIX
	rv    yaroze.GsRVIEW2
	v2    yaroze.GsVIEW2
)

func build(flags ...uint32) *yaroze.GsCOORDINATE2 {
	var up *yaroze.GsCOORDINATE2
	for i, f := range flags {
		yaroze.GsInitCoordinate2(up, &chain[i])
		chain[i].Coord = yaroze.MATRIX{
			M: [3][3]int16{{2896, -2896, 0}, {2896, 2896, 0}, {0, 0, 4096}},
			T: [3]int32{int32(100 * (i + 1)), 200, 300},
		}
		chain[i].Workm = chain[i].Coord
		chain[i].Flg = f
		up = &chain[i]
	}
	return up
}

func run(what string, call func()) {
	fill()
	call()
	ps(what)
	ps(":")
	changed()
}

func main() {
	gp0 = getGP()
	yaroze.ResetGraph(0)
	yaroze.GsInitGraph(320, 240, yaroze.GsOFSGPU, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, 240)
	yaroze.GsInit3D()
	*yaroze.GsWSMATRIX() = chain[0].Coord
	yaroze.GsWSMATRIX().T = [3]int32{1, 2, 3}
	C := psdCNT
	x := C + 1

	c := build(C)
	run("Lw root flag C", func() { yaroze.GsGetLw(c, &lw) })
	c = build(x)
	run("Lw root flag x", func() { yaroze.GsGetLw(c, &lw) })
	c = build(0)
	run("Lw root flag 0", func() { yaroze.GsGetLw(c, &lw) })
	c = build(x, x, C)
	run("Lw target C", func() { yaroze.GsGetLw(c, &lw) })
	c = build(x, x, x)
	run("Lw xxx", func() { yaroze.GsGetLw(c, &lw) })
	c = build(x, x, 0)
	run("Lw xx0", func() { yaroze.GsGetLw(c, &lw) })
	c = build(0, 0, 0)
	run("Lw 000", func() { yaroze.GsGetLw(c, &lw) })
	c = build(x, x, C)
	run("Ls target C", func() { yaroze.GsGetLs(c, &ls) })
	c = build(0, 0, 0)
	run("Ls 000", func() { yaroze.GsGetLs(c, &ls) })
	c = build(x)
	run("Ls root x", func() { yaroze.GsGetLs(c, &ls) })
	c = build(x, x, C)
	run("Lws target C", func() { yaroze.GsGetLws(c, &lw, &ls) })
	c = build(0, 0, 0)
	run("Lws 000", func() { yaroze.GsGetLws(c, &lw, &ls) })
	run("GsInitCoordinate2 nil", func() { yaroze.GsInitCoordinate2(nil, &chain[0]) })
	run("GsInitCoordinate2 up", func() { yaroze.GsInitCoordinate2(&chain[0], &chain[1]) })
	c = build(0, 0, 0)
	v2.View = chain[0].Coord
	v2.Super = nil
	run("GsSetView2 world", func() { yaroze.GsSetView2(&v2) })
	v2.Super = c
	run("GsSetView2 chain 000", func() { yaroze.GsSetView2(&v2) })
	c = build(x, x, x)
	v2.Super = c
	run("GsSetView2 chain xxx", func() { yaroze.GsSetView2(&v2) })
	rv = yaroze.GsRVIEW2{Vpx: 100, Vpy: 200, Vpz: -1000, Vrx: 10, Vry: 20, Vrz: 30}
	run("GsSetRefView2 world", func() { yaroze.GsSetRefView2(&rv) })
	c = build(0, 0, 0)
	rv.Super = c
	run("GsSetRefView2 chain 000", func() { yaroze.GsSetRefView2(&rv) })
	// A rotation that saturates, and what the flag register holds after it.
	c = build(x, 0)
	chain[0].Workm.M = [3][3]int16{{32767, 32767, 32767}, {32767, 32767, 32767}, {32767, 32767, 32767}}
	chain[1].Coord.M = chain[0].Workm.M
	run("Lw saturating", func() { yaroze.GsGetLw(c, &lw) })
	ps("flag after saturating ")
	hex(readCtrl(31))
	ps("\n")
	c = build(x, 0)
	run("Lw plain", func() { yaroze.GsGetLw(c, &lw) })
	ps("flag after plain ")
	hex(readCtrl(31))
	ps("\n")
	a, b := chain[0].Coord, chain[1].Coord
	run("CompMatrix", func() { yaroze.CompMatrix(&a, &b, &comp) })
	println("END")
	for {
	}
}
