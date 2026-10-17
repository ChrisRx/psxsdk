package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/examples/internal/demo"
	"go.chrisrx.dev/psx/yaroze"
)

// The animation: the demo's mime.c and control.c.
//
// MIME is the GTE's vertex interpolation: GteMIMefunc adds a set of
// differences to a run of a model's vertices, scaled by a proportion. The
// differences come from the VDF file, which holds sets of them, each for a
// run of one object's vertices from an offset on. Every frame the vertices
// are put back from a copy taken at startup, and then each set is added
// in its proportion for the frame.

const (
	mimeMax = 16

	// maxVertices is the most vertices the copy of the model's can hold,
	// which is more than the dinosaur has.
	maxVertices = 2048
)

// A mdfObject is one set of differences: for which object, from which
// vertex, how many, and where the differences are.
type mdfObject struct {
	top    uintptr // the differences, SVECTORs
	offset uint32
	total  uint32
	object uint32
}

var (
	orgvtx   [maxVertices]yaroze.SVECTOR // the vertices as they came
	mdfvtx0  [mimeMax]mdfObject
	mdfTotal uint32
	mimepr   [mimeMax]int32 // the proportion of each set this frame
)

// objects returns the mapped model's object table.
func objects() *[objectMax]demo.TMD_OBJ {
	return (*[objectMax]demo.TMD_OBJ)(unsafe.Pointer(md0TMD.Table()))
}

func vertAt(o *demo.TMD_OBJ, i uint32) *yaroze.SVECTOR {
	return (*yaroze.SVECTOR)(yaroze.Pointer(uintptr(o.VertTop) + uintptr(i)*unsafe.Sizeof(yaroze.SVECTOR{})))
}

// setTMDData is where the original copied the model's object table; here
// the table is read in place, so there is nothing to do but check that
// the copy of the vertices has room for them.
func setTMDData() {
	var n uint32
	for i := range objnum {
		n += objects()[i].NVert
	}
	if n > maxVertices {
		objnum = 0
	}
}

// setVertexMimeData saves the vertices as they came and reads the sets of
// differences from the VDF file.
func setVertexMimeData() {
	originalVertexSave()
	setMdfVertex(uintptr(unsafe.Pointer(&vdfData[0])))
}

// vertexMime puts the vertices back and adds each set in its proportion.
func vertexMime() {
	resetMimeVertex()
	setMimeVertex()
}

// setMdfVertex reads the VDF: a count, then for each set the object, the
// offset, the count of vertices, and the differences themselves.
func setMdfVertex(addr uintptr) {
	p := addr
	n := *yaroze.At(p)
	p += 4
	mdfTotal = min(n, mimeMax)
	for i := range mdfTotal {
		m := &mdfvtx0[i]
		m.object = *yaroze.At(p)
		m.offset = *yaroze.At(p + 4)
		m.total = *yaroze.At(p + 8)
		m.top = p + 12
		p += 12 + uintptr(m.total)*unsafe.Sizeof(yaroze.SVECTOR{})
	}
}

func originalVertexSave() {
	n := 0
	for j := range objnum {
		o := &objects()[j]
		for i := range o.NVert {
			orgvtx[n] = *vertAt(o, i)
			n++
		}
	}
}

func resetMimeVertex() {
	n := 0
	for j := range objnum {
		o := &objects()[j]
		for i := range o.NVert {
			*vertAt(o, i) = orgvtx[n]
			n++
		}
	}
}

func setMimeVertex() {
	for i := range mdfTotal {
		m := &mdfvtx0[i]
		if mimepr[i] == 0 || m.object >= objnum {
			continue
		}
		o := &objects()[m.object]
		if m.offset+m.total > o.NVert {
			continue
		}
		yaroze.GteMIMefunc(vertAt(o, m.offset), (*yaroze.SVECTOR)(yaroze.Pointer(m.top)), int32(m.total), mimepr[i])
	}
}

// The transfer functions: each is a filter over the last num frames of
// its input, weighted by a window of num values in ONE units.

const (
	ctlMax  = 16
	ctlTime = 64
)

type ctlFunc struct {
	cnv []int32 // the window
	in  int32
	out int32
}

var (
	cntrlarry [ctlMax][ctlTime]int32 // each function's last inputs
	ctlfc     [ctlMax]ctlFunc
)

// The windows: a hump over sixteen frames, which the legs use; a sharp
// rise and a long fall that goes below zero, for the mouth; and a slower
// hump over twenty-four frames, with a dip after, for the rest.
var (
	cnv0 = [16]int32{
		39, 79, 118, 197, 315, 394, 433, 473,
		473, 433, 394, 315, 197, 118, 79, 39,
	}
	cnv2 = [12]int32{
		400, 900, 1000, 900, 600, 400, 200, 110,
		-100, -200, -80, -20,
	}
	cnv3 = [24]int32{
		200, 325, 450, 475, 500, 475, 450, 375,
		300, 250, 200, 150, 100, 80, 55, 0,
		-50, -75, -100, -70, -40, -25, -10, -5,
	}
)

// initCntrlarry gives every function the same window and clears its
// history.
func initCntrlarry(cnv []int32) {
	for i := range ctlfc {
		ctlfc[i] = ctlFunc{cnv: cnv}
		for j := range cntrlarry[i] {
			cntrlarry[i][j] = 0
		}
	}
}

// setCntrl records each function's input for frame frm and works out its
// output: the inputs of the last num frames weighted by the window.
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
