package ecoffelf

import (
	"bytes"
	"fmt"

	"go.chrisrx.dev/psx/format/ecoff"
)

// describe adds to u what a source file's stabs say: its types, its
// variables, each function's result, parameters and locals, and the blocks
// the locals are in. Stack locations are offsets from the stack pointer on
// entry, which is the frame base each function has (the CFA). globals has
// the addresses of the program's global variables, which their stabs leave
// to the symbol table.
func describe(u *unit, sf *ecoff.SourceFile, globals map[string]uint32) error {
	sb := newStabsFile()
	d := &describer{u: u, types: map[*stype]*die{}}
	funcAt := map[uint32]int{}
	for i, f := range u.funcs {
		funcAt[f.addr] = i
	}

	fn := -1 // the function the stabs are in, an index into u.funcs
	var pending []*die
	var blocks []*die
	endFunc := func() {
		if fn >= 0 {
			u.funcs[fn].body = append(u.funcs[fn].body, pending...)
		}
		pending, blocks, fn = nil, nil, -1
	}

	for _, s := range sf.Symbols {
		if !s.IsStab() {
			continue
		}
		switch s.Stab() {
		case stabLBRAC:
			b := &die{tag: tagLexicalBlock, children: pending}
			pending = nil
			b.add(atLowPC, formAddr, sf.Address+s.Value)
			if len(blocks) > 0 {
				blocks[len(blocks)-1].child(b)
			} else if fn >= 0 {
				u.funcs[fn].body = append(u.funcs[fn].body, b)
			}
			blocks = append(blocks, b)
			continue
		case stabRBRAC:
			if len(blocks) == 0 {
				continue
			}
			b := blocks[len(blocks)-1]
			blocks = blocks[:len(blocks)-1]
			low := b.attrs[0].val.(uint32)
			if high := sf.Address + s.Value; high > low {
				b.add(atHighPC, formData4, high-low)
			}
			continue
		}

		st, ok, err := sb.parseStab(s.Name)
		if err != nil {
			// A stab this does not understand leaves its symbol without a
			// type, rather than the program without debugging information.
			d.skipped++
			continue
		}
		if !ok {
			continue
		}
		switch s.Stab() {
		case stabFUN:
			endFunc()
			if i, ok := funcAt[s.Value]; ok {
				fn = i
				if st.typ != nil {
					u.funcs[i].ret = d.typ(st.typ)
				}
			}
		case stabLSYM:
			switch st.class {
			case 't', 'T':
				if st.typ != nil {
					d.typ(st.typ)
				}
			case 0:
				pending = append(pending, d.variable(st, fbreg(int32(s.Value))))
			}
		case stabPSYM:
			if fn >= 0 {
				u.funcs[fn].body = append(u.funcs[fn].body, d.param(st, fbreg(int32(s.Value))))
			}
		case stabRSYM:
			loc := []byte{opReg0 + byte(s.Value&31)}
			switch {
			case st.class == 'P' || st.class == 'R':
				if fn >= 0 {
					u.funcs[fn].body = append(u.funcs[fn].body, d.param(st, loc))
				}
			default:
				pending = append(pending, d.variable(st, loc))
			}
		case stabSTSYM, stabLCSYM:
			v := d.variable(st, addr(s.Value))
			if st.class == 'V' && fn >= 0 {
				pending = append(pending, v)
			} else {
				u.decls = append(u.decls, v)
			}
		case stabGSYM:
			a, ok := globals[st.name]
			if !ok {
				continue
			}
			v := d.variable(st, addr(a))
			v.add(atExternal, formFlag, true)
			u.decls = append(u.decls, v)
		}
	}
	endFunc()
	u.decls = append(d.decls, u.decls...)
	return nil
}

func fbreg(off int32) []byte {
	var b bytes.Buffer
	b.WriteByte(opFbreg)
	sleb(&b, int64(off))
	return b.Bytes()
}

func addr(a uint32) []byte {
	var b bytes.Buffer
	b.WriteByte(opAddr)
	u32(&b, a)
	return b.Bytes()
}

// A describer makes a unit's type entries, once each.
type describer struct {
	u       *unit
	types   map[*stype]*die
	decls   []*die
	skipped int // stabs not understood
}

func (d *describer) variable(st stab, loc []byte) *die {
	v := &die{tag: tagVariable}
	v.add(atName, formString, st.name)
	if t := d.typ(st.typ); t != nil {
		v.add(atType, formRef4, t)
	}
	v.add(atLocation, formExprloc, loc)
	return v
}

func (d *describer) param(st stab, loc []byte) *die {
	v := d.variable(st, loc)
	v.tag = tagFormalParameter
	return v
}

