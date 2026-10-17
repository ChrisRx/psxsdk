// A box to move around the screen with the directional buttons: a program
// to start from.
package main

import (
	"go.chrisrx.dev/psx/yaroze"
)

type App struct {
	Width, Height uint16
	ot            [2]yaroze.GsOT
	otTags        [2][2]yaroze.GsOT_TAG
	packetArea    [2][1024 / 4]uint32
}

func NewApp() App {
	return App{
		Width:  320,
		Height: 240,
	}
}

func (a *App) Setup() {
	yaroze.SetVideoMode(yaroze.MODE_NTSC)
	yaroze.GsInitGraph(a.Width, a.Height, yaroze.GsOFSGPU, 1, 0)
	yaroze.GsDefDispBuff(0, 0, 0, a.Height)
	for i := range a.ot {
		a.ot[i].Length = 1
		a.ot[i].Org = &a.otTags[i][0]
	}
	yaroze.PadInit()
}

func (a *App) HandleInput(fn func(pad uint32)) {
	fn(yaroze.PadRead())
}

func (a *App) Render(fn func(ot *yaroze.GsOT)) {
	active := yaroze.GsGetActiveBuff()
	ot := &a.ot[active]
	yaroze.GsSetWorkBase(yaroze.Packets(&a.packetArea[active][0]))
	yaroze.GsClearOt(0, 0, ot)

	fn(ot)

	yaroze.DrawSync(0)
	yaroze.VSync(0)
	yaroze.GsSwapDispBuff()
	yaroze.GsSortClear(40, 60, 100, ot)
	yaroze.GsDrawOt(ot)
}

func main() {
	app := NewApp()
	app.Setup()

	var textBuf [64]byte
	hint := yaroze.CString(textBuf[:], "the directions move the box")
	yaroze.FntLoad(960, 256)
	yaroze.FntOpen(16, 16, 256, 64, 0, 512)

	box := yaroze.GsBOXF{
		X: int16(app.Width)/2 - 20,
		Y: int16(app.Height)/2 - 20,
		W: 40,
		H: 40,
		R: 255, G: 160, B: 32,
	}

	for {
		app.HandleInput(func(pad uint32) {
			if pad&yaroze.PADLup != 0 && box.Y > 0 {
				box.Y -= 2
			}
			if pad&yaroze.PADLdown != 0 && box.Y+int16(box.H) < int16(app.Height) {
				box.Y += 2
			}
			if pad&yaroze.PADLleft != 0 && box.X > 0 {
				box.X -= 2
			}
			if pad&yaroze.PADLright != 0 && box.X+int16(box.W) < int16(app.Width) {
				box.X += 2
			}
		})
		app.Render(func(ot *yaroze.GsOT) {
			yaroze.GsSortBoxFill(&box, ot, 0)
			yaroze.FntPrint(hint)
			yaroze.FntFlush(-1)
		})
	}
}
