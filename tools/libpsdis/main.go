// Command libpsdis writes an annotated disassembly of the Net Yaroze
// resident library, for reading when reimplementing it in Go
// (exp/libps). Its output is not committed: it is Sony's code.
//
// The image is the one eco2exe combines into every program, link.Libps.
// The only names are the entry points the SDK's symusr.o gives, converted
// to playstation-yaroze-symusr.S; every other function is named by its
// address. Each function's listing resolves what an instruction reaches
// for: the function a jal calls, the address a lui/addiu pair forms and
// what is there, a gp-relative access, and the BIOS function a jump
// through 0xA0, 0xB0 or 0xC0 with t1 set names.
package main

import (
	"bufio"
	"bytes"
	"cmp"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"go.chrisrx.dev/psx/link"
)

func main() {
	var image, symbols, out string
	var names, dumps []string
	cmd := &cobra.Command{
		Use:   "libpsdis [flags]",
		Short: "annotated disassembly of the Net Yaroze resident library",
		Long: `libpsdis disassembles the Net Yaroze resident library, the image link.Libps
holds, naming its entry points from the symbol file converted from the
SDK's symusr.o and every other function by its address, and annotates each
instruction with what it reaches for. -o writes the whole image to
DIR/libps.txt; -f prints the functions named to standard output.`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			img, err := LoadImage(image, symbols)
			if err != nil {
				return err
			}
			fns := img.Functions()
			for _, spec := range dumps {
				addrs, count, _ := strings.Cut(spec, ":")
				addr, err := strconv.ParseUint(strings.TrimPrefix(addrs, "0x"), 16, 32)
				if err != nil {
					return fmt.Errorf("-d %q: %v", spec, err)
				}
				n := 16
				if count != "" {
					if n, err = strconv.Atoi(count); err != nil {
						return fmt.Errorf("-d %q: %v", spec, err)
					}
				}
				img.Dump(cmd.OutOrStdout(), uint32(addr), n, fns)
			}
			if len(names) > 0 {
				for _, name := range names {
					f := fns.Named(name)
					if f == nil {
						return fmt.Errorf("no function named %s", name)
					}
					img.Write(cmd.OutOrStdout(), f, fns)
				}
				return nil
			}
			if out == "" {
				if len(dumps) > 0 {
					return nil
				}
				return fmt.Errorf("-o names the directory to write to, -f the functions to print, or -d the words to dump")
			}
			if err := os.MkdirAll(out, 0o777); err != nil {
				return err
			}
			w, err := os.Create(filepath.Join(out, "libps.txt"))
			if err != nil {
				return err
			}
			bw := bufio.NewWriter(w)
			for _, f := range fns {
				img.Write(bw, f, fns)
			}
			if err := bw.Flush(); err != nil {
				return err
			}
			return w.Close()
		},
	}
	f := cmd.Flags()
	f.StringVar(&image, "image", "", "a PSX-EXE of the resident library, in place of the one built in")
	f.StringVar(&symbols, "symbols", "internal/toolchain/_tinygo/targets/playstation-yaroze-symusr.S", "the symbol file naming the entry points")
	f.StringVarP(&out, "out", "o", "", "the directory to write libps.txt into")
	f.StringArrayVarP(&names, "function", "f", nil, "a function to print; may be repeated")
	f.StringArrayVarP(&dumps, "dump", "d", nil, "ADDR[:N]: N words of data to print, 16 if not given, each with what it addresses; may be repeated")
	cmd.AddCommand(scoreCommand())
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// An Image is the resident library's text and the names it has.
type Image struct {
	Base uint32
	Data []byte
	syms map[uint32]string
	// data are the named symbols that are not functions.
	data map[string]bool
}

// A Function is a span of the image: a named entry point or a jal target,
// ending where the next begins.
type Function struct {
	Name     string
	Start    uint32
	Size     uint32
	Calls    []string // the functions it calls, by name
	CalledBy []string
}

// Functions is every function in the image, by address.
type Functions []*Function

// Named finds a function by name, or nil.
func (fns Functions) Named(name string) *Function {
	for _, f := range fns {
		if f.Name == name {
			return f
		}
	}
	return nil
}

// At finds the function starting at addr, or nil.
func (fns Functions) At(addr uint32) *Function {
	i, ok := slices.BinarySearchFunc(fns, addr, func(f *Function, a uint32) int { return cmp.Compare(f.Start, a) })
	if !ok {
		return nil
	}
	return fns[i]
}

