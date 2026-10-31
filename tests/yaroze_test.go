package tests

import (
	"archive/zip"
	"bytes"
	"cmp"
	"debug/elf"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.chrisrx.dev/x/assert"

	"go.chrisrx.dev/psx/emu"
	"go.chrisrx.dev/psx/emu/emutest"
	"go.chrisrx.dev/psx/format/cdrom"
	"go.chrisrx.dev/psx/format/psx"
	"go.chrisrx.dev/psx/serial"
)

// The Net Yaroze boot disc's monitor checks for the access card, which is
// not a memory card but a device answering address 21h on the memory card
// port, in a function at 800702CC, in MON\MON.EXE. bootDiscPatch makes it
// return 1, found, straight away.
const (
	bootDiscSector = 79    // MON.EXE's sector holding the function
	bootDiscOffset = 0x2cc // in the sector's data
)

var (
	bootDiscCheck = []byte{0xe8, 0xff, 0xbd, 0x27, 0x14, 0x00, 0xbf, 0xaf} // addiu sp, sp, -18h; sw ra, 14h(sp)
	bootDiscPatch = []byte{0x08, 0x00, 0xe0, 0x03, 0x01, 0x00, 0x02, 0x34} // jr ra; ori v0, zero, 1
)

// bootDisc returns the cue sheet of a copy of the Net Yaroze boot disc with
// the access card check patched out, made from the image PSX_YAROZE_BOOT
// names, a .bin or a .zip holding one as redump has it, or skips the test.
func bootDisc(t *testing.T) string {
	t.Helper()
	path := os.Getenv("PSX_YAROZE_BOOT")
	if path == "" {
		t.Skip("PSX_YAROZE_BOOT is not set")
	}
	img, err := readBin(path)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	sector := img[bootDiscSector*2352 : (bootDiscSector+1)*2352]
	sh, err := cdrom.DecodeSubheader(sector)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	data, err := cdrom.Form1Data(sector)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	data = bytes.Clone(data)
	if !bytes.Equal(data[bootDiscOffset:][:len(bootDiscCheck)], bootDiscCheck) {
		t.Fatalf("%s is not the Net Yaroze boot disc this patches", path)
	}
	copy(data[bootDiscOffset:], bootDiscPatch)
	if !assert.NoError(t, cdrom.EncodeForm1(sector, bootDiscSector, sh, data)) {
		t.FailNow()
	}
	dir := t.TempDir()
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "boot.bin"), img, 0o644))
	cue := filepath.Join(dir, "boot.cue")
	assert.NoError(t, os.WriteFile(cue, []byte("FILE \"boot.bin\" BINARY\r\n  TRACK 01 MODE2/2352\r\n    INDEX 01 00:00:00\r\n"), 0o644))
	return cue
}

// readBin reads a disc image, or the one .bin in a .zip.
func readBin(path string) ([]byte, error) {
	if !strings.EqualFold(filepath.Ext(path), ".zip") {
		return os.ReadFile(path)
	}
	z, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer z.Close()
	for _, f := range z.File {
		if strings.EqualFold(filepath.Ext(f.Name), ".bin") {
			r, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer r.Close()
			return io.ReadAll(r)
		}
	}
	return nil, fmt.Errorf("%s holds no .bin", path)
}

// bootCard returns a memory card holding the file the boot disc's monitor
// reads its line speed from, BWDTLS-03030CIP, set to baud: a little-endian
// word at 22Ch in the file.
func bootCard(t *testing.T, baud uint32) *emu.MemoryCard {
	t.Helper()
	card := make([]byte, 128*1024)
	frame := func(n int) []byte { return card[n*128 : n*128+128] }
	sum := func(f []byte) {
		f[127] = 0
		for _, b := range f[:127] {
			f[127] ^= b
		}
	}
	copy(frame(0), "MC")
	sum(frame(0))
	for i := 1; i < 16; i++ {
		f := frame(i)
		f[0], f[8], f[9] = 0xa0, 0xff, 0xff // free, no next block
		if i == 1 {
			f[0] = 0x51 // the first block of a file
			binary.LittleEndian.PutUint32(f[4:], 8192)
			copy(f[10:], "BWDTLS-03030CIP")
		}
		sum(f)
	}
	for i := 16; i < 36; i++ {
		f := frame(i)
		binary.LittleEndian.PutUint32(f, 0xffffffff) // no broken sector
		f[8], f[9] = 0xff, 0xff
		sum(f)
	}
	binary.LittleEndian.PutUint32(card[8192+0x22c:], baud)
	c, err := emu.NewMemoryCard(card)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	return c
}

