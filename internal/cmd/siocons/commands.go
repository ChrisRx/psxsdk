package siocons

import (
	"bufio"
	"context"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"go.chrisrx.dev/psx/serial"
)

// The help SIOCONS shows: the function keys at a glance, then, asked for
// more, each key and the batch file command that does the same.
const (
	shortHelp = "\033[36m" +
		"       [F1]  [F2]  [F3]  [F4]    [F5]  [F6]  [F7]  [F8]    [F9]  [F10]\r\n" +
		"       help  dos   auto  load    log               edit     -      -  \r\n" +
		"[F9]+  help              dload  logstop                               \r\n" +
		"[F10]+ help  quit        dsave \r\n" +
		"\033[0m"
	longHelp = "\033[36m" +
		"   Key functions                           Local command for auto\r\n" +
		"-------------------------------------- | --------------------------------------\r\n" +
		"  F1      - display this message       |  help\r\n" +
		"  F2      - execute dos command        |  dos{sh,!} <dos command strings>\r\n" +
		"  F10 F2  - exit this program          |  quit\r\n" +
		"  F3      - auto operation             |  auto <filename>\r\n" +
		"  F4      - formated file down load    |  load <filename>\r\n" +
		"  F9 F4   - binary file down load      |  dload <filename> <address>\r\n" +
		"  F10 F4  - binary file up load        |  dsave <filename> <address> <size>\r\n" +
		"  F5      - operation logging start    |  log <filename>\r\n" +
		"  F9 F5   - logging stop               |  log\r\n" +
		"  F8      - local edit mode toggle     |  \r\n" +
		"                                       |  auto-again\r\n" +
		"                                       |  wait-prompt\r\n" +
		"                                       |  sleep <sec>\r\n" +
		"                                       |  pause [<strings>]\r\n" +
		"                                       |  echo <strings>\r\n" +
		"                                       |  beep\r\n" +
		"\033[0m"
	f9Help = "\033[36m" +
		"    F9 F4   - binary file down load\r\n" +
		"    F9 F5   - logging stop\r\n" +
		"\033[0m"
	f10Help = "\033[36m" +
		"    F10 F2  - exit this program\r\n" +
		"    F10 F4  - binary file up load\r\n" +
		"\033[0m"
)

func (c *Console) help(ctx context.Context) error {
	c.puts(shortHelp)
	c.puts("\033[32mmore ?(y/N)\033[0m ")
	k, err := c.key()
	if err != nil {
		return err
	}
	if k == 'y' || k == 'Y' {
		c.puts("\r\033[K\n")
		c.puts(longHelp)
	} else if !k.function() {
		c.printf("%c\r\n", rune(k))
	} else {
		c.puts("\r\n")
	}
	return nil
}

func (c *Console) helpAuto(ctx context.Context, args string) error {
	c.puts(longHelp)
	return nil
}

func (c *Console) helpF9(ctx context.Context) error { c.puts(f9Help); return nil }
func (c *Console) helpF9Auto(ctx context.Context, args string) error {
	c.puts(f9Help)
	return nil
}

func (c *Console) helpF10(ctx context.Context) error { c.puts(f10Help); return nil }
func (c *Console) helpF10Auto(ctx context.Context, args string) error {
	c.puts(f10Help)
	return nil
}

// dos runs a command on the host, which SIOCONS ran under DOS.
func (c *Console) dos(ctx context.Context) error {
	line, _, err := c.input("\033[32mDos%s#\033[0m ", editMax, false)
	if err != nil || line == "" {
		return err
	}
	c.logLocal("dos", line)
	return c.dosAuto(ctx, line)
}

func (c *Console) dosAuto(ctx context.Context, args string) error {
	if c.Shell == nil {
		c.puts("no shell to run commands with\r\n")
		return nil
	}
	if err := c.Shell(strings.TrimSpace(args)); err != nil {
		c.printf("%v\r\n", err)
	}
	return nil
}

func (c *Console) quitKey(ctx context.Context) error {
	c.logLocal("quit", "")
	c.quit = true
	return nil
}

func (c *Console) quitAuto(ctx context.Context, args string) error {
	c.quit = true
	return nil
}

