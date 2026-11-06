//go:build yaroze

package cleanlib

import "unsafe"

// What the Go API, exp/ps, calls by name that the C library has no Go name
// for: entries it has under their C names, the sequences C leaves to macros
// in its headers, and its variables. None is exported to C, so nothing here
// reaches a program that does not call it, nor the resident image.

// Rand and Srand are rand and srand.
func Rand() int32       { return rand() }
func Srand(seed uint32) { srand(seed) }

// The memory card's files, by the kernel: open, read, write, close, format,
// delete, firstfile and nextfile.
func FileOpen(name *byte, flags int32) int32                  { return fileOpen(name, flags) }
func FileRead(fd int32, buf *byte, n int32) int32             { return fileRead(fd, buf, n) }
func FileWrite(fd int32, buf *byte, n int32) int32            { return fileWrite(fd, buf, n) }
func FileClose(fd int32) int32                                { return fileClose(fd) }
func FileFormat(name *byte) int32                             { return fileFormat(name) }
func FileDelete(name *byte) int32                             { return fileDelete(name) }
func FileFirst(name *byte, dir unsafe.Pointer) unsafe.Pointer { return fileFirst(name, dir) }
func FileNext(dir unsafe.Pointer) unsafe.Pointer              { return fileNext(dir) }

// DispEnv is GsDISPENV, the display environment GsSwapDispBuff puts.
func DispEnv() *DISPENV { return &gsDISPENV }

// WSMatrix is GsWSMATRIX, the world-to-screen matrix of the last view set.
func WSMatrix() *MATRIX { return &gsWSMATRIX }

// Origin is POSITION, the screen origin GsInit3D moves to the center.
func Origin() (x, y int16) { return position.Offx, position.Offy }

// FntWrite appends s to stream id as it stands, where FntPrint takes a format
// and writes to stream 0. An id that is not open is stream 0, as for FntFlush.
// What does not fit is dropped, and a full stream is not cut, which leaves the
// next stream's first byte alone: a flush stops at the stream's room anyway.
func FntWrite(id int32, s string) {
	FntWriteBytes(id, unsafe.Slice(unsafe.StringData(s), len(s)))
}

// FntWriteBytes is FntWrite on bytes.
func FntWriteBytes(id int32, b []byte) {
	if uint32(id) >= uint32(font.open) {
		id = 0
	}
	st := &font.streams[id]
	room := len(st.text) - 1
	if st.n >= room {
		return
	}
	st.n += copy(st.text[st.n:room], b)
	if st.n < room {
		st.text[st.n] = 0
	}
}

// GteMIMefunc is gteMIMefunc.
func GteMIMefunc(otp, dfp *SVECTOR, n, p int32) { gteMIMefunc(otp, dfp, n, p) }

// RotTransPers3 projects three vertices with RTPT, stores their screen points
// in sxy and returns the GTE's flag, negative when a result was out of range.
// Their depths stay in the GTE for AverageZ3, and the points for NormalClip.
func RotTransPers3(v0, v1, v2 *SVECTOR, sxy *[3]uint32) int32 {
	rtptAt(uintptr(unsafe.Pointer(v0)), uintptr(unsafe.Pointer(v1)), uintptr(unsafe.Pointer(v2)))
	sxy[0], sxy[1], sxy[2] = sxy0(), sxy1(), sxy2()
	return int32(flag())
}

// RotTransPers is RotTransPers3 on one vertex, with RTPS. Its point and depth
// push the GTE's queues, so after RotTransPers3 it makes a quad's fourth corner.
func RotTransPers(v *SVECTOR, sxy *uint32) int32 {
	rtpsAt(uintptr(unsafe.Pointer(v)))
	*sxy = sxy2()
	return int32(flag())
}

// NormalClip, AverageZ3 and AverageZ4 are NCLIP, AVSZ3 and AVSZ4.
func NormalClip() int32 { return nclip() }
func AverageZ3() int32  { return avsz3() }
func AverageZ4() int32  { return avsz4() }
