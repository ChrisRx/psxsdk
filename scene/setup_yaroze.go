//go:build yaroze

package scene

import "go.chrisrx.dev/psx/yaroze"

// setUp makes the library's calls for what parse read: the textures into
// video memory, the models mapped, the objects linked and the camera and
// lights set.
func (s *Scene) setUp() {
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
	if s.HasBackground {
		// The cells are cut from the texture in reading order, as many
		// across as it is wide; its brightness bits say its depth.
		im := s.Textures[s.backgroundTex].Image()
		tpage := yaroze.GetTPage(int32(im.Pmode&3), 0, int32(im.Px), int32(im.Py))
		clut := yaroze.GetClut(int32(im.Cx), int32(im.Cy))
		width := int(im.Pw) * 4 >> (im.Pmode & 3) // units to pixels: 4-bit is 4 a unit, 8-bit 2, 16-bit 1
		across := max(1, width/cellSize)
		for i := range s.cells {
			s.cells[i] = yaroze.GsCELL{U: uint8(i % across * cellSize), V: uint8(i / across * cellSize), Cba: clut, Tpage: tpage}
		}
		s.Background.Attribute = uint32(im.Pmode&3) << 24
		yaroze.GsInitFixBg16(&s.Background, &s.work[0])
	}
	yaroze.GsSetProjection(s.Projection)
	s.Camera.Set()
	for i := range s.NLights {
		s.Lights[i].Set(int32(i))
	}
	yaroze.GsSetAmbient(s.Ambient[0], s.Ambient[1], s.Ambient[2])
	yaroze.GsSetLightMode(0)
}
