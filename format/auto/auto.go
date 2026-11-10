// Package auto reads the batch files, each called auto, that told a Net
// Yaroze's SIOCONS what to send to the console: the program, the data it
// expected at fixed addresses, and the go that ran it. A program handed
// around without its source is a directory of those files and an auto that
// names them, which is all a host needs to run it.
//
// Two dialects are read. SIOCONS's own, where a line starting with local is
// one of its commands and any other line goes to the console's monitor:
//
//	local dload data\sprite.tim 80090000
//	local load main
//	go
//
// And the DOS batch files of the earlier tools, which some samples kept:
//
//	dload -nrsd\wall0.tmd -a80090000
//	load -ntest
//	go
package auto

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Names are the names an auto file goes by, matched without regard to case.
var Names = []string{"auto", "auto.bat"}

// A Load is a file put into memory at an address before the program runs.
type Load struct {
	File string // the name as the batch file gives it, or with Read a host path
	Addr uint32
}

// A Batch is what an auto file says to do.
type Batch struct {
	// Programs are the executables loaded, in order: their names as the
	// batch file gives them, or with Read host paths. The last is the one
	// that runs.
	Programs []string

	// Loads are the files put into memory, in order.
	Loads []Load

	// Go says the batch file starts the program.
	Go bool

	// Skipped are the lines that load nothing and start nothing: commands
	// for the monitor, and SIOCONS's other local commands, such as echo
	// and log.
	Skipped []string

	// Dir is the directory the batch file is in, with Read.
	Dir string

	// Missing names the files the batch file loads that Read did not find.
	Missing []string
}

// Program returns the program that runs, or empty if the batch file loads
// none.
func (b *Batch) Program() string {
	if len(b.Programs) == 0 {
		return ""
	}
	return b.Programs[len(b.Programs)-1]
}

// Parse reads a batch file.
func Parse(r io.Reader) (*Batch, error) {
	b := &Batch{}
	scan := bufio.NewScanner(r)
	for n := 1; scan.Scan(); n++ {
		line := strings.TrimSpace(scan.Text())
		if line == "" {
			continue
		}
		if err := b.parseLine(line); err != nil {
			return nil, fmt.Errorf("line %d: %q: %w", n, line, err)
		}
	}
	return b, scan.Err()
}

func (b *Batch) parseLine(line string) error {
	f := strings.Fields(line)
	if strings.EqualFold(f[0], "local") {
		f = f[1:]
		if len(f) == 0 {
			return nil
		}
	}
	switch strings.ToLower(f[0]) {
	case "go":
		b.Go = true
	case "load":
		names, _, err := options(f[1:])
		if err != nil {
			return err
		}
		if len(names) == 0 {
			return fmt.Errorf("load names no program")
		}
		b.Programs = append(b.Programs, names...)
	case "dload", "ld":
		names, addrs, err := options(f[1:])
		if err != nil {
			return err
		}
		if len(names) == 0 || len(names) != len(addrs) {
			return fmt.Errorf("want a file and an address for each load")
		}
		for i, name := range names {
			b.Loads = append(b.Loads, Load{File: name, Addr: addrs[i]})
		}
	default:
		b.Skipped = append(b.Skipped, line)
	}
	return nil
}

// options reads a load or dload's arguments: SIOCONS's files, each followed
// by its address for dload, or the earlier tools' -nFILE and -aADDR, with
// -s and -v, which only said how much to print, dropped.
func options(args []string) (names []string, addrs []uint32, err error) {
	for _, a := range args {
		switch {
		case a == "-s" || a == "-v":
		case len(a) > 2 && strings.EqualFold(a[:2], "-n"):
			names = append(names, a[2:])
		case len(a) > 2 && strings.EqualFold(a[:2], "-a"):
			addr, err := parseAddr(a[2:])
			if err != nil {
				return nil, nil, err
			}
			addrs = append(addrs, addr)
		case len(names) > len(addrs) && isHex(a):
			addr, err := parseAddr(a)
			if err != nil {
				return nil, nil, err
			}
			addrs = append(addrs, addr)
		default:
			names = append(names, a)
		}
	}
	return names, addrs, nil
}

func parseAddr(s string) (uint32, error) {
	s = strings.ToLower(s)
	s = strings.TrimPrefix(s, "0x")
	a, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return 0, fmt.Errorf("bad address %q", s)
	}
	return uint32(a), nil
}

// isHex reports whether s is an address: hexadecimal, with or without 0x,
// of at least five digits, since a file name of letters a to f would be
// taken for one otherwise.
func isHex(s string) bool {
	s = strings.TrimPrefix(strings.ToLower(s), "0x")
	if len(s) < 5 || len(s) > 8 {
		return false
	}
	for _, c := range s {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}

// Read reads the batch file at path, and finds the files it names as DOS
// did, relative to its directory, with either slash and in any case, so
// that its Programs and Loads are host paths. Those not found are left out
// and named in Missing.
func Read(path string) (*Batch, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := Parse(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if b.Dir, err = filepath.Abs(filepath.Dir(path)); err != nil {
		return nil, err
	}
	var programs []string
	for _, name := range b.Programs {
		if p, ok := findProgram(b.Dir, name); ok {
			programs = append(programs, p)
		} else {
			b.Missing = append(b.Missing, name)
		}
	}
	b.Programs = programs
	var loads []Load
	for _, l := range b.Loads {
		if p, ok := Find(b.Dir, l.File); ok {
			loads = append(loads, Load{File: p, Addr: l.Addr})
		} else {
			b.Missing = append(b.Missing, l.File)
		}
	}
	b.Loads = loads
	return b, nil
}

// findProgram finds a program as SIOCONS did, which took main for main.exe.
func findProgram(dir, name string) (string, bool) {
	if p, ok := Find(dir, name); ok {
		return p, true
	}
	return Find(dir, name+".exe")
}

// Find resolves a path as DOS would, with either slash and in any case,
// relative to dir unless it is absolute. An exact match wins over one that
// differs in case. It finds directories as well as files.
func Find(dir, path string) (string, bool) {
	path = strings.ReplaceAll(path, `\`, "/")
	cur := dir
	if strings.HasPrefix(path, "/") {
		cur = "/"
	}
	for part := range strings.SplitSeq(path, "/") {
		switch part {
		case "", ".":
			continue
		case "..":
			cur = filepath.Dir(cur)
			continue
		}
		entries, err := os.ReadDir(cur)
		if err != nil {
			return "", false
		}
		var match string
		for _, e := range entries {
			if e.Name() == part {
				match = part
				break
			}
			if match == "" && strings.EqualFold(e.Name(), part) {
				match = e.Name()
			}
		}
		if match == "" {
			return "", false
		}
		cur = filepath.Join(cur, match)
	}
	return cur, true
}

// IsName reports whether name, a file's base name, is an auto file's.
func IsName(name string) bool {
	for _, n := range Names {
		if strings.EqualFold(name, n) {
			return true
		}
	}
	return false
}
