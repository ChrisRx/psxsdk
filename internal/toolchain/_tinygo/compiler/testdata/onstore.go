package main

// The onstore experiment: pack runs after each store to Blend, Unlit or
// Tex, or to what is inside Tex (onstore_test.go).

type Tex struct {
	Page uint16
	U, V uint8
}

type S struct {
	word  uint32
	X     int16
	Tex   Tex
	Blend uint8
	Unlit bool
}

//go:onstore Blend Unlit Tex
func (s *S) pack() {
	w := uint32(s.Blend)<<28 | uint32(s.Tex.Page>>7&3)<<24
	if s.Unlit {
		w |= 1 << 6
	}
	s.word = w
}

// E has an S inside it.
type E struct {
	S
	Speed int
}

// ship's word is packed at compile time.
var ship = S{Blend: 5, Tex: Tex{Page: 2 << 7}, Unlit: true}

// One store, one call.
func storeOne(s *S, b uint8) {
	s.Blend = b
}

// Two stores to the same sprite in a row, one call after the second.
func storeTwo(s *S, b uint8) {
	s.Blend = b
	s.Unlit = true
}

// A store between two others ends the first run.
func storeSplit(s, t *S, b uint8) {
	s.Blend = b
	t.Blend = b
	s.Unlit = true
}

// A store to an element of a slice calls pack on the element.
func storeIndex(a []S, i int, b uint8) {
	a[i].Blend = b
	a[i].Unlit = false
}

// Through embedding.
func storeEmbedded(e *E) {
	e.S.Unlit = true
	e.Blend = 3
}

// Inside a watched field.
func storeNested(s *S, page uint16) {
	s.Tex.Page = page
}

// A field pack does not watch.
func storeUnwatched(s *S, x int16) {
	s.X = x
	s.Tex.U = 1 // but this is in Tex
}

// A copy of the whole struct carries the word with it.
func storeCopy(d, s *S) {
	*d = *s
}

// Reads call nothing.
func read(s *S) uint8 {
	return s.Blend
}
