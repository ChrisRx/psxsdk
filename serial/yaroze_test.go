package serial

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.chrisrx.dev/x/assert"

	"go.chrisrx.dev/psx/format/ecoff"
	"go.chrisrx.dev/psx/format/psx"
)

// fakeMonitor is a Line with a Net Yaroze's monitor on the other end, as
// far as package serial uses it and as the real one behaves: lines of text
// echoed and answered with a prompt, sr setting a register, go, and the
// binary transfers, bwr into mem and brd out of it. A brd sends a byte
// each time the host pulses RTS.
type fakeMonitor struct {
	mem    map[uint32]byte
	regs   map[string]uint32
	ran    bool
	cts    bool
	silent bool // never answers
	refuse int  // blocks still to answer N whatever their sum, or send with a bad sum

	out  bytes.Buffer // text the console has sent and the host not read
	line []byte       // text so far

	// A binary transfer: mode is "bwr" or "brd", and head and block collect
	// the header and a written block as they come. addr and left are where
	// the transfer has got to.
	mode        string
	sessions    int // binary sessions begun
	head, block []byte
	addr, left  uint32
	blockLen    int  // of the block being read, sent or not
	answer      bool // a read block has gone, and its Y or N is due

	// A read block goes a byte at a time: gated holds it, and released is
	// how much of it the host's RTS pulses have let through.
	gated    []byte
	released int
	rts      bool
}

func newFakeMonitor() *fakeMonitor {
	return &fakeMonitor{mem: map[uint32]byte{}, regs: map[string]uint32{}, cts: true, rts: true}
}

func (f *fakeMonitor) CTS() (bool, error) { return f.cts, nil }

func (f *fakeMonitor) SetRTS(on bool) error {
	if on && !f.rts {
		f.released++
	}
	f.rts = on
	return nil
}

func (f *fakeMonitor) Read(p []byte) (int, error) {
	if f.silent {
		return 0, nil
	}
	if f.out.Len() > 0 {
		return f.out.Read(p)
	}
	n := copy(p, f.gated[:min(f.released, len(f.gated))])
	f.gated, f.released = f.gated[n:], max(f.released-n, 0)
	return n, nil
}

func (f *fakeMonitor) Write(p []byte) (int, error) {
	for _, b := range p {
		f.feed(b)
	}
	return len(p), nil
}

func (f *fakeMonitor) feed(b byte) {
	switch {
	case f.mode == "":
		f.text(b)
	case len(f.head) > 0 && len(f.head) < 9:
		f.head = append(f.head, b)
		if len(f.head) == 9 {
			f.addr = binary.BigEndian.Uint32(f.head[1:])
			f.left = binary.BigEndian.Uint32(f.head[5:])
		}
	case f.answer:
		f.answer = false
		if b == 'Y' {
			f.addr += uint32(f.blockLen)
			f.left -= uint32(f.blockLen)
		}
	case len(f.block) > 0 && len(f.block) < 1+blockSize:
		f.block = append(f.block, b)
	case len(f.block) > 0:
		f.written(b)
	case f.left == 0 && b == transferStart:
		f.head = append(f.head[:0], b)
	case f.left == 0:
		f.mode, f.head = "", f.head[:0]
		f.out.WriteString("end binary\r\r>>")
	case b == blockStart && f.mode == "bwr":
		f.block = append(f.block[:0], b)
	case b == blockStart:
		f.sendBlock()
	}
}

// written takes a written block's sum.
func (f *fakeMonitor) written(sum byte) {
	data := f.block[1:]
	f.block = f.block[:0]
	if sum != checksum(data) || f.refuse > 0 {
		f.refuse--
		f.out.WriteByte('N')
		return
	}
	n := min(blockSize, int(f.left))
	for i, d := range data[:n] {
		f.mem[f.addr+uint32(i)] = d
	}
	f.addr += uint32(n)
	f.left -= uint32(n)
	f.out.WriteByte('Y')
}

