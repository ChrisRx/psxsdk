package ecoffelf

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

// The stabs GCC 2.95.3 writes for C, as binutils' include/aout/stab.def
// names them. In ECOFF they are local symbols marked as stabs, with the
// stab's type in the symbol's index and its value in the symbol's value;
// the desc field of a.out stabs has no place.
const (
	stabGSYM  = 0x20 // global variable: its address is the symbol's
	stabFUN   = 0x24 // function
	stabSTSYM = 0x26 // static data
	stabLCSYM = 0x28 // static bss
	stabRSYM  = 0x40 // register variable or parameter
	stabLSYM  = 0x80 // stack local, or a type
	stabPSYM  = 0xa0 // stack parameter
	stabLBRAC = 0xc0 // block start, an offset from the file's start
	stabRBRAC = 0xe0 // block end
)

// A typeKind is the kind of a stabs type.
type typeKind int

const (
	kindPending typeKind = iota // referred to, not yet defined
	kindVoid
	kindBase
	kindPointer
	kindArray
	kindStruct
	kindUnion
	kindEnum
	kindFunc
	kindTypedef
	kindReference
)

// A stype is a type the stabs define, by number within its file.
type stype struct {
	kind typeKind
	name string // the base type's, the tag's or the typedef's

	// typedefName names a struct, union or enum that a typedef defines in
	// place, as older GCCs write typedef struct {...} T: the type is the
	// anonymous one, named by a typedef.
	typedefName string
	size        uint32 // in bytes, for base, struct, union and enum types

	enc byte // a base type's DW_ATE encoding

	elem     *stype // a pointer's, an array's, a function's result, a typedef's
	lo, hi   int64  // an array's bounds
	fields   []field
	enumVals []enumVal
	bases    []base // a C++ class's base classes

	die *die // its entry, once written
}

type field struct {
	name         string
	typ          *stype
	bitOff, bits uint32 // bits is 0 where the type's size is the field's
}

type base struct {
	typ     *stype
	offset  uint32 // in bytes
	virtual bool
}

type enumVal struct {
	name  string
	value int64
}

// DWARF base type encodings.
const (
	ateFloat        = 0x04
	ateSigned       = 0x05
	ateSignedChar   = 0x06
	ateUnsigned     = 0x07
	ateUnsignedChar = 0x08
	ateComplexFloat = 0x03
	ateBoolean      = 0x02
)

// A stabsFile holds one source file's types, by their numbers.
type stabsFile struct {
	types map[[2]int]*stype
}

func newStabsFile() *stabsFile {
	return &stabsFile{types: map[[2]int]*stype{}}
}

// lookup returns the type numbered n, a pending one if not yet defined.
func (f *stabsFile) lookup(n [2]int) *stype {
	t, ok := f.types[n]
	if !ok && n[1] < 0 {
		t = builtin(n[1], 0)
		f.types[n] = t
		return t
	}
	if !ok {
		t = &stype{kind: kindPending}
		f.types[n] = t
	}
	return t
}

// A stab is a parsed symbol stab: "name:" a class letter and a type.
type stab struct {
	name  string
	class byte // 't', 'T', 'G', 'S', 'V', 'F', 'f', 'p', 'P', 'R', 'r', or 0 for a stack local
	typ   *stype
}

// parseStab parses a stab's string, defining the types it defines on the
// way. A stab that is no symbol, such as a file name, returns ok false.
func (f *stabsFile) parseStab(s string) (st stab, ok bool, err error) {
	i := strings.IndexByte(s, ':')
	if i < 0 {
		return stab{}, false, nil
	}
	st.name = s[:i]
	p := &parser{s: s, pos: i + 1, f: f}
	if p.pos < len(s) {
		c := s[p.pos]
		if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' {
			st.class = c
			p.pos++
			// Tt: a C++ class, which is a tag and a typedef at once.
			if c == 'T' && p.peek() == 't' {
				p.pos++
			}
		}
	}
	if p.pos >= len(s) {
		return st, true, nil
	}
	t, err := p.typ()
	if err != nil {
		return st, false, fmt.Errorf("stab %q: %w", s, err)
	}
	st.typ = t
	// A tag or typedef names the type it defines: a typedef that defines
	// a struct, union or enum in place names it as a typedef, not a tag.
	if (st.class == 't' || st.class == 'T') && t.name == "" && st.name != "" && st.name != " " {
		switch {
		case st.class == 't' && (t.kind == kindStruct || t.kind == kindUnion || t.kind == kindEnum):
			if t.typedefName == "" {
				t.typedefName = st.name
			}
		default:
			t.name = st.name
		}
	}
	return st, true, nil
}

type parser struct {
	s   string
	pos int
	f   *stabsFile
}

