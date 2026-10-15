package psxtrace

import (
	"fmt"
	"io"
	"slices"
	"strings"
)

// CompareOptions controls what Compare treats as a difference.
type CompareOptions struct {
	// IgnoreCycles leaves the cycle counts out of the comparison. Cycles
	// differing is a timing difference, which only shows up as a different
	// instruction once an interrupt arrives at a different point.
	IgnoreCycles bool

	// Context is how many of the matching records before a divergence to
	// keep, for seeing how the two got there.
	Context int
}

// Divergence is the first record two traces disagree on.
type Divergence struct {
	Index   uint64   // record number, from 0
	A, B    Record   // the records that differ
	Fields  []string // which parts of them differ
	Context []Record // the matching records before it, oldest first
}

func (d *Divergence) String() string {
	return fmt.Sprintf("record %d differs in %s", d.Index, strings.Join(d.Fields, ", "))
}

// Compare reads two traces in step until they disagree, and returns where.
// It stops without a divergence at the end of the shorter trace, since a
// trace is normally cut off at an arbitrary count. n is the number of
// records both traces matched on.
func Compare(a, b *Reader, opts CompareOptions) (n uint64, d *Divergence, err error) {
	var history []Record
	for ; ; n++ {
		ra, err := a.Next()
		if err == io.EOF {
			return n, nil, nil
		}
		if err != nil {
			return n, nil, fmt.Errorf("first trace, record %d: %w", n, err)
		}
		ra = clone(ra) // kept in the history past the next a.Next

		rb, err := b.Next()
		if err == io.EOF {
			return n, nil, nil
		}
		if err != nil {
			return n, nil, fmt.Errorf("second trace, record %d: %w", n, err)
		}

		if fields := differences(ra, rb, opts); len(fields) > 0 {
			return n, &Divergence{
				Index:   n,
				A:       ra,
				B:       clone(rb),
				Fields:  fields,
				Context: history,
			}, nil
		}

		if opts.Context > 0 {
			if len(history) == opts.Context {
				history = slices.Delete(history, 0, 1)
			}
			history = append(history, ra)
		}
	}
}

// differences names the parts of two records that disagree.
func differences(a, b Record, opts CompareOptions) []string {
	var fields []string
	if a.PC != b.PC {
		fields = append(fields, "pc")
	}
	if a.Insn != b.Insn {
		fields = append(fields, "insn")
	}
	if a.Excode != b.Excode {
		fields = append(fields, "exception")
	}
	if !opts.IgnoreCycles && a.Cycles != b.Cycles {
		fields = append(fields, "cycles")
	}
	if !slices.Equal(a.Writes, b.Writes) {
		fields = append(fields, "registers")
	}
	return fields
}

// clone gives a record its own copy of the Writes a Reader reuses.
func clone(r Record) Record {
	r.Writes = slices.Clone(r.Writes)
	return r
}