// A boot boots a monitor with a memory card setting the line to 115200
// bps and a cable in the serial port, and returns the machine, the cable's
// host end, and the monitor on its other end, waiting at its prompt. opts
// are emutest's, after the BIOS and the disc.
type boot func(t *testing.T, opts ...emutest.Option) (*emutest.Machine, *serial.Port, *serial.Monitor)

// eachMonitor runs test against both monitors, each in a subtest: Sony's,
// on the patched boot disc, and ours, serial/monitor on its own disc. What
// the tests check is what Sony's does, and ours has to do the same.
func eachMonitor(t *testing.T, test func(t *testing.T, boot boot)) {
	t.Run("sony", func(t *testing.T) {
		parallel(t)
		test(t, sonyMonitor)
	})
	t.Run("ours", func(t *testing.T) {
		parallel(t)
		test(t, ourMonitor)
	})
}

// sonyMonitor boots the patched boot disc. Its monitor reads the line
// through the kernel's TTY device, which only Sony's BIOS has, so it comes
// from PSX_SONY_BIOS, and the test is skipped without it.
func sonyMonitor(t *testing.T, opts ...emutest.Option) (*emutest.Machine, *serial.Port, *serial.Monitor) {
	t.Helper()
	bios := os.Getenv("PSX_SONY_BIOS")
	if bios == "" {
		t.Skip("PSX_SONY_BIOS is not set")
	}
	return startMonitor(t, emutest.Boot(t, append([]emutest.Option{emutest.BIOS(bios), emutest.Disc(bootDisc(t))}, opts...)...))
}

// ourMonitor boots serial/monitor's disc, as task build:serial builds it,
// with the BIOS the other tests use, or skips the test if it has not been
// built.
func ourMonitor(t *testing.T, opts ...emutest.Option) (*emutest.Machine, *serial.Port, *serial.Monitor) {
	t.Helper()
	cue := disc(t, "monitor")
	return startMonitor(t, emutest.Boot(t, append([]emutest.Option{emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc(cue)}, opts...)...))
}

// startMonitor puts the card and the cable in, waits for the monitor to
// turn its RTS on, and checks it greets the host.
func startMonitor(t *testing.T, m *emutest.Machine) (*emutest.Machine, *serial.Port, *serial.Monitor) {
	t.Helper()
	m.InsertCard(1, bootCard(t, 115200))
	line := m.ConnectSerial()
	m.RunUntil(func() bool { return m.Bus.Read16(0x1f80105a)&0x20 != 0 }, 60*emutest.Second, "the monitor to raise RTS")
	port := serial.NewPort(line)
	mon, err := serial.NewMonitor(t.Context(), port)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	reply, err := mon.Command(t.Context(), "")
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	assert.Equal(t, true, strings.Contains(reply, "Communication baud rate 115200 bps"), reply)
	return m, port, mon
}

func TestYarozeMonitorHelp(t *testing.T) {
	eachMonitor(t, func(t *testing.T, boot boot) {
		_, _, mon := boot(t)
		reply, err := mon.Command(t.Context(), "help")
		assert.NoError(t, err)
		assert.Equal(t, true, strings.Contains(reply, "PlayStation Debug Monitor"), reply)
	})
}

func TestYarozeSetRegister(t *testing.T) {
	eachMonitor(t, func(t *testing.T, boot boot) {
		_, _, mon := boot(t)
		assert.NoError(t, mon.SetRegister(t.Context(), "epc", 0x80012345))
		reply, err := mon.Command(t.Context(), "dr")
		assert.NoError(t, err)
		assert.Equal(t, true, strings.Contains(reply, "pc(epc)=80012345"), reply)
	})
}