// typ returns t's entry, making it, and the entries it refers to, the first
// time. It returns nil for void.
func (d *describer) typ(t *stype) *die {
	if t == nil {
		return nil
	}
	if e, ok := d.types[t]; ok {
		return e
	}
	switch t.kind {
	case kindVoid:
		return nil
	case kindTypedef:
		if t.name == "" {
			// An alias with no name of its own, such as a const.
			return d.typ(t.elem)
		}
	case kindPending:
		if t.name == "" {
			return nil
		}
	}
	if t.typedefName != "" {
		// The typedef, made first, is what the type's number refers to.
		td := &die{tag: tagTypedef}
		d.types[t] = td
		d.decls = append(d.decls, td)
		td.add(atName, formString, t.typedefName)
		inner := *t
		inner.typedefName = ""
		if e := d.typ(&inner); e != nil {
			td.add(atType, formRef4, e)
		}
		return td
	}
	e := &die{}
	d.types[t] = e // before the parts, which may refer back to it
	d.decls = append(d.decls, e)
	switch t.kind {
	case kindBase:
		finishBase(t)
		e.tag = tagBaseType
		e.add(atName, formString, cmpName(t.name, baseName(t)))
		e.add(atByteSize, formData1, t.size)
		e.add(atEncoding, formData1, uint32(t.enc))
	case kindReference:
		e.tag = tagReferenceType
		e.add(atByteSize, formData1, uint32(4))
		if el := d.typ(t.elem); el != nil {
			e.add(atType, formRef4, el)
		}
	case kindPointer:
		e.tag = tagPointerType
		e.add(atByteSize, formData1, uint32(4))
		if el := d.typ(t.elem); el != nil {
			e.add(atType, formRef4, el)
		}
	case kindTypedef:
		e.tag = tagTypedef
		e.add(atName, formString, t.name)
		if el := d.typ(t.elem); el != nil {
			e.add(atType, formRef4, el)
		}
	case kindArray:
		e.tag = tagArrayType
		if el := d.typ(t.elem); el != nil {
			e.add(atType, formRef4, el)
		}
		r := e.child(&die{tag: tagSubrangeType})
		if t.lo != 0 {
			r.add(lowerBoundAttribute, formSdata, t.lo)
		}
		if t.hi >= t.lo {
			r.add(atUpperBound, formSdata, t.hi)
		}
	case kindStruct, kindUnion, kindPending:
		e.tag = tagStructureType
		if t.kind == kindUnion {
			e.tag = tagUnionType
		}
		if t.name != "" {
			e.add(atName, formString, t.name)
		}
		if t.kind == kindPending || (t.size == 0 && len(t.fields) == 0) {
			e.add(atDeclaration, formFlagPresent, true)
			break
		}
		e.add(atByteSize, formUdata, t.size)
		for _, b := range t.bases {
			in := e.child(&die{tag: tagInheritance})
			if bt := d.typ(b.typ); bt != nil {
				in.add(atType, formRef4, bt)
			}
			in.add(atDataMemberLoc, formUdata, b.offset)
			if b.virtual {
				in.add(atVirtuality, formData1, uint32(1))
			}
		}
		for _, f := range t.fields {
			m := e.child(&die{tag: tagMember})
			m.add(atName, formString, f.name)
			ft := d.typ(f.typ)
			if ft != nil {
				m.add(atType, formRef4, ft)
			}
			if size := typeSize(f.typ); f.bitOff%8 == 0 && (f.bits == 0 || f.bits == size*8 || size == 0) {
				m.add(atDataMemberLoc, formUdata, f.bitOff/8)
			} else {
				m.add(atDataBitOffset, formUdata, f.bitOff)
				m.add(atBitSize, formUdata, f.bits)
			}
		}
	case kindEnum:
		e.tag = tagEnumerationType
		if t.name != "" {
			e.add(atName, formString, t.name)
		}
		e.add(atByteSize, formData1, uint32(4))
		for _, v := range t.enumVals {
			c := e.child(&die{tag: tagEnumerator})
			c.add(atName, formString, v.name)
			c.add(atConstValue, formSdata, v.value)
		}
	case kindFunc:
		e.tag = tagSubroutineType
		if r := d.typ(t.elem); r != nil {
			e.add(atType, formRef4, r)
		}
	default:
		panic(fmt.Sprintf("type kind %d", t.kind))
	}
	return e
}

// typeSize is t's size in bytes, through typedefs, 0 if not known.
func typeSize(t *stype) uint32 {
	for i := 0; t != nil && i < 32; i++ {
		switch t.kind {
		case kindTypedef:
			t = t.elem
		case kindArray:
			n := t.hi - t.lo + 1
			if n < 0 {
				return 0
			}
			return uint32(n) * typeSize(t.elem)
		default:
			return t.size
		}
	}
	return 0
}

// baseName names a base type no stab named, from what it is.
func baseName(t *stype) string {
	switch t.enc {
	case ateFloat:
		if t.size == 4 {
			return "float"
		}
		return "double"
	case ateSigned:
		return fmt.Sprintf("int%d", t.size*8)
	default:
		return fmt.Sprintf("uint%d", t.size*8)
	}
}

func cmpName(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
