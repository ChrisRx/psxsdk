package yaroze

// Go verbs on the library's types. Each is the library function it names,
// with the type's own value as the argument it takes first, and inlines to
// the same call; the functions remain for anything the methods leave out.
// The types keep the header's names so that the documentation reads across,
// and the methods say what a program does with them.

// Clear is GsClearOt on the table, with no offset and no representative
// depth.
//
//go:inline
func (ot *GsOT) Clear() { GsClearOt(0, 0, ot) }

// Draw is GsDrawOt on the table.
//
//go:inline
func (ot *GsOT) Draw() { GsDrawOt(ot) }

// SortInto is GsSortOt: it sorts the whole table into dest at the depth
// ot.Point gives.
//
//go:inline
func (ot *GsOT) SortInto(dest *GsOT) { GsSortOt(ot, dest) }

// Sort is GsSortSprite on the sprite, with rotation and scaling.
//
//go:inline
func (sp *GsSPRITE) Sort(ot *GsOT, pri uint16) { GsSortSprite(sp, ot, pri) }

// SortFast is GsSortFastSprite on the sprite, without rotation or scaling.
//
//go:inline
func (sp *GsSPRITE) SortFast(ot *GsOT, pri uint16) { GsSortFastSprite(sp, ot, pri) }

// Sort is GsSortBoxFill on the rectangle.
//
//go:inline
func (bp *GsBOXF) Sort(ot *GsOT, pri uint16) { GsSortBoxFill(bp, ot, pri) }

// Sort is GsSortLine on the line.
//
//go:inline
func (lp *GsLINE) Sort(ot *GsOT, pri uint16) { GsSortLine(lp, ot, pri) }

// Sort is GsSortGLine on the line.
//
//go:inline
func (lp *GsGLINE) Sort(ot *GsOT, pri uint16) { GsSortGLine(lp, ot, pri) }

// Sort is GsSortObject4 on the object, with the CPU's scratchpad as the
// work space. shift is 14 minus the table's Length.
//
//go:inline
func (o *GsDOBJ2) Sort(ot *GsOT, shift int32) { GsSortObject4(o, ot, shift, Scratch()) }

// Init is GsInitCoordinate2 on the coordinate system, with super as its
// parent, or WORLD.
//
//go:inline
func (c *GsCOORDINATE2) Init(super *GsCOORDINATE2) { GsInitCoordinate2(super, c) }

// Lw is GsGetLw: the transformation from the coordinate system to the
// world, into out.
//
//go:inline
func (c *GsCOORDINATE2) Lw(out *MATRIX) { GsGetLw(c, out) }

// Ls is GsGetLs: the transformation from the coordinate system to the
// screen, into out.
//
//go:inline
func (c *GsCOORDINATE2) Ls(out *MATRIX) { GsGetLs(c, out) }

// Lws is GsGetLws: the transformations from the coordinate system to the
// world and to the screen, into outw and outs.
//
//go:inline
func (c *GsCOORDINATE2) Lws(outw, outs *MATRIX) { GsGetLws(c, outw, outs) }

// Rotate is RotMatrix: it makes the matrix a rotation by the angles in r,
// with 4096 as a full turn.
//
//go:inline
func (m *MATRIX) Rotate(r *SVECTOR) *MATRIX { return RotMatrix(r, m) }

// RotateX is RotMatrixX: it rotates the matrix about the X axis by r, with
// 4096 as a full turn.
//
//go:inline
func (m *MATRIX) RotateX(r int32) *MATRIX { return RotMatrixX(r, m) }

// RotateY is RotMatrixY: it rotates the matrix about the Y axis by r, with
// 4096 as a full turn.
//
//go:inline
func (m *MATRIX) RotateY(r int32) *MATRIX { return RotMatrixY(r, m) }

// RotateZ is RotMatrixZ: it rotates the matrix about the Z axis by r, with
// 4096 as a full turn.
//
//go:inline
func (m *MATRIX) RotateZ(r int32) *MATRIX { return RotMatrixZ(r, m) }