// TestYarozeWriteMemory writes three blocks, the last of a byte, and checks
// they land and nothing past them does, though the last block is sent
// padded to 2048 bytes.
func TestYarozeWriteMemory(t *testing.T) {
	eachMonitor(t, func(t *testing.T, boot boot) {
		m, _, mon := boot(t)
		const addr = 0x80100000
		data := make([]byte, 2*2048+1)
		for i := range data {
			data[i] = byte(i*7 + 1)
		}
		for i := range 16 {
			m.Bus.Write8(addr+uint32(len(data)+i), 0xaa)
		}
		assert.NoError(t, mon.WriteMemory(t.Context(), addr, data))
		got := make([]byte, len(data)+16)
		for i := range got {
			got[i] = m.Bus.Read8(addr + uint32(i))
		}
		assert.Equal(t, data, got[:len(data)])
		assert.Equal(t, bytes.Repeat([]byte{0xaa}, 16), got[len(data):])
	})
}

// serialProgram returns the path of a program under testdata/serial, as
// task build:serial builds it, an ELF, or skips the test if it has not
// been built.
func serialProgram(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(cmp.Or(os.Getenv("PSX_DISCS"), "../.build"), "serial", name+".elf")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("no build of %s: %v", name, err)
	}
	return path
}

// TestYarozePrint uploads a program that prints with println, and checks
// what it prints comes down the cable to the host.
func TestYarozePrint(t *testing.T) {
	eachMonitor(t, func(t *testing.T, boot boot) {
		exe := serialProgram(t, "print")
		_, port, mon := boot(t)
		assert.NoError(t, mon.LoadFile(t.Context(), exe))
		var out bytes.Buffer
		port.Output = &out
		assert.NoError(t, mon.Run(t.Context()))
		_, err := port.ReadUntil(t.Context(), "second 2")
		assert.NoError(t, err)
		t.Logf("%q", out.String())
	})
}

// TestYarozeLoadAndRun uploads an ECOFF executable, one of Sony's samples,
// as sioload does, starts it, and checks what it draws.
func TestYarozeLoadAndRun(t *testing.T) {
	eachMonitor(t, func(t *testing.T, boot boot) {
		m, _, mon := boot(t)
		assert.NoError(t, mon.LoadFile(t.Context(), "../format/ecoff/testdata/main-ecoff"))
		assert.NoError(t, mon.Run(t.Context()))
		m.RunFrames(300)
		m.Golden("yaroze-load.png")
	})
}

// rawTransfer starts a binary transfer by hand, for the tests of what the
// monitor does with what package serial never sends it: the command, the
// "binary" line it answers with, and the header.
func rawTransfer(t *testing.T, port *serial.Port, command string, addr, size uint32) {
	t.Helper()
	port.Discard()
	_, err := port.Write([]byte(command + "\r"))
	assert.NoError(t, err)
	_, err = port.ReadUntil(t.Context(), "binary\r")
	assert.NoError(t, err)
	head := []byte{1, 0, 0, 0, 0, 0, 0, 0, 0}
	binary.BigEndian.PutUint32(head[1:], addr)
	binary.BigEndian.PutUint32(head[5:], size)
	assert.NoError(t, port.Send(t.Context(), head))
}

// rawWriteBlock sends a block of a binary write with the sum given, right
// or wrong, and returns the monitor's answer.
func rawWriteBlock(t *testing.T, port *serial.Port, block []byte, sum byte) byte {
	t.Helper()
	assert.NoError(t, port.SendByte(t.Context(), 2))
	assert.NoError(t, port.Send(t.Context(), block))
	assert.NoError(t, port.SendByte(t.Context(), sum))
	b, err := port.ReceiveByte(t.Context())
	assert.NoError(t, err)
	return b
}

// rawReadBlock asks for a block of a binary read, and returns it and the
// sum the monitor sent with it.
func rawReadBlock(t *testing.T, port *serial.Port) ([]byte, byte) {
	t.Helper()
	assert.NoError(t, port.SendByte(t.Context(), 2))
	block := make([]byte, 2049)
	for i := range block {
		b, err := port.ReceiveByte(t.Context())
		if !assert.NoError(t, err) {
			t.FailNow()
		}
		block[i] = b
	}
	return block[:2048], block[2048]
}

func filled(b byte) ([]byte, byte) {
	block := bytes.Repeat([]byte{b}, 2048)
	return block, byte(2048 * int(b))
}

