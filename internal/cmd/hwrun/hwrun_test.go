package hwrun

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.chrisrx.dev/x/assert"

	"go.chrisrx.dev/psx/serial"
)

// console is a line to a monitor that has no program to run: it echoes commands
// and answers with a prompt, and is silent after the reset button is wanted
// until it's pressed. With reboots set, go with epc at the BIOS's reset address
// brings up the boot disc's monitor, whose first word is bootWord.
type console struct {
	out      bytes.Buffer
	line     []byte
	stuck    bool
	word     string // what a dw command reads
	epc      string
	reboots  bool
	bootWord string
}

// booted is what the boot disc's monitor prints as it starts.
const booted = "Connected CIP ver2.0\r\nCommunication baud rate 115200 bps\r\nOK\r\n>>"

func (c *console) CTS() (bool, error)   { return !c.stuck, nil }
func (c *console) SetRTS(on bool) error { return nil }

func (c *console) Read(p []byte) (int, error) {
	if c.stuck || c.out.Len() == 0 {
		return 0, nil
	}
	return c.out.Read(p)
}

func (c *console) Write(p []byte) (int, error) {
	for _, b := range p {
		switch b {
		case '\r':
			c.out.WriteString(string(c.line) + "\r\n")
			if epc, ok := strings.CutPrefix(string(c.line), "sr epc "); ok {
				c.epc = epc
			}
			if string(c.line) == "go" && c.reboots && c.epc == "bfc00000" {
				c.out.WriteString(booted)
				c.word = c.bootWord
				c.line = c.line[:0]
				continue
			}
			if strings.HasPrefix(string(c.line), "dw ") {
				c.out.WriteString(strings.TrimPrefix(string(c.line), "dw ")[:8] + ":" + c.word + "\r\n")
			}
			c.out.WriteString(">>")
			c.line = c.line[:0]
		case 3:
			c.out.WriteString("^C\r\n>>")
		default:
			c.line = append(c.line, b)
		}
	}
	return len(p), nil
}

// A console stuck after a step fails gets a reset, and is talked to again.
func TestResetWaitsForTheMonitor(t *testing.T) {
	c := &console{stuck: true}
	port := serial.NewPort(c)
	port.Timeout = 200 * time.Millisecond
	asked := ""
	s := &Session{
		Port: port,
		Log:  io.Discard,
		Reset: func(why string) error {
			asked = why
			c.stuck = false
			c.out.WriteString("Connected CIP ver2.0\r\nCommunication baud rate 115200 bps\r\nOK\r\n>>")
			return nil
		},
	}
	assert.NoError(t, s.reset(t.Context(), "stuck"))
	assert.Equal(t, "stuck", asked)
	assert.Equal(t, 200*time.Millisecond, port.Timeout)
	assert.Equal(t, true, s.mon != nil)
}

// A run for Sony's library under the clean monitor reboots the console from
// the monitor's prompt first, since its programs would run on the clean
// library, and needs no button.
func TestSonyRunUnderCleanMonitor(t *testing.T) {
	c := &console{word: "240bffff", reboots: true, bootWord: "3c1a8006"}
	port := serial.NewPort(c)
	port.Timeout = 200 * time.Millisecond
	asked := ""
	s := &Session{
		Port:   port,
		Marker: 0x240bffff,
		Log:    io.Discard,
		Reset: func(why string) error {
			asked = why
			return nil
		},
	}
	assert.NoError(t, s.Start(t.Context()))
	assert.Equal(t, "", asked)
	assert.Equal(t, "3c1a8006", c.word)
	assert.Equal(t, 200*time.Millisecond, port.Timeout)
}

// A console that doesn't reboot from the prompt gets the reset button.
func TestRebootFallsBackToTheButton(t *testing.T) {
	c := &console{word: "240bffff"}
	port := serial.NewPort(c)
	port.Timeout = 200 * time.Millisecond
	asked := ""
	s := &Session{
		Port:   port,
		Marker: 0x240bffff,
		Boot:   200 * time.Millisecond,
		Log:    io.Discard,
		Reset: func(why string) error {
			asked = why
			c.word = "3c1a8006"
			c.out.WriteString(booted)
			return nil
		},
	}
	assert.NoError(t, s.Start(t.Context()))
	assert.Equal(t, "the clean monitor is up, and Sony's library is wanted", asked)

	asked = ""
	assert.NoError(t, s.Start(t.Context()))
	assert.Equal(t, "", asked)
}

func TestArchiveJobs(t *testing.T) {
	dir := t.TempDir()
	data := filepath.Join(dir, "DATA.TIM")
	assert.NoError(t, os.WriteFile(data, []byte{1, 2, 3}, 0o644))
	exe := make([]byte, 0x800)
	copy(exe, "PS-X EXE")
	binary.LittleEndian.PutUint32(exe[0x10:], 0x80090000)
	binary.LittleEndian.PutUint32(exe[0x18:], 0x80090000)
	binary.LittleEndian.PutUint32(exe[0x1c:], 0x800)
	exe = append(exe, make([]byte, 0x800)...)
	path := filepath.Join(dir, "MAIN")
	assert.NoError(t, os.WriteFile(path, exe, 0o644))
	tsv := filepath.Join(dir, "programs.tsv")
	lines := []string{
		"one\t" + dir + "\t" + path + "\t" + data + "@800f0000",
		"two\t" + dir + "\t" + path + "\t",
	}
	assert.NoError(t, os.WriteFile(tsv, []byte(strings.Join(lines, "\n")+"\n"), 0o644))

	jobs, err := ArchiveJobs(tsv, []string{"one", "two"})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(jobs))
	assert.Equal(t, []string{data + "@800f0000"}, jobs[0].Loads)
	assert.Equal(t, 0, len(jobs[1].Loads))

	prog, err := jobs[0].program()
	assert.NoError(t, err)
	last := prog.Segments[len(prog.Segments)-1]
	assert.Equal(t, uint32(0x800f0000), last.Addr)
	assert.Equal(t, []byte{1, 2, 3}, last.Data)

	if _, err := ArchiveJobs(tsv, []string{"three"}); err == nil {
		t.Error("an unknown name should fail")
	}
}

func TestParseRegion(t *testing.T) {
	r, err := ParseRegion("801f0000:64")
	assert.NoError(t, err)
	assert.Equal(t, Region{Addr: 0x801f0000, Size: 64}, r)
	r, err = ParseRegion("0x1f801814:0x4")
	assert.NoError(t, err)
	assert.Equal(t, Region{Addr: 0x1f801814, Size: 4}, r)
	_, err = ParseRegion("801f0000")
	assert.Equal(t, true, err != nil, "no size")
	_, err = ParseRegion("zz:4")
	assert.Equal(t, true, err != nil, "not hexadecimal")
}
