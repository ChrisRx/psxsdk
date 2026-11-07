package serial

import (
	"bytes"
	"testing"
	"time"

	"go.chrisrx.dev/x/assert"
)

// loopLine is an adapter with its lines joined. A broken one can drop or
// change bytes, or leave CTS stuck where it is.
type loopLine struct {
	buf      bytes.Buffer
	rts, cts bool
	stuck    bool // CTS ignores RTS
	flip     int  // change the byte at this offset of a write, if not 0
}

func (l *loopLine) Read(p []byte) (int, error) {
	if l.buf.Len() == 0 {
		return 0, nil
	}
	return l.buf.Read(p)
}

func (l *loopLine) Write(p []byte) (int, error) {
	data := bytes.Clone(p)
	if l.flip > 0 && l.flip < len(data) {
		data[l.flip] ^= 0xff
	}
	return l.buf.Write(data)
}

func (l *loopLine) CTS() (bool, error) { return l.cts, nil }

func (l *loopLine) SetRTS(on bool) error {
	l.rts = on
	if !l.stuck {
		l.cts = on
	}
	return nil
}

func loopback(t *testing.T, l *loopLine) (Loopback, error) {
	p := NewPort(l)
	p.Timeout = 50 * time.Millisecond
	return RunLoopback(t.Context(), p)
}

func TestLoopback(t *testing.T) {
	got, err := loopback(t, &loopLine{rts: true, cts: true})
	assert.NoError(t, err)
	assert.Equal(t, true, got.OK())
	assert.Equal(t, 4096, got.Received)
	assert.Equal(t, true, got.BlockTime > 0)
}

func TestLoopbackWrongBytes(t *testing.T) {
	got, err := loopback(t, &loopLine{rts: true, cts: true, flip: 7})
	assert.NoError(t, err)
	assert.Equal(t, 1, got.Wrong)
	assert.Equal(t, false, got.OK())
}

// A stuck CTS is the adapter's RTS or CTS not being wired through.
func TestLoopbackCTSStuck(t *testing.T) {
	got, err := loopback(t, &loopLine{rts: true, cts: true, stuck: true})
	if err == nil {
		t.Error("a stuck CTS should fail")
	}
	assert.Equal(t, false, got.Follows)
	assert.Equal(t, false, got.OK())
}
