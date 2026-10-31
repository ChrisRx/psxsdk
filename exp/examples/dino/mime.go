package main

import "go.chrisrx.dev/psx/exp/ps"

// The animation: the demo's mime.c and control.c.
//
// Every frame the model's vertices are put back from a copy taken at
// startup, and then each set of differences from the VDF file is added
// in its proportion for the frame, through the GTE's vertex
// interpolation, ps.Blend.

const (
	mimeMax = 16

	// maxVertices is the most vertices the copy of the model's can hold,
	// which is more than the dinosaur has.
	maxVertices = 2048
)

// A mdfObject is one set of differences: for which object, from which
// vertex, and the differences themselves.
type mdfObject struct {
	object int
	offset int
	diffs  []ps.Vertex
}

var (
	orgvtx   [maxVertices]ps.Vertex // the vertices as they came
	mdfvtx0  [mimeMax]mdfObject
	mdfTotal int
	mimepr   [mimeMax]int // the proportion of each set this frame
)

// setVertexMimeData saves the vertices as they came and reads the sets of
// differences from the VDF file, if the copy has room for the model.
func setVertexMimeData() {
	n := 0
	for i := range objnum {
		n += len(model.Vertices(i))
	}
	if n > maxVertices {
		objnum = 0
	}
	originalVertexSave()
	setMdfVertex(vdfData[:])
}

// vertexMime puts the vertices back and adds each set in its proportion.
func vertexMime() {
	resetMimeVertex()
	setMimeVertex()
}

// setMdfVertex reads the VDF: a count, then for each set the object, the
// offset, the count of vertices, and the differences themselves, two
// words each.
func setMdfVertex(words []uint32) {
	mdfTotal = int(min(words[0], mimeMax))
	p := 1
	for i := range mdfTotal {
		m := &mdfvtx0[i]
		m.object, m.offset = int(words[p]), int(words[p+1])
		total := int(words[p+2])
		p += 3
		m.diffs = ps.Vertices(words[p : p+2*total])
		p += 2 * total
	}
}

func originalVertexSave() {
	n := 0
	for j := range objnum {
		n += copy(orgvtx[n:], model.Vertices(j))
	}
}

func resetMimeVertex() {
	n := 0
	for j := range objnum {
		n += copy(model.Vertices(j), orgvtx[n:])
	}
}

func setMimeVertex() {
	for i := range mdfTotal {
		m := &mdfvtx0[i]
		if mimepr[i] == 0 || m.object >= objnum {
			continue
		}
		verts := model.Vertices(m.object)
		if m.offset+len(m.diffs) > len(verts) {
			continue
		}
		ps.Blend(verts[m.offset:], m.diffs, ps.Fixed(mimepr[i]))
	}
}

// The transfer functions: each is a filter over the last num frames of
// its input, weighted by a window of num values in 4096 units.

const (
	ctlMax  = 16
	ctlTime = 64
)

type ctlFunc struct {
	cnv []int // the window
	in  int
	out int
}

var (
	cntrlarry [ctlMax][ctlTime]int // each function's last inputs
	ctlfc     [ctlMax]ctlFunc
)

// The windows: a hump over sixteen frames, which the legs use; a sharp
// rise and a long fall that goes below zero, for the mouth; and a slower
// hump over twenty-four frames, with a dip after, for the rest.
var (
	cnv0 = [16]int{
		39, 79, 118, 197, 315, 394, 433, 473,
		473, 433, 394, 315, 197, 118, 79, 39,
	}
	cnv2 = [12]int{
		400, 900, 1000, 900, 600, 400, 200, 110,
		-100, -200, -80, -20,
	}
	cnv3 = [24]int{
		200, 325, 450, 475, 500, 475, 450, 375,
		300, 250, 200, 150, 100, 80, 55, 0,
		-50, -75, -100, -70, -40, -25, -10, -5,
	}
)

// initCntrlarry gives every function the same window and clears its
// history.
func initCntrlarry(cnv []int) {
	for i := range ctlfc {
		ctlfc[i] = ctlFunc{cnv: cnv}
		for j := range cntrlarry[i] {
			cntrlarry[i][j] = 0
		}
	}
}

// setCntrl records each function's input for frame frm and works out its
// output: the inputs of the last num frames weighted by the window. frm
// is unsigned, as the original's frame count was, so that in the first
// frames the history is read from where the original read it: frm-j
// wraps rather than going negative, and a window of 12 or 24 does not
// divide the wrap evenly.
func setCntrl(frm uint32) {
	for i := range ctlfc {
		f := &ctlfc[i]
		num := uint32(len(f.cnv))
		cntrlarry[i][frm%num] = f.in
		f.out = 0
		for j, w := range f.cnv {
			f.out += cntrlarry[i][(frm-uint32(j))%num] * w
		}
		f.out >>= 12
	}
}
