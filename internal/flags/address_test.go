package flags

import (
	"testing"

	"go.chrisrx.dev/x/assert"
)

func TestAddress(t *testing.T) {
	var a Address
	for _, s := range []string{"0x801FFFF0", "0x801ffff0", "2149580784"} {
		if assert.NoError(t, a.Set(s), "Set(%q)", s) {
			assert.Equal(t, Address(0x801ffff0), a, "Set(%q)", s)
		}
	}
	assert.Equal(t, "0x801FFFF0", a.String())

	assert.Error(t, "not an address", a.Set("801FFFF0"))
	assert.Error(t, "not an address", a.Set("0x1801FFFF0"))
}

func TestAddresses(t *testing.T) {
	var a Addresses
	assert.NoError(t, a.Set("0x80010000"))
	assert.NoError(t, a.Set("0x8001000c"))
	assert.Equal(t, Addresses{0x80010000, 0x8001000c}, a)
	assert.Equal(t, "0x80010000,0x8001000C", a.String())
	assert.Error(t, "not an address", a.Set("main.main"))
}
