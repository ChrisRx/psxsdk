package psxtrace

import (
	"bytes"
	"encoding/hex"
	"io"
	"strings"
	"testing"

	"go.chrisrx.dev/x/assert"
)

// psxeBoot is the start of a trace written by the patched psxe: the header
// and the first three instructions of the SCPH-1001 BIOS.
const psxeBoot = "" +
	"5053585452414345" + "01000000" + "00000000" +
	"0000c0bf" + "1300083c" + "ff" + "14" + "01" + "08" + "00001300" + // lui t0, 0x13
	"0400c0bf" + "3f240835" + "ff" + "14" + "01" + "08" + "3f241300" + // ori t0, t0, 0x243f
	"0800c0bf" + "801f013c" + "ff" + "14" + "01" + "01" + "0000801f" //   lui at, 0x1f80

func TestReadPsxeTrace(t *testing.T) {
	data, err := hex.DecodeString(psxeBoot)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	assert.Equal(t, []Record{
		{PC: 0xbfc00000, Insn: 0x3c080013, Excode: NoException, Cycles: 20, Writes: []Write{{8, 0x00130000}}},
		{PC: 0xbfc00004, Insn: 0x3508243f, Excode: NoException, Cycles: 20, Writes: []Write{{8, 0x0013243f}}},
		{PC: 0xbfc00008, Insn: 0x3c011f80, Excode: NoException, Cycles: 20, Writes: []Write{{1, 0x1f800000}}},
	}, readAll(t, bytes.NewReader(data)))
}

func TestRoundTrip(t *testing.T) {
	records := []Record{
		{PC: 0x80010000, Insn: 0x27bdfff8, Excode: NoException, Cycles: 1, Writes: []Write{{29, 0x801feff8}}},
		{PC: 0x80010004, Insn: 0x0000000c, Excode: 0x08, Cycles: 2, Writes: []Write{{SR, 0x40000404}, {Cause, 0x20}, {EPC, 0x80010004}}},
		{PC: 0x80000080, Insn: 0x00000000, Excode: NoException, Cycles: 1},
	}
	assert.Equal(t, records, readAll(t, bytes.NewReader(write(t, records))))
}

func TestWriteSortsRegisters(t *testing.T) {
	data := write(t, []Record{{Excode: NoException, Writes: []Write{{LO, 2}, {HI, 1}, {4, 3}}}})
	got := readAll(t, bytes.NewReader(data))
	assert.Equal(t, []Write{{4, 3}, {HI, 1}, {LO, 2}}, got[0].Writes)
}

func TestReadErrors(t *testing.T) {
	_, err := NewReader(strings.NewReader("NOTATRACE......."))
	assert.Error(t, "bad magic", err)

	_, err = NewReader(strings.NewReader("PSXTRACE\x02\x00\x00\x00\x00\x00\x00\x00"))
	assert.Error(t, "version 2", err)

	// Cut off inside the second record's register list.
	data := write(t, []Record{
		{Excode: NoException, Writes: []Write{{1, 1}}},
		{Excode: NoException, Writes: []Write{{1, 1}}},
	})
	r, err := NewReader(bytes.NewReader(data[:len(data)-2]))
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	_, err = r.Next()
	assert.NoError(t, err)
	_, err = r.Next()
	assert.Error(t, io.ErrUnexpectedEOF, err)
}

func TestCompare(t *testing.T) {
	base := []Record{
		{PC: 0x80010000, Insn: 1, Excode: NoException, Cycles: 1, Writes: []Write{{2, 10}}},
		{PC: 0x80010004, Insn: 2, Excode: NoException, Cycles: 1, Writes: []Write{{3, 11}}},
		{PC: 0x80010008, Insn: 3, Excode: NoException, Cycles: 1, Writes: []Write{{4, 12}}},
		{PC: 0x8001000c, Insn: 4, Excode: NoException, Cycles: 1},
	}
	other := func(i int, change func(*Record)) []Record {
		recs := make([]Record, len(base))
		for j, r := range base {
			recs[j] = clone(r)
		}
		change(&recs[i])
		return recs
	}

	t.Run("identical", func(t *testing.T) {
		n, d, err := compare(t, base, base, CompareOptions{})
		assert.NoError(t, err)
		assert.Equal(t, uint64(4), n)
		assert.Equal(t, (*Divergence)(nil), d)
	})

	t.Run("shorter trace ends the comparison", func(t *testing.T) {
		n, d, err := compare(t, base, base[:2], CompareOptions{})
		assert.NoError(t, err)
		assert.Equal(t, uint64(2), n)
		assert.Equal(t, (*Divergence)(nil), d)
	})

	t.Run("register value", func(t *testing.T) {
		b := other(2, func(r *Record) { r.Writes[0].Value = 99 })
		n, d, err := compare(t, base, b, CompareOptions{Context: 1})
		assert.NoError(t, err)
		assert.Equal(t, uint64(2), n)
		assert.Equal(t, &Divergence{
			Index:   2,
			A:       base[2],
			B:       b[2],
			Fields:  []string{"registers"},
			Context: []Record{base[1]},
		}, d)
	})

	t.Run("pc and exception", func(t *testing.T) {
		b := other(1, func(r *Record) { r.PC, r.Excode = 0x80000080, 0 })
		_, d, err := compare(t, base, b, CompareOptions{})
		assert.NoError(t, err)
		assert.Equal(t, []string{"pc", "exception"}, d.Fields)
	})

	t.Run("cycles", func(t *testing.T) {
		b := other(3, func(r *Record) { r.Cycles = 5 })
		_, d, err := compare(t, base, b, CompareOptions{})
		assert.NoError(t, err)
		assert.Equal(t, []string{"cycles"}, d.Fields)

		_, d, err = compare(t, base, b, CompareOptions{IgnoreCycles: true})
		assert.NoError(t, err)
		assert.Equal(t, (*Divergence)(nil), d)
	})
}

func write(t *testing.T, records []Record) []byte {
	t.Helper()
	var buf bytes.Buffer
	w, err := NewWriter(&buf)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	for _, r := range records {
		if !assert.NoError(t, w.Write(r)) {
			t.FailNow()
		}
	}
	if !assert.NoError(t, w.Flush()) {
		t.FailNow()
	}
	return buf.Bytes()
}

func readAll(t *testing.T, r io.Reader) []Record {
	t.Helper()
	tr, err := NewReader(r)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	var out []Record
	for {
		rec, err := tr.Next()
		if err == io.EOF {
			return out
		}
		if !assert.NoError(t, err) {
			t.FailNow()
		}
		out = append(out, clone(rec))
	}
}

func compare(t *testing.T, a, b []Record, opts CompareOptions) (uint64, *Divergence, error) {
	t.Helper()
	ra, err := NewReader(bytes.NewReader(write(t, a)))
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	rb, err := NewReader(bytes.NewReader(write(t, b)))
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	return Compare(ra, rb, opts)
}
