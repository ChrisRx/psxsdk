package emu

import (
	"bytes"
	"context"
	"debug/elf"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"go.chrisrx.dev/psx/emu/cpu"
)

// LevelTrace is the level below slog.LevelDebug, for what happens many
// times a frame.
const LevelTrace = slog.Level(-8)

// What a machine logs, once SetLogger gives it somewhere to, and at which
// level:
//
//   - Error: the program has crashed. The CPU took an exception the program
//     does not handle, such as a misaligned store, or is looping on one
//     instruction, as a Go program does once it has panicked.
//   - Warn: an access to an address nothing is mapped at.
//   - Info: the display being turned on or off, and its mode changing.
//   - Debug: each call into the BIOS through its A, B and C tables, but
//     those polled many times a frame, and each command the CD-ROM drive
//     is given.
//   - LevelTrace: the polled BIOS calls, system calls, DMA transfers and
//     moves of the display area.
//
// A machine without a logger checks for none of it. With one, the CPU
// hands back to the machine whenever it enters the first 256 bytes of
// memory, where the exception vector and the BIOS's tables are, which costs
// a little speed.

// logger is the machine's logger, which the devices that log share.
type logger struct {
	l *slog.Logger
}

func (g *logger) on(level slog.Level) bool {
	return g != nil && g.l != nil && g.l.Enabled(context.Background(), level)
}

func (g *logger) log(level slog.Level, msg string, args ...any) {
	if g.on(level) {
		g.l.Log(context.Background(), level, msg, args...)
	}
}

// SetLogger has the machine log to l, or with nil, stop logging.
func (m *Machine) SetLogger(l *slog.Logger) {
	m.log.l = l
	m.stuckAt = 0
}

// A Symbol is a function or label in the program, for naming addresses in
// what the machine logs.
type Symbol struct {
	Name string
	Addr uint32
	Size uint32 // 0 if unknown
}

