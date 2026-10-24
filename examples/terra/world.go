package main

// The world: an area is a map of cells, a column of up to twelve levels
// at each square of a grid of up to 32 by 32, each cell a type of block,
// which entering the area turns into a mesh. Every face a block shows is
// a textured quad on shared vertices, lit once from its accumulated
// normals, and the quads are kept by chunk, four squares on a side, each
// with the box around it, so that a frame draws only the chunks in view.
// Trees are not part of the mesh: a cell of their type puts a model
// instance in the chunk instead.
//
// Units: a cell is 512 on a side, and the world's y runs up while the
// mesh's runs down, as the screen's does, so the mesh negates y and z.

import (
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

// Block types, as the map's nibbles have them.
const (
	cellEmpty  = 0
	cellWater  = 1
	cellGround = 2
	cellSlope  = 3
	cellStone  = 4
	cellTree   = 5
	cellStairs = 6
	cellPillar = 7
	cellTower  = 8
)

// The table sizes the original had room for.
const (
	maxCells   = 32 * 8 * 32
	maxVerts   = 1404
	maxPrims   = 1300
	maxObjects = 64
	numChunks  = 64
)

// A worldVertex is one vertex of the mesh. The layout is the original's,
// with the position and the normal where the GTE can load them.
type worldVertex struct {
	shadow uint8 // under an overhang: lit at a sixteenth
	border uint8 // on the map's edge: lit from above, fully
	_      [2]byte
	pos    yaroze.SVECTOR
	n      yaroze.SVECTOR // accumulated from the faces, then normalized
}

// A worldPrim is one quad of the mesh: its lit colors, its texture, and its
// vertices.
type worldPrim struct {
	rgb   [4]uint32
	uv0   uint16
	clut  uint16
	uv1   uint16
	tpage uint16
	uv2   uint16
	bias  int16 // added to the depth it sorts at
	uv3   uint16
	abr   int16 // the semi-transparency rate, or -1 for opaque
	v     [4]uint16
	n     yaroze.SVECTOR
}

// A chunk is four squares on a side of the mesh: its run of quads, the
// eight corners of the box around it, and its run of tree instances.
type chunk struct {
	first, count     int32
	corners          [8]yaroze.SVECTOR
	firstObj, numObj int32
}

var (
	cells                  [maxCells]uint8
	mapW, mapH, mapD       int // squares across, levels up, squares deep
	verts                  [maxVerts]worldVertex
	prims                  [maxPrims]worldPrim
	objects                [maxObjects]yaroze.GsCOORDINATE2
	chunks                 [numChunks]chunk
	numVerts, numPrims     int
	numObjects             int
	vertexGrid             [33 * 12 * 33]int16
	area                   int // the area the player is in
	areaStyle              int // areaOutdoor, areaIndoor or areaDark
	textureFrame           int // the animated textures' clock
	animatedTexturesIndoor bool
)

// Area styles: outdoors, in the ruins, and outdoors at night.
const (
	areaOutdoor = 0
	areaIndoor  = 1
	areaDark    = 2
)

// cell returns the type of the cell at x, y, z, or empty outside the map.
func cell(x, y, z int) int {
	if x < 0 || y < 0 || z < 0 || x >= mapW || y >= mapH || z >= mapD {
		return 0
	}
	return int(cells[(z*mapW+x)*mapH+y] & 0x7f)
}

// cellTop reports whether the cell at x, y, z is the top of its column, as
// loadMap marked it.
func cellTop(x, y, z int) bool {
	return cells[(z*mapW+x)*mapH+y]&0x80 != 0
}

// setCellTop marks or unmarks the cell as the top of its column.
func setCellTop(x, y, z int, top bool) {
	c := &cells[(z*mapW+x)*mapH+y]
	if top {
		*c |= 0x80
	} else {
		*c &= 0x7f
	}
}

// loadMap unpacks map n of the first archive into cells, a byte a cell
// from its nibbles, and marks cells as the top of their column the way the
// original did: from the top of each column down, a cell keeps the mark
// when only air lies between it and a whole block, or the ground, below,
// and loses it when water, a tree or a tower comes first.
func loadMap(n int) {
	b := mapScratch[:]
	arc1.unpack(0, n, b)
	mapW, mapH, mapD = int(int32(le32(b))), int(int32(le32(b[4:]))), int(int32(le32(b[8:])))
	packed := b[12:]
	for i := 0; i < mapW*mapH*mapD/2; i++ {
		cells[2*i] = packed[i] >> 4
		cells[2*i+1] = packed[i] & 0xf
	}
	for z := 0; z < mapD; z++ {
		if mapW <= 0 {
			continue
		}
		x := 0
		top := mapH
		var y1, y7, t int
	column:
		y1 = top - 1
		if y1 > 0 {
			setCellTop(x, y1, z, true)
			y7 = top - 2
			top = y1
			if y7 >= 0 {
				for {
					t = cell(x, y7, z)
					if t == 1 || t == 5 || t == 7 || t == 8 {
						break
					}
					if y7 == 0 && t == 0 {
						break
					}
					if t != 0 {
						goto column
					}
					y7--
					if y7 < 0 {
						goto column
					}
				}
				setCellTop(x, y1, z, false)
			}
		}
		x++
		top = mapH
		if x < mapW {
			goto column
		}
	}
}

// mapScratch holds a map while it is unpacked; the largest is 3084 bytes.
var mapScratch [4096]byte

func le32(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

// Face numbers, as the tables below index them: the top, the four sides
// going round, and the bottom.
const (
	faceTop    = 0
	faceNorth  = 1 // toward -z
	faceEast   = 2 // toward +x
	faceSouth  = 3
	faceWest   = 4
	faceBottom = 5
)

// faceCorners is each face's four corners as offsets from the cell's
// origin, in the mesh's units, with y and z already negated.
var faceCorners = [6][4][3]int16{
	{{0, 0, 0}, {512, 0, 0}, {0, 0, -512}, {512, 0, -512}},
	{{0, 0, -512}, {512, 0, -512}, {0, 512, -512}, {512, 512, -512}},
	{{512, 0, -512}, {512, 0, 0}, {512, 512, -512}, {512, 512, 0}},
	{{512, 0, 0}, {0, 0, 0}, {512, 512, 0}, {0, 512, 0}},
	{{0, 0, 0}, {0, 0, -512}, {0, 512, 0}, {0, 512, -512}},
	{{512, 512, 0}, {0, 512, 0}, {512, 512, -512}, {0, 512, -512}},
}

// faceNormals is each face's normal.
var faceNormals = [6]yaroze.SVECTOR{
	{Vy: -4096}, {Vz: -4096}, {Vx: 4096}, {Vz: 4096}, {Vx: -4096}, {Vy: 4096},
}

// faceNeighbors is the cell across each face.
var faceNeighbors = [6][3]int{
	{0, 1, 0}, {0, 0, 1}, {1, 0, 0}, {0, 0, -1}, {-1, 0, 0}, {0, -1, 0},
}

// uvFlips chooses, for each of six ways of laying a texture on a face,
// which corners take the far edge of the texture: pairs of u and v flags
// for the four corners.
var uvFlips = [6][8]uint8{
	{0, 0, 1, 0, 0, 1, 1, 1},
	{0, 1, 0, 0, 1, 1, 1, 0},
	{1, 0, 1, 1, 0, 0, 0, 1},
	{1, 0, 0, 0, 1, 1, 0, 1},
	{0, 1, 1, 1, 0, 0, 1, 0},
	{1, 1, 0, 1, 1, 0, 0, 0},
}

// findVertex returns the vertex at x, y, z in the mesh's units, adding it
// if it is new. A vertex on the cell grid is found through a grid of the
// cells' corners; any other by searching.
func findVertex(x, y, z int) int {
	var v *worldVertex
	i := numVerts
	if x&0x1ff == 0 && (0x200-y)&0x1ff == 0 && (-z)&0x1ff == 0 {
		g := ((-z>>9)*33+(x>>9))*12 + ((0x200 - y) >> 9)
		if vertexGrid[g] != 0 {
			return int(vertexGrid[g]) - 1
		}
		vertexGrid[g] = int16(numVerts + 1)
	} else {
		for j := range numVerts {
			p := &verts[j].pos
			if int(p.Vx) == x && int(p.Vy) == y && int(p.Vz) == z {
				return j
			}
		}
	}
	v = &verts[i]
	v.pos = yaroze.SVECTOR{Vx: int16(x), Vy: int16(y), Vz: int16(z)}
	v.n = yaroze.SVECTOR{}
	v.shadow = 0
	v.border = 0
	if x == 0 || z == 0 || x == mapW<<9 || z == mapD*-0x200 {
		v.border = 1
	}
	numVerts++
	return i
}

// shadeTop marks the four corners of the top of the cell at x, y, z, in
// the world's units, as in shadow.
func shadeTop(x, y, z int) {
	y, z = -y, -z
	verts[findVertex(x, y, z)].shadow = 1
	verts[findVertex(x+0x200, y, z)].shadow = 1
	verts[findVertex(x, y, z-0x200)].shadow = 1
	verts[findVertex(x+0x200, y, z-0x200)].shadow = 1
}

// addFace adds a quad for face f of the cell at x, y, z in the world's
// units, textured with tile t laid on as uvFlips[flipRow] says, at the
// semi-transparency rate abr or opaque with -1, and turned over if flip
// is set. The quad's normal is added to its vertices'.
func addFace(x, y, z, f, t, abr, flipRow int, flip bool) {
	p := &prims[numPrims]
	numPrims++
	tile := &tiles[t]
	flips := &uvFlips[flipRow]
	p.uv0, p.uv1, p.uv2, p.uv3 = faceUV(tile, flips, 0), faceUV(tile, flips, 1), faceUV(tile, flips, 2), faceUV(tile, flips, 3)
	tp := tile.tpageY>>4&0x10 | (tile.tpageX&0x3ff)>>6 | (tile.tpageY&0x200)<<2
	if abr != -1 {
		tp |= uint16(abr&3) << 5
	}
	p.tpage = tp
	p.clut = tile.clut()
	p.n = faceNormals[f]
	dip := 0
	if t == 0x2a {
		dip = -0x180
		if tiles[0x2a].h == 0x18 {
			dip = -0x80
		}
	}
	c := &faceCorners[f]
	y, z = -y, -z
	p.v[0] = uint16(findVertex(x+int(c[0][0]), y+int(c[0][1]), z+int(c[0][2])))
	p.v[1] = uint16(findVertex(x+int(c[1][0]), y+int(c[1][1]), z+int(c[1][2])))
	p.v[2] = uint16(findVertex(x+int(c[2][0]), y+int(c[2][1])+dip, z+int(c[2][2])))
	p.v[3] = uint16(findVertex(x+int(c[3][0]), y+int(c[3][1])+dip, z+int(c[3][2])))
	for _, i := range p.v[:3] {
		n := &verts[i].n
		n.Vx += p.n.Vx
		n.Vy += p.n.Vy
		n.Vz += p.n.Vz
	}
	if flip {
		p.v[0], p.v[1] = p.v[1], p.v[0]
		p.v[2], p.v[3] = p.v[3], p.v[2]
		p.n.Vx, p.n.Vy, p.n.Vz = -p.n.Vx, -p.n.Vy, -p.n.Vz
	}
	switch {
	case f == faceTop:
		p.bias = -8
	case flip:
		p.bias = -4
	default:
		p.bias = 0
	}
	p.abr = int16(abr)
}

// faceUV returns corner c's texture coordinates on the tile, at the near
// or far edge of the texture as the flips say.
func faceUV(tile *texture, flips *[8]uint8, c int) uint16 {
	u, v := int(tile.u), int(tile.v)
	if flips[2*c] != 0 {
		u += int(tile.w) - 1
	}
	if flips[2*c+1] != 0 {
		v += int(tile.h) - 1
	}
	return uint16(uint8(u)) | uint16(uint8(v))<<8
}

// solid reports whether a block type is one that hides a neighbor's face:
// ground, slope and stone. Stairs do not, as the original had it, though
// they shade what is under them like the rest.
func solid(t int) bool {
	return t == 2 || t == 3 || t == 4
}

// buildWorld makes the mesh from the cells: chunk by chunk, every cell
// that is not empty adds the faces its type shows, and the chunk's box is
// sized to the tallest cell in it.
func buildWorld() {
	numVerts, numPrims, numObjects = 0, 0, 0
	clear(vertexGrid[:])
	for cz := range 8 {
		for cx := range 8 {
			ch := &chunks[cz*8+cx]
			ch.first = int32(numPrims)
			ch.firstObj = int32(numObjects)
			tallest := 0
			x0, x1 := cx*4, cx*4+4
			z0, z1 := cz*4, cz*4+4
			for z := z0; z < z1; z++ {
				for x := x0; x < x1; x++ {
					for y := range 8 {
						t := cell(x, y, z)
						if t == 0 {
							continue
						}
						tallest = max(tallest, y)
						shadeOverhang(x, y, z, t)
						switch t {
						case cellWater:
							buildWater(x, y, z)
						case cellGround:
							buildGround(x, y, z)
						case cellSlope:
							buildSlope(x, y, z)
						case cellStone:
							buildStone(x, y, z)
						case cellTree:
							buildTree(x, y, z)
						case cellStairs:
							buildStairs(x, y, z)
						case cellPillar, cellTower:
							buildTower(x, y, z, t)
						}
					}
				}
			}
			setChunkBox(ch, cx, cz, tallest)
		}
	}
	for i := range numChunks - 1 {
		chunks[i].count = chunks[i+1].first - chunks[i].first
		chunks[i].numObj = chunks[i+1].firstObj - chunks[i].firstObj
	}
	last := &chunks[numChunks-1]
	last.count = int32(numPrims) - last.first
	last.numObj = int32(numObjects) - last.firstObj
	lightWorld()
}

// setChunkBox sets the chunk's corners: the top four at the tallest cell
// and the bottom four a cell below the ground.
func setChunkBox(ch *chunk, cx, cz, tallest int) {
	x0, x1 := int16(cx<<11), int16((cx+1)<<11)
	z0, z1 := int16(-cz*4*0x200), int16(-(cz+1)*4*0x200)
	top, bottom := int16((1-tallest)*0x200), int16(0x200)
	ch.corners = [8]yaroze.SVECTOR{
		{Vx: x0, Vy: top, Vz: z0}, {Vx: x1, Vy: top, Vz: z0},
		{Vx: x0, Vy: top, Vz: z1}, {Vx: x1, Vy: top, Vz: z1},
		{Vx: x0, Vy: bottom, Vz: z0}, {Vx: x1, Vy: bottom, Vz: z0},
		{Vx: x0, Vy: bottom, Vz: z1}, {Vx: x1, Vy: bottom, Vz: z1},
	}
}

// shadeOverhang marks the top of a whole block as in shadow when a block
// stands over it further up the column with nothing, or water, directly
// above.
func shadeOverhang(x, y, z, t int) {
	if !(t == 2 || t == 3 || t == 4 || t == 6 || (y == 0 && t == 1)) {
		return
	}
	above := cell(x, y+1, z)
	if above != 0 && above != 1 {
		return
	}
	for yy := y + 2; yy < mapH; yy++ {
		if above := cell(x, yy, z); solid(above) || above == 6 {
			top := y << 9
			if t == 1 {
				top = (y - 1) * 0x200
			}
			shadeTop(x<<9, top, z<<9)
		}
	}
}

// buildWater builds a water cell: at the bottom of the map, in the ruins,
// falls of water down the sides of whole blocks and towers next to it,
// and then its surface, a little below the top, semi-transparent.
func buildWater(x, y, z int) {
	if y == 0 {
		if areaStyle == areaIndoor {
			for f := 1; f < 5; f++ {
				nb := &faceNeighbors[f]
				t := cell(x+nb[0], nb[1]+1, z+nb[2])
				if t == 7 || t == 8 || t == 2 || t == 3 {
					tiles[0x2a].h = 0x18
					addFace(x<<9, 0x180, z<<9, f, 0x2a, 1, 0, true)
					tiles[0x2a].v += 0x18
					tiles[0x2a].h = 8
					addFace(x<<9, 0, z<<9, f, 0x2a, 1, 0, true)
					tiles[0x2a].v -= 0x18
					tiles[0x2a].h = 0x20
				}
			}
		}
		if cell(x, 1, z) != 1 {
			addFace(x<<9, -0x80, z<<9, faceTop, 10, 0, 0, false)
		}
		addFace(x<<9, -0x200, z<<9, faceTop, 0xb, -1, 0, false)
		return
	}
	addFace(x<<9, y*0x200-0x80, z<<9, faceTop, 10, 0, 0, false)
}

// buildGround builds a whole block of ground: each face whose neighbor
// does not hide it, with a texture chosen by what is around it: grass on
// top, with paths and edges where slopes and stairs meet it, and earth or
// cliff on the sides.
func buildGround(x, y, z int) {
	faces := 6
	if y == 0 {
		faces = 5
	}
	for f := 0; f < faces; f++ {
		nb := &faceNeighbors[f]
		t := cell(x+nb[0], y+nb[1], z+nb[2])
		if solid(t) || t == 7 || t == 8 {
			continue
		}
		tex, row := 0x27, 0
		if areaStyle != areaIndoor {
			if f != 0 {
				tex = 5
				if f != 5 {
					above := cell(x, y+1, z)
					if above != 2 && above != 3 {
						if cell(x+1, y+1, x) == 1 || cell(x, y+1, z+1) == 1 || cell(x-1, y+1, z) == 1 || cell(x, y+1, z-1) == 1 {
							tex = 8
						} else {
							tex = 6
						}
					}
				}
			} else {
				tex, row = groundTop(x, y, z)
			}
		}
		addFace(x<<9, y<<9, z<<9, f, tex, -1, row, false)
	}
}

// groundTop chooses the texture and its orientation for the top of a block
// of ground from the slopes, stairs and water around it.
func groundTop(x, y, z int) (tex, row int) {
	above := cell(x, y+1, z)
	if above == 1 {
		return 0xb, 0
	}
	if above == 6 && (cell(x-1, y+1, z) == 1 || cell(x+1, y+1, z) == 1 || cell(x, y+1, z-1) == 1 || cell(x, y+1, z+1) == 1) {
		return 0xb, 0
	}
	e, s, w, n := cell(x+1, y, z) == 3, cell(x, y, z+1) == 3, cell(x-1, y, z) == 3, cell(x, y, z-1) == 3
	switch {
	case e && s:
		return 1, 5
	case s && w:
		return 1, 4
	case w && n:
		return 1, 0
	case n && e:
		return 1, 3
	case e:
		return 3, 5
	case s:
		return 2, 5
	case w:
		return 3, 0
	case n:
		return 2, 0
	}
	return 0, 0
}

// buildSlope builds a cell of slope ground: a whole block whose top is the
// slope texture, with its sides as a block's.
func buildSlope(x, y, z int) {
	faces := 6
	if y == 0 {
		faces = 5
	}
	for f := 0; f < faces; f++ {
		nb := &faceNeighbors[f]
		t := cell(x+nb[0], y+nb[1], z+nb[2])
		if solid(t) || t == 7 || t == 8 {
			continue
		}
		tex := 0x28
		if areaStyle != areaIndoor {
			switch {
			case f == 0:
				tex = 4
			case f == 5:
				tex = 5
			default:
				tex = 7
				if above := cell(x, y+1, z); above == 2 || above == 3 {
					tex = 5
				}
			}
		}
		addFace(x<<9, y<<9, z<<9, f, tex, -1, 0, false)
	}
}

// buildStone builds a cell of stone: a block with one of six stone
// textures on top, chosen at random, and rock down the sides.
func buildStone(x, y, z int) {
	faces := 6
	if y == 0 {
		faces = 5
	}
	for f := 0; f < faces; f++ {
		nb := &faceNeighbors[f]
		t := cell(x+nb[0], y+nb[1], z+nb[2])
		if solid(t) || t == 7 || t == 8 {
			continue
		}
		tex, row := 0, 0
		if f == 0 {
			tex = 0x33
			row = int(yaroze.Rand() % 6)
		} else if areaStyle == areaIndoor {
			tex = 0x28
		} else {
			tex = 9
			if above := cell(x, y+1, z); above == 2 || above == 3 {
				tex = 5
			}
		}
		addFace(x<<9, y<<9, z<<9, f, tex, -1, row, false)
	}
}

// Working storage for buildTree, whose addresses go to the library.
var (
	treeRot   yaroze.SVECTOR
	treeScale yaroze.VECTOR
)

// buildTree puts a tree in the chunk: a model instance in the middle of
// the cell, turned a random quarter and stretched a random amount
// upward.
func buildTree(x, y, z int) {
	o := &objects[numObjects]
	numObjects++
	treeRot = yaroze.SVECTOR{Vy: int16(yaroze.Rand()%4) * 0x400}
	treeScale = yaroze.VECTOR{Vx: 0x1000, Vy: 0x1000 + yaroze.Rand()%0x400, Vz: 0x1000}
	o.Flg = 0
	yaroze.RotMatrix(&treeRot, &o.Coord)
	yaroze.ScaleMatrix(&o.Coord, &treeScale)
	o.Coord.T = [3]int32{int32(x*0x200 + 0x100), int32((1 - y) * 0x200), int32(-(z*0x200 + 0x100))}
	o.Super = yaroze.WORLD
}

// buildStairs builds a cell of stairs, following the original's run of
// cases: a whole block where stairs stand over it; otherwise side walls
// and a top of steps running toward whichever neighbor they lead to,
// with the risers' textures turned to match, and a closing face a unit up
// where the run ends.
func buildStairs(x, y, z int) {
	if y == 0 || cell(x, y-1, z) == 2 || cell(x, y-1, z) == 3 || cell(x, y-1, z) == 4 {
		shadeTop(x<<9, (y-1)*0x200, z<<9)
	}
	if y == 0 {
		addFace(x<<9, -0x200, z<<9, faceTop, 0xb, -1, 0, false)
	}
	X, Y, Z := x<<9, y*0x200, z<<9
	var topRow, tex, lastRow, lastFace int
	var lastFlip bool
	if cell(x, y+1, z) == 6 {
		for f := 1; f < 5; f++ {
			addFace(X, Y, Z, f, 0x20, -1, 0, false)
			addFace(X, Y+1, Z, f, 0x20, -1, 0, true)
		}
		goto water
	}
	if cell(x-1, y, z) == 6 || cell(x-1, y, z) == 0 || cell(x, y, z-1) == 6 {
		goto sideways
	}
	if cell(x, y, z+1) == 6 {
		goto sideways
	}
	topRow = 0
	addFace(X, Y, Z, 1, 0x1e, -1, 0, false)
	addFace(X, Y, Z, 3, 0x1e, -1, 3, false)
	addFace(X, Y+1, Z, 1, 0x1e, -1, 3, true)
	tex, lastRow, lastFlip, lastFace = 0x1e, 0, true, 3
	goto closing

sideways:
	if cell(x+1, y, z) != 6 && cell(x+1, y, z) != 0 && cell(x, y, z-1) != 6 {
		if cell(x, y, z+1) != 6 {
			topRow = 0
			addFace(X, Y, Z, 1, 0x1e, -1, 3, false)
			addFace(X, Y, Z, 3, 0x1e, -1, 0, false)
			addFace(X, Y+1, Z, 1, 0x1e, -1, 0, true)
			tex, lastRow, lastFlip, lastFace = 0x1e, 3, true, 3
			goto closing
		}
	}
	if cell(x, y, z-1) != 6 {
		if cell(x, y, z-1) == 0 || cell(x-1, y, z) == 6 || cell(x+1, y, z) == 6 {
			goto south
		}
		addFace(X, Y, Z, 2, 0x1e, -1, 0, false)
		addFace(X, Y, Z, 4, 0x1e, -1, 3, false)
		addFace(X, Y+1, Z, 2, 0x1e, -1, 3, true)
		tex, lastRow = 0x1e, 0
		goto acrossClosing
	}
south:
	if cell(x, y, z+1) != 6 && cell(x, y, z+1) != 0 && cell(x-1, y, z) != 6 && cell(x+1, y, z) != 6 {
		addFace(X, Y, Z, 2, 0x1e, -1, 3, false)
		addFace(X, Y, Z, 4, 0x1e, -1, 0, false)
		addFace(X, Y+1, Z, 2, 0x1e, -1, 0, true)
		tex, lastRow = 0x1e, 3
		goto acrossClosing
	}
	if cell(x, y-1, z) != 6 {
		if cell(x-1, y, z) == 6 || cell(x+1, y, z) == 6 {
			topRow = 0
			addFace(X, Y, Z, 1, 0x1d, -1, 0, false)
			addFace(X, Y, Z, 3, 0x1d, -1, 0, false)
			addFace(X, Y+1, Z, 1, 0x1d, -1, 0, true)
			tex, lastRow, lastFlip, lastFace = 0x1d, 0, true, 3
			goto closing
		}
		addFace(X, Y, Z, 2, 0x1d, -1, 0, false)
		addFace(X, Y, Z, 4, 0x1d, -1, 0, false)
		addFace(X, Y+1, Z, 2, 0x1d, -1, 0, true)
		tex, lastRow = 0x1d, 0
		goto acrossClosing
	}
	if cell(x-1, y, z) == 6 || cell(x+1, y, z) == 6 {
		topRow = 0
	} else {
		topRow = 1
	}
	for f := 1; f < 5; f++ {
		addFace(X, Y, Z, f, 0x1f, -1, 0, false)
		addFace(X, Y+1, Z, f, 0x1f, -1, 0, true)
	}
	goto top

acrossClosing:
	lastFace, lastFlip, topRow = 4, true, 1
closing:
	addFace(X, Y+1, Z, lastFace, tex, -1, lastRow, lastFlip)
top:
	addFace(X, Y, Z, faceTop, 0x1c, -1, topRow, false)
	addFace(X, Y, Z, faceTop, 0x1c, -1, topRow, true)
water:
	stairsWaterEdge(x, y, z)
}

// stairsWaterEdge adds a water surface over stairs that stand in water
// with open air beside them a level up.
func stairsWaterEdge(x, y, z int) {
	if cell(x-1, y, z) == 1 || cell(x+1, y, z) == 1 || cell(x, y, z-1) == 1 || cell(x, y, z+1) == 1 {
		if cell(x-1, y+1, z) == 0 || cell(x+1, y+1, z) == 0 || cell(x, y+1, z-1) == 0 || cell(x, y+1, z+1) == 0 {
			addFace(x<<9, y*0x200-0x80, z<<9, faceTop, 10, 0, 0, false)
		}
	}
}

// buildTower builds a pillar or a tower: three blocks high, each side
// whose neighbor is open in three textures top to bottom, and a cap.
func buildTower(x, y, z, t int) {
	X, Z := x<<9, z<<9
	for f := 1; f < 5; f++ {
		nb := &faceNeighbors[f]
		n := cell(x+nb[0], y+nb[1], z+nb[2])
		if solid(n) || n == 7 || n == 8 {
			continue
		}
		low, mid, high := 0x26, 0x25, 0x24
		if t == 7 {
			low, mid, high = 0x23, 0x22, 0x21
		}
		addFace(X, y<<9, Z, f, low, -1, 0, false)
		addFace(X, (y+1)*0x200, Z, f, mid, -1, 0, false)
		addFace(X, (y+2)*0x200, Z, f, high, -1, 0, false)
	}
	addFace(X, (y+2)*0x200, Z, faceTop, 0x29, -1, 0, false)
}

// Working storage for lightWorld.
var worldRGB [3]uint32

// lightWorld finishes the mesh: each vertex's normal is normalized, dimmed
// to a sixteenth in shadow, or pointed straight up and four times over on
// the map's edge, and then every quad's corners are lit under the world's
// light for the colors it is drawn with, at the top of the mesh's
// transparency codes.
func lightWorld() {
	for i := range numVerts {
		v := &verts[i]
		normalize(&v.n)
		if v.border != 0 {
			v.n = yaroze.SVECTOR{Vy: -0x4000}
		} else if v.shadow != 0 {
			v.n.Vx >>= 4
			v.n.Vy >>= 4
			v.n.Vz >>= 4
		}
	}
	yaroze.GsSetLsMatrix(yaroze.GsWSMATRIX())
	yaroze.GsSetLightMatrix(&yaroze.IDMATRIX)
	for i := range numPrims {
		p := &prims[i]
		if p.abr == -1 {
			yaroze.SetRGBC(0x3c808080)
		} else {
			yaroze.SetRGBC(0x3e808080)
		}
		yaroze.NormalColorCol3(&verts[p.v[0]].n, &verts[p.v[1]].n, &verts[p.v[2]].n, &worldRGB)
		p.rgb[0], p.rgb[1], p.rgb[2] = worldRGB[0], worldRGB[1], worldRGB[2]
		p.rgb[3] = yaroze.NormalColorCol(&verts[p.v[3]].n)
	}
}

// normalize scales the vector to a length of 4096 the way the original
// did it with the GTE: the squared length over 4096, its square root, and
// a scale of 0x40000 over that, applied over 4096 again; a zero vector
// stays zero.
func normalize(v *yaroze.SVECTOR) {
	x, y, z := int(v.Vx), int(v.Vy), int(v.Vz)
	n2 := (x*x + y*y + z*z) >> 12
	if n2 == 0 {
		*v = yaroze.SVECTOR{}
		return
	}
	scale := 0x40000 / isqrt(n2)
	v.Vx = int16((x * scale) >> 12)
	v.Vy = int16((y * scale) >> 12)
	v.Vz = int16((z * scale) >> 12)
}

// isqrt is the integer square root by Newton's method, as the original
// computed it.
func isqrt(n int) int {
	if n == 0 {
		return 0
	}
	x := n
	for {
		next := (n/x + x) >> 1
		if next >= x {
			return x
		}
		x = next
	}
}

// Working storage for the drawing.
var (
	chunkXYZ yaroze.VECTOR
	worldSXY [3]uint32
	worldSZ  [4]int32
)

// drawWorld draws the mesh: every chunk whose box the camera can see, its
// quads and then its trees, into the frame's ordering table.
func drawWorld(ot *yaroze.GsOT) {
	yaroze.GsSetLsMatrix(yaroze.GsWSMATRIX())
	for i := range chunks {
		ch := &chunks[i]
		if ch.count == 0 || !chunkInView(ch) {
			continue
		}
		drawPrims(ot, int(ch.first), int(ch.count))
		drawTrees(ot, ch)
	}
}

// chunkInView reports whether the chunk's box is in view: a corner of it
// within the view's pyramid, or corners in front of the camera on both
// sides of the middle, which is as far as the original looked.
func chunkInView(ch *chunk) bool {
	minX, maxX, maxZ := 1000000, -1000000, -1000000
	for i := range ch.corners {
		yaroze.RotTrans(&ch.corners[i], &chunkXYZ)
		if chunkXYZ.Vz < 0 {
			continue
		}
		x, y, z := int(chunkXYZ.Vx), int(chunkXYZ.Vy), int(chunkXYZ.Vz)>>1
		if abs(x) <= z && abs(y) <= z {
			return true
		}
		minX = min(minX, x)
		maxX = max(maxX, x)
		maxZ = max(maxZ, z)
	}
	return maxZ > 0 && minX < 0 && maxX > 0
}

// drawPrims draws count quads from first: each projected, kept if it faces
// the camera and projects whole, and sorted at its farthest corner's depth
// plus its bias.
func drawPrims(ot *yaroze.GsOT, first, count int) {
	shift := 14 - ot.Length
	maxDepth := int32(1)<<ot.Length - 1
	for i := first; i < first+count; i++ {
		p := &prims[i]
		flag := yaroze.RotTransPers3(&verts[p.v[0]].pos, &verts[p.v[1]].pos, &verts[p.v[2]].pos, &worldSXY)
		pk := packet(13)
		pk[3] = uint32(p.uv0) | uint32(p.clut)<<16
		pk[6] = uint32(p.uv1) | uint32(p.tpage)<<16
		if flag < 0 {
			continue
		}
		clip := yaroze.NormalClip()
		pk[9] = uint32(p.uv2)
		if clip < 0 {
			continue
		}
		pk[2], pk[5], pk[8] = worldSXY[0], worldSXY[1], worldSXY[2]
		yaroze.RotTransPers(&verts[p.v[3]].pos, &sxy3)
		pk[12] = uint32(p.uv3)
		yaroze.ReadSZ4(&worldSZ)
		z := max(worldSZ[0], worldSZ[1], worldSZ[2], worldSZ[3]) >> 2
		pk[11] = sxy3
		pk[1], pk[4], pk[7], pk[10] = p.rgb[0], p.rgb[1], p.rgb[2], p.rgb[3]
		pk[0] = 12 << 24
		ot.Link(int(min(max((int32(p.bias)+z)>>shift, 0), maxDepth)), pk)
		commit(13)
	}
}

// drawTrees draws the chunk's trees: the tree model at each instance, a
// dozen depths behind the mesh, with the GTE's matrices kept around them.
func drawTrees(ot *yaroze.GsOT, ch *chunk) {
	if ch.numObj == 0 {
		return
	}
	yaroze.PushMatrix()
	for i := ch.firstObj; i < ch.firstObj+ch.numObj; i++ {
		drawStatic(models[0], &objects[i], ot, 12)
	}
	yaroze.PopMatrix()
}

// drawStatic draws a model in its rest pose at a coordinate system, with
// every depth moved down by offset entries.
func drawStatic(m *model, coord *yaroze.GsCOORDINATE2, ot *yaroze.GsOT, offset int32) {
	ctx := drawContext{
		m:     m,
		words: unsafe.Pointer(&m.words[0]),
		ot:    &shiftedOT,
		shift: 14 - ot.Length,
		max:   1<<ot.Length - 1 - offset,
	}
	shiftedOT = *ot
	shiftedOT.Org = (*yaroze.GsOT_TAG)(unsafe.Add(unsafe.Pointer(ot.Org), 4*offset))
	yaroze.GsGetLws(coord, &lwMatrix, &lsMatrix)
	yaroze.GsSetLightMatrix(&lwMatrix)
	yaroze.GsSetLsMatrix(&lsMatrix)
	ctx.drawChain(m.chains[1])
}

// Working storage for animateTextures.
var texRect, clutRect yaroze.RECT

// copyTexture moves src's pixels, and its palette if clut is set, over
// dst's in video memory.
func copyTexture(src, dst *texture, clut bool) {
	texRect = yaroze.RECT{X: src.px, Y: src.py, W: src.w >> 2, H: src.h}
	yaroze.MoveImage(&texRect, int32(dst.px), int32(dst.py))
	if clut {
		clutRect = yaroze.RECT{X: src.cx, Y: src.cy, W: 16, H: 1}
		yaroze.MoveImage(&clutRect, int32(dst.cx), int32(dst.cy))
	}
}

// animateTextures moves the next frame of each animated texture into
// place: the water, the sea and the torches, from runs of frames in the
// tile table, and in the ruins the waterfall with its palette, a frame
// every four ticks of a clock of 32.
func animateTextures() {
	frame := textureFrame >> 2
	copyTexture(&tiles[12+frame], &tiles[10], false)
	copyTexture(&tiles[20+frame], &tiles[11], false)
	copyTexture(&tiles[52+frame], &tiles[51], false)
	if areaStyle == areaIndoor {
		copyTexture(&tiles[43+frame], &tiles[42], true)
	}
	textureFrame++
	if textureFrame == 32 {
		textureFrame = 0
	}
}