func (p *parser) peek() byte {
	if p.pos < len(p.s) {
		return p.s[p.pos]
	}
	return 0
}

func (p *parser) expect(c byte) error {
	if p.peek() != c {
		return fmt.Errorf("want %q at %d, have %q", c, p.pos, p.s[min(p.pos, len(p.s)):])
	}
	p.pos++
	return nil
}

// number reads a type number: n, or (file,n).
func (p *parser) number() ([2]int, error) {
	if p.peek() == '(' {
		p.pos++
		a, err := p.int()
		if err != nil {
			return [2]int{}, err
		}
		if err := p.expect(','); err != nil {
			return [2]int{}, err
		}
		b, err := p.int()
		if err != nil {
			return [2]int{}, err
		}
		return [2]int{int(a), int(b)}, p.expect(')')
	}
	n, err := p.int()
	return [2]int{0, int(n)}, err
}

func (p *parser) int() (int64, error) {
	start := p.pos
	if p.peek() == '-' {
		p.pos++
	}
	for p.peek() >= '0' && p.peek() <= '9' {
		p.pos++
	}
	return strconv.ParseInt(p.s[start:p.pos], 10, 64)
}

// bound reads a range bound up to its ';', returning it as a big integer:
// GCC writes the bounds of 64-bit types in octal, with a leading 0.
func (p *parser) bound() (*big.Int, string, error) {
	end := strings.IndexByte(p.s[p.pos:], ';')
	if end < 0 {
		return nil, "", fmt.Errorf("unterminated bound at %d", p.pos)
	}
	text := p.s[p.pos : p.pos+end]
	p.pos += end + 1
	v := new(big.Int)
	base := 10
	if len(text) > 1 && text[0] == '0' {
		base = 8
	}
	if _, ok := v.SetString(text, base); !ok {
		return nil, "", fmt.Errorf("bad bound %q", text)
	}
	return v, text, nil
}

// typ reads a type: a number, which defines it if '=' follows.
func (p *parser) typ() (*stype, error) {
	c := p.peek()
	if !(c >= '0' && c <= '9' || c == '(' || c == '-') {
		// An anonymous type, defined in place.
		t := &stype{}
		return t, p.def(t, [2]int{-1, -1})
	}
	n, err := p.number()
	if err != nil {
		return nil, err
	}
	t := p.f.lookup(n)
	if p.peek() != '=' {
		return t, nil
	}
	p.pos++
	return t, p.def(t, n)
}

