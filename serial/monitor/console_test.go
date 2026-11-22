package main

import (
	"strings"
	"testing"

	"go.chrisrx.dev/x/assert"
)

// screen returns the log as it would be drawn, a line a row, with the
// spaces at the ends left off.
func (c *console) screen() []string {
	var rows []string
	for r := range conLogRows {
		l := c.line(r)
		s := strings.TrimRight(strings.Map(func(r rune) rune {
			if r == 0 {
				return ' '
			}
			return r
		}, string(l)), " ")
		rows = append(rows, s)
	}
	return rows
}

func (c *console) write(s string) {
	for i := range len(s) {
		c.put(s[i])
	}
}

func TestConsoleText(t *testing.T) {
	var c console
	c.write("\r\nConnected CIP ver2.0\r\nCommunication baud rate 115200 bps\r\nOK\r\n")
	// The prompt clears its line with more spaces than the console is wide.
	c.write("\r                                                    \r>>")
	c.write("dw 80010000 4\r\n")
	c.write("80010000:3c1d801f 37bdfff0 0c004010 00000000 \r\n")
	rows := c.screen()
	assert.Equal(t, []string{
		"Connected CIP ver2.0",
		"Communication baud rate 115200 bps",
		"OK",
		">>dw 80010000 4",
		// A line longer than the console goes on the next.
		"80010000:3c1d801f 37bdfff0 0c004010",
		"00000000",
		// The cursor's line.
		"",
	}, rows[len(rows)-7:], strings.Join(rows, "\n"))
}

func TestConsoleScroll(t *testing.T) {
	var c console
	for i := range conLogRows + 5 {
		c.write("line ")
		c.put(byte('a' + i%26))
		c.write("\r\n")
	}
	rows := c.screen()
	// The newest line is empty, the cursor's, and the one above it the last
	// written.
	assert.Equal(t, "", rows[conLogRows-1])
	assert.Equal(t, "line "+string(rune('a'+(conLogRows+4)%26)), rows[conLogRows-2])
	assert.Equal(t, conLogRows+5, c.scrolls)
}

func TestConsoleBackspaceAndBinary(t *testing.T) {
	var c console
	c.write(">>dx\b \bw\r\n")
	c.binary = true
	c.write("Y\x00\xffN")
	c.binary = false
	c.write("end binary\r\n")
	rows := c.screen()
	assert.Equal(t, []string{">>dw", "end binary", ""}, rows[conLogRows-3:])
}

func TestConsoleIdle(t *testing.T) {
	var c console
	line := uint32(0)
	advance := func(lines uint32, busy bool) {
		// The counter is read often, as the monitor does while it waits.
		for lines > 0 {
			n := min(lines, 100)
			line += n
			activity = busy
			c.tick(uint16(line))
			lines -= n
		}
	}
	// Busy, the console moves after a minute.
	advance(shiftLines-200, true)
	assert.Equal(t, 0, c.shift)
	advance(200, true)
	assert.Equal(t, 1, c.shift, "moved after a minute")
	assert.Equal(t, false, c.saver)

	// Idle, the logo comes.
	advance(idleLines-1, false)
	assert.Equal(t, false, c.saver)
	advance(100, false)
	assert.Equal(t, true, c.saver, "the logo once idle")
	c.full = false
	advance(1, true)
	assert.Equal(t, false, c.saver, "a byte brings the console back")
	assert.Equal(t, true, c.full)
}

func TestLogoBounce(t *testing.T) {
	var l logo
	l.start(0)
	clock := uint32(0)
	color := l.color
	bounces := 0
	for range 2000 {
		clock += logoLines
		assert.Equal(t, true, l.step(clock))
		assert.Equal(t, true, l.x >= logoMargin && l.x <= 320-logoMargin-logoW, l.x)
		assert.Equal(t, true, l.y >= logoMargin && l.y <= conHeight-logoMargin-logoH, l.y)
		if l.color != color {
			color = l.color
			bounces++
		}
	}
	assert.Equal(t, true, bounces > 4, bounces)
	assert.Equal(t, false, l.step(clock), "no step before its time")

}

func TestConsoleRestarted(t *testing.T) {
	var c console
	assert.Equal(t, false, c.restarted(), "the first start")
	c.write("\r\nOK\r\n>>go\r\nhello")
	assert.Equal(t, true, c.restarted())
	rows := c.screen()
	assert.Equal(t, []string{">>go", "hello", divider, ""}, rows[conLogRows-4:])
}

func TestStatusText(t *testing.T) {
	line.baud = 115200
	var c console
	var a, b [conCols]byte
	text := func(l [conCols]byte) string {
		return strings.TrimRight(strings.ReplaceAll(string(l[:]), "\x00", " "), " ")
	}
	c.statusText(&a, &b)
	assert.Equal(t, "CLEAN MONITOR             115200 bps", text(a))
	assert.Equal(t, "ready", text(b))

	c.startUpload(stateReceive, 0x80140000, 36*1024)
	c.block(2048)
	c.statusText(&a, &b)
	assert.Equal(t, "receiving 80140000 2048/36K", text(b))
	assert.Equal(t, 36*2048/(36*1024), c.progress(36))
	c.retry()
	c.statusText(&a, &b)
	assert.Equal(t, "receiving 80140000 2048/36K 1 again", text(b))

	c.block(34 * 1024)
	c.clock += 32 * tenthLines
	c.endUpload()
	c.statusText(&a, &b)
	assert.Equal(t, "ready, loaded 36K in 3.2s", text(b))
}

func TestConsoleHold(t *testing.T) {
	var c console
	c.hold()
	activity = true // the monitor's own prompt after the command
	c.tick(1)
	assert.Equal(t, true, c.held, "only the host's bytes end the hold")
	received, activity = true, true
	c.tick(2)
	assert.Equal(t, false, c.held)
	assert.Equal(t, true, c.full && c.remode)
}

func TestLogoPace(t *testing.T) {
	var l logo
	l.start(0)
	flip, next := l.pace(100)
	assert.Equal(t, [2]bool{false, true}, [2]bool{flip, next}, "the first frame is drawn, not shown")
	x := l.x
	flip, next = l.pace(101)
	assert.Equal(t, [2]bool{false, false}, [2]bool{flip, next}, "a blank later, nothing")
	flip, next = l.pace(102)
	assert.Equal(t, [2]bool{true, true}, [2]bool{flip, next}, "two later, shown and the next drawn")
	assert.Equal(t, 1, l.x-x, "a pixel a flip")
	flip, _ = l.pace(102)
	assert.Equal(t, false, flip, "once a blank")
}