// TestYarozeWriteResent sends a block with a wrong sum, which the monitor
// answers with N and does not store, whether the host gives up on it or
// sends it again, which the monitor answers with Y and stores. The monitor
// answers before it has stored a block, so memory is looked at once the
// session has ended.
func TestYarozeWriteResent(t *testing.T) {
	eachMonitor(t, func(t *testing.T, boot boot) {
		m, port, _ := boot(t)
		const addr = 0x80100000
		end := func() {
			t.Helper()
			assert.NoError(t, port.SendByte(t.Context(), '\r'))
			_, err := port.ReadUntil(t.Context(), ">>")
			assert.NoError(t, err)
		}
		block, sum := filled(0x22)
		rawTransfer(t, port, "bwr", addr, 2048)
		assert.Equal(t, byte('N'), rawWriteBlock(t, port, block, sum+1))
		end()
		assert.Equal(t, byte(0), m.Bus.Read8(addr), "a block with a wrong sum should not be stored")

		rawTransfer(t, port, "bwr", addr, 2048)
		assert.Equal(t, byte('N'), rawWriteBlock(t, port, block, sum+1))
		assert.Equal(t, byte('Y'), rawWriteBlock(t, port, block, sum))
		end()
		assert.Equal(t, byte(0x22), m.Bus.Read8(addr))
		assert.Equal(t, byte(0x22), m.Bus.Read8(addr+2047))
	})
}

// TestYarozeWriteSession writes twice in one binary session: after the last
// block of a write the monitor waits for another header, and only says "end
// binary", and goes back to its prompt, once the host sends a carriage
// return in its place.
func TestYarozeWriteSession(t *testing.T) {
	eachMonitor(t, func(t *testing.T, boot boot) {
		m, port, mon := boot(t)
		var out bytes.Buffer
		port.Output = &out
		rawTransfer(t, port, "bwr", 0x80100000, 2048)
		block, sum := filled(0x33)
		assert.Equal(t, byte('Y'), rawWriteBlock(t, port, block, sum))
		head := []byte{1, 0x80, 0x10, 0x08, 0x00, 0, 0, 0x08, 0x00} // 80100800, 2048 bytes
		assert.NoError(t, port.Send(t.Context(), head))
		block, sum = filled(0x44)
		assert.Equal(t, byte('Y'), rawWriteBlock(t, port, block, sum))
		assert.Equal(t, false, strings.Contains(out.String(), "end binary"), out.String())
		assert.NoError(t, port.SendByte(t.Context(), '\r'))
		_, err := port.ReadUntil(t.Context(), "end binary\r")
		assert.NoError(t, err)
		_, err = port.ReadUntil(t.Context(), ">>")
		assert.NoError(t, err)
		assert.Equal(t, byte(0x33), m.Bus.Read8(0x80100000))
		assert.Equal(t, byte(0x44), m.Bus.Read8(0x80100800))
		_, err = mon.Command(t.Context(), "dr")
		assert.NoError(t, err, "the monitor should be back at its prompt")
	})
}

// TestYarozeReadMemory reads RAM back with brd, which needs the host to
// pulse RTS after each byte, as Port does.
func TestYarozeReadMemory(t *testing.T) {
	eachMonitor(t, func(t *testing.T, boot boot) {
		m, _, mon := boot(t)
		const addr = 0x80101000
		want := make([]byte, 2048+16)
		for i := range want {
			want[i] = byte(i*5 + 3)
			m.Bus.Write8(addr+uint32(i), want[i])
		}
		got, err := mon.ReadMemory(t.Context(), addr, len(want))
		assert.NoError(t, err)
		assert.Equal(t, want, got)
	})
}

// TestYarozeReadResent answers N to a block of a binary read, which the
// monitor sends again. A short block is padded with what is in RAM after
// it, not zeros.
func TestYarozeReadResent(t *testing.T) {
	eachMonitor(t, func(t *testing.T, boot boot) {
		m, port, _ := boot(t)
		const addr = 0x80101000
		for i := range 2048 {
			m.Bus.Write8(addr+uint32(i), 0xaa)
		}
		for i := range 16 {
			m.Bus.Write8(addr+uint32(i), byte(i))
		}
		rawTransfer(t, port, "brd", addr, 16)
		first, sum := rawReadBlock(t, port)
		var want byte
		for _, b := range first {
			want += b
		}
		assert.Equal(t, want, sum)
		assert.Equal(t, bytes.Repeat([]byte{0xaa}, 2048-16), first[16:])
		assert.NoError(t, port.SendByte(t.Context(), 'N'))
		again, sum2 := rawReadBlock(t, port)
		assert.Equal(t, first, again, "the block should come again")
		assert.Equal(t, sum, sum2)
		assert.NoError(t, port.SendByte(t.Context(), 'Y'))
		assert.NoError(t, port.SendByte(t.Context(), '\r'))
		_, err := port.ReadUntil(t.Context(), "end binary\r")
		assert.NoError(t, err)
	})
}