// load uploads programs, each a PS-X EXE, an ECOFF executable or an ELF,
// and sets the registers it starts with, as SIOCONS's load did: after -s,
// saying nothing, and otherwise what goes where.
func (c *Console) loadKey(ctx context.Context) error {
	line, err := c.ask("Load")
	if err != nil {
		return err
	}
	c.logLocal("load", line)
	return c.load(ctx, line)
}

func (c *Console) load(ctx context.Context, args string) error {
	c.hush()
	defer c.unhush("")
	verbose := true
	files := strings.Fields(args)
	if len(files) > 0 && (files[0] == "-s" || files[0] == "-v") {
		verbose = files[0] == "-v"
		files = files[1:]
	}
	for _, file := range files {
		if _, err := os.Stat(file); err != nil {
			c.printf(" \"%s\" not found\a\n\r", file)
			return errFailed
		}
		prog, err := serial.OpenProgram(file)
		if err != nil {
			c.printf(" \"%s\" unknown format\a\n\r", file)
			return errFailed
		}
		if err := c.transfer(ctx, file, verbose, prog.Segments, func(ctx context.Context) error {
			return c.monitor.WriteSegments(ctx, prog.Segments...)
		}); err != nil {
			return err
		}
		if verbose {
			c.printf("  PC=%08x, GP=%08x, SP=%08x\r\n", prog.PC, prog.GP, prog.SP)
		}
		if err := c.monitor.SetRegister(ctx, "epc", prog.PC); err != nil {
			return err
		}
		if prog.GP != 0 {
			if err := c.monitor.SetRegister(ctx, "gp", prog.GP); err != nil {
				return err
			}
		}
		if err := c.monitor.SetRegister(ctx, "sp", prog.SP); err != nil {
			return err
		}
		// A carriage return brings the prompt up again, for a batch
		// file's next line to see.
		c.port.Write([]byte("\r"))
	}
	return nil
}

// transfer runs a binary transfer of segs, from or to file, showing each
// segment's addresses and how much of it has gone, as SIOCONS did. Control-C
// or Escape during it stops it, and SIOCONS.
func (c *Console) transfer(ctx context.Context, file string, verbose bool, segs []serial.Segment, run func(context.Context) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stopped := make(chan struct{})
	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			select {
			case k, ok := <-c.keys:
				if !ok || k == keyEscape || k == keyCtrlC {
					close(stopped)
					cancel()
					return
				}
			case <-done:
				return
			}
		}
	}()
	start := time.Now()
	shown := ""
	c.monitor.Progress = func(seg serial.Segment, n int) {
		if !verbose {
			return
		}
		if seg.Name != shown {
			if shown != "" {
				c.puts("\r\n")
			}
			shown = seg.Name
			switch {
			case file == "":
				c.printf("%s  ", seg.Name)
			case len(segs) > 1:
				c.printf("%s [%6s] ", file, seg.Name)
			default:
				c.printf("%s  ", file)
			}
			c.printf("address:%08x-%08x size:%06x  ", seg.Addr, seg.Addr+uint32(len(seg.Data))-1, len(seg.Data))
		}
		c.printf("%06x:%5dsec.%s", n, int(time.Since(start).Seconds()), strings.Repeat("\b", 16))
	}
	defer func() { c.monitor.Progress = nil }()
	err := run(ctx)
	if verbose && shown != "" {
		c.puts("\r\n")
	}
	select {
	case <-stopped:
		return errTerminate
	default:
	}
	return err
}

// dload uploads files into RAM, each at the address after it, all in one
// binary session.
func (c *Console) dloadKey(ctx context.Context) error {
	line, err := c.ask("Dload")
	if err != nil {
		return err
	}
	c.logLocal("dload", line)
	return c.dload(ctx, line)
}