// SymbolsFromELF reads the symbols of an ELF executable's code: its
// functions, and the labels the Net Yaroze library's entry points are
// declared as.
func SymbolsFromELF(data []byte) ([]Symbol, error) {
	f, err := elf.NewFile(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	syms, err := f.Symbols()
	if err != nil {
		return nil, err
	}
	var out []Symbol
	for _, s := range syms {
		typ := elf.ST_TYPE(s.Info)
		if s.Name == "" || s.Value < 0x80000000 || (typ != elf.STT_FUNC && typ != elf.STT_NOTYPE) {
			continue
		}
		if typ == elf.STT_NOTYPE && elf.ST_BIND(s.Info) == elf.STB_LOCAL {
			continue // local labels within functions
		}
		out = append(out, Symbol{Name: s.Name, Addr: uint32(s.Value), Size: uint32(s.Size)})
	}
	return out, nil
}

// SetSymbols gives the machine the program's symbols, to name the
// addresses it logs with.
func (m *Machine) SetSymbols(syms []Symbol) {
	m.syms = append([]Symbol(nil), syms...)
	sort.Slice(m.syms, func(i, j int) bool { return m.syms[i].Addr < m.syms[j].Addr })
}

// symbol returns the symbol addr falls in, if any: one whose size covers
// it, or for one of unknown size, as the Net Yaroze library's entry points
// are, the nearest before it within 2KB. The library's own functions have
// no symbols, so beyond that the nearest entry point would name the wrong
// function more often than not.
func (m *Machine) symbol(addr uint32) (Symbol, bool) {
	addr |= 0x80000000
	i := sort.Search(len(m.syms), func(i int) bool { return m.syms[i].Addr > addr }) - 1
	if i < 0 {
		return Symbol{}, false
	}
	s := m.syms[i]
	if s.Size != 0 && addr-s.Addr >= s.Size || s.Size == 0 && addr-s.Addr >= 0x800 {
		return Symbol{}, false
	}
	return s, true
}

// where names an address: by its symbol and offset, where it has one.
func (m *Machine) where(addr uint32) string {
	if s, ok := m.symbol(addr); ok {
		if addr|0x80000000 == s.Addr {
			return fmt.Sprintf("%08x %s", addr, s.Name)
		}
		return fmt.Sprintf("%08x %s+%#x", addr, s.Name, addr|0x80000000-s.Addr)
	}
	return fmt.Sprintf("%08x", addr)
}

// peek32 reads a word of RAM without the bus, so that nothing is charged
// for it; outside RAM it reads 0.
func (m *Machine) peek32(addr uint32) uint32 {
	a := addr & 0x1fffffff
	if a >= RAMSize {
		return 0
	}
	return m.ram.memory.Read32(a)
}

// backtrace finds the return addresses on the stack, words just after a
// jal or jalr into code with a symbol, from the stack pointer up, and names
// them, innermost first.
func (m *Machine) backtrace() []string {
	if len(m.syms) == 0 {
		return nil
	}
	var frames []string
	sp := m.CPU.R[29] &^ 3
	for a := sp; a < sp+4096 && a&0x1fffffff < RAMSize && len(frames) < 12; a += 4 {
		ret := m.peek32(a)
		if ret&3 != 0 || ret < 0x80000008 || ret&0x1fffffff >= RAMSize {
			continue
		}
		if _, ok := m.symbol(ret); !ok {
			continue
		}
		call := m.peek32(ret - 8)
		if call>>26 != 3 && !(call>>26 == 0 && call&0x3f == 9) { // jal, jalr
			continue
		}
		frames = append(frames, m.where(ret-8))
	}
	return frames
}

// panicKinds are the TinyGo runtime's functions that panic with a runtime
// error, by what the error is.
var panicKinds = map[string]string{
	"runtime.lookupPanic":        "index out of range",
	"runtime.slicePanic":         "slice bounds out of range",
	"runtime.sliceToArrayPanic":  "slice converted to an array of the wrong length",
	"runtime.unsafeSlicePanic":   "unsafe.Slice with an invalid length",
	"runtime.nilPanic":           "nil pointer dereference",
	"runtime.divideByZeroPanic":  "integer divide by zero",
	"runtime.negativeShiftPanic": "negative shift amount",
	"runtime.chanMakePanic":      "make of a channel with an invalid size",
	"runtime._panic":             "panic",
}

// hooked looks at a step into the first 256 bytes of memory: an exception
// taken, or a call through one of the BIOS's tables.
func (m *Machine) hooked() {
	c := m.CPU
	switch c.PC & 0x3fffffff {
	case 0x80:
		m.exceptionTaken()
	case 0xa0:
		m.biosCall('A', c.R[9])
	case 0xb0:
		m.biosCall('B', c.R[9])
	case 0xc0:
		m.biosCall('C', c.R[9])
	}
}

var excNames = map[uint32]string{
	cpu.ExcAdEL: "address error on a load or fetch (AdEL)",
	cpu.ExcAdES: "address error on a store (AdES)",
	cpu.ExcBp:   "breakpoint (Bp)",
	cpu.ExcRI:   "reserved instruction (RI)",
	cpu.ExcCpU:  "coprocessor unusable (CpU)",
	cpu.ExcOv:   "arithmetic overflow (Ov)",
	6:           "bus error on a fetch (IBE)",
	7:           "bus error on a load or store (DBE)",
}

func (m *Machine) exceptionTaken() {
	c := m.CPU
	code := c.COP0[cpu.Cause] >> 2 & 0x1f
	switch code {
	case cpu.ExcInt:
		return
	case cpu.ExcSyscall:
		if m.log.on(LevelTrace) {
			name := map[uint32]string{1: "EnterCriticalSection", 2: "ExitCriticalSection", 3: "ChangeThreadSubFunction"}[c.R[4]]
			if name == "" {
				name = fmt.Sprintf("syscall(%#x)", c.R[4])
			}
			m.log.log(LevelTrace, fmt.Sprintf("syscall %s from %s", name, m.where(c.COP0[cpu.EPC])))
		}
		return
	}
	if !m.log.on(slog.LevelError) {
		return
	}
	name := excNames[code]
	if name == "" {
		name = fmt.Sprintf("exception %d", code)
	}
	epc := c.COP0[cpu.EPC]
	msg := fmt.Sprintf("crash: %s at %s", name, m.where(epc))
	if code == cpu.ExcAdEL || code == cpu.ExcAdES {
		msg += fmt.Sprintf(", address %08x", c.COP0[cpu.BadVAddr])
	}
	m.log.log(slog.LevelError, msg, m.crashAttrs()...)
}

// crashAttrs is what is logged with a crash: the return address, the
// stack, and the registers.
func (m *Machine) crashAttrs() []any {
	c := m.CPU
	args := []any{slog.String("ra", m.where(c.R[31]))}
	if bt := m.backtrace(); len(bt) > 0 {
		args = append(args, slog.String("stack", strings.Join(bt, " <- ")))
	}
	var regs strings.Builder
	for i, r := range c.R {
		switch {
		case i > 0 && i%8 == 0:
			regs.WriteByte('\n')
		case i > 0:
			regs.WriteByte(' ')
		}
		fmt.Fprintf(&regs, "%s=%08x", regNames[i], r)
	}
	return append(args, slog.String("registers", regs.String()))
}

var regNames = [32]string{
	"zero", "at", "v0", "v1", "a0", "a1", "a2", "a3",
	"t0", "t1", "t2", "t3", "t4", "t5", "t6", "t7",
	"s0", "s1", "s2", "s3", "s4", "s5", "s6", "s7",
	"t8", "t9", "k0", "k1", "gp", "sp", "fp", "ra",
}

// checkStuck reports the CPU looping on a jump to itself, once for each
// place it does: a Go program's runtime does that once it has panicked,
// and once main returns. It runs once a frame.
func (m *Machine) checkStuck() {
	pc := m.CPU.PC
	for _, at := range [2]uint32{pc, pc - 4} {
		op := m.peek32(at)
		self := op == 0x1000ffff || // beq $zero, $zero, .
			op>>26 == 2 && at&0xf0000000|(op&0x3ffffff)<<2 == at // j .
		if !self {
			continue
		}
		if m.stuckAt == at {
			return
		}
		m.stuckAt = at
		msg := fmt.Sprintf("crash: the CPU is looping on a jump to itself at %s", m.where(at))
		bt := m.backtrace()
		for _, f := range bt {
			for fn, kind := range panicKinds {
				if strings.Contains(f, " "+fn+"+") || strings.HasSuffix(f, " "+fn) {
					msg = fmt.Sprintf("crash: the program panicked (%s); the CPU is looping at %s", kind, m.where(at))
					break
				}
			}
		}
		if s, ok := m.symbol(at); ok && len(bt) == 0 && strings.HasPrefix(s.Name, "runtime.") {
			msg += ", in the Go runtime, which does that after a panic or once main returns"
		}
		m.log.log(slog.LevelError, msg, m.crashAttrs()...)
		return
	}
	m.stuckAt = 0
}

// biosCall logs a call through one of the BIOS's function tables, with
// the function number in $t1. What putchar and the like print shows in the
// TTY, so those calls are left out.
func (m *Machine) biosCall(table byte, fn uint32) {
	if !m.log.on(slog.LevelDebug) {
		return
	}
	level := slog.LevelDebug
	switch {
	case table == 'A' && fn >= 0x3b && fn <= 0x3f, table == 'B' && fn >= 0x3b && fn <= 0x3f,
		table == 'B' && fn == 0x35 && m.CPU.R[4] == 1:
		// putchar, puts, printf, and FileWrite to standard output.
		return
	case table == 'B' && (fn == 0x07 || fn == 0x0b || fn == 0x17 || fn == 0x20):
		// DeliverEvent, TestEvent, ReturnFromException and
		// UnDeliverEvent, which the library calls many times a frame.
		level = LevelTrace
		if !m.log.on(level) {
			return
		}
	}
	names := map[byte][]string{'A': biosA, 'B': biosB, 'C': biosC}[table]
	name := ""
	if int(fn) < len(names) {
		name = names[fn]
	}
	if name == "" {
		name = "?"
	}
	c := m.CPU
	m.log.log(level, fmt.Sprintf("BIOS %c(%02xh) %s(%08x, %08x, %08x, %08x) from %s",
		table, fn, name, c.R[4], c.R[5], c.R[6], c.R[7], m.where(c.R[31]-8)))
}

// The BIOS's functions, by table and number, as the nocash PlayStation
// specifications name them. An empty name is one not listed.
var biosA = []string{
	0x00: "FileOpen", "FileSeek", "FileRead", "FileWrite", "FileClose", "FileIoctl", "exit", "FileGetDeviceFlag",
	0x08: "FileGetc", "FilePutc", "todigit", "atof", "strtoul", "strtol", "abs", "labs",
	0x10: "atoi", "atol", "atob", "SaveState", "RestoreState", "strcat", "strncat", "strcmp",
	0x18: "strncmp", "strcpy", "strncpy", "strlen", "index", "rindex", "strchr", "strrchr",
	0x20: "strpbrk", "strspn", "strcspn", "strtok", "strstr", "toupper", "tolower", "bcopy",
	0x28: "bzero", "bcmp", "memcpy", "memset", "memmove", "memcmp", "memchr", "rand",
	0x30: "srand", "qsort", "strtod", "malloc", "free", "lsearch", "bsearch", "calloc",
	0x38: "realloc", "InitHeap", "SystemErrorExit", "std_in_getchar", "std_out_putchar", "std_in_gets", "std_out_puts", "printf",
	0x40: "SystemErrorUnresolvedException", "LoadExeHeader", "LoadExeFile", "DoExecute", "FlushCache", "init_a0_b0_c0_vectors", "GPU_dw", "gpu_send_dma",
	0x48: "SendGP1Command", "GPU_cw", "GPU_cwp", "send_gpu_linked_list", "gpu_abort_dma", "GetGPUStatus", "gpu_sync", "SystemError",
	0x50: "SystemError", "LoadAndExecute", "GetSysSp", "SystemError", "CdInit", "_bu_init", "CdRemove", "",
	0x70: "_bu_init", "CdInit", "CdRemove", "", "", "", "", "",
	0x78: "CdAsyncSeekL", "", "", "", "CdAsyncGetStatus", "", "CdAsyncReadSector", "",
	0x80: "", "CdAsyncSetMode", "", "", "", "", "", "",
	0x90: "CdromIoIrqFunc1", "CdromDmaIrqFunc1", "CdromIoIrqFunc2", "CdromDmaIrqFunc2", "CdromGetInt5errCode", "CdInitSubFunc", "AddCDROMDevice", "AddMemCardDevice",
	0x98: "AddDuartTtyDevice", "AddDummyTtyDevice", "", "", "SetConf", "GetConf", "SetCdromIrqAutoAbort", "SetMemSize",
	0xa0: "WarmBoot", "SystemErrorBootOrDiskFailure", "EnqueueCdIntr", "DequeueCdIntr", "CdGetLbn", "CdReadSector", "CdGetStatus", "bu_callback_okay",
	0xa8: "bu_callback_err_write", "bu_callback_err_busy", "bu_callback_err_eject", "_card_info", "_card_async_load_directory", "set_card_auto_format", "bu_callback_err_prev_write", "card_write_test",
	0xb4: "GetSystemInfo",
}

var biosB = []string{
	0x00: "alloc_kernel_memory", "free_kernel_memory", "init_timer", "get_timer", "enable_timer_irq", "disable_timer_irq", "restart_timer", "DeliverEvent",
	0x08: "OpenEvent", "CloseEvent", "WaitEvent", "TestEvent", "EnableEvent", "DisableEvent", "OpenThread", "CloseThread",
	0x10: "ChangeThread", "jump_to_00000000h", "InitPad", "StartPad", "StopPad", "OutdatedPadInitAndStart", "OutdatedPadGetButtons", "ReturnFromException",
	0x18: "SetDefaultExitFromException", "SetCustomExitFromException", "SystemError", "", "", "", "", "",
	0x20: "UnDeliverEvent", "", "", "", "", "", "", "",
	0x30: "", "", "FileOpen", "FileSeek", "FileRead", "FileWrite", "FileClose", "FileIoctl",
	0x38: "exit", "FileGetDeviceFlag", "FileGetc", "FilePutc", "std_in_getchar", "std_out_putchar", "std_in_gets", "std_out_puts",
	0x40: "chdir", "FormatDevice", "firstfile", "nextfile", "FileRename", "FileDelete", "FileUndelete", "AddDevice",
	0x48: "RemoveDevice", "PrintInstalledDevices", "InitCard", "StartCard", "StopCard", "_card_info_subfunc", "write_card_sector", "read_card_sector",
	0x50: "allow_new_card", "Krom2RawAdd", "", "Krom2Offset", "GetLastError", "GetLastFileError", "GetC0Table", "GetB0Table",
	0x58: "get_bu_callback_port", "testdevice", "", "ChangeClearPad", "get_card_status", "wait_card_status",
}

var biosC = []string{
	0x00: "EnqueueTimerAndVblankIrqs", "EnqueueSyscallHandler", "SysEnqIntRP", "SysDeqIntRP", "get_free_EvCB_slot", "get_free_TCB_slot", "ExceptionHandler", "InstallExceptionHandlers",
	0x08: "SysInitMemory", "SysInitKernelVariables", "ChangeClearRCnt", "SystemError", "InitDefInt", "SetIrqAutoAck", "", "",
	0x10: "", "", "InstallDevices", "FlushStdInOutPut", "", "tty_cdevinput", "tty_cdevscan", "tty_circgetc",
	0x18: "tty_circputc", "ioabort", "set_card_find_mode", "KernelRedirect", "AdjustA0Table", "get_card_find_mode",
}
