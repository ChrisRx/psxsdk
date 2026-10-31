package siocons

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// maxHistory is how many lines the history keeps.
const maxHistory = 50

// history is the lines typed at SIOCONS's prompts, oldest first, and how
// many lines have been put in it in all, which numbers the prompts.
type history struct {
	lines []string
	count int
}

// put adds s at the end, moving it there if it is already in.
func (h *history) put(s string) {
	for i, l := range h.lines {
		if l == s {
			h.lines = append(h.lines[:i], h.lines[i+1:]...)
			break
		}
	}
	h.lines = append(h.lines, s)
	h.count++
	if len(h.lines) > maxHistory {
		h.lines = h.lines[1:]
	}
}

// load reads lines into the history, as SIOCONS reads its history file.
func (h *history) load(r io.Reader) {
	s := bufio.NewScanner(r)
	for s.Scan() {
		if l := strings.TrimRightFunc(s.Text(), func(r rune) bool { return r < ' ' }); l != "" {
			h.put(l)
		}
	}
}

// dump writes the history, a line at a time, oldest first.
func (h *history) dump(w io.Writer) {
	for _, l := range h.lines {
		fmt.Fprintln(w, l)
	}
}

// wrap brings a history position, counted back from the end as -1 for the
// last line, round to the other end when it goes past one.
func (h *history) wrap(offset int) int {
	n := len(h.lines)
	if offset < -n {
		offset += n
	}
	if offset >= 0 {
		offset -= n
	}
	return offset
}

// at returns the line at a history position.
func (h *history) at(offset int) string { return h.lines[len(h.lines)+offset] }

// search returns the position of the nearest line from from on, going
// back if step is -1 and forward if it is 1, that starts with prefix, and
// whether there is one. It does not go round.
func (h *history) search(prefix string, from, step int) (int, bool) {
	if strings.TrimSpace(prefix) == "" {
		return 0, false
	}
	i := len(h.lines) + h.wrap(from)
	for ; i >= 0 && i < len(h.lines); i += step {
		if strings.HasPrefix(h.lines[i], prefix) {
			return i - len(h.lines), true
		}
	}
	return 0, false
}

// Editing keys, as SIOCONS's line editor takes them.
const (
	ctrlA = 'A' - '@' // to the start of the line
	ctrlD = 'D' - '@' // right
	ctrlE = 'E' - '@' // back through the history
	ctrlF = 'F' - '@' // to the end of the line
	ctrlG = 'G' - '@' // delete under the cursor
	ctrlH = 'H' - '@' // delete before the cursor
	ctrlI = 'I' - '@' // search back through the history
	ctrlJ = 'J' - '@' // search forward through the history
	ctrlK = 'K' - '@' // delete to the end of the line
	ctrlS = 'S' - '@' // left
	ctrlX = 'X' - '@' // forward through the history
	ctrlY = 'Y' - '@' // delete the line, and leave the history
)

// errTerminate is the Escape key, which stops SIOCONS wherever it is
// pressed, as SIOCONS's getkey does. The User Guide has Escape push the
// line into the history in the line editor, but the key never got there.
var errTerminate = fmt.Errorf("terminated by the user")

