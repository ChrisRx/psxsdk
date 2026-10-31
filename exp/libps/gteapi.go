//go:build yaroze

package libps

import "unsafe"

// The GTE commands a program issues itself, for polygons it builds
// rather than models it leaves to the library: as the yaroze package's
// gte_yaroze.go offers them, over the helpers the library's own model
// drawing uses. The matrices they transform through are what
// GsSetLsMatrix last loaded.

// RotTransPers3 projects three vertices to the screen with RTPT, stores
// their screen points in sxy, and returns the flag, negative when a
// result overflowed. Their depths stay in the coprocessor for AverageZ3
// and the points for NormalClip.
//
//go:inline
func RotTransPers3(v0, v1, v2 *SVECTOR, sxy *[3]uint32) int32 {
	gteSetV3(unsafe.Pointer(v0), unsafe.Pointer(v1), unsafe.Pointer(v2))
	gteRTPT()
	gteSXYTo(&sxy[0], &sxy[1], &sxy[2])
	return gteFlag()
}

// RotTransPers projects one vertex with RTPS, stores its screen point in
// sxy and returns the flag. The point and its depth push the
// coprocessor's queues, so after RotTransPers3 it makes a quad's fourth
// corner, which AverageZ4 then sees.
//
//go:inline
func RotTransPers(v *SVECTOR, sxy *uint32) int32 {
	gteSetV0(unsafe.Pointer(v))
	gteRTPS()
	*sxy = gteSXY2()
	return gteFlag()
}

// NormalClip is NCLIP on the last three screen points: the signed area
// they enclose, positive when they turn clockwise on the screen, a
// polygon facing the viewer.
//
//go:inline
func NormalClip() int32 { return gteNCLIP() }

// AverageZ3 and AverageZ4 are the average of the last three or four
// depths, scaled by the ordering table factor, as the depth to sort at.
//
//go:inline
func AverageZ3() int32 { return gteAVSZ3() }

//go:inline
func AverageZ4() int32 { return gteAVSZ4() }

// WSMatrix is the world-to-screen matrix of the camera last set, for a
// program that loads it with GsSetLsMatrix to transform its own points.
func WSMatrix() *MATRIX { return &GsWSMATRIX }
