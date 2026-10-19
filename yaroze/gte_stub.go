//go:build !yaroze

package yaroze

// The stubs of the GTE functions gte_yaroze.go declares, for every build that
// is not for the console, as libps_stub.go has for the library. Each panics
// through stub.

// RotTransPers3 transforms three vertices to the screen with RTPT, stores
// their screen points in sxy, and returns the flag. Their depths remain in
// the coprocessor for AverageZ3, and the points for NormalClip.
func RotTransPers3(v0, v1, v2 *SVECTOR, sxy *[3]uint32) int32 { panic(stub("RotTransPers3")) }

// RotTransPers transforms one vertex to the screen with RTPS, stores its
// screen point in sxy, and returns the flag. The point and its depth push
// the coprocessor's three-deep queues, so after RotTransPers3 it makes the
// fourth corner of a quad, and AverageZ4 sees all four depths.
func RotTransPers(v *SVECTOR, sxy *uint32) int32 { panic(stub("RotTransPers")) }

// NormalClip is NCLIP on the last three screen points: the signed area they
// enclose, positive when they turn clockwise on the screen, which is a
// polygon facing the viewer, and negative or zero for one to leave out.
func NormalClip() int32 { panic(stub("NormalClip")) }

// AverageZ3 is AVSZ3: the average of the last three depths, scaled by the
// ordering table factor the library set, as the depth to sort at.
func AverageZ3() int32 { panic(stub("AverageZ3")) }

// AverageZ4 is AVSZ4: the average of the last four depths, scaled, as the
// depth to sort at.
func AverageZ4() int32 { panic(stub("AverageZ4")) }

// ReadSZ3 returns the depth of the last vertex transformed, before it is
// averaged, for a program that keeps transformed vertices to reuse.
func ReadSZ3() int32 { panic(stub("ReadSZ3")) }

// NormalColorCol3 lights three normals with NCCT, the color each gives under
// the lights set, times the color SetRGBC gave, into rgb with its command
// code.
func NormalColorCol3(n0, n1, n2 *SVECTOR, rgb *[3]uint32) { panic(stub("NormalColorCol3")) }

// NormalColorCol lights one normal with NCCS and returns its color with the
// command code.
func NormalColorCol(n *SVECTOR) uint32 { panic(stub("NormalColorCol")) }

// NormalColor lights one normal with NCS, leaving out the color matrix: the
// result is the light falling on it, IR1 to IR3, which RotColor can turn
// into a color under any color later, for a program that lights its
// normals once and draws them many times.
func NormalColor(n *SVECTOR, ir *[3]int16) { panic(stub("NormalColor")) }

// RotColor multiplies the vector ir, as NormalColor left it, by the rotation
// matrix, clamped at zero, into out. With the rotation matrix set to a
// color on its diagonal, SetRotMatrix with a MATRIX whose M[0][0], M[1][1]
// and M[2][2] are red, green and blue, it colors a lit normal.
func RotColor(ir *[3]int16, out *[3]int32) { panic(stub("RotColor")) }

// RotTrans rotates and translates one vector into out, without projecting
// it, and returns the flag.
func RotTrans(v *SVECTOR, out *VECTOR) int32 { panic(stub("RotTrans")) }

// SetRGBC sets the color the lighting commands multiply by, with the GPU
// command code in the top byte, which they carry into every color they
// produce: 0x34808080 for a gouraud textured triangle at full brightness.
func SetRGBC(c uint32) { panic(stub("SetRGBC")) }

// SetRotMatrix loads the rotation matrix from m. GsSetLsMatrix does this
// too, with the translation; a program that sets a color matrix for
// RotColor sets the real one again afterwards.
func SetRotMatrix(m *MATRIX) { panic(stub("SetRotMatrix")) }

// SetTransMatrix loads the translation vector from m.
func SetTransMatrix(m *MATRIX) { panic(stub("SetTransMatrix")) }

// LoadScreen3 loads three screen points and depths that an earlier
// RotTransPers produced, so that NormalClip and AverageZ3 can run on
// vertices transformed once and shared between polygons.
func LoadScreen3(sxy, sz *[3]uint32) { panic(stub("LoadScreen3")) }

// LoadScreen4 loads three screen points and four depths, for NormalClip and
// AverageZ4 on a quad of vertices transformed earlier.
func LoadScreen4(sxy *[3]uint32, sz *[4]uint32) { panic(stub("LoadScreen4")) }

// ReadSZ4 stores the depths of the last four vertices transformed, which
// RotTransPers3 and then RotTransPers leave as a quad's four corners, for
// a program that sorts by the farthest rather than the average.
func ReadSZ4(sz *[4]int32) { panic(stub("ReadSZ4")) }

// ReadMAC0 returns MAC0, which after RotTransPers holds the depth cue the
// point was given, and after NormalClip the area.
func ReadMAC0() int32 { panic(stub("ReadMAC0")) }

// ReadFlag returns the flag register as the last command left it.
func ReadFlag() int32 { panic(stub("ReadFlag")) }
