package main

// Models, animations and actors.
//
// A model is the game's own format, group 4 of the second archive: a
// skeleton of bones, each a coordinate system under its parent, and for
// each bone a chain of nodes, each naming the arrays its primitives index,
// vertices and normals, and the primitives themselves in the layout the
// TMD format gives them, with a code in front of each run saying how to
// draw it. A bone's chain may also transform its vertices into a buffer
// shared with the other bones, which the last chain, with no bone of its
// own, draws polygons across: the skin at a joint.
//
// An animation, group 5, is a frame count and for each bone a track of
// keys: a frame number and the bone's position, rotation and scale there.
// Between keys the bone moves in a straight line or along a cubic
// B-spline through the keys around it, which is what the original's
// interpolation amounts to.
//
// An actor is a model playing an animation: the root coordinate system the
// game places, the bones under it, and where in the animation it is.

import (
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

// A model is one model file, unpacked. The words are the file, which the
// drawing code indexes by the offsets the file holds, and which the
// pre-transforming chains write their buffers into.
type model struct {
	words  []uint32
	nparts int    // chains, two more than the bones
	bones  []bone // each bone's rest matrix and parent
	chains []int  // the first node of each chain, as a word index, or 0
	env    bool   // the ship: its quads are mapped with the sea's reflection

	// Actors of the model given back, for newActor to hand out again:
	// the program's allocator never frees, so what the original freed and
	// allocated again is kept here instead.
	free  [maxActorsPerModel]*actor
	nfree int
}

// maxActorsPerModel is how many freed actors of one model are kept.
const maxActorsPerModel = 32

// A bone is a coordinate system of the skeleton as the file gives it.
type bone struct {
	m      yaroze.MATRIX
	parent int // the bone under which it moves, or -1 for the root
}

// Node fields, as word offsets from a node's start.
const (
	nodeNext   = 0 // the next node, or -1
	nodeList   = 1 // the node's array list
	nodeCount  = 2 // the primitives that follow, under bit 31
	nodePrims  = 3
	primCode   = 0 // what the primitives are, with bit 23 ignored
	primCount  = 1 // their number in the high half, under bit 31
	primData   = 2 // where they start in the list's first array, in words
	primSize   = 3
	primXform  = 0x1000000 // the code of a transforming record, which is longer
	primXformN = 8
)

// parseModel reads the model held in words.
func parseModel(words []uint32) *model {
	m := &model{words: words}
	m.nparts = int(words[3])
	m.chains = make([]int, m.nparts)
	for c := range m.chains {
		m.chains[c] = int(words[4+c])
	}
	first := 5 + m.nparts
	m.bones = make([]bone, m.nparts-2)
	for i := range m.bones {
		at := first + 20*i
		b := &m.bones[i]
		b.m = *(*yaroze.MATRIX)(unsafe.Pointer(&words[at+1]))
		b.parent = -1
		if super := int(words[at+19]); super != 0 {
			b.parent = (super - first) / 20
		}
	}
	return m
}

// list returns the arrays a node names: word indexes into the model.
func (m *model) list(node int) []int {
	at := int(m.words[node+nodeList] & 0x7fffffff)
	n := int(m.words[at])
	list := listBuf[:n]
	for i := range list {
		list[i] = int(m.words[at+1+i] & 0x7fffffff)
	}
	return list
}

// listBuf holds a node's list while it is drawn; no list has more than
// five arrays.
var listBuf [8]int

// svector returns the vector at word index i, a vertex or a normal.
func (m *model) svector(i int) *yaroze.SVECTOR {
	return (*yaroze.SVECTOR)(unsafe.Pointer(&m.words[i]))
}

// An animation is one animation file: its length in frames and a track of
// keys for each bone. The file's shorts are kept whole, since the original
// reads a key past the end of a track when a bone reaches its last, taking
// whatever follows in the file, and the port must read the same.
type animation struct {
	shorts  []int16
	nframes int
	tracks  []track
}

// A track is one bone's keys: how many, and the index in shorts of the
// first.
type track struct {
	nkeys int
	at    int
}

// A key is ten shorts: the frame, the position, the rotation and the
// scale.
const keyLen = 10

// parseAnimation reads the animation held in words.
func parseAnimation(words []uint32) *animation {
	// A key past the last track's end reads beyond the file; a key of
	// zeros is kept there for it, where the original found whatever its
	// heap held.
	shorts := make([]int16, 2*len(words)+keyLen)
	copy(shorts, unsafe.Slice((*int16)(unsafe.Pointer(&words[0])), 2*len(words)))
	a := &animation{shorts: shorts}
	a.nframes = int(uint16(shorts[0]))
	nbones := int(uint16(shorts[1]))
	off := int(words[1]) / 2
	a.tracks = make([]track, nbones)
	for b := range a.tracks {
		rec := off + 4*b
		a.tracks[b] = track{
			nkeys: int(uint16(shorts[rec+1])),
			at:    int(uint32(uint16(shorts[rec+2]))|uint32(uint16(shorts[rec+3]))<<16) / 2,
		}
	}
	return a
}

// key returns key k of track t: its ten shorts, from the file, even past
// the track's end.
func (a *animation) key(t *track, k int) []int16 {
	at := t.at + keyLen*k
	return a.shorts[at : at+keyLen]
}

// The models and animations, loaded by loadModels.
var (
	models     []*model
	animations []*animation
)

// loadModels unpacks every model and animation of the archive. The last
// model is the ship, drawn with the sea's reflection on it.
func loadModels(a *archive) {
	models = make([]*model, a.entries(4))
	for i := range models {
		words, _ := a.words(4, i)
		models[i] = parseModel(words)
	}
	models[len(models)-1].env = true
	animations = make([]*animation, a.entries(5))
	for i := range animations {
		words, _ := a.words(5, i)
		animations[i] = parseAnimation(words)
	}
}

// Animation modes, as actorSetAnim takes them.
const (
	animLoop = 0 // play again from the start at the end
	animOnce = 1 // stop at the end
	animStep = 2 // advance one frame and stop
	animLast = 3 // show the last frame
)

// An actor is a model playing an animation, placed in the world by its root
// coordinate system.
type actor struct {
	animating bool
	mode      int
	spline    bool // interpolate along a spline rather than a line
	model     int
	anim      int // -1 for none: drawn in the rest pose
	root      yaroze.GsCOORDINATE2
	bones     []yaroze.GsCOORDINATE2
	frame     int
	keys      []int // each bone's current key
}

// newActor makes an actor of the model, playing the animation, which may be
// -1, in the mode. Its root starts at the world's origin, unturned. An
// actor freeActor gave back is used again before any is made.
func newActor(modelIndex, anim, mode int, spline bool) *actor {
	m := models[modelIndex]
	var a *actor
	if m.nfree > 0 {
		m.nfree--
		a = m.free[m.nfree]
		bones, keys := a.bones, a.keys
		*a = actor{model: modelIndex, bones: bones, keys: keys}
	} else {
		a = &actor{model: modelIndex}
		a.bones = make([]yaroze.GsCOORDINATE2, len(m.bones))
		a.keys = make([]int, len(m.bones))
	}
	a.root.Init(yaroze.WORLD)
	for i := range a.bones {
		a.bones[i] = yaroze.GsCOORDINATE2{}
		b := &a.bones[i]
		b.Coord = m.bones[i].m
		b.Super = &a.root
		if m.bones[i].parent >= 0 {
			b.Super = &a.bones[m.bones[i].parent]
		}
	}
	a.setAnim(anim, mode, spline)
	return a
}

// freeActor gives an actor back for newActor to use again.
func freeActor(a *actor) {
	m := models[a.model]
	if m.nfree < len(m.free) {
		m.free[m.nfree] = a
		m.nfree++
	}
}

// setAnim starts the actor on an animation, or none with -1, in the mode.
// animLast starts it on its last frame, and the rest on its first.
func (a *actor) setAnim(anim, mode int, spline bool) {
	a.anim, a.mode, a.spline = anim, mode, spline
	if anim == -1 {
		a.animating = false
		return
	}
	a.animating = true
	an := animations[anim]
	if mode == animLast {
		a.frame = an.nframes - 1
		for b := range a.keys {
			a.keys[b] = an.tracks[b].nkeys - 1
		}
	} else {
		a.frame = 0
		clear(a.keys)
	}
}

// place sets the actor's root: turned by rot, or not at all with nil, scaled
// by scale, at pos.
func (a *actor) place(pos *[3]int16, rot *yaroze.SVECTOR, scale *yaroze.VECTOR) {
	m := &a.root.Coord
	if rot == nil {
		*m = yaroze.IDMATRIX
	} else {
		yaroze.RotMatrix(rot, m)
	}
	yaroze.ScaleMatrix(m, scale)
	m.T = [3]int32{int32(pos[0]), int32(pos[1]), int32(pos[2])}
	a.root.Flg = 0
}

// Working storage for animate, kept out of the frame loop's stack because
// the library takes their addresses.
var (
	animPos   [3]int16
	animRot   yaroze.SVECTOR
	animScale yaroze.VECTOR
)

// animate works out every bone's matrix for the actor's frame, and then
// moves on a frame if advance is set. It reports whether the animation
// reached its end this frame: an actor in animLoop goes back to the start,
// and the others stop.
func (a *actor) animate(advance bool) bool {
	if !a.animating {
		return true
	}
	an := animations[a.anim]
	// The animation moves as many bones as it has tracks, which may be
	// fewer than the model has; the rest keep their matrices.
	for b := range min(len(an.tracks), len(a.bones)) {
		t := &an.tracks[b]
		k := a.keys[b]
		key := an.key(t, k)
		switch {
		case a.frame == 0 || t.nkeys == 1:
			a.keys[b] = 0
			readKey(key)
		case k == t.nkeys:
			// Past the last key: toward the first again.
			dt := int(int16(a.frame) - key[0])
			span := int(int16(an.nframes) - key[0])
			if !a.spline {
				lerpKey(key, an.key(t, 0), span, dt)
			} else {
				splineKey(an.key(t, (k-2)%t.nkeys), an.key(t, (k-1)%t.nkeys), key, an.key(t, 0), span, dt)
			}
		default:
			next := an.key(t, k+1)
			if a.spline {
				if a.frame == int(uint16(next[0])) {
					k++
					a.keys[b] = k
					key, next = next, an.key(t, k+1)
				}
				dt := int(int16(a.frame) - key[0])
				span := int(next[0] - key[0])
				var k2, k1 int
				if a.mode == animLoop {
					k2, k1 = (k-2)%t.nkeys, (k-1)%t.nkeys
				} else {
					k2, k1 = max(k-2, 0), max(k-1, 0)
				}
				splineKey(an.key(t, k2), an.key(t, k1), key, next, span, dt)
			} else if a.frame != int(uint16(next[0])) {
				dt := int(int16(a.frame) - key[0])
				span := int(next[0] - key[0])
				lerpKey(key, next, span, dt)
			} else {
				readKey(next)
				a.keys[b]++
			}
		}
		c := &a.bones[b]
		yaroze.RotMatrix(&animRot, &c.Coord)
		yaroze.ScaleMatrix(&c.Coord, &animScale)
		c.Coord.T = [3]int32{int32(animPos[0]), int32(animPos[1]), int32(animPos[2])}
		c.Flg = 0
	}
	if advance {
		a.frame++
	}
	if a.frame == an.nframes {
		if a.mode != animOnce && a.mode != animLast {
			a.frame = 0
			return true
		}
	} else if a.mode != animStep {
		return false
	}
	a.animating = false
	return true
}

// readKey takes a key as the bone's pose.
func readKey(key []int16) {
	animPos = [3]int16{key[1], key[2], key[3]}
	animRot = yaroze.SVECTOR{Vx: key[4], Vy: key[5], Vz: key[6]}
	animScale = yaroze.VECTOR{Vx: int32(key[7]), Vy: int32(key[8]), Vz: int32(key[9])}
}

// lerpKey takes the pose dt frames of the span from key a toward key b.
func lerpKey(a, b []int16, span, dt int) {
	var k [keyLen]int16
	for i := 1; i < keyLen; i++ {
		k[i] = a[i] + int16((int(b[i])-int(a[i]))*dt/span)
	}
	takeKey(&k)
}

// splineKey takes the pose dt frames of the span from key p2 toward p3,
// on the B-spline through the four keys.
func splineKey(p0, p1, p2, p3 []int16, span, dt int) {
	t := (dt << 12) / span
	var k [keyLen]int16
	for i := 1; i < keyLen; i++ {
		k[i] = int16(bspline(t, int(p0[i]), int(p1[i]), int(p2[i]), int(p3[i])))
	}
	takeKey(&k)
}

// takeKey takes the interpolated shorts of a key as the bone's pose.
func takeKey(k *[keyLen]int16) {
	animPos = [3]int16{k[1], k[2], k[3]}
	animRot = yaroze.SVECTOR{Vx: k[4], Vy: k[5], Vz: k[6]}
	animScale = yaroze.VECTOR{Vx: int32(k[7]), Vy: int32(k[8]), Vz: int32(k[9])}
}

// bspline is the uniform cubic B-spline through a, b, c and d at t, a
// fraction with 4096 as one, in the original's fixed point arithmetic.
func bspline(t, a, b, c, d int) int {
	r := (b*0x800 - a*0x2aa - c*0x800 + d*0x2aa) >> 12
	r = (r * t >> 12) + ((a*0x800 - b*0x1000 + c*0x800) >> 12)
	r = (r * t >> 12) + ((c*0x800 - a*0x800) >> 12)
	r = (r * t >> 12) + ((a*0x2aa + b*0xaaa + c*0x2aa) >> 12)
	return r
}
