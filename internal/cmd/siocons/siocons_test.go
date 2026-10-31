package siocons

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"go.chrisrx.dev/x/assert"
)

func TestEscapeSequence(t *testing.T) {
	for seq, want := range map[string]Key{
		"\x1bOP":     KeyF1,
		"\x1bOS":     KeyF4,
		"\x1b[15~":   KeyF5,
		"\x1b[21~":   KeyF10,
		"\x1b[24~":   KeyF12,
		"\x1b[[A":    KeyF1,
		"\x1b[A":     KeyUp,
		"\x1bOD":     KeyLeft,
		"\x1b[3~":    KeyDelete,
		"\x1b[1;5P":  0,
		"\x1b[15;2~": KeyF5,
	} {
		k, done := escapeSequence([]byte(seq))
		assert.Equal(t, true, done, seq)
		assert.Equal(t, want, k, seq)
	}
	_, done := escapeSequence([]byte("\x1b[1"))
	assert.Equal(t, false, done, "a sequence with its parameters still coming")
}

// keysOf runs readKeys over a terminal that sends parts, a pause between
// each, and returns the keys.
func keysOf(parts ...string) []Key {
	r, w := io.Pipe()
	go func() {
		for _, p := range parts {
			w.Write([]byte(p))
			time.Sleep(2 * escWait)
		}
		w.Close()
	}()
	ch := make(chan Key, 64)
	go readKeys(r, ch)
	var keys []Key
	for k := range ch {
		keys = append(keys, k)
	}
	return keys
}

func TestReadKeys(t *testing.T) {
	assert.Equal(t, []Key{'a', KeyF9, KeyF4, 'b'}, keysOf("a\x1b[20~\x1bOSb"))
	assert.Equal(t, []Key{keyEscape, 'x'}, keysOf("\x1b", "x"), "an escape on its own")
}

func TestHistory(t *testing.T) {
	var h history
	for _, l := range []string{"one", "two", "three", "two"} {
		h.put(l)
	}
	assert.Equal(t, []string{"one", "three", "two"}, h.lines)
	assert.Equal(t, 4, h.count, "a line put in again still counts")
	assert.Equal(t, -1, h.wrap(-4), "back past the first goes round to the last")
	assert.Equal(t, -3, h.wrap(0), "forward past the last goes round to the first")
	o, ok := h.search("t", -1, -1)
	assert.Equal(t, true, ok)
	assert.Equal(t, "two", h.at(o))
	o, ok = h.search("t", o-1, -1)
	assert.Equal(t, true, ok)
	assert.Equal(t, "three", h.at(o))
	_, ok = h.search("t", o-1, -1)
	assert.Equal(t, false, ok, "the search does not go round")
	for i := range 60 {
		h.put(strings.Repeat("x", i+1))
	}
	assert.Equal(t, maxHistory, len(h.lines))
}

// editor returns a console that takes keys and shows what it shows.
func editor(keys ...Key) (*Console, *bytes.Buffer) {
	ch := make(chan Key, len(keys))
	for _, k := range keys {
		ch <- k
	}
	close(ch)
	var screen bytes.Buffer
	return &Console{screen: &screen, keys: ch}, &screen
}

func typed(s string) []Key {
	var keys []Key
	for _, c := range []byte(s) {
		keys = append(keys, Key(c))
	}
	return keys
}

func TestInput(t *testing.T) {
	keys := append(typed("abc"), KeyLeft, KeyLeft, 'X', KeyEnd, 0x7f, '\r')
	c, screen := editor(keys...)
	line, k, err := c.input("Load%s: ", editMax, false)
	assert.NoError(t, err)
	assert.Equal(t, Key(0), k)
	assert.Equal(t, "aXb", line)
	assert.Equal(t, true, strings.HasPrefix(screen.String(), "Load[1]: "), screen.String())
	assert.Equal(t, []string{"aXb"}, c.history.lines)

	keys = append(append(typed("first\r"), typed("second\r")...), KeyUp, KeyUp, '\r')
	c, screen = editor(keys...)
	c.input("%s: ", editMax, false)
	c.input("%s: ", editMax, false)
	line, _, _ = c.input("%s: ", editMax, false)
	assert.Equal(t, "first", line, "two lines back")
	assert.Equal(t, true, strings.Contains(screen.String(), "[1/2]: first"), screen.String())

	c, _ = editor(append(typed("half"), ctrlY, 'z', '\r')...)
	line, _, _ = c.input("%s: ", editMax, false)
	assert.Equal(t, "z", line, "Control-Y takes the line")

	c, _ = editor(append(typed("abc"), KeyF4)...)
	_, k, _ = c.input("%s>>", editMax, true)
	assert.Equal(t, KeyF4, k, "a function key ends the line in local edit mode")

	c, _ = editor(typed("ab")...)
	c.keys = func() chan Key { ch := make(chan Key, 3); ch <- 'a'; ch <- keyEscape; close(ch); return ch }()
	_, _, err = c.input("%s: ", editMax, false)
	assert.Error(t, errTerminate, err, "Escape stops SIOCONS")
}