// def reads the definition of t, numbered n.
func (p *parser) def(t *stype, n [2]int) error {
	// Attributes come first: @s32; gives the size in bits.
	bitSize := 0
	for p.peek() == '@' {
		end := strings.IndexByte(p.s[p.pos:], ';')
		if end < 0 {
			return fmt.Errorf("unterminated attribute")
		}
		if a := p.s[p.pos+1 : p.pos+end]; strings.HasPrefix(a, "s") {
			bitSize, _ = strconv.Atoi(a[1:])
		}
		p.pos += end + 1
	}
	switch c := p.peek(); {
	case c >= '0' && c <= '9' || c == '(' || c == '-':
		// Another type, by number: a type that is itself is void, and a
		// negative number is a built-in type.
		m, err := p.number()
		if err != nil {
			return err
		}
		if m[1] < 0 {
			*t = *builtin(m[1], bitSize)
			return nil
		}
		if m == n {
			t.kind = kindVoid
			return nil
		}
		other := p.f.lookup(m)
		if p.peek() == '=' {
			p.pos++
			if err := p.def(other, m); err != nil {
				return err
			}
		}
		t.kind, t.elem = kindTypedef, other
		return nil
	case c == 'r':
		p.pos++
		if _, err := p.typ(); err != nil {
			return err
		}
		if err := p.expect(';'); err != nil {
			return err
		}
		lo, _, err := p.bound()
		if err != nil {
			return err
		}
		hi, hiText, err := p.bound()
		if err != nil {
			return err
		}
		rangeType(t, lo, hi, hiText)
		return nil
	case c == '*':
		p.pos++
		e, err := p.typ()
		t.kind, t.elem, t.size = kindPointer, e, 4
		return err
	case c == '&':
		p.pos++
		e, err := p.typ()
		t.kind, t.elem, t.size = kindReference, e, 4
		return err
	case c == '#':
		// A C++ method's type: ##result; or #class,result,arg,...;
		p.pos++
		if p.peek() == '#' {
			p.pos++
			r, err := p.typ()
			if err != nil {
				return err
			}
			t.kind, t.elem = kindFunc, r
			return p.expect(';')
		}
		var r *stype
		for i := 0; ; i++ {
			a, err := p.typ()
			if err != nil {
				return err
			}
			if i == 1 {
				r = a
			}
			if p.peek() == ';' {
				p.pos++
				break
			}
			if err := p.expect(','); err != nil {
				return err
			}
		}
		t.kind, t.elem = kindFunc, r
		return nil
	case c == 'k' || c == 'B':
		// const and volatile, which change nothing a debugger shows here.
		p.pos++
		e, err := p.typ()
		t.kind, t.elem = kindTypedef, e
		return err
	case c == 'a':
		p.pos++
		if err := p.expect('r'); err != nil {
			return err
		}
		if _, err := p.typ(); err != nil {
			return err
		}
		if err := p.expect(';'); err != nil {
			return err
		}
		lo, _, err := p.bound()
		if err != nil {
			return err
		}
		hi, _, err := p.bound()
		if err != nil {
			return err
		}
		e, err := p.typ()
		t.kind, t.elem, t.lo, t.hi = kindArray, e, lo.Int64(), hi.Int64()
		return err
	case c == 's' || c == 'u':
		p.pos++
		t.kind = kindStruct
		if c == 'u' {
			t.kind = kindUnion
		}
		size, err := p.int()
		if err != nil {
			return err
		}
		t.size = uint32(size)
		// C++ base classes: !count, then each as virtual, visibility,
		// offset in bits and type.
		if p.peek() == '!' {
			p.pos++
			nb, err := p.int()
			if err != nil {
				return err
			}
			if err := p.expect(','); err != nil {
				return err
			}
			for range nb {
				virt := p.peek() == '1'
				p.pos += 2 // virtual and visibility
				off, err := p.int()
				if err != nil {
					return err
				}
				if err := p.expect(','); err != nil {
					return err
				}
				bt, err := p.typ()
				if err != nil {
					return err
				}
				if err := p.expect(';'); err != nil {
					return err
				}
				t.bases = append(t.bases, base{typ: bt, offset: uint32(off / 8), virtual: virt})
			}
		}
		for p.peek() != ';' {
			colon := strings.IndexByte(p.s[p.pos:], ':')
			if colon < 0 {
				return fmt.Errorf("unterminated field at %d", p.pos)
			}
			name := p.s[p.pos : p.pos+colon]
			p.pos += colon + 1
			if p.peek() == ':' {
				// A C++ method and its overloads, which define types.
				p.pos++
				if err := p.methods(); err != nil {
					return err
				}
				continue
			}
			fl := field{name: name}
			if p.peek() == '/' {
				p.pos += 2 // C++ visibility
			}
			if fl.typ, err = p.typ(); err != nil {
				return err
			}
			if p.peek() == ':' {
				// A static member: its symbol's name, which a debugger
				// finds by itself.
				end := strings.IndexByte(p.s[p.pos:], ';')
				if end < 0 {
					return fmt.Errorf("unterminated static member at %d", p.pos)
				}
				p.pos += end + 1
				continue
			}
			if err := p.expect(','); err != nil {
				return err
			}
			off, err := p.int()
			if err != nil {
				return err
			}
			// The vtable pointer has an offset and no size.
			bits := int64(0)
			if p.peek() == ',' {
				p.pos++
				if bits, err = p.int(); err != nil {
					return err
				}
			}
			if err := p.expect(';'); err != nil {
				return err
			}
			fl.bitOff, fl.bits = uint32(off), uint32(bits)
			t.fields = append(t.fields, fl)
		}
		p.pos++
		// ~%type; names the class holding the vtable pointer.
		if strings.HasPrefix(p.s[p.pos:], "~%") {
			p.pos += 2
			if _, err := p.typ(); err != nil {
				return err
			}
			if p.peek() == ';' {
				p.pos++
			}
		}
		return nil
	case c == 'e':
		p.pos++
		t.kind, t.size = kindEnum, 4
		for p.peek() != ';' {
			colon := strings.IndexByte(p.s[p.pos:], ':')
			if colon < 0 {
				return fmt.Errorf("unterminated enumerator at %d", p.pos)
			}
			name := p.s[p.pos : p.pos+colon]
			p.pos += colon + 1
			v, err := p.int()
			if err != nil {
				return err
			}
			if err := p.expect(','); err != nil {
				return err
			}
			t.enumVals = append(t.enumVals, enumVal{name, v})
		}
		p.pos++
		return nil
	case c == 'f':
		p.pos++
		e, err := p.typ()
		t.kind, t.elem = kindFunc, e
		return err
	case c == 'x':
		// A reference to a tag not yet defined: struct, union or enum.
		p.pos++
		k := p.peek()
		p.pos++
		colon := strings.IndexByte(p.s[p.pos:], ':')
		if colon < 0 {
			return fmt.Errorf("unterminated cross reference at %d", p.pos)
		}
		name := p.s[p.pos : p.pos+colon]
		p.pos += colon + 1
		if t.kind == kindPending {
			t.name = name
			switch k {
			case 'u':
				t.kind = kindUnion
			case 'e':
				t.kind, t.size = kindEnum, 4
			default:
				t.kind = kindStruct
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown type descriptor %q at %d", c, p.pos)
	}
}

// rangeType makes t the base type a range describes, as GCC writes them:
// a float as a range of its size in bytes to 0, an integer by its bounds.
func rangeType(t *stype, lo, hi *big.Int, hiText string) {
	t.kind = kindBase
	switch {
	case hi.Sign() == 0 && lo.Sign() > 0:
		t.size, t.enc = uint32(lo.Int64()), ateFloat
	case lo.Sign() == 0 && hiText == "-1":
		t.size, t.enc = 4, ateUnsigned
	case lo.Cmp(hi) > 0:
		// A signed type too wide for the bounds to be written as signed
		// numbers, such as long long: GCC writes the lower bound's bits in
		// octal, the sign bit set, so it reads as more than the upper.
		t.size, t.enc = uint32((hi.BitLen()+1+7)/8), ateSigned
	default:
		bits := hi.BitLen()
		if lo.Sign() < 0 {
			bits = max(bits, new(big.Int).Neg(lo).BitLen()-1) + 1
		}
		switch {
		case bits <= 8:
			t.size = 1
		case bits <= 16:
			t.size = 2
		case bits <= 32:
			t.size = 4
		default:
			t.size = 8
		}
		t.enc = ateUnsigned
		if lo.Sign() < 0 {
			t.enc = ateSigned
		}
	}
}

// finishBase gives named base types the encoding their names say: char is
// a character, and a complex type is complex.
func finishBase(t *stype) {
	if t.kind != kindBase {
		return
	}
	switch {
	case strings.Contains(t.name, "complex") && t.enc == ateFloat:
		t.enc, t.size = ateComplexFloat, t.size*2
	case t.size == 1 && strings.Contains(t.name, "char"):
		if t.enc == ateSigned || t.name == "char" {
			t.enc = ateSignedChar
		} else {
			t.enc = ateUnsignedChar
		}
	}
}

// methods reads a C++ method's overloads, after its name's "::", up to the
// ';' that ends them: each a type, ':' and a physical name to ';', then
// visibility, qualifiers and kind, a virtual one's vtable index and class.
func (p *parser) methods() error {
	for p.peek() != ';' {
		if _, err := p.typ(); err != nil {
			return err
		}
		if err := p.expect(':'); err != nil {
			return err
		}
		end := strings.IndexByte(p.s[p.pos:], ';')
		if end < 0 {
			return fmt.Errorf("unterminated method name at %d", p.pos)
		}
		p.pos += end + 1
		p.pos += 2 // visibility and qualifiers
		switch p.peek() {
		case '*':
			p.pos++
			if _, err := p.int(); err != nil {
				return err
			}
			if err := p.expect(';'); err != nil {
				return err
			}
			if _, err := p.typ(); err != nil {
				return err
			}
			if err := p.expect(';'); err != nil {
				return err
			}
		default: // '.' and '?'
			p.pos++
		}
		if p.pos > len(p.s) {
			return fmt.Errorf("method list past the end")
		}
	}
	p.pos++
	return nil
}

// builtin is the stabs built-in type numbered n, which is negative, of
// bitSize bits if an attribute gave one.
func builtin(n int, bitSize int) *stype {
	t := &stype{kind: kindBase}
	set := func(name string, size uint32, enc byte) {
		t.name, t.size, t.enc = name, size, enc
	}
	switch -n {
	case 1, 15, 29:
		set("int", 4, ateSigned)
	case 2:
		set("char", 1, ateSignedChar)
	case 3, 28:
		set("short", 2, ateSigned)
	case 4:
		set("long", 4, ateSigned)
	case 5, 20:
		set("unsigned char", 1, ateUnsignedChar)
	case 6, 27:
		set("signed char", 1, ateSignedChar)
	case 7:
		set("unsigned short", 2, ateUnsigned)
	case 8, 9:
		set("unsigned int", 4, ateUnsigned)
	case 10:
		set("unsigned long", 4, ateUnsigned)
	case 11:
		return &stype{kind: kindVoid}
	case 12, 17:
		set("float", 4, ateFloat)
	case 13, 14, 18:
		set("double", 8, ateFloat)
	case 16, 21, 22, 23, 24:
		set("bool", 4, ateBoolean)
	case 30:
		set("wchar_t", 2, ateUnsigned)
	case 31, 34:
		set("long long", 8, ateSigned)
	case 32:
		set("unsigned long long", 8, ateUnsigned)
	default:
		set(fmt.Sprintf("builtin%d", -n), 4, ateSigned)
	}
	if bitSize > 0 {
		t.size = uint32(bitSize / 8)
	}
	return t
}
