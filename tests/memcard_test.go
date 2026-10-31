package tests

import (
	"os"
	"testing"

	"go.chrisrx.dev/x/assert"

	"go.chrisrx.dev/psx/emu"
	"go.chrisrx.dev/psx/emu/emutest"
)

// TestBIOSFormatsMemoryCard opens the BIOS shell's memory card screen with a
// blank card in slot 1, which the BIOS formats when it finds it: a header
// frame starting "MC", and fifteen free directory frames, each starting
// A0h. That takes the card's flag, reads and writes, with their checksums
// and acknowledges, all working as Sony's BIOS expects.
//
// Only Sony's BIOS has that screen: OpenBIOS halts on a boot failure when
// there is no disc, before any shell. So the BIOS comes from PSX_SONY_BIOS
// rather than PSX_BIOS, and the test is skipped without it.
func TestBIOSFormatsMemoryCard(t *testing.T) {
	parallel(t)
	path := os.Getenv("PSX_SONY_BIOS")
	if path == "" {
		t.Skip("PSX_SONY_BIOS is not set")
	}
	m := emutest.Boot(t, emutest.BIOS(path))
	card, err := emu.NewMemoryCard(nil)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	m.InsertCard(1, card)

	m.RunFrames(600) // to the shell's main menu, on MEMORY CARD
	m.Hold(1, emu.ButtonCross, 5)
	m.RunFrames(300)

	data := card.Data()
	assert.Equal(t, "MC", string(data[:2]))
	for i := 1; i < 16; i++ {
		assert.Equal(t, uint8(0xa0), data[i*128], "directory frame %d is free", i)
	}
}
