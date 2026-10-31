package main

import (
	"testing"

	"go.chrisrx.dev/x/assert"
)

func TestHex(t *testing.T) {
	assert.Equal(t, uint32(0x80012345), hex([]byte("80012345")))
	assert.Equal(t, uint32(0xabcdef), hex([]byte("ABCdef")))
	assert.Equal(t, uint32(0x12), hex([]byte("12zz")))
	assert.Equal(t, uint32(0), hex([]byte("zz")))
}

func TestSplit(t *testing.T) {
	got := split([]byte("  sr epc\t80012345 "))
	assert.Equal(t, 3, len(got))
	assert.Equal(t, "sr", string(got[0]))
	assert.Equal(t, "epc", string(got[1]))
	assert.Equal(t, "80012345", string(got[2]))
	assert.Equal(t, true, is([]byte("SR"), "sr"))
	assert.Equal(t, 32, register([]byte("epc")))
	assert.Equal(t, -1, register([]byte("pc")))
}