// containing finds the function addr falls in, or nil.
func (fns Functions) containing(addr uint32) *Function {
	i, _ := slices.BinarySearchFunc(fns, addr, func(f *Function, a uint32) int { return cmp.Compare(f.Start, a) })
	if i < len(fns) && fns[i].Start == addr {
		return fns[i]
	}
	if i == 0 {
		return nil
	}
	f := fns[i-1]
	if addr < f.Start+f.Size {
		return f
	}
	return nil
}

var symbolLine = regexp.MustCompile(`^\s*\.set\s+([A-Za-z_][A-Za-z0-9_]*),\s*0x([0-9A-Fa-f]+)`)

// LoadImage reads the image, the built-in one when image is empty, and
// the symbol file.
func LoadImage(image, symbols string) (*Image, error) {
	img := &Image{syms: map[uint32]string{}, data: map[string]bool{}}
	if image == "" {
		exe, err := link.Libps()
		if err != nil {
			return nil, err
		}
		text := exe.Section("text")
		img.Base, img.Data = text.Addr, text.Data
	} else {
		data, err := os.ReadFile(image)
		if err != nil {
			return nil, err
		}
		if len(data) < 2048 {
			return nil, fmt.Errorf("%s: too short for a PSX-EXE", image)
		}
		img.Base = uint32(data[0x18]) | uint32(data[0x19])<<8 | uint32(data[0x1a])<<16 | uint32(data[0x1b])<<24
		img.Data = data[2048:]
	}
	f, err := os.Open(symbols)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		m := symbolLine.FindStringSubmatch(sc.Text())
		if m == nil {
			continue
		}
		addr, err := strconv.ParseUint(m[2], 16, 32)
		if err != nil {
			return nil, fmt.Errorf("%s: %q: %v", symbols, sc.Text(), err)
		}
		img.syms[uint32(addr)] = m[1]
	}
	return img, sc.Err()
}

func (img *Image) word(addr uint32) (uint32, bool) {
	off := int64(addr) - int64(img.Base)
	if off < 0 || off+4 > int64(len(img.Data)) {
		return 0, false
	}
	return uint32(img.Data[off]) | uint32(img.Data[off+1])<<8 | uint32(img.Data[off+2])<<16 | uint32(img.Data[off+3])<<24, true
}

func (img *Image) contains(addr uint32) bool {
	return addr >= img.Base && addr < img.Base+uint32(len(img.Data))
}

// Functions finds the functions: every jal target in the image and every
// named symbol before the last of those. A named symbol after the last
// call target is data, since the image keeps its variables after its
// code, unless a jr ra follows it before the next symbol: longjmp is
// there, called by nobody in the image.
func (img *Image) Functions() Functions {
	starts := map[uint32]bool{}
	var last uint32
	// afterReturn reports whether a function could begin at addr: the
	// instruction before its delay slot is a return.
	afterReturn := func(addr uint32) bool {
		w, ok := img.word(addr - 8)
		return ok && w == 0x03e00008 && addr&3 == 0 && img.contains(addr)
	}
	var hi [32]uint32
	for addr := img.Base; addr+4 <= img.Base+uint32(len(img.Data)); addr += 4 {
		w, _ := img.word(addr)
		switch {
		case w>>26 == 3:
			t := (addr+4)&0xf0000000 | (w&0x3ffffff)<<2
			if img.contains(t) {
				starts[t] = true
				last = max(last, t)
			}
		case afterReturn(addr) && opensFrame(img, addr):
			// A frame opens right after a return: a function nothing
			// calls by name, reached through a pointer.
			starts[addr] = true
		case afterReturn(w):
			// A data word, or an immediate, that addresses code right
			// after a return: a function pointer in a table.
			starts[w] = true
		}
		// A function pointer formed in code, lui then addiu.
		in := Decode(addr, w)
		if in.Kind == insnLui {
			hi[in.Rd] = uint32(in.Imm&0xffff) << 16
		} else if in.Mnemonic == "addiu" && in.Rs == in.Rd && hi[in.Rs] != 0 {
			if t := hi[in.Rs] + uint32(in.Imm); afterReturn(t) {
				starts[t] = true
			}
		}
		if in.Rd >= 0 && in.Kind != insnLui {
			hi[in.Rd] = 0
		}
	}
	for addr, name := range img.syms {
		if addr <= last || img.returns(addr) {
			starts[addr] = true
		} else {
			img.data[name] = true
		}
	}
	addrs := make([]uint32, 0, len(starts))
	for a := range starts {
		addrs = append(addrs, a)
	}
	slices.Sort(addrs)
	fns := make(Functions, len(addrs))
	for i, a := range addrs {
		end := img.Base + uint32(len(img.Data))
		if i+1 < len(addrs) {
			end = addrs[i+1]
		} else {
			// The last function is followed by the image's data, which
			// is not its own: it ends at its return and the delay slot.
			for addr := a; addr < end; addr += 4 {
				if w, _ := img.word(addr); w == 0x03e00008 {
					end = addr + 8
					break
				}
			}
		}
		fns[i] = &Function{Name: img.name(a), Start: a, Size: end - a}
	}
	for _, f := range fns {
		for addr := f.Start; addr < f.Start+f.Size; addr += 4 {
			w, _ := img.word(addr)
			in := Decode(addr, w)
			if in.Kind != insnCall {
				continue
			}
			if callee := fns.At(in.Target); callee != nil {
				if !slices.Contains(f.Calls, callee.Name) {
					f.Calls = append(f.Calls, callee.Name)
				}
				if !slices.Contains(callee.CalledBy, f.Name) {
					callee.CalledBy = append(callee.CalledBy, f.Name)
				}
			}
		}
	}
	return fns
}