// sendBlock queues a read block and its sum, a byte to go at once and the
// rest a pulse at a time.
func (f *fakeMonitor) sendBlock() {
	f.blockLen = min(blockSize, int(f.left))
	block := make([]byte, blockSize)
	for i := range block[:f.blockLen] {
		block[i] = f.mem[f.addr+uint32(i)]
	}
	sum := checksum(block)
	if f.refuse > 0 {
		f.refuse--
		sum++
	}
	f.gated, f.released, f.answer = append(block, sum), 1, true
}

func (f *fakeMonitor) text(b byte) {
	switch b {
	case interrupt:
		f.out.WriteString("^C\r\n>>")
	case '\r':
		f.command(string(f.line))
		f.line = f.line[:0]
	default:
		f.line = append(f.line, b)
	}
}

func (f *fakeMonitor) command(line string) {
	f.out.WriteString(line + "\r\n")
	switch fields := strings.Fields(line); {
	case line == "bwr" || line == "brd":
		f.mode, f.left = line, 0
		f.sessions++
		f.out.WriteString("binary\r")
		return
	case line == "go":
		f.ran = true
		return
	case len(fields) == 3 && fields[0] == "sr":
		var v uint32
		fmt.Sscanf(fields[2], "%x", &v)
		f.regs[fields[1]] = v
	}
	f.out.WriteString(">>")
}

func setup(t *testing.T) (*fakeMonitor, *Port, *Monitor) {
	f := newFakeMonitor()
	p := NewPort(f)
	p.Timeout = 100 * time.Millisecond
	m, err := NewMonitor(t.Context(), p)
	assert.NoError(t, err)
	return f, p, m
}

func TestLoad(t *testing.T) {
	f, _, m := setup(t)
	exe, err := ecoff.Open("../format/ecoff/testdata/main-ecoff")
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	defer exe.Close()
	prog, err := ECOFFProgram(exe)
	assert.NoError(t, err)
	assert.NoError(t, m.ClearScreen(t.Context()))
	assert.NoError(t, m.Load(t.Context(), prog))
	assert.NoError(t, m.Run(t.Context()))

	assert.Equal(t, map[string]uint32{"epc": exe.Entry, "gp": exe.GpValue, "sp": defaultStack}, f.regs)
	assert.Equal(t, true, f.ran)
	assert.Equal(t, 1, f.sessions, "the sections should go in one binary session")
	n := 0
	for _, s := range exe.Sections {
		data, err := s.Data()
		assert.NoError(t, err)
		for i, b := range data {
			if f.mem[s.PhysicalAddress+uint32(i)] != b {
				t.Fatalf("%s: byte %d at %08X is %02X, not %02X", s.SectionName(), i, s.PhysicalAddress+uint32(i), f.mem[s.PhysicalAddress+uint32(i)], b)
			}
		}
		n += len(data)
	}
	assert.Equal(t, n, len(f.mem), "only the sections' bytes should be written, not the last block's padding")
}

func TestOpenProgram(t *testing.T) {
	dir := t.TempDir()

	exe := psx.New(0x80090000, 0x80090010, []byte{1, 2, 3, 4})
	exe.GP0 = 0x80098000
	path := filepath.Join(dir, "main.exe")
	assert.NoError(t, exe.WriteFile(path))
	p, err := OpenProgram(path)
	assert.NoError(t, err)
	assert.Equal(t, uint32(0x80090010), p.PC)
	assert.Equal(t, uint32(0x80098000), p.GP)
	assert.Equal(t, uint32(defaultStack), p.SP, "with no stack in the header")
	assert.Equal(t, 1, len(p.Segments))
	assert.Equal(t, uint32(0x80090000), p.Segments[0].Addr)
	assert.Equal(t, int(exe.TextSize), len(p.Segments[0].Data))

	exe.StackAddr, exe.StackSize = 0x801f0000, 0x8000
	assert.NoError(t, exe.WriteFile(path))
	p, err = OpenProgram(path)
	assert.NoError(t, err)
	assert.Equal(t, uint32(0x801f8000), p.SP, "the stack's end, from the header")

	p, err = OpenProgram("../format/ecoff/testdata/main-ecoff")
	assert.NoError(t, err)
	assert.Equal(t, uint32(defaultStack), p.SP)
	assert.Equal(t, 4, len(p.Segments), ".text, .rdata, .data and .sdata")

	srec := filepath.Join(dir, "main.mot")
	assert.NoError(t, os.WriteFile(srec, []byte("S00600004844521B\n"), 0o644))
	_, err = OpenProgram(srec)
	assert.Equal(t, true, err != nil && strings.Contains(err.Error(), "S-records"), err)
}