// TestYarozeExit uploads a program that returns from main: the monitor
// starts again, greets the host as it does at power on, and is back at its
// prompt.
func TestYarozeExit(t *testing.T) {
	eachMonitor(t, func(t *testing.T, boot boot) {
		exe := serialProgram(t, "exit")
		_, port, mon := boot(t)
		assert.NoError(t, mon.LoadFile(t.Context(), exe))
		var out bytes.Buffer
		port.Output = &out
		assert.NoError(t, mon.Run(t.Context()))
		_, err := port.ReadUntil(t.Context(), "Communication baud rate 115200 bps")
		assert.NoError(t, err)
		_, err = port.ReadUntil(t.Context(), ">>")
		assert.NoError(t, err)
		t.Logf("%q", out.String())
		assert.Equal(t, true, strings.Contains(out.String(), "returning\r\r\n"), out.String())
		reply, err := mon.Command(t.Context(), "help")
		assert.NoError(t, err)
		assert.Equal(t, true, strings.Contains(reply, "PlayStation Debug Monitor"), reply)
	})
}

// TestYarozeCrash uploads a program that loads a word from an odd address:
// the address error stops the console, and the monitor answers nothing
// more.
func TestYarozeCrash(t *testing.T) {
	eachMonitor(t, func(t *testing.T, boot boot) {
		exe := serialProgram(t, "crash")
		_, port, mon := boot(t, emutest.AllowFaults())
		assert.NoError(t, mon.LoadFile(t.Context(), exe))
		var out bytes.Buffer
		port.Output = &out
		assert.NoError(t, mon.Run(t.Context()))
		_, err := port.ReadUntil(t.Context(), "crashing\r\r\n")
		assert.NoError(t, err)
		port.Timeout = 2 * time.Second
		_, err = mon.Command(t.Context(), "help")
		assert.Error(t, serial.ErrTimeout, err)
		assert.Equal(t, false, strings.Contains(out.String(), "survived"), out.String())
	})
}

// TestYarozeLoadEXE uploads a program as a PS-X EXE, the form SIOCONS's
// Load takes: the printing program's ELF laid out as one text section, as
// a program alone with no library combined in.
func TestYarozeLoadEXE(t *testing.T) {
	eachMonitor(t, func(t *testing.T, boot boot) {
		path := serialProgram(t, "print")
		f, err := elf.Open(path)
		if !assert.NoError(t, err) {
			t.FailNow()
		}
		defer f.Close()
		var lo, hi uint64 = ^uint64(0), 0
		for _, p := range f.Progs {
			if p.Type == elf.PT_LOAD && p.Filesz > 0 {
				lo, hi = min(lo, p.Vaddr), max(hi, p.Vaddr+p.Filesz)
			}
		}
		text := make([]byte, hi-lo)
		for _, p := range f.Progs {
			if p.Type == elf.PT_LOAD && p.Filesz > 0 {
				_, err := p.ReadAt(text[p.Vaddr-lo:][:p.Filesz], 0)
				assert.NoError(t, err)
			}
		}
		exe := filepath.Join(t.TempDir(), "print.exe")
		assert.NoError(t, psx.New(uint32(lo), uint32(f.Entry), text).WriteFile(exe))

		_, port, mon := boot(t)
		assert.NoError(t, mon.LoadFile(t.Context(), exe))
		var out bytes.Buffer
		port.Output = &out
		assert.NoError(t, mon.Run(t.Context()))
		_, err = port.ReadUntil(t.Context(), "second 1")
		assert.NoError(t, err)
		assert.Equal(t, true, strings.Contains(out.String(), "hello from the console"), out.String())
	})
}