// opensFrame reports whether one of the four instructions from addr is
// addiu sp, sp, -N, a function's prologue, which may follow a load or
// two of what the function looks at first.
func opensFrame(img *Image, addr uint32) bool {
	for a := addr; a < addr+16; a += 4 {
		w, ok := img.word(a)
		if !ok {
			return false
		}
		if w>>16 == 0x27bd && int16(w) < 0 {
			return true
		}
	}
	return false
}

// returns reports whether a jr ra follows addr before the next named
// symbol, within 256 words, which is what a function nobody calls looks
// like.
func (img *Image) returns(addr uint32) bool {
	for a := addr; a < addr+256*4; a += 4 {
		if a != addr {
			if _, named := img.syms[a]; named {
				return false
			}
		}
		w, ok := img.word(a)
		if !ok {
			return false
		}
		if w == 0x03e00008 {
			return true
		}
	}
	return false
}

// name is a symbol's name, or func_ADDR for an unnamed function.
func (img *Image) name(addr uint32) string {
	if name, ok := img.syms[addr]; ok {
		return name
	}
	return fmt.Sprintf("func_%08x", addr)
}

// biosNames are the BIOS functions the library calls, by table and number.
var biosNames = map[string]string{
	"A(0x3f)": "printf", "A(0x44)": "FlushCache", "A(0x49)": "GPU_cw", "A(0x4a)": "GPU_cwp", "A(0x4d)": "GPU_sync",
	"A(0x48)": "GPU_dw", "A(0x4b)": "GPU_SendPackets", "A(0x4e)": "GPU_GetGPUStatus", "A(0x4f)": "GPU_ResetGPU",
	"A(0x33)": "malloc", "A(0x34)": "free", "A(0x39)": "InitHeap", "A(0x3b)": "getchar", "A(0x3c)": "putchar",
	"A(0x0e)": "strcpy", "A(0x17)": "strcmp", "A(0x1b)": "strlen", "A(0x2a)": "memcpy", "A(0x28)": "bzero", "A(0x2b)": "memset",
	"A(0x70)": "_bu_init", "A(0x71)": "CdInit", "A(0x72)": "CdRemove", "A(0x78)": "CdAsyncSeekL", "A(0x7c)": "CdAsyncGetStatus",
	"A(0x7e)": "CdAsyncReadSector", "A(0x81)": "CdAsyncSetMode", "A(0x95)": "CdInitSubFunc", "A(0x96)": "AddCDROMDevice",
	"A(0x97)": "AddMemCardDevice", "A(0x98)": "AddDuartTtyDevice", "A(0x99)": "AddDummyTtyDevice", "A(0xa1)": "SystemError",
	"A(0xa2)": "EnqueueCdIntr", "A(0xa3)": "DequeueCdIntr", "A(0xa9)": "bu_callback_okay", "A(0xab)": "_card_info",
	"A(0xac)": "_card_load", "A(0xad)": "_card_auto", "A(0xae)": "bu_callback_err_eject", "A(0xb2)": "InitPad2", "A(0xb4)": "GetSystemInfo",
	"B(0x00)": "alloc_kernel_memory", "B(0x02)": "init_timer", "B(0x03)": "get_timer", "B(0x04)": "enable_timer_irq",
	"B(0x05)": "disable_timer_irq", "B(0x06)": "restart_timer", "B(0x07)": "DeliverEvent", "B(0x08)": "OpenEvent",
	"B(0x09)": "CloseEvent", "B(0x0a)": "WaitEvent", "B(0x0b)": "TestEvent", "B(0x0c)": "EnableEvent", "B(0x0d)": "DisableEvent",
	"B(0x0e)": "OpenThread", "B(0x0f)": "CloseThread", "B(0x10)": "ChangeThread", "B(0x12)": "InitPad", "B(0x13)": "StartPad",
	"B(0x14)": "StopPad", "B(0x15)": "OutdatedPadInitAndStart", "B(0x16)": "OutdatedPadGetButtons", "B(0x17)": "ReturnFromException",
	"B(0x18)": "SetDefaultExitFromException", "B(0x19)": "SetCustomExitFromException", "B(0x20)": "UnDeliverEvent",
	"B(0x32)": "open", "B(0x33)": "lseek", "B(0x34)": "read", "B(0x35)": "write", "B(0x36)": "close", "B(0x37)": "ioctl",
	"B(0x38)": "exit", "B(0x3b)": "getc", "B(0x3c)": "putc", "B(0x3d)": "putchar", "B(0x3f)": "puts", "B(0x40)": "cd",
	"B(0x41)": "format", "B(0x42)": "firstfile", "B(0x43)": "nextfile", "B(0x44)": "rename", "B(0x45)": "erase",
	"B(0x47)": "AddDevice", "B(0x48)": "RemoveDevice", "B(0x49)": "PrintInstalledDevices", "B(0x4a)": "InitCard",
	"B(0x4b)": "StartCard", "B(0x4c)": "StopCard", "B(0x4d)": "_card_info_subfunc", "B(0x4e)": "write_card_sector",
	"B(0x4f)": "read_card_sector", "B(0x50)": "allow_new_card", "B(0x51)": "Krom2RawAdd", "B(0x53)": "Krom2Offset",
	"B(0x54)": "GetLastError", "B(0x55)": "GetLastFileError", "B(0x56)": "GetC0Table", "B(0x57)": "GetB0Table",
	"B(0x58)": "get_bu_callback_port", "B(0x5b)": "ChangeClearPad", "B(0x5c)": "get_card_status", "B(0x5d)": "wait_card_status",
	"C(0x00)": "EnqueueTimerAndVblankIrqs", "C(0x01)": "EnqueueSyscallHandler", "C(0x02)": "SysEnqIntRP", "C(0x03)": "SysDeqIntRP",
	"C(0x04)": "get_free_EvCB_slot", "C(0x05)": "get_free_TCB_slot", "C(0x06)": "ExceptionHandler", "C(0x07)": "InstallExceptionHandlers",
	"C(0x08)": "SysInitMemory", "C(0x09)": "SysInitKernelVariables", "C(0x0a)": "ChangeClearRCnt", "C(0x0c)": "InitDefInt",
	"C(0x0d)": "SetIrqAutoAck", "C(0x12)": "InstallDevices", "C(0x13)": "FlushStdInOutPut", "C(0x15)": "tty_cdevinput",
	"C(0x16)": "tty_cdevscan", "C(0x17)": "tty_circgetc", "C(0x18)": "tty_circputc", "C(0x19)": "ioabort", "C(0x1b)": "KernelRedirect",
	"C(0x1c)": "AdjustA0Table", "C(0x1d)": "get_card_status", "C(0x1e)": "wait_card_status",
}

