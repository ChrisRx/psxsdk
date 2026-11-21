package gdbserver

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
)

// An event is what a debugger sends: a packet's payload, or the Control-C
// byte that asks for the program to be stopped.
type event struct {
	packet    []byte
	interrupt bool
}

// conn is a connection to a debugger: packets framed as $payload#sum, each
// acknowledged with a + until the debugger turns that off.
type conn struct {
	r     *bufio.Reader
	mu    sync.Mutex // guards w
	w     io.Writer
	noAck atomic.Bool
	logf  func(format string, args ...any)
}

func newConn(rw io.ReadWriter, logf func(string, ...any)) *conn {
	return &conn{
		r:    bufio.NewReader(rw),
		w:    rw,
		logf: logf,
	}
}

// read returns the next event. Acknowledgments are dropped: a reply is
// never sent twice, a TCP connection losing nothing.
func (c *conn) read() (event, error) {
	for {
		b, err := c.r.ReadByte()
		if err != nil {
			return event{}, err
		}
		switch b {
		case 0x03:
			c.logf("<- ^C")
			return event{interrupt: true}, nil
		case '$':
		default:
			continue
		}
		payload, err := c.r.ReadBytes('#')
		if err != nil {
			return event{}, err
		}
		payload = payload[:len(payload)-1]
		var sum [2]byte
		if _, err := io.ReadFull(c.r, sum[:]); err != nil {
			return event{}, err
		}
		if !c.noAck.Load() {
			var want [1]byte
			if _, err := hex.Decode(want[:], sum[:]); err != nil || want[0] != checksum(payload) {
				c.raw([]byte{'-'})
				continue
			}
			c.raw([]byte{'+'})
		}
		c.logf("<- %s", printable(payload))
		return event{packet: payload}, nil
	}
}

func (c *conn) raw(b []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, err := c.w.Write(b)
	return err
}

// write sends a packet.
func (c *conn) write(payload []byte) error {
	c.logf("-> %s", printable(payload))
	b := make([]byte, 0, len(payload)+4)
	b = append(b, '$')
	b = append(b, payload...)
	b = fmt.Appendf(b, "#%02x", checksum(payload))
	return c.raw(b)
}

func checksum(b []byte) byte {
	var sum byte
	for _, c := range b {
		sum += c
	}
	return sum
}

// escape makes binary data safe inside a packet: the framing's own bytes,
// and the * of run-length encoding, each become } and the byte with bit 5
// turned over.
func escape(data []byte) []byte {
	out := make([]byte, 0, len(data))
	for _, b := range data {
		switch b {
		case '$', '#', '}', '*':
			out = append(out, '}', b^0x20)
		default:
			out = append(out, b)
		}
	}
	return out
}

// unescape undoes escape.
func unescape(data []byte) []byte {
	out := make([]byte, 0, len(data))
	for i := 0; i < len(data); i++ {
		b := data[i]
		if b == '}' && i+1 < len(data) {
			i++
			b = data[i] ^ 0x20
		}
		out = append(out, b)
	}
	return out
}

// printable is a packet for the log, cut short, with a binary one's bytes
// that are not text shown as dots.
func printable(b []byte) string {
	const most = 120
	more := ""
	if len(b) > most {
		b, more = b[:most], "..."
	}
	return string(bytes.Map(func(r rune) rune {
		if r < 0x20 || r > 0x7e {
			return '.'
		}
		return r
	}, b)) + more
}
