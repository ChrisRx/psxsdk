//go:build yaroze

// Command gtelib is a probe for spec 18: MulMatrix and SetLightMatrix. Each case
// fills the GTE's registers with a pattern, makes the call, and lists the registers
// that changed; then the matrices. MulMatrix goes over edge cases and a random
// sequence, against MulMatrix0 on the same inputs. gp is compared around each
// call, and a final group gives the costs from the second counter.
package main

import (
	"device"
	"runtime/volatile"
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

//export MulMatrix
func mulMatrix(m0, m1 *yaroze.MATRIX) unsafe.Pointer

//export SetLightMatrix
func setLightMatrix(m *yaroze.MATRIX)

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

func show(what string, m *yaroze.MATRIX) {
	ps(what)
	for r := range 3 {
		for c := range 3 {
			putc(' ')
			pi(int32(m.M[r][c]))
		}
	}
	ps(" |")
	for r := range 3 {
		putc(' ')
		pi(m.T[r])
	}
	ps("\n")
}

var seed uint32 = 4242

func next(lo, hi int32) int32 {
	seed = seed*1103515245 + 12345
	return lo + int32((seed>>8)%uint32(hi-lo))
}

var edges = [...]int16{0, 1, -1, 4096, -4096, 32767, -32768, 16384, -16384, 2048, 4095}

func randMatrix(m *yaroze.MATRIX, small bool) {
	for r := range 3 {
		for c := range 3 {
			switch {
			case small:
				m.M[r][c] = int16(next(-4096, 4097))
			case next(0, 4) == 0:
				m.M[r][c] = edges[next(0, int32(len(edges)))]
			default:
				m.M[r][c] = int16(next(-32768, 32768))
			}
		}
		m.T[r] = next(-32768, 32768)<<16 | next(0, 65536)
	}
}

func checkGP(what string) {
	if g := getGP(); g != gp0 {
		ps("gp changed by ")
		ps(what)
		putc(' ')
		hex(g)
		ps("\n")
		setGP(gp0)
	}
}

var a, b, c, d yaroze.MATRIX

func mul(what string) {
	c = a
	d = b
	fill()
	r := mulMatrix(&c, &d)
	checkGP("MulMatrix")
	ps(what)
	switch r {
	case unsafe.Pointer(&c):
		ps(" returns m0")
	case unsafe.Pointer(&d):
		ps(" returns m1")
	default:
		ps(" returns other")
	}
	changed()
	show(" m0", &c)
	show(" m1", &d)
	var e yaroze.MATRIX
	e = a
	fill()
	yaroze.MulMatrix0(&a, &b, &e)
	ps(" MulMatrix0")
	changed()
	if e.M != c.M {
		show(" MulMatrix0 differs", &e)
	}
}

var (
	count = (*uint32)(unsafe.Pointer(uintptr(0x1f801120)))
	mode  = (*uint32)(unsafe.Pointer(uintptr(0x1f801124)))
)

func timed(what string, call func()) {
	best := uint32(0xffffffff)
	for range 7 {
		volatile.StoreUint32(mode, 0)
		t0 := volatile.LoadUint32(count)
		call()
		t1 := volatile.LoadUint32(count)
		if t1-t0 < best {
			best = t1 - t0
		}
	}
	ps("cost ")
	ps(what)
	putc(' ')
	pi(int32(best))
	ps("\n")
}

func main() {
	gp0 = getGP()
	// Registers, with no call, as a baseline for the fill.
	fill()
	ps("baseline")
	changed()
	for i := range 32 {
		ps("reg ")
		pi(int32(i))
		putc(' ')
		hex(data0[i])
		putc(' ')
		hex(ctrl0[i])
		ps("\n")
	}

	identity := yaroze.MATRIX{M: [3][3]int16{{4096, 0, 0}, {0, 4096, 0}, {0, 0, 4096}}}
	a = identity
	b = identity
	mul("identity")
	a = yaroze.MATRIX{M: [3][3]int16{{1, 2, 3}, {4, 5, 6}, {7, 8, 9}}, T: [3]int32{10, 20, 30}}
	b = yaroze.MATRIX{M: [3][3]int16{{4096, 8192, 12288}, {-4096, -8192, -12288}, {100, 200, 300}}, T: [3]int32{40, 50, 60}}
	mul("small")
	// One large term, and three that add past the range.
	a = yaroze.MATRIX{M: [3][3]int16{{32767, 0, 0}, {0, 32767, 0}, {0, 0, 32767}}}
	b = a
	mul("large")
	a = yaroze.MATRIX{M: [3][3]int16{{-32768, -32768, -32768}, {32767, 32767, 32767}, {-32768, 32767, -32768}}}
	b = yaroze.MATRIX{M: [3][3]int16{{-32768, -32768, -32768}, {-32768, 32767, 32767}, {32767, -32768, 32767}}}
	mul("overflow")
	for i := range 16 {
		randMatrix(&a, true)
		randMatrix(&b, true)
		ps("r")
		pi(int32(i))
		mul(" small random")
	}
	for i := range 32 {
		randMatrix(&a, false)
		randMatrix(&b, false)
		ps("r")
		pi(int32(i))
		mul(" large random")
	}
	// The result the same matrix as an operand.
	a = yaroze.MATRIX{M: [3][3]int16{{1000, 2000, 3000}, {-1000, 500, 250}, {7, 8, 9}}, T: [3]int32{1, 2, 3}}
	c = a
	fill()
	r := mulMatrix(&c, &c)
	checkGP("MulMatrix")
	if r != unsafe.Pointer(&c) {
		ps("alias: other result\n")
	}
	ps("alias")
	changed()
	show(" m", &c)
	var e yaroze.MATRIX
	yaroze.MulMatrix0(&a, &a, &e)
	show(" MulMatrix0", &e)

	// SetLightMatrix.
	for i, m := range [...]yaroze.MATRIX{
		identity,
		{M: [3][3]int16{{1, 2, 3}, {4, 5, 6}, {7, 8, 9}}, T: [3]int32{10, 20, 30}},
		{M: [3][3]int16{{-1, -2, -3}, {-4, -5, -6}, {-7, -8, -9}}, T: [3]int32{-10, -20, -30}},
		{M: [3][3]int16{{32767, -32768, 16384}, {-16384, 4096, -4096}, {255, 256, -256}}},
	} {
		c = m
		fill()
		setLightMatrix(&c)
		checkGP("SetLightMatrix")
		ps("light ")
		pi(int32(i))
		changed()
		show(" m", &c)
	}
	a = identity
	b = identity
	timed("nothing", func() {})
	timed("MulMatrix", func() { mulMatrix(&a, &b) })
	timed("MulMatrix0", func() { yaroze.MulMatrix0(&a, &b, &c) })
	timed("SetLightMatrix", func() { setLightMatrix(&a) })
	timed("SetRotMatrix", func() { yaroze.SetRotMatrix(&a) })
	end()
}

// libps_biosB calls function fn of the kernel's B0 table.
//
//export libps_biosB
func libps_biosB(fn, a0, a1, a2, a3 uintptr) int32

// end prints the line that stops the run, through the kernel's putchar, since
// write would put a "\r" before the "\n" and the harness looks for "END\n".
func end() {
	for _, c := range [...]byte{'E', 'N', 'D', '\n'} {
		libps_biosB(0x3d, uintptr(c), 0, 0, 0)
	}
	for {
	}
}