// Write writes one function's listing.
func (img *Image) Write(w io.Writer, f *Function, fns Functions) {
	fmt.Fprintf(w, "\n%s 0x%08X size %d", f.Name, f.Start, f.Size)
	if len(f.Calls) > 0 {
		fmt.Fprintf(w, "  calls: %s", strings.Join(f.Calls, " "))
	}
	if len(f.CalledBy) > 0 {
		fmt.Fprintf(w, "  called by: %s", strings.Join(f.CalledBy, " "))
	}
	fmt.Fprintln(w)

	// The upper halves lui left in registers, for the pair that follows,
	// and the values li left, for a BIOS call: the table's address in a
	// register, jr through it, and the function number put in t1 in the
	// delay slot.
	var hi [32]struct {
		set bool
		val uint32
	}
	var li [32]struct {
		set bool
		val int32
	}
	for addr := f.Start; addr < f.Start+f.Size; addr += 4 {
		word, _ := img.word(addr)
		in := Decode(addr, word)
		var note string
		switch in.Kind {
		case insnJumpReg:
			if v := li[in.Rs].val; li[in.Rs].set && (v == 0xa0 || v == 0xb0 || v == 0xc0) {
				next, _ := img.word(addr + 4)
				slot := Decode(addr+4, next)
				n := li[9].val
				if slot.Mnemonic == "li" && slot.Rd == 9 {
					n = slot.Imm
				}
				table := fmt.Sprintf("%c(0x%02x)", "ABC"[(li[in.Rs].val-0xa0)/0x10], n)
				note = "BIOS " + table
				if name, ok := biosNames[table]; ok {
					note += " " + name
				}
			}
		case insnCall, insnJump:
			switch {
			case in.Kind == insnCall:
				if callee := fns.At(in.Target); callee != nil {
					note = callee.Name
				}
			default:
				if g := fns.containing(in.Target); g != nil && g != f {
					note = fmt.Sprintf("%s+0x%x", g.Name, in.Target-g.Start)
				}
			}
		case insnLui:
			hi[in.Rd] = struct {
				set bool
				val uint32
			}{true, uint32(in.Imm&0xffff) << 16}
			if in.Imm < 0 {
				hi[in.Rd].val = uint32(in.Imm) << 16
			}
		case insnLoadStore:
			if in.Rs == 28 {
				note = fmt.Sprintf("_gp%+d", in.Imm)
			} else if hi[in.Rs].set {
				target := hi[in.Rs].val + uint32(in.Imm)
				note = img.describe(target, fns)
			}
		default:
			if (in.Mnemonic == "addiu" || in.Mnemonic == "ori") && hi[in.Rs].set {
				target := hi[in.Rs].val + uint32(in.Imm)
				if in.Mnemonic == "ori" {
					target = hi[in.Rs].val | uint32(in.Imm)&0xffff
				}
				note = img.describe(target, fns)
			}
		}
		if in.Rd >= 0 {
			hi[in.Rd].set = in.Kind == insnLui
			li[in.Rd].set = in.Mnemonic == "li"
			li[in.Rd].val = in.Imm
		}
		fmt.Fprintf(w, "%08X: %08X  %-8s %s", addr, word, in.Mnemonic, in.Operands)
		if note != "" {
			fmt.Fprintf(w, "%*s; %s", max(1, 28-len(in.Operands)), "", note)
		}
		fmt.Fprintln(w)
	}
}

