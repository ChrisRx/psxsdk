// Package psxtrace reads and writes instruction traces, which record what an
// emulated R3000A did one instruction at a time so that two emulators can be
// compared until the first point they disagree.
//
// The format is the one written by psxe with nix/psxe-trace.patch applied,
// where psx/trace.h describes it. All values are little-endian:
//
//	header:  "PSXTRACE"  u32 version (1)  u32 reserved (0)
//	record:  u32 pc      address the instruction was fetched from
//	         u32 insn    the instruction word
//	         u8  excode  exception taken during this step, or 0xff
//	         u8  cycles  cycles charged for the step, saturating at 255
//	         u8  n       number of registers that changed
//	         n x { u8 reg, u32 value }
//
// A record lists only the registers a step changed, with their values after
// it. An interrupt is taken in place of executing the fetched instruction, so
// its record has excode 0 with the fetched pc and insn.
package psxtrace

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"slices"
)

const (
	magic   = "PSXTRACE"
	version = 1
)

// NoException is the Excode of a step that took no exception.
const NoException = 0xff

// Reg names a register a record can report. 0 to 31 are the general
// registers.
type Reg uint8

const (
	HI Reg = 32 + iota
	LO
	SR
	Cause
	EPC

	// NumRegs is the number of registers a trace can report.
	NumRegs
)

var gprNames = [32]string{
	"zero", "at", "v0", "v1", "a0", "a1", "a2", "a3",
	"t0", "t1", "t2", "t3", "t4", "t5", "t6", "t7",
	"s0", "s1", "s2", "s3", "s4", "s5", "s6", "s7",
	"t8", "t9", "k0", "k1", "gp", "sp", "fp", "ra",
}

func (r Reg) String() string {
	switch {
	case r < 32:
		return gprNames[r]
	case r == HI:
		return "hi"
	case r == LO:
		return "lo"
	case r == SR:
		return "sr"
	case r == Cause:
		return "cause"
	case r == EPC:
		return "epc"
	}
	return fmt.Sprintf("reg%d", uint8(r))
}

// Write is a register's value after a step.
type Write struct {
	Reg   Reg
	Value uint32
}

// Record is one step of the CPU.
type Record struct {
	PC     uint32
	Insn   uint32
	Excode uint8 // NoException if none was taken
	Cycles uint8
	Writes []Write // in register order
}

// Reader reads a trace one record at a time.
type Reader struct {
	r   *bufio.Reader
	buf [11]byte
	ws  []Write
}

// NewReader reads the trace header from r.
func NewReader(r io.Reader) (*Reader, error) {
	br := bufio.NewReaderSize(r, 1<<20)
	var hdr [16]byte
	if _, err := io.ReadFull(br, hdr[:]); err != nil {
		return nil, fmt.Errorf("reading trace header: %w", err)
	}
	if string(hdr[:8]) != magic {
		return nil, errors.New("not a trace: bad magic")
	}
	if v := binary.LittleEndian.Uint32(hdr[8:]); v != version {
		return nil, fmt.Errorf("trace version %d, only %d is supported", v, version)
	}
	return &Reader{r: br, ws: make([]Write, 0, NumRegs)}, nil
}

// Next returns the next record, or io.EOF at the end of the trace. A trace
// that stops part way through a record returns io.ErrUnexpectedEOF.
//
// The record's Writes are only valid until the next call.
func (r *Reader) Next() (Record, error) {
	b := r.buf[:]
	// ReadFull already tells the two endings apart: io.EOF if the trace
	// ends between records, io.ErrUnexpectedEOF if it ends inside one.
	if _, err := io.ReadFull(r.r, b); err != nil {
		return Record{}, err
	}
	rec := Record{
		PC:     binary.LittleEndian.Uint32(b[0:]),
		Insn:   binary.LittleEndian.Uint32(b[4:]),
		Excode: b[8],
		Cycles: b[9],
	}
	n := int(b[10])
	if n > int(NumRegs) {
		return Record{}, fmt.Errorf("record at pc %#08x reports %d registers, more than the %d there are", rec.PC, n, NumRegs)
	}

	r.ws = r.ws[:0]
	for range n {
		if _, err := io.ReadFull(r.r, b[:5]); err != nil {
			return Record{}, unexpected(err)
		}
		r.ws = append(r.ws, Write{Reg: Reg(b[0]), Value: binary.LittleEndian.Uint32(b[1:])})
	}
	if n > 0 {
		rec.Writes = r.ws
	}
	return rec, nil
}

// unexpected reports running out of input inside a record, which a plain
// io.EOF would make look like the end of the trace.
func unexpected(err error) error {
	if err == io.EOF {
		return io.ErrUnexpectedEOF
	}
	return err
}

// Writer writes a trace. Flush must be called to write out what is buffered.
type Writer struct {
	w   *bufio.Writer
	buf []byte
}

// NewWriter writes the trace header to w.
func NewWriter(w io.Writer) (*Writer, error) {
	bw := bufio.NewWriterSize(w, 1<<20)
	hdr := binary.LittleEndian.AppendUint32([]byte(magic), version)
	hdr = binary.LittleEndian.AppendUint32(hdr, 0)
	if _, err := bw.Write(hdr); err != nil {
		return nil, err
	}
	return &Writer{w: bw}, nil
}

// Write writes one record. Its Writes are written in register order, which
// is how a reader of the other emulator's trace will find them.
func (w *Writer) Write(rec Record) error {
	if len(rec.Writes) > int(NumRegs) {
		return fmt.Errorf("record at pc %#08x has %d register writes, more than the %d registers there are", rec.PC, len(rec.Writes), NumRegs)
	}
	ws := rec.Writes
	if !slices.IsSortedFunc(ws, compareWrites) {
		ws = slices.SortedFunc(slices.Values(ws), compareWrites)
	}

	b := w.buf[:0]
	b = binary.LittleEndian.AppendUint32(b, rec.PC)
	b = binary.LittleEndian.AppendUint32(b, rec.Insn)
	b = append(b, rec.Excode, rec.Cycles, byte(len(ws)))
	for _, wr := range ws {
		b = append(b, byte(wr.Reg))
		b = binary.LittleEndian.AppendUint32(b, wr.Value)
	}
	w.buf = b
	_, err := w.w.Write(b)
	return err
}

// Flush writes out any buffered records.
func (w *Writer) Flush() error {
	return w.w.Flush()
}

func compareWrites(a, b Write) int {
	return int(a.Reg) - int(b.Reg)
}