func (c *Console) dload(ctx context.Context, args string) error {
	c.hush()
	defer c.unhush("end binary\r")
	fields := strings.Fields(args)
	var segs []serial.Segment
	for i := 0; i < len(fields) && len(segs) < 20; i += 2 {
		if i+1 >= len(fields) || !isHex(fields[i+1]) {
			c.printf("\"%s\" file address missing\r\n", fields[i])
			return errFailed
		}
		addr, _ := strconv.ParseUint(hexPrefix(fields[i+1]), 16, 32)
		data, err := os.ReadFile(fields[i])
		if err != nil {
			c.printf("Can't open \"%s\"\n\r", fields[i])
			return errFailed
		}
		segs = append(segs, serial.Segment{Name: fields[i], Addr: uint32(addr), Data: data})
	}
	if len(segs) == 0 {
		return errFailed
	}
	for _, s := range segs {
		c.printf("dload [%s]\r\n", s.Name)
	}
	return c.transfer(ctx, "", true, nil, func(ctx context.Context) error {
		return c.monitor.WriteSegments(ctx, segs...)
	})
}

// dsave writes RAM into a file: its name, the address and the size, the
// two in hexadecimal.
func (c *Console) dsaveKey(ctx context.Context) error {
	line, err := c.ask("Dsave")
	if err != nil {
		return err
	}
	c.logLocal("dsave", line)
	return c.dsave(ctx, line)
}

func (c *Console) dsave(ctx context.Context, args string) error {
	c.hush()
	defer c.unhush("end binary\r")
	fields := strings.Fields(args)
	if len(fields) < 3 {
		c.puts("filename adr size\r\n")
		return errFailed
	}
	addr, _ := strconv.ParseUint(hexPrefix(fields[1]), 16, 32)
	size, _ := strconv.ParseUint(hexPrefix(fields[2]), 16, 32)
	start := time.Now()
	var data []byte
	err := c.transfer(ctx, fields[0], false, nil, func(ctx context.Context) error {
		var err error
		data, err = c.monitor.ReadMemory(ctx, uint32(addr), int(size))
		return err
	})
	if err != nil {
		return err
	}
	if err := os.WriteFile(fields[0], data, 0o644); err != nil {
		c.printf("\"%s\" can't open\r\n", fields[0])
		return errFailed
	}
	c.printf("DSAVE %s %x %x/%x\r\n", fields[0], addr, size, size)
	c.printf("%d(sec)\r\n", int(time.Since(start).Seconds()))
	return nil
}

// isHex reports whether s starts with a hexadecimal digit, which is how
// SIOCONS tells an address from a file name.
func isHex(s string) bool {
	return s != "" && strings.ContainsRune("0123456789abcdefABCDEF", rune(s[0]))
}

// hexPrefix is the hexadecimal digits s starts with, as scanf's %lx reads.
func hexPrefix(s string) string {
	s = strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
	n := 0
	for n < len(s) && strings.ContainsRune("0123456789abcdefABCDEF", rune(s[n])) {
		n++
	}
	if n == 0 {
		return "0"
	}
	return s[:n]
}

// log starts logging what the monitor sends into a file, appending to it
// if it is there, or stops it, with no file.
func (c *Console) logKey(ctx context.Context) error {
	line, err := c.ask("Log")
	if err != nil {
		return err
	}
	return c.logCmd(ctx, line)
}

func (c *Console) logOff(ctx context.Context) error {
	c.puts("log off\n\r")
	return c.logCmd(ctx, "")
}

func (c *Console) logCmd(ctx context.Context, args string) error {
	c.closeLog()
	name := strings.TrimSpace(args)
	if name == "" {
		return nil
	}
	_, err := os.Stat(name)
	appending := err == nil
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		c.printf("\"%s\" can't open\n\r", name)
		return nil
	}
	c.log = f
	if appending {
		c.printf("start log append into \"%s\"\n\r", name)
	} else {
		c.printf("start log into \"%s\"\n\r", name)
	}
	return nil
}

func (c *Console) closeLog() {
	if c.log != nil {
		c.log.Close()
		c.log = nil
	}
}

// editToggle turns local edit mode on or off: on, a line typed at the
// monitor's prompt is edited here, with the history, and sent whole.
// SIOCONS sends Control-U with it.
func (c *Console) editToggleKey(ctx context.Context) error { return c.editToggle(ctx, "") }

func (c *Console) editToggle(ctx context.Context, args string) error {
	c.editMode = !c.editMode
	c.port.Write([]byte{0x15})
	c.promptSeen = false
	return nil
}

func (c *Console) beepKey(ctx context.Context) error { c.puts("\a"); return nil }
func (c *Console) beep(ctx context.Context, args string) error {
	c.puts("\a")
	return nil
}