func TestStream(t *testing.T) {
	f, p, _ := setup(t)
	f.out.Reset()
	var out bytes.Buffer
	p.Output = &out
	f.out.WriteString("hello\r\n")
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	assert.Error(t, context.DeadlineExceeded, p.Stream(ctx))
	assert.Equal(t, "hello\r\n", out.String())
}

func TestWriteMemory(t *testing.T) {
	f, _, m := setup(t)
	// Two blocks and a byte: the third block is padded.
	data := make([]byte, 2*blockSize+1)
	for i := range data {
		data[i] = byte(i * 7)
	}
	assert.NoError(t, m.WriteMemory(t.Context(), 0x80010000, data))
	assert.Equal(t, len(data), len(f.mem))
	assert.Equal(t, data[len(data)-1], f.mem[0x80010000+uint32(len(data)-1)])
	assert.Equal(t, "", f.mode, "the write should have ended")
}

func TestWriteMemoryResent(t *testing.T) {
	f, _, m := setup(t)
	f.refuse = 2
	assert.NoError(t, m.WriteMemory(t.Context(), 0x80010000, []byte{1, 2, 3}))
	assert.Equal(t, map[uint32]byte{0x80010000: 1, 0x80010001: 2, 0x80010002: 3}, f.mem)
}

func TestWriteMemoryRefused(t *testing.T) {
	f, _, m := setup(t)
	f.refuse = blockTries
	err := m.WriteMemory(t.Context(), 0x80010000, []byte{1, 2, 3})
	assert.Equal(t, true, err != nil && strings.Contains(err.Error(), "refused a block 8 times"), err)
}

func TestReadMemory(t *testing.T) {
	f, _, m := setup(t)
	want := make([]byte, blockSize+5)
	for i := range want {
		want[i] = byte(i*3 + 1)
		f.mem[0x80020000+uint32(i)] = want[i]
	}
	got, err := m.ReadMemory(t.Context(), 0x80020000, len(want))
	assert.NoError(t, err)
	assert.Equal(t, want, got)
	assert.Equal(t, "", f.mode, "the read should have ended")
}

func TestReadMemoryResent(t *testing.T) {
	f, _, m := setup(t)
	f.mem[0x80020000] = 9
	f.refuse = 1
	got, err := m.ReadMemory(t.Context(), 0x80020000, 1)
	assert.NoError(t, err)
	assert.Equal(t, []byte{9}, got)
}

func TestNoAnswer(t *testing.T) {
	f, _, m := setup(t)
	f.silent = true
	_, err := m.Command(t.Context(), "cls")
	assert.Error(t, ErrTimeout, err)
}

func TestNoCTS(t *testing.T) {
	f, p, _ := setup(t)
	f.cts = false
	assert.Error(t, ErrTimeout, p.SendByte(t.Context(), 0))
}

func TestCanceled(t *testing.T) {
	f, p, m := setup(t)
	p.Timeout = time.Minute
	f.silent = true
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := m.Command(ctx, "cls")
	assert.Error(t, context.Canceled, err)
	f.silent, f.cts = false, false
	assert.Error(t, context.Canceled, p.SendByte(ctx, 0))
}

func TestReadUntilKeepsTheRest(t *testing.T) {
	f, p, _ := setup(t)
	f.out.Reset()
	f.out.WriteString("one>>two>>")
	s, err := p.ReadUntil(t.Context(), ">>")
	assert.NoError(t, err)
	assert.Equal(t, "one>>", s)
	s, err = p.ReadUntil(t.Context(), ">>")
	assert.NoError(t, err)
	assert.Equal(t, "two>>", s)
}
