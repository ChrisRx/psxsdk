package main

type Tex struct {
	Page uint16
}

type S struct {
	word  uint32
	Tex   Tex
	Blend uint8
}

//go:onstore Blend Tex
func (s *S) pack() {
	s.word = uint32(s.Blend) | uint32(s.Tex.Page)
}

func set(p *uint8) { *p = 1 }

func (t *Tex) setPage() { t.Page = 1 }

// Taking a watched field's address is refused.
func escape(s *S) {
	set(&s.Blend)
}

// So is calling a method with a pointer receiver on a watched field.
func method(s *S) {
	s.Tex.setPage()
}

// And keeping a pointer to what is inside one.
var kept *uint16

func keep(s *S) {
	kept = &s.Tex.Page
}

// A watched field's address used to store and load is allowed.
func fine(s *S) uint8 {
	s.Blend++
	return s.Blend
}

type Bad struct{ x int }

//go:onstore y
func (b *Bad) pack() {}
