package ecoffelf

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
)

// A die is a debugging information entry: a tag, attributes and children.
type die struct {
	tag      uint16
	attrs    []attr
	children []*die

	offset uint32 // within its unit, set when encoded
}

// An attr is an attribute: its name, its form and a value of the Go type
// the form takes: string, uint32 (addr, data1, data2, data4, sec_offset,
// udata), int64 (sdata), bool (flag), []byte (exprloc) or *die (ref4).
type attr struct {
	at, form uint16
	val      any
}

func (d *die) add(at, form uint16, val any) *die {
	d.attrs = append(d.attrs, attr{at, form, val})
	return d
}

func (d *die) child(c *die) *die {
	d.children = append(d.children, c)
	return c
}

// More tags, attributes and forms, from the DWARF 4 standard.
const (
	tagArrayType       = 0x01
	tagEnumerationType = 0x04
	tagFormalParameter = 0x05
	tagLexicalBlock    = 0x0b
	tagMember          = 0x0d
	tagPointerType     = 0x0f
	tagStructureType   = 0x13
	tagSubroutineType  = 0x15
	tagTypedef         = 0x16
	tagUnionType       = 0x17
	tagSubrangeType    = 0x21
	tagBaseType        = 0x24
	tagEnumerator      = 0x28
	tagVariable        = 0x34
	tagReferenceType   = 0x10
	tagInheritance     = 0x1c

	atLocation          = 0x02
	atByteSize          = 0x0b
	atBitSize           = 0x0d
	atConstValue        = 0x1c
	atUpperBound        = 0x2f
	atDataMemberLoc     = 0x38
	atDeclaration       = 0x3c
	atEncoding          = 0x3e
	atType              = 0x49
	atDataBitOffset     = 0x6b
	atVirtuality        = 0x4c
	formData1           = 0x0b
	formSdata           = 0x0d
	formUdata           = 0x0f
	formRef4            = 0x13
	formFlagPresent     = 0x19
	opAddr              = 0x03
	opFbreg             = 0x91
	opReg0              = 0x50
	lowerBoundAttribute = 0x22
)

// encoder writes units' entries and the abbreviations they share.
type encoder struct {
	abbrev bytes.Buffer
	codes  map[string]uint64
}

func newEncoder() *encoder {
	return &encoder{codes: map[string]uint64{}}
}

// code returns d's abbreviation code, adding the abbreviation if new.
func (e *encoder) code(d *die) uint64 {
	var key strings.Builder
	fmt.Fprintf(&key, "%x/%t", d.tag, len(d.children) > 0)
	for _, a := range d.attrs {
		fmt.Fprintf(&key, "/%x:%x", a.at, a.form)
	}
	if c, ok := e.codes[key.String()]; ok {
		return c
	}
	c := uint64(len(e.codes) + 1)
	e.codes[key.String()] = c
	uleb(&e.abbrev, c)
	uleb(&e.abbrev, uint64(d.tag))
	if len(d.children) > 0 {
		e.abbrev.WriteByte(childrenYes)
	} else {
		e.abbrev.WriteByte(childrenNo)
	}
	for _, a := range d.attrs {
		uleb(&e.abbrev, uint64(a.at))
		uleb(&e.abbrev, uint64(a.form))
	}
	e.abbrev.Write([]byte{0, 0})
	return c
}

// unit encodes the compile unit rooted at cu, header and all.
func (e *encoder) unit(cu *die) []byte {
	var body bytes.Buffer
	type patch struct {
		at     int
		target *die
	}
	var patches []patch
	const header = 11 // length, version, abbreviations offset, address size
	var write func(d *die)
	write = func(d *die) {
		d.offset = uint32(header + body.Len())
		uleb(&body, e.code(d))
		for _, a := range d.attrs {
			switch a.form {
			case formString:
				cstring(&body, a.val.(string))
			case formAddr, formData4, formSecOffset:
				u32(&body, a.val.(uint32))
			case formData2:
				u16(&body, uint16(a.val.(uint32)))
			case formData1:
				body.WriteByte(byte(a.val.(uint32)))
			case formFlag:
				if a.val.(bool) {
					body.WriteByte(1)
				} else {
					body.WriteByte(0)
				}
			case formFlagPresent:
			case formUdata:
				uleb(&body, uint64(a.val.(uint32)))
			case formSdata:
				sleb(&body, a.val.(int64))
			case formExprloc:
				b := a.val.([]byte)
				uleb(&body, uint64(len(b)))
				body.Write(b)
			case formRef4:
				patches = append(patches, patch{body.Len(), a.val.(*die)})
				u32(&body, 0)
			default:
				panic(fmt.Sprintf("form %#x", a.form))
			}
		}
		if len(d.children) > 0 {
			for _, c := range d.children {
				write(c)
			}
			body.WriteByte(0)
		}
	}
	write(cu)
	out := body.Bytes()
	for _, p := range patches {
		binary.LittleEndian.PutUint32(out[p.at:], p.target.offset)
	}
	var u bytes.Buffer
	u32(&u, uint32(header-4+len(out)))
	u16(&u, 4)
	u32(&u, 0)
	u.WriteByte(4)
	u.Write(out)
	return u.Bytes()
}
