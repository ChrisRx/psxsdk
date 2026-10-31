// Package siocons is SIOCONS, the console Sony's Net Yaroze came with for
// talking to the console over its serial cable, rewritten from SIOCONS.EXE's
// source and the User Guide: what is typed goes to the monitor and what the
// monitor and the programs it runs send comes back, the function keys and
// batch files run SIOCONS's own local commands, which load programs and
// data, save memory, log, run shell commands and run other batch files,
// and its line editor keeps a history. It is yaroze's sio command and the
// siocons tool.
package siocons

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"go.chrisrx.dev/psx/serial"
)

// editMax is how long a line typed at one of SIOCONS's prompts can be.
const editMax = 63

// maxNest is how many batch files deep auto goes.
const maxNest = 16

// The prompt trap: the end of the monitor's prompt, which tells SIOCONS the
// monitor is waiting for a command.
const promptTrap = "  \r>>"

// The S-record traps: the monitor asking for an S-record file, which this
// SIOCONS does not send.
var srecordTraps = []string{"\r\nS-format data load start\r\n", "\r\nS-format file load start\r\n"}

// Console is SIOCONS on a port with a Net Yaroze's monitor on the other
// end. Run runs it.
type Console struct {
	port    *serial.Port
	monitor *serial.Monitor
	screen  io.Writer
	keys    <-chan Key

	// Shell runs a command line on the host, for the dos local command.
	// It is nil where there is no shell, and dos says so.
	Shell func(cmd string) error

	history history
	log     *os.File

	// promptSeen is whether the monitor has shown its prompt since it last
	// sent anything else, which a batch file's next line to the monitor
	// waits for.
	promptSeen bool
	trap       []byte // the end of what the monitor last sent, for the traps
	editMode   bool   // F8's local edit mode
	quiet      bool   // a local command is talking to the monitor itself
	hidden     []byte // what the monitor sent it
	prefix     Key    // F9 or F10, pressed before another function key

	// The batch files running, innermost last, the line read from the
	// innermost to run next, whether it waits for the prompt, and a sleep
	// one has started.
	autos      []*batch
	autoLine   string
	waitPrompt bool
	sleepFor   time.Duration
	sleepFrom  time.Time

	quit bool
}

// batch is a batch file being run.
type batch struct {
	f *os.File
	r *bufio.Reader
}

// New returns SIOCONS on port, showing what it shows on screen and taking
// keys from keys, which a nil channel leaves with none.
func New(port *serial.Port, mon *serial.Monitor, screen io.Writer, keys <-chan Key) *Console {
	c := &Console{port: port, monitor: mon, screen: screen, keys: keys}
	port.Output = display{c}
	return c
}

// display is where what the monitor sends goes: to the screen and the log,
// and through the traps. While a local command talks to the monitor, what
// the monitor answers it goes nowhere, as SIOCONS read it from the port
// itself, where only its main loop shows what comes.
type display struct{ c *Console }

func (d display) Write(p []byte) (int, error) {
	c := d.c
	if c.quiet {
		c.hidden = append(c.hidden, p...)
		return len(p), nil
	}
	c.screen.Write(p)
	if c.log != nil {
		c.log.Write(p)
	}
	c.promptSeen = false
	c.trap = append(c.trap, p...)
	if bytes.Contains(c.trap, []byte(promptTrap)) {
		c.promptSeen = true
	}
	for _, t := range srecordTraps {
		if bytes.Contains(c.trap, []byte(t)) {
			c.puts("\r\nS-records are not supported\r\n")
			c.port.Write([]byte{3})
		}
	}
	if n := len(c.trap) - 64; n > 0 {
		c.trap = c.trap[n:]
	}
	if c.promptSeen {
		c.trap = c.trap[:0]
	}
	return len(p), nil
}

// hush starts a local command's talk with the monitor, which unhush ends,
// showing what the monitor sent after the last of after in it, if after is
// not empty: what SIOCONS left on the port for its main loop to show, such
// as the prompt after a binary transfer's "end binary".
func (c *Console) hush() {
	c.quiet = true
	c.hidden = c.hidden[:0]
}

