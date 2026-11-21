package ecoffelf

import (
	"testing"

	"go.chrisrx.dev/x/assert"
)

func parseAll(t *testing.T, stabs ...string) *stabsFile {
	t.Helper()
	f := newStabsFile()
	for _, s := range stabs {
		if _, _, err := f.parseStab(s); err != nil {
			t.Fatalf("%v", err)
		}
	}
	return f
}

// TestStabsBase checks the base types GCC 2.95.3 writes as ranges.
func TestStabsBase(t *testing.T) {
	f := parseAll(t,
		"int:t1=r1;-2147483648;2147483647;",
		"char:t2=r2;0;127;",
		"unsigned int:t4=r4;0;-1;",
		"long long int:t6=r1;01000000000000000000000;0777777777777777777777;",
		"long long unsigned int:t7=r1;0000000000000;01777777777777777777777;",
		"short unsigned int:t9=r9;0;65535;",
		"float:t12=r1;4;0;",
		"double:t13=r1;8;0;",
		"void:t19=19",
	)
	for n, want := range map[int][2]uint32{
		1: {4, ateSigned}, 4: {4, ateUnsigned}, 6: {8, ateSigned}, 7: {8, ateUnsigned},
		9: {2, ateUnsigned}, 12: {4, ateFloat}, 13: {8, ateFloat},
	} {
		typ := f.types[[2]int{0, n}]
		assert.Equal(t, want, [2]uint32{typ.size, uint32(typ.enc)}, n)
	}
	c := f.types[[2]int{0, 2}]
	finishBase(c)
	assert.Equal(t, [2]uint32{1, ateSignedChar}, [2]uint32{c.size, uint32(c.enc)})
	assert.Equal(t, kindVoid, f.types[[2]int{0, 19}].kind)
}

// TestStabsStruct checks a struct with nested definitions and bit fields,
// and a typedef that defines its struct in place, as older GCCs wrote
// typedef struct {...} T.
func TestStabsStruct(t *testing.T) {
	f := parseAll(t,
		"int:t1=r1;-2147483648;2147483647;",
		"short int:t8=r8;-32768;32767;",
		"MATRIX:t52=53=s32m:54=ar1;0;2;55=ar1;0;2;8,0,144;t:56=ar1;0;2;1,160,96;;",
		"flags:T22=s8a:4,0,3;b:4,3,5;c:1,32,32;;",
		"RECT:t29=s8x:8,0,16;y:8,16,16;w:8,32,16;h:8,48,16;;",
	)
	m := f.types[[2]int{0, 52}]
	assert.Equal(t, kindTypedef, m.kind)
	assert.Equal(t, "MATRIX", m.name)
	s := m.elem
	assert.Equal(t, kindStruct, s.kind)
	assert.Equal(t, uint32(32), s.size)
	assert.Equal(t, 2, len(s.fields))
	assert.Equal(t, kindArray, s.fields[0].typ.kind)
	assert.Equal(t, int64(2), s.fields[0].typ.hi)
	assert.Equal(t, kindArray, s.fields[0].typ.elem.kind)
	assert.Equal(t, uint32(144), s.fields[0].bits)

	fl := f.types[[2]int{0, 22}]
	assert.Equal(t, "flags", fl.name)
	assert.Equal(t, [2]uint32{3, 5}, [2]uint32{fl.fields[1].bitOff, fl.fields[1].bits})

	r := f.types[[2]int{0, 29}]
	assert.Equal(t, kindStruct, r.kind)
	assert.Equal(t, "", r.name)
	assert.Equal(t, "RECT", r.typedefName)
}

// TestStabsCPlusPlus parses what g++ 2.95.3 writes for a class with a
// virtual destructor, private members, constructors and a method: the type
// of this, Ball *, is defined only inside the method list.
func TestStabsCPlusPlus(t *testing.T) {
	f := parseAll(t,
		"int:t1=r1;-2147483648;2147483647;",
		"void:t20=20",
		"bool:t19=@s32;-16;",
		"__vtbl_ptr_type:t22=s8__delta:8,0,16;__index:8,16,16;__pfn:23=*20,32,32;__delta2:8,32,16;;",
		"Ball:T24=s20x:1,0,32;y:1,32,32;dx:/01,64,32;dy:/01,96,32;$vf24:25=*22,128;__as::26=##27=&24;:RC4Ball;2A.;Ball::28=##29=*24;:RC4Ball;2A.30=##29;:ii;2A.31=#24,20,29,1,20;:_$_4Ball;2A*1;24;;move::32=##20;:;2A.;;~%24;",
		"Ball:Tt24",
	)
	b := f.types[[2]int{0, 24}]
	assert.Equal(t, kindStruct, b.kind)
	assert.Equal(t, "Ball", b.name)
	var names []string
	for _, fl := range b.fields {
		names = append(names, fl.name)
	}
	assert.Equal(t, []string{"x", "y", "dx", "dy", "$vf24"}, names)
	assert.Equal(t, uint32(0), b.fields[4].bits)
	this := f.types[[2]int{0, 29}]
	assert.Equal(t, kindPointer, this.kind)
	assert.Equal(t, b, this.elem)
	assert.Equal(t, kindReference, f.types[[2]int{0, 27}].kind)
	bl := f.types[[2]int{0, 19}]
	assert.Equal(t, [2]uint32{4, ateBoolean}, [2]uint32{bl.size, uint32(bl.enc)})
}

func TestDemangle(t *testing.T) {
	for in, want := range map[string]string{
		"move__4Ball":    "Ball::move",
		"__4Ballii":      "Ball::Ball",
		"_$_4Ball":       "Ball::~Ball",
		"next__7Counter": "Counter::next",
		"__nw__FUiPv":    "operator new",
		"draw__Fii":      "draw",
		"main":           "main",
	} {
		got, _ := demangle(in)
		assert.Equal(t, want, got, in)
	}
}