// input reads a line at a prompt, which has %s for the history position,
// with SIOCONS's line editor and history, as its input_with_history does.
// A line of "his" lists the history instead. With retill, a function key
// ends the input too, and is returned instead of a line.
func (c *Console) input(prompt string, max int, retill bool) (string, Key, error) {
	offset := 0
	search := ""
	buf := ""
	for {
		position := fmt.Sprintf("[%d]", c.history.count+1)
		if offset != 0 {
			position = fmt.Sprintf("[%d/%d]", c.history.count+offset+1, c.history.count)
		}
		fmt.Fprintf(c.screen, prompt, position)
		var end Key
		var err error
		buf, end, err = c.edit(buf, max, retill)
		if err != nil {
			return "", 0, err
		}
		switch {
		case end == ctrlY:
			c.puts("\r\033[K")
			buf, offset = "", 0
		case end == ctrlE || end == ctrlX:
			if len(c.history.lines) > 0 {
				if end == ctrlE {
					offset = c.history.wrap(offset - 1)
				} else {
					offset = c.history.wrap(offset + 1)
				}
				buf = c.history.at(offset)
			} else {
				buf, offset = "", 0
			}
			c.puts("\r\033[K")
		case end == ctrlI || end == ctrlJ:
			if len(c.history.lines) > 0 {
				if offset == 0 {
					search = buf
				}
				step := -1
				if end == ctrlJ {
					step = 1
				}
				if o, ok := c.history.search(search, offset+step, step); ok {
					offset, buf = o, c.history.at(o)
				}
			}
			c.puts("\r\033[K")
		case end == '\r':
			c.puts("\r\n")
			if (buf == "his" || buf == "HIS") && len(c.history.lines) > 0 {
				first := c.history.count - len(c.history.lines) + 1
				for i, l := range c.history.lines {
					fmt.Fprintf(c.screen, "%4d: %s\n\r", first+i, l)
				}
				buf, offset = "", 0
				continue
			}
			if strings.TrimSpace(buf) != "" {
				c.history.put(buf)
			}
			return buf, 0, nil
		case end.function():
			if strings.TrimSpace(buf) != "" {
				c.history.put(buf)
			}
			return "", end, nil
		}
	}
}

// edit edits buf, which is shown after the prompt with the cursor at its
// end, until a key that ends the editing, which it returns with the line.
func (c *Console) edit(buf string, max int, retill bool) (string, Key, error) {
	line := []byte(buf)
	cur := len(line)
	c.puts(buf)
	back := func(n int) { c.puts(strings.Repeat("\b", n)) }
	// deleteAt takes the character at i out, redrawing the rest of the
	// line and putting the cursor back at i.
	deleteAt := func(i int) {
		line = append(line[:i], line[i+1:]...)
		c.puts(string(line[i:]) + " ")
		back(len(line) - i + 1)
	}
	for {
		k, err := c.key()
		if err != nil {
			return "", 0, err
		}
		switch k {
		case KeyUp:
			k = ctrlE
		case KeyDown:
			k = ctrlX
		case KeyLeft:
			k = ctrlS
		case KeyRight:
			k = ctrlD
		case KeyDelete:
			k = ctrlG
		case KeyHome:
			k = ctrlA
		case KeyEnd:
			k = ctrlF
		case 0x7f:
			k = ctrlH
		case keyCtrlC:
			k = '\r'
		}
		switch {
		case k == ctrlA:
			back(cur)
			cur = 0
		case k == ctrlF:
			c.puts(string(line[cur:]))
			cur = len(line)
		case k == ctrlS:
			if cur > 0 {
				back(1)
				cur--
			}
		case k == ctrlD:
			if cur < len(line) {
				c.puts(string(line[cur]))
				cur++
			}
		case k == ctrlH:
			if cur > 0 {
				cur--
				back(1)
				deleteAt(cur)
			}
		case k == ctrlG:
			if cur < len(line) {
				deleteAt(cur)
			}
		case k == ctrlK || k == ctrlY:
			if k == ctrlY {
				back(cur)
				cur = 0
			}
			c.puts(strings.Repeat(" ", len(line)-cur))
			back(len(line) - cur)
			line = line[:cur]
			if k == ctrlY {
				return "", ctrlY, nil
			}
		case k == '\r' || k == ctrlE || k == ctrlX || k == ctrlI || k == ctrlJ:
			return string(line), k, nil
		case k.function():
			if retill {
				return string(line), k, nil
			}
		case k >= ' ' && k < 0x7f && len(line) < max:
			line = append(line[:cur], append([]byte{byte(k)}, line[cur:]...)...)
			c.puts(string(line[cur:]))
			cur++
			back(len(line) - cur)
		}
	}
}