// Translate is TransMatrix: it sets the matrix's translation to v.
//
//go:inline
func (m *MATRIX) Translate(v *VECTOR) *MATRIX { return TransMatrix(m, v) }

// Scale is ScaleMatrix: it scales the matrix by v, with 4096 as 1.0.
//
//go:inline
func (m *MATRIX) Scale(v *VECTOR) *MATRIX { return ScaleMatrix(m, v) }

// Mul is MulMatrix0: the matrix's rotation times by's, into out, which it
// returns.
//
//go:inline
func (m *MATRIX) Mul(by, out *MATRIX) *MATRIX { return MulMatrix0(m, by, out) }

// Comp is CompMatrix: the matrix's transformation followed by by's,
// translation included, into out, which it returns.
//
//go:inline
func (m *MATRIX) Comp(by, out *MATRIX) *MATRIX { return CompMatrix(m, by, out) }

// Transpose is TransposeMatrix: the transpose of the matrix's rotation,
// into out, which it returns.
//
//go:inline
func (m *MATRIX) Transpose(out *MATRIX) *MATRIX { return TransposeMatrix(m, out) }

// Apply is ApplyMatrix: the matrix's rotation applied to v0, into v1, which
// it returns.
//
//go:inline
func (m *MATRIX) Apply(v0 *SVECTOR, v1 *VECTOR) *VECTOR { return ApplyMatrix(m, v0, v1) }

// Set is GsSetRefView2 on the view.
//
//go:inline
func (pv *GsRVIEW2) Set() int32 { return GsSetRefView2(pv) }

// Set is GsSetView2 on the view.
//
//go:inline
func (pv *GsVIEW2) Set() int32 { return GsSetView2(pv) }

// Set is GsSetFlatLight: it installs the light as light id, 0, 1 or 2.
//
//go:inline
func (lt *GsF_LIGHT) Set(id int32) int32 { return GsSetFlatLight(id, lt) }

// Set is GsSetFogParam on the fog.
//
//go:inline
func (fp *GsFOGPARAM) Set() { GsSetFogParam(fp) }

// Read is GsGetTimInfo: it fills the image in from the TIM whose header is
// at tim, the word after the file's ID.
//
//go:inline
func (im *GsIMAGE) Read(tim *uint32) { GsGetTimInfo(tim, im) }

// Load puts the image into video memory, where its header says: the pixels,
// and the palette if it has one. The transfers are queued, and DrawSync
// waits for them.
func (im *GsIMAGE) Load() {
	rect := RECT{X: im.Px, Y: im.Py, W: int16(im.Pw), H: int16(im.Ph)}
	LoadImage(&rect, im.Pixel)
	if im.Pmode&8 != 0 {
		rect = RECT{X: im.Cx, Y: im.Cy, W: int16(im.Cw), H: int16(im.Ch)}
		LoadImage(&rect, im.Clut)
	}
}

// Load is LoadImage: it copies pixels from memory at p into the rectangle.
//
//go:inline
func (r *RECT) Load(p *uint32) int32 { return LoadImage(r, p) }

// Store is StoreImage: it copies the rectangle out to memory at p.
//
//go:inline
func (r *RECT) Store(p *uint32) int32 { return StoreImage(r, p) }

// Clear is ClearImage: it fills the rectangle with a color.
//
//go:inline
func (r *RECT) Clear(red, green, blue uint8) int32 { return ClearImage(r, red, green, blue) }

// Move is MoveImage: it copies the rectangle to x, y.
//
//go:inline
func (r *RECT) Move(x, y int32) int32 { return MoveImage(r, x, y) }

// Put is PutDrawEnv on the environment.
//
//go:inline
func (env *DRAWENV) Put() *DRAWENV { return PutDrawEnv(env) }

// Put is PutDispEnv on the environment.
//
//go:inline
func (env *DISPENV) Put() *DISPENV { return PutDispEnv(env) }