// Dump prints n words from addr, each with what it addresses: the way to
// read a table of function pointers or a block of variables.
func (img *Image) Dump(w io.Writer, addr uint32, n int, fns Functions) {
	for a := addr; a < addr+uint32(n)*4; a += 4 {
		word, ok := img.word(a)
		if !ok {
			return
		}
		fmt.Fprintf(w, "%08X: %08X", a, word)
		if img.contains(word) && word&3 == 0 {
			fmt.Fprintf(w, "  %s", img.describe(word, fns))
		}
		fmt.Fprintln(w)
	}
}

// describe says what is at addr: a function or an offset into one, a
// named variable, a string, or the first word there.
func (img *Image) describe(addr uint32, fns Functions) string {
	if name, ok := img.syms[addr]; ok {
		if img.data[name] {
			if w, ok := img.word(addr); ok {
				return fmt.Sprintf("%s = 0x%08x", name, w)
			}
			return name
		}
		return name
	}
	if g := fns.containing(addr); g != nil {
		if g.Start == addr {
			return g.Name
		}
		return fmt.Sprintf("%s+0x%x", g.Name, addr-g.Start)
	}
	if !img.contains(addr) {
		return fmt.Sprintf("0x%08x", addr)
	}
	off := addr - img.Base
	if s := img.cstring(off); s != "" {
		return fmt.Sprintf("0x%08x %q", addr, s)
	}
	w, _ := img.word(addr &^ 3)
	return fmt.Sprintf("0x%08x = 0x%08x", addr, w)
}

// cstring returns the printable NUL-terminated string at off, or "".
func (img *Image) cstring(off uint32) string {
	end := bytes.IndexByte(img.Data[off:], 0)
	if end < 2 || end > 200 {
		return ""
	}
	s := img.Data[off : off+uint32(end)]
	for _, c := range s {
		if (c < 0x20 || c > 0x7e) && c != '\n' && c != '\t' && c != '\r' {
			return ""
		}
	}
	return string(s)
}
