package emu

import (
	"testing"

	"go.chrisrx.dev/x/assert"
)

// cardTransfer selects the card in slot 1 and exchanges bytes with it
// through JOY_DATA, as the BIOS does, returning what it sent back.
func cardTransfer(m *Machine, send ...uint8) []uint8 {
	const (
		data = 0x1f801040
		ctrl = 0x1f80104a
	)
	m.Bus.Write16(ctrl, joyTXEnable|joyOutput|joyRXEnable)
	var got []uint8
	for _, b := range send {
		m.Bus.Write8(data, b)
		got = append(got, m.Bus.Read8(data))
	}
	m.Bus.Write16(ctrl, 0) // deselect
	return got
}

func cardSector(n int) []uint8 {
	b := make([]uint8, 128)
	for i := range b {
		b[i] = uint8(n + 3*i)
	}
	return b
}

func TestMemoryCardWriteRead(t *testing.T) {
	m := newTestMachine(t)
	card, err := NewMemoryCard(nil)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	m.InsertCard(1, card)

	for _, sector := range []int{0x001, 0x3ff} {
		msb, lsb := uint8(sector>>8), uint8(sector)
		data := cardSector(sector)
		var chk uint8 = msb ^ lsb
		for _, b := range data {
			chk ^= b
		}

		send := append([]uint8{0x81, 'W', 0, 0, msb, lsb}, data...)
		send = append(send, chk, 0, 0, 0)
		got := cardTransfer(m, send...)
		assert.Equal(t, []uint8{0x5a, 0x5d, 0x00, msb}, got[2:6], "sector %x: the command's header", sector)
		assert.Equal(t, []uint8{0x5c, 0x5d, 'G'}, got[len(got)-3:], "sector %x: acknowledged and good", sector)
		assert.Equal(t, data, card.Data()[sector*128:(sector+1)*128], "sector %x: written whole", sector)

		send = []uint8{0x81, 'R', 0, 0, msb, lsb, 0, 0, 0, 0}
		send = append(send, make([]uint8, 128+2)...)
		got = cardTransfer(m, send...)
		assert.Equal(t, []uint8{0x5a, 0x5d, 0x00, msb, 0x5c, 0x5d, msb, lsb}, got[2:10], "sector %x: the read's header", sector)
		assert.Equal(t, data, got[10:138], "sector %x: read back", sector)
		assert.Equal(t, []uint8{chk, 'G'}, got[138:], "sector %x: checksum and end", sector)
	}
	assert.Equal(t, make([]uint8, 128), card.Data()[0x1ff*128:0x200*128], "sector 3FFh does not wrap onto 1FFh")
}

func TestMemoryCardFlagAndID(t *testing.T) {
	m := newTestMachine(t)
	card, _ := NewMemoryCard(nil)
	m.InsertCard(1, card)

	got := cardTransfer(m, 0x81, 'S', 0, 0, 0, 0, 0, 0, 0, 0)
	assert.Equal(t, []uint8{0xff, 0x08, 0x5a, 0x5d, 0x5c, 0x5d, 0x04, 0x00, 0x00, 0x80}, got,
		"a new card's flag, then its ID")
	got = cardTransfer(m, 0x81, 'S', 0)
	assert.Equal(t, uint8(0x00), got[1], "the flag is cleared once read, as psxe has it")

	got = cardTransfer(m, 0x81, 'X', 0, 0)
	assert.Equal(t, []uint8{0xff, 0x00, 0xff, 0xff}, got, "an unknown command is not answered past the flag")

	m.InsertCard(1, nil)
	got = cardTransfer(m, 0x81, 'S', 0)
	assert.Equal(t, []uint8{0xff, 0xff, 0xff}, got, "an empty slot")
}

func TestNewMemoryCardSize(t *testing.T) {
	_, err := NewMemoryCard(make([]byte, 100))
	assert.Equal(t, true, err != nil)
	c, err := NewMemoryCard(make([]byte, MemoryCardSize))
	assert.NoError(t, err)
	assert.Equal(t, MemoryCardSize, len(c.Data()))
}
