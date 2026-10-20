package main

import (
	"os"
	"testing"
	"unsafe"
)

// loadArchive reads one of the data files as package disc would.
func loadArchive(t *testing.T, name string) *archive {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	words := make([]uint32, (len(b)+3)/4)
	copy(unsafe.Slice((*byte)(unsafe.Pointer(&words[0])), len(b)), b)
	return parseArchive(words)
}

// TestModelsIndexWithinBounds walks every primitive of every model and
// checks that the vertices and normals it names, and the buffers a
// transforming record fills, lie within the model.
func TestModelsIndexWithinBounds(t *testing.T) {
	a := loadArchive(t, "assets/DATA2.ARC")
	for i := range a.entries(4) {
		words, _ := a.words(4, i)
		m := parseModel(words)
		check := func(what string, at int) {
			t.Helper()
			if at < 0 || at >= len(words) {
				t.Errorf("model %d: %s at word %d, of %d", i, what, at, len(words))
			}
		}
		for c, node := range m.chains {
			for node != 0 {
				list := m.list(node)
				n := int(words[node+nodeCount] & 0x7fffffff)
				rec := node + nodePrims
				for ; n > 0; n-- {
					code := words[rec+primCode] &^ 0x800000
					if code == primXform {
						if len(list) < 5 {
							t.Errorf("model %d chain %d: transform with a list of %d", i, c, len(list))
							break
						}
						nv, src, dst := int(words[rec+2]), list[1]+2*int(words[rec+3]), list[2]+2*int(words[rec+4])
						check("vertex source", src+2*nv-1)
						check("vertex buffer", dst+2*nv-1)
						nn, src, dst := int(words[rec+5]), list[3]+2*int(words[rec+6]), list[4]+2*int(words[rec+7])
						check("normal source", src+2*nn-1)
						check("light buffer", dst+2*nn-1)
						rec += primXformN
						continue
					}
					count := int(words[rec+primCount]&0x7fffffff) >> 16
					data := list[0] + int(words[rec+primData])
					// Each kind's layout: its stride, and which words hold
					// a normal in the low half and a vertex in the high.
					var stride int
					var pairs []int
					var quad bool
					switch code {
					case 0xc, 0x100000c:
						stride, pairs = 4, []int{1, 2, 3}
					case 0x14, 0x1000014:
						stride, pairs = 5, []int{1, 2, 3, 4}
					case 0xd, 0x100000d:
						stride, pairs = 6, []int{3, 4, 5}
					case 0x15, 0x200015, 0x1000015:
						stride, pairs, quad = 7, []int{4, 5, 6}, true
					default:
						t.Errorf("model %d chain %d: code %x", i, c, code)
					}
					shared := code&primXform != 0
					vertex, normal := func(idx uint32) {
						check("vertex", list[1]+2*int(idx)+1)
					}, func(idx uint32) {
						check("normal", list[2]+2*int(idx)+1)
					}
					if shared {
						vertex, normal = func(idx uint32) {
							check("shared vertex", list[2]+2*int(idx)+1)
						}, func(idx uint32) {
							check("light", list[4]+2*int(idx)+1)
						}
					}
					for k := 0; k < count; k++ {
						at := data + k*stride
						check("primitive", at+stride-1)
						for _, w := range pairs {
							vertex(words[at+w] >> 16)
							normal(words[at+w] & 0xffff)
						}
						if quad {
							normal(words[at+2] >> 16)
							vertex(words[at+3] >> 16)
						}
					}
					rec += primSize
				}
				next := int32(words[node+nodeNext])
				if next == -1 {
					break
				}
				node = int(next)
			}
		}
	}
}
