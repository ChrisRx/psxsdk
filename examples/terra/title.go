package main

// The title: over the port, once the opening has whited out, the logo
// fades in piece by piece with the choice of language under it, waits for
// a press, and fades away as the camera's tour of the port goes on into
// the first conversation.

import "go.chrisrx.dev/psx/gpu"

var (
	// sceneT is the frame counter the overlays share.
	sceneT int

	titleOn  bool
	language int // 0 for Japanese, 1 for English
)

// A titlePiece is one sprite of the logo: where it goes, its texture's
// color depth and transparency rate, and the language it belongs to, or -1
// for both.
type titlePiece struct {
	x, y     int16
	depth    uint32
	abr      uint32
	language int16
}

var titlePieces = [10]titlePiece{
	{19, 44, 1, 1, -1}, {147, 44, 1, 1, -1}, {275, 44, 1, 1, -1},
	{14, 40, 0, 2, -1}, {270, 40, 0, 2, -1},
	{61, 144, 0, 1, 0}, {63, 146, 0, 2, 0},
	{185, 144, 0, 1, 1}, {187, 146, 0, 2, 1},
	{111, 200, 0, 1, -1},
}

// titleStart puts the title up, from a white screen.
func titleStart() {
	sceneT = 0
	titleOn = true
	probeOn = true
	setAmbient(true)
}

// titleFrame draws a frame of the title and moves it on.
func titleFrame() {
	t := sceneT
	if t < 0x80 {
		c := max(0xff-2*t, 0)
		fillRect(over, 0, -160, -120, 320, 240, uint8(c), uint8(c), uint8(c), true, 1)
	}
	if t == 0x80 {
		spawnGull(0x1e, 5, 0x1e, -1)
		spawnGull(0x32, 3, 0x16, 0x16)
	}
	if t == 0x83 {
		spawnGull(0x21, 3, 0x1b, -1)
	}
	if t-0xa0 >= 0 && t-0xa0 < 0x1a1 {
		bright := (t - 0xa0) * 2
		if t > 0xdf {
			bright = 0x80
			if t >= 0x200 {
				bright = (0x240 - t) * 2
			}
		}
		for i := range titlePieces {
			piece := &titlePieces[i]
			tex := &panels[13+i]
			c := bright
			if piece.language != -1 && int(piece.language) != language {
				c = bright / 4
			}
			p := over.Packet[gpu.Sprite](0)
			p.Color = gray(uint8(c))
			p.Code |= gpu.CodeTranslucent
			p.XY = gpu.XY{X: piece.x - 0xa0, Y: piece.y - 0x78}
			p.UV = gpu.UV{U: tex.u, V: tex.v}
			p.Clut = tex.clut()
			p.W, p.H = uint16(tex.w), uint16(tex.h)
			over.Packet[gpu.DrawMode](0).Page = tex.mode() | piece.depth&3<<7 | piece.abr&3<<5
		}
	}
	if t == 0xe0 {
		r := padRepeat()
		if r&0x840 != 0 {
			t = 0x200
			playEffect(2)
		} else if r&0xa100 != 0 {
			language = 1 - language
			playEffect(2)
		}
	} else {
		t++
	}
	if t == 0x220 {
		nextCameraKey()
	}
	if t == 0x2c0 {
		titleOn = false
		startDialog(0, false)
	}
	sceneT = t
}
