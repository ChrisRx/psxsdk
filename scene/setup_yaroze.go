//go:build yaroze

package scene

import "go.chrisrx.dev/psx/yaroze"

// setup makes the library's calls for what parse read: the textures into
// video memory, the models mapped, the objects linked and the camera and
// lights set.
func (s *Scene) setup() {
	for i := range s.NTextures {
		s.Textures[i].Load()
	}
	for i := range s.NModels {
		s.Models[i].Map()
	}
	for i := range s.NObjects {
		o := &s.Objects[i]
		o.Init(yaroze.WORLD)
		o.Link(s.Models[s.objectModel[i]], s.objectIndex[i])
	}
	for i := range s.nLayers {
		// The cells are cut from the texture in reading order, as many
		// across as it is wide, from where it starts in its texture page;
		// its mode says its depth, 4-bit pixels being 4 to a unit of
		// video memory, 8-bit 2 and 16-bit 1.
		l := &s.layers[i]
		im := s.Textures[l.tex].Image()
		mode := im.Pmode & 3
		tpage := yaroze.GetTPage(int32(mode), 0, int32(im.Px), int32(im.Py))
		clut := yaroze.GetClut(int32(im.Cx), int32(im.Cy))
		perUnit := 4 >> mode
		u0, v0 := int(im.Px)%64*perUnit, int(im.Py)%256
		across := max(1, int(im.Pw)*perUnit/CellSize)
		for k := range l.cells {
			l.cells[k] = yaroze.GsCELL{U: uint8(u0 + k%across*CellSize), V: uint8(v0 + k/across*CellSize), Cba: clut, Tpage: tpage}
		}
		l.BG.Attribute = uint32(mode) << 24
		yaroze.GsInitFixBg16(&l.BG, &l.work[0])
	}
	// A file without a camera, at the origin looking at the origin, is a
	// room's, drawn through the program's own camera: it leaves the
	// camera, the projection and the lights as the program set them.
	c := &s.Camera
	if c.Vpx == 0 && c.Vpy == 0 && c.Vpz == 0 && c.Vrx == 0 && c.Vry == 0 && c.Vrz == 0 {
		return
	}
	yaroze.GsSetProjection(s.Projection)
	c.Set()
	for i := range s.NLights {
		s.Lights[i].Set(int32(i))
	}
	yaroze.GsSetAmbient(s.Ambient[0], s.Ambient[1], s.Ambient[2])
	yaroze.GsSetLightMode(0)
}
