package mkpsxiso

import (
	"testing"

	"go.chrisrx.dev/x/assert"
)

func TestSystemCNF(t *testing.T) {
	assert.Equal(t, "BOOT=cdrom:\\MAIN.EXE;1\nTCB=4\nEVENT=10\nSTACK=801FF000\n",
		string(systemCNF("main.exe;1", defaultStack)))
	assert.Equal(t, "BOOT=cdrom:\\MAIN.EXE;1\nTCB=4\nEVENT=10\nSTACK=801FFFF0\n",
		string(systemCNF("MAIN.EXE;1", 0x801ffff0)))
}

func TestCheckStack(t *testing.T) {
	for _, sp := range []uint32{defaultStack, 0x801ffff0, 0x80200000, 0x80100000} {
		assert.NoError(t, checkStack(sp), "stack %#08x", sp)
	}

	assert.Error(t, "outside main RAM", checkStack(0x80000000))
	assert.Error(t, "outside main RAM", checkStack(0x80200008))
	assert.Error(t, "outside main RAM", checkStack(0x001ff000))
	assert.Error(t, "outside main RAM", checkStack(0xa01ff000))
	assert.Error(t, "not 8-byte aligned", checkStack(0x801ffff4))
}
