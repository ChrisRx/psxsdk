//go:build yaroze

package scene

import "go.chrisrx.dev/psx/yaroze"

// setup makes the library's calls for what parse read: the textures into
// video memory, the models mapped, the objects linked and the camera and
// lights set, and the layers made ready for the screen.
func (s *Scene) setup() {
	for i := range s.NTextures {
		s.Textures[i] = yaroze.LoadTIM(s.textureWords[i])
	}
	for i := range s.NModels {
		s.Models[i] = yaroze.LoadTMD(s.Models[i])
	}
	for i := range s.NObjects {
		o := &s.Objects[i]
		o.Init(nil)
		o.Link(s.Models[s.objectModel[i]], s.objectIndex[i])
	}
	for i := range s.nLayers {
		// The cells are cut from the texture in reading order, as many
		// across as it is wide, from where it starts in its texture
		// page.
		l := &s.layers[i]
		im := s.Textures[l.tex]
		across := max(1, im.Width()/CellSize)
		for k := range l.cells {
			l.cells[k] = im.Cell(k%across*CellSize, k/across*CellSize)
		}
		l.BG.InitBuffer(s.screen, &l.tileMap, l.work[:])
	}
	// A file without a camera, at the origin looking at the origin, is a
	// room's, drawn through the program's own camera: it leaves the
	// camera, the projection and the lights as the program set them.
	c := &s.Camera
	if c.Eye == (yaroze.Vector{}) && c.Target == (yaroze.Vector{}) {
		return
	}
	c.Set()
	for i := range s.NLights {
		yaroze.SetLight(i, s.Lights[i])
	}
	yaroze.SetAmbient(s.Ambient[0], s.Ambient[1], s.Ambient[2])
	yaroze.SetFog(yaroze.Fog{})
}