func (c *Console) unhush(after string) {
	c.quiet = false
	if after == "" {
		return
	}
	if i := bytes.LastIndex(c.hidden, []byte(after)); i >= 0 {
		display{c}.Write(c.hidden[i+len(after):])
	}
}

func (c *Console) puts(s string) { io.WriteString(c.screen, s) }

func (c *Console) printf(format string, args ...any) { fmt.Fprintf(c.screen, format, args...) }

// key waits for a key. Escape stops SIOCONS, as does the keyboard closing.
func (c *Console) key() (Key, error) {
	k, ok := <-c.keys
	if !ok || k == keyEscape {
		return 0, errTerminate
	}
	return k, nil
}

// pollKey returns a key if one has been pressed, or 0.
func (c *Console) pollKey() (Key, error) {
	select {
	case k, ok := <-c.keys:
		if !ok || k == keyEscape {
			return 0, errTerminate
		}
		return k, nil
	default:
		return 0, nil
	}
}

// LoadHistory reads the line editor's history, and SaveHistory writes it.
func (c *Console) LoadHistory(r io.Reader) { c.history.load(r) }
func (c *Console) SaveHistory(w io.Writer) { c.history.dump(w) }

// Run runs SIOCONS until it is told to quit, or Escape is pressed, which
// it reports as an error, or the port fails. A batch file named by auto,
// if not empty, runs first.
func (c *Console) Run(ctx context.Context, auto string) error {
	defer c.closeLog()
	if auto != "" {
		c.autoAuto(auto)
	}
	for !c.quit {
		if err := ctx.Err(); err != nil {
			return err
		}
		var k Key
		var err error
		switch {
		case c.sleepFor > 0:
			if time.Since(c.sleepFrom) > c.sleepFor {
				c.sleepFor = 0
				c.nextAuto()
			} else {
				_, err = c.pollKey()
			}
		case len(c.autos) > 0 && !c.waitPrompt:
			if err = c.execAuto(ctx); err == nil && !c.quit && c.sleepFor == 0 {
				wait := c.waitPrompt
				c.nextAuto()
				c.waitPrompt = c.waitPrompt || wait
			}
		case c.promptSeen && len(c.autos) > 0 && c.waitPrompt:
			if err = c.execAuto(ctx); err == nil {
				c.nextAuto()
			}
		case c.promptSeen && c.editMode:
			c.puts("\r")
			var line string
			line, k, err = c.input("%s>>", editMax, true)
			if err == nil && k == 0 {
				c.promptSeen = false
				c.puts("\r\033[K>>")
				c.port.Write([]byte(line + "\r"))
			}
		default:
			k, err = c.pollKey()
		}
		if err != nil {
			return c.terminate(err)
		}
		if k != 0 {
			if err := c.keyPressed(ctx, k); err != nil {
				return c.terminate(err)
			}
		}
		if c.quit {
			break
		}
		if err := c.port.Poll(); err != nil {
			return err
		}
	}
	return nil
}

// terminate stops SIOCONS for err, saying so if it is the user's doing.
func (c *Console) terminate(err error) error {
	if err == errTerminate {
		c.puts("\r\nTerminate by user!\r\n")
	}
	return err
}

// keyPressed handles a key pressed at the console: a byte goes to the
// monitor, F9 and F10 wait for the function key they prefix, and other
// function keys run local commands.
func (c *Console) keyPressed(ctx context.Context, k Key) error {
	switch {
	case !k.function():
		c.prefix = 0
		c.port.Write([]byte{byte(k)})
	case c.prefix == 0 && (k == KeyF9 || k == KeyF10):
		c.prefix = k
	default:
		prefix := c.prefix
		c.prefix = 0
		return c.kick(ctx, k, prefix)
	}
	return nil
}

// A command is one of SIOCONS's local commands: its function keys, on
// their own and after F9 and F10, its name in a batch file, and what it
// does from each.
type command struct {
	keys  [3]Key
	name  string
	key   func(c *Console, ctx context.Context) error
	batch func(c *Console, ctx context.Context, args string) error
}

// commands is SIOCONS's table, in its order: a name is matched by the
// first entry it starts with.
var commands []command

