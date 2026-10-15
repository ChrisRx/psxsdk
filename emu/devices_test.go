package emu

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"go.chrisrx.dev/x/assert"
)

// writeTestDisc writes a single-track data disc of n sectors, each sector's
// data starting with its index, and a primary volume descriptor naming it a
// PlayStation disc.
func writeTestDisc(t *testing.T, n int) string {
	t.Helper()
	dir := t.TempDir()
	bin := make([]byte, n*sectorSize)
	for i := range n {
		bin[i*sectorSize+24] = byte(i)
	}
	copy(bin[16*sectorSize+0x20:], "PLAYSTATION")
	if !assert.NoError(t, os.WriteFile(filepath.Join(dir, "test.bin"), bin, 0o644)) {
		t.FailNow()
	}
	cue := "FILE \"test.bin\" BINARY\n  TRACK 01 MODE2/2352\n    INDEX 01 00:00:00\n"
	path := filepath.Join(dir, "test.cue")
	if !assert.NoError(t, os.WriteFile(path, []byte(cue), 0o644)) {
		t.FailNow()
	}
	return path
}

func openTestDisc(t *testing.T, n int) *Disc {
	t.Helper()
	d, err := OpenDisc(writeTestDisc(t, n))
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestDiscLayout(t *testing.T) {
	d := openTestDisc(t, 20)

	assert.Equal(t, []int{sectorData, sectorData, sectorFar},
		[]int{d.query(150), d.query(169), d.query(170)}, "the track runs from 00:02:00 for 20 sectors")
	assert.Equal(t, uint32(150), d.trackLBA(1))
	assert.Equal(t, uint32(170), d.trackLBA(0), "track 0 is the end of the disc")
	assert.Equal(t, 1, d.trackNumber(160))
	assert.Equal(t, discLicensed, d.kind())

	var buf [sectorSize]byte
	assert.Equal(t, sectorData, d.read(155, buf[:]))
	assert.Equal(t, byte(5), buf[24], "LBA 155 is the file's sixth sector")
}

// TestOpenDiscFS opens a disc from memory, with the cue sheet in a
// directory naming its file relative to it, as a frontend without a file
// system does.
func TestOpenDiscFS(t *testing.T) {
	bin := make([]byte, 20*sectorSize)
	bin[5*sectorSize+24] = 5
	fsys := fstest.MapFS{
		"discs/test.cue": {Data: []byte("FILE \"test.bin\" BINARY\n  TRACK 01 MODE2/2352\n    INDEX 01 00:00:00\n")},
		"discs/test.bin": {Data: bin},
	}
	d, err := OpenDiscFS(fsys, "discs/test.cue")
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	defer d.Close()

	assert.Equal(t, uint32(170), d.trackLBA(0), "the track runs from 00:02:00 for 20 sectors")
	var buf [sectorSize]byte
	assert.Equal(t, sectorData, d.read(155, buf[:]))
	assert.Equal(t, byte(5), buf[24])

	_, err = OpenDiscFS(fstest.MapFS{"test.cue": fsys["discs/test.cue"]}, "test.cue")
	assert.Error(t, fs.ErrNotExist, err, "the cue sheet's file is missing")
}

// cdCommand sends a CD-ROM command, runs until each of its responses
// arrives, acknowledging each, and returns them with their interrupt
// numbers.
func cdCommand(t *testing.T, m *Machine, cmd uint8, params ...uint8) [][]uint8 {
	t.Helper()
	const (
		index = 0x1f801800
		reg1  = 0x1f801801
		reg2  = 0x1f801802
		reg3  = 0x1f801803
	)
	m.Bus.Write8(index, 1)
	m.Bus.Write8(reg2, 0x1f) // IER: every interrupt
	m.Bus.Write8(index, 0)
	for _, p := range params {
		m.Bus.Write8(reg2, p)
	}
	m.Bus.Write8(reg1, cmd)

	var responses [][]uint8
	for range 100_000 {
		if !assert.NoError(t, m.Step()) {
			t.FailNow()
		}
		m.Bus.Write8(index, 1)
		irq := m.Bus.Read8(reg3) & 7
		if irq == 0 {
			continue
		}
		resp := []uint8{irq}
		m.Bus.Write8(index, 0)
		for m.Bus.Read8(index)&0x20 != 0 {
			resp = append(resp, m.Bus.Read8(reg1))
		}
		responses = append(responses, resp)
		m.Bus.Write8(index, 1)
		m.Bus.Write8(reg3, 0x1f) // acknowledge
		// GetID answers twice, unless it fails; every other command used
		// here once.
		if cmd != cdGetID || irq == 5 || len(responses) == 2 {
			return responses
		}
	}
	t.Fatalf("command %#02x: no response in 100k steps", cmd)
	return nil
}

func TestCDROMGetID(t *testing.T) {
	m := newTestMachine(t)
	m.InsertDisc(openTestDisc(t, 20))

	assert.Equal(t, [][]uint8{
		{3, cdStatSpindle},
		{2, 0x02, 0x00, 0x20, 0x00, 'S', 'C', 'E', 'A'},
	}, cdCommand(t, m, cdGetID))
}

func TestCDROMErrors(t *testing.T) {
	m := newTestMachine(t)
	m.InsertDisc(openTestDisc(t, 20))

	assert.Equal(t, [][]uint8{{5, cdStatError | cdStatSpindle, cdErrWrongParams}},
		cdCommand(t, m, cdSetloc, 0x00), "Setloc takes three parameters")
	assert.Equal(t, [][]uint8{{5, cdStatError | cdStatSpindle, cdErrInvalidCmd}},
		cdCommand(t, m, 0x17), "17h is not a command")
	assert.Equal(t, [][]uint8{{3, 0x94, 0x09, 0x19, 0xc0}},
		cdCommand(t, m, cdTest, 0x20), "Test 20h reports the firmware date")

	empty := newTestMachine(t)
	// psxe reports this one without the spindle bit.
	assert.Equal(t, [][]uint8{{5, cdStatError | cdStatShellOpen, cdErrNoDisc}},
		cdCommand(t, empty, cdGetID), "with no disc")
}

func TestPadTransfer(t *testing.T) {
	m := newTestMachine(t)
	const (
		data = 0x1f801040
		ctrl = 0x1f80104a
	)
	m.Pad(1).Press(ButtonStart | ButtonCross)

	m.Bus.Write16(ctrl, joyTXEnable|joyOutput) // select port 1
	m.Bus.Write8(data, 0x01)                   // address the pad
	m.Bus.Write8(data, 0x42)                   // read buttons

	var reply [5]uint8
	for i := range reply {
		reply[i] = m.Bus.Read8(data)
	}
	// Buttons are active low: Start is bit 3 of the first, Cross bit 6 of
	// the second.
	assert.Equal(t, [5]uint8{0xff, 0x41, 0x5a, 0xf7, 0xbf}, reply)
	assert.Equal(t, uint8(0xff), m.Bus.Read8(data), "the transfer is over")
}
