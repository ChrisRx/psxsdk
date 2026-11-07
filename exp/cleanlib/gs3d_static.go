//go:build yaroze && !cleanimage

package cleanlib

// GsInitCoordinate2 makes base a coordinate system under super, or under
// the world when super is nil, its matrix a copy of GsIDMATRIX, and makes
// base super's Sub. It leaves base's Workm, Param and Sub. The image has it in
// assembly (gs3d_image.go).
//
//export GsInitCoordinate2
func GsInitCoordinate2(super, base *GsCOORDINATE2) {
	base.Coord = gsIDMATRIX
	base.Flg = 0
	base.Super = super
	if super != nil {
		super.Sub = base
	}
}