func (c *Console) echo(ctx context.Context, args string) error {
	c.printf("%s\n\r", args)
	return nil
}

// pause waits for a key, after a message or a question mark. F3 stops the
// batch files.
func (c *Console) pause(ctx context.Context, args string) error {
	if s := strings.TrimSpace(args); s != "" {
		c.printf("%s ", s)
	} else {
		c.puts("?")
	}
	k, err := c.key()
	if err != nil {
		return err
	}
	c.puts("\r\n")
	if k == KeyF3 && len(c.autos) > 0 {
		c.autoClean()
	}
	return nil
}

func (c *Console) sleep(ctx context.Context, args string) error {
	n, _ := strconv.Atoi(strings.TrimSpace(args))
	c.sleepFor = time.Duration(n) * time.Second
	c.sleepFrom = time.Now()
	return nil
}

func (c *Console) waitForPrompt(ctx context.Context, args string) error {
	c.waitPrompt = true
	return nil
}

// auto runs a batch file: each line is typed to the monitor once it shows
// its prompt, or, starting with "local", is a local command, run at once.
// F3 while batch files run stops them all.
func (c *Console) autoKey(ctx context.Context) error {
	if len(c.autos) > 0 {
		c.autoClean()
		return nil
	}
	line, err := c.ask("Auto")
	if err != nil || line == "" {
		return err
	}
	c.logLocal("auto", line)
	return c.autoCmd(ctx, line)
}

func (c *Console) autoCmd(ctx context.Context, args string) error {
	if !c.autoAuto(args) {
		return errFailed
	}
	return nil
}

func (c *Console) autoAuto(name string) bool {
	if len(c.autos) > maxNest {
		c.puts("Too many nest Auto command \n\r")
		return false
	}
	name = strings.TrimSpace(name)
	f, err := os.Open(name)
	if err != nil {
		c.printf(" \"%s\" not found\n\r", name)
		return false
	}
	c.autos = append(c.autos, &batch{f: f, r: bufio.NewReader(f)})
	c.nextAuto()
	return true
}

func (c *Console) autoAgain(ctx context.Context, args string) error {
	if n := len(c.autos); n > 0 {
		b := c.autos[n-1]
		b.f.Seek(0, io.SeekStart)
		b.r.Reset(b.f)
	}
	return nil
}

// nextAuto reads the next line of the innermost batch file, going out to
// the one that ran it at its end, and notes whether it waits for the
// prompt: a line for the monitor does, a local command does not.
func (c *Console) nextAuto() {
	for len(c.autos) > 0 {
		b := c.autos[len(c.autos)-1]
		line, err := b.r.ReadString('\n')
		if line != "" || err == nil {
			c.autoLine = strings.TrimRight(line, "\r\n")
			c.waitPrompt = !isLocal(c.autoLine)
			return
		}
		b.f.Close()
		c.autos = c.autos[:len(c.autos)-1]
	}
}

func isLocal(line string) bool {
	return strings.HasPrefix(strings.TrimLeft(line, " \t"), "local")
}

// execAuto runs the batch line read last: a local command, which stops the
// batch files if it fails, or a line typed to the monitor.
func (c *Console) execAuto(ctx context.Context) error {
	line := strings.TrimLeft(c.autoLine, " \t")
	if isLocal(line) {
		cmd := strings.TrimLeft(line[len("local"):], " \t")
		ok := true
		if cmd != "" {
			var err error
			if ok, err = c.localCommand(ctx, cmd); err != nil {
				return err
			}
		}
		if !ok {
			c.autoClean()
		}
		return nil
	}
	// SIOCONS types the line as it is in the file, up to 64 characters.
	c.promptSeen = false
	_, err := c.port.Write([]byte(c.autoLine[:min(len(c.autoLine), editMax+1)] + "\r"))
	return err
}

// autoClean stops every batch file.
func (c *Console) autoClean() {
	c.sleepFor = 0
	c.waitPrompt = false
	for len(c.autos) > 0 {
		c.autos[len(c.autos)-1].f.Close()
		c.autos = c.autos[:len(c.autos)-1]
		if len(c.autos) == 0 {
			c.puts("auto commands all stop\n\r")
		}
	}
}