func init() {
	commands = []command{
		{[3]Key{0, KeyF1, 0}, "helpf9", (*Console).helpF9, (*Console).helpF9Auto},
		{[3]Key{0, 0, KeyF1}, "helpf10", (*Console).helpF10, (*Console).helpF10Auto},
		{[3]Key{KeyF1, 0, 0}, "help", (*Console).help, (*Console).helpAuto},
		{[3]Key{KeyF2, KeyF2, 0}, "dos", (*Console).dos, (*Console).dosAuto},
		{[3]Key{KeyF2, KeyF2, 0}, "sh", (*Console).dos, (*Console).dosAuto},
		{[3]Key{KeyF2, KeyF2, 0}, "!", (*Console).dos, (*Console).dosAuto},
		{[3]Key{0, 0, KeyF2}, "quit", (*Console).quitKey, (*Console).quitAuto},
		{[3]Key{}, "auto-again", nil, (*Console).autoAgain},
		{[3]Key{KeyF3, KeyF3, KeyF3}, "auto", (*Console).autoKey, (*Console).autoCmd},
		{[3]Key{KeyF4, 0, 0}, "load", (*Console).loadKey, (*Console).load},
		{[3]Key{KeyF4, 0, 0}, "lds", (*Console).loadKey, (*Console).load},
		{[3]Key{0, KeyF4, 0}, "dload", (*Console).dloadKey, (*Console).dload},
		{[3]Key{0, KeyF4, 0}, "ld", (*Console).dloadKey, (*Console).dload},
		{[3]Key{0, 0, KeyF4}, "dsave", (*Console).dsaveKey, (*Console).dsave},
		{[3]Key{KeyF5, 0, 0}, "log", (*Console).logKey, (*Console).logCmd},
		{[3]Key{0, KeyF5, KeyF5}, "log", (*Console).logOff, (*Console).logCmd},
		{[3]Key{KeyF8, KeyF8, KeyF8}, "edittgl", (*Console).editToggleKey, (*Console).editToggle},
		{[3]Key{}, "beep", (*Console).beepKey, (*Console).beep},
		{[3]Key{}, "echo", nil, (*Console).echo},
		{[3]Key{}, "pause", nil, (*Console).pause},
		{[3]Key{}, "sleep", nil, (*Console).sleep},
		{[3]Key{}, "wait-prompt", nil, (*Console).waitForPrompt},
	}
}

// kick runs the local command a function key, after prefix if not 0, is
// bound to.
func (c *Console) kick(ctx context.Context, k, prefix Key) error {
	i := 0
	switch prefix {
	case KeyF9:
		i = 1
	case KeyF10:
		i = 2
	}
	for _, cmd := range commands {
		if cmd.keys[i] == k && cmd.key != nil {
			c.puts("\n\r")
			return cmd.key(c, ctx)
		}
	}
	c.puts("?\n\r")
	return nil
}

// localCommand runs a local command from a batch file, by the first name
// in the table its text starts with, whatever the case, the rest being its
// arguments. It reports false if the command failed, or there is none,
// which stops the batch files.
func (c *Console) localCommand(ctx context.Context, text string) (bool, error) {
	text = strings.TrimLeft(text, " \t")
	for _, cmd := range commands {
		if len(text) >= len(cmd.name) && strings.EqualFold(text[:len(cmd.name)], cmd.name) {
			err := cmd.batch(c, ctx, strings.TrimLeft(text[len(cmd.name):], " \t"))
			if err == errFailed {
				return false, nil
			}
			return err == nil, err
		}
	}
	if text != "" {
		return false, c.help(ctx)
	}
	c.puts("?\n\r")
	return false, nil
}

// errFailed is a local command that did not do what it was asked, having
// said why.
var errFailed = fmt.Errorf("failed")

// logLocal records a local command run from a function key in the log, as
// a batch file's line, so that a log can be run again.
func (c *Console) logLocal(cmd, args string) {
	if c.log != nil {
		fmt.Fprintf(c.log, "local %s %s\n", cmd, args)
	}
}

// ask reads the line a local command run from a function key asks for, at
// its prompt.
func (c *Console) ask(name string) (string, error) {
	line, _, err := c.input("\033[32m"+name+"%s:\033[0m ", editMax, false)
	return line, err
}
