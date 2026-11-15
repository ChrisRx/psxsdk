// A dinosaur walking in front of a castle, animated vertex by vertex by
// the GTE, to music with reverb, on the yaroze package's Go API: examples/dino
// written against the Go library's own surface. It draws the same
// frames, which the tests check.
//
// A port of Sony's Net Yaroze demo "dino" (1996). The dinosaur is one
// model of 2,724 polygons in a hundred objects, animated by the GTE's
// vertex interpolation: the model file comes with a file of vertex
// differences, nine sets of them, each moving part of the body, and
// every frame the model's vertices are put back as they were and each
// set is added in some proportion. The proportions come from the
// controls through a bank of transfer functions, sixteen filters that
// each smooth their input over the last few frames with a window of
// weights, so that a button pressed becomes a step taken. The walk
// itself is four of them cycling through a table. The castle is a
// full-screen 16-bit picture copied under every frame, the screen's
// backdrop, with the model drawn over it in fog under three lights.
//
// The demo lights the model with its matrix to the screen rather than
// to the world, so each object is drawn through Transform and Sort with
// the screen matrix set as the lights' too, rather than through Draw.
//
// # Controls
//
// The dinosaur walks by itself. Square and Circle turn it, Triangle and
// Cross tip it, the d-pad moves it across and up and down, and Start
// and Select bring it nearer and further. R2 opens its mouth, L2 blinks,
// R1 turns its head and L1 lifts its tail. Holding L1 and R1 with Select
// stops the walk and puts the legs on the buttons instead: R1, R2, L1
// and L2 lift each in turn, with Down the feet and with Up the knees.
// L1 and R1 with Start starts the walk again, and then Up and Down
// choose between the buttons above and R1 and R2 changing the walk's
// speed, with L1 putting it back. Start and Select together quit.
package main

import (
	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/yaroze"
)

const (
	objectMax = 100

	mvol = 127
	svol = 127

	// readTimeout is how many frames one disc read may take.
	readTimeout = 300
)

// The data, read into the program's own memory, in buffers of whole
// sectors, from the original example's assets. The sound bank's body is
// only needed until it has been sent to the sound processor, so the
// dinosaur's textures are read into its buffer afterwards, as the demo
// did; the buffer is sized for the larger of the two.
var (
	seqData [(39616 + 2047) / 2048 * 512]uint32
	vhData  [(60448 + 2047) / 2048 * 512]uint32
	vbData  [(401360 + 2047) / 2048 * 512]uint32
	md0Data [(115112 + 2047) / 2048 * 512]uint32
	vdfData [(56056 + 2047) / 2048 * 512]uint32
	castle  [(307220 + 2047) / 2048 * 512]uint32

	seqFile     = disc.Register("ASSETS/SOUND/SAMPLE1.SEQ", "../../../../examples/dino/assets/SAMPLE1.SEQ").Into(seqData[:])
	vh          = disc.Register("ASSETS/SOUND/STD0.VH", "../../../../examples/dino/assets/STD0.VH").Into(vhData[:])
	vb          = disc.Register("ASSETS/SOUND/STD0.VB", "../../../../examples/dino/assets/STD0.VB").Into(vbData[:])
	dinonewFile = disc.Register("ASSETS/DINONEW.TIM", "../../../../examples/dino/assets/DINONEW.TIM").Into(vbData[:])
	md0File     = disc.Register("ASSETS/MD0.TMD", "../../../../examples/dino/assets/MD0.TMD").Into(md0Data[:])
	vdfFile     = disc.Register("ASSETS/DINO.VDF", "../../../../examples/dino/assets/DINO.VDF").Into(vdfData[:])
	castleFile  = disc.Register("ASSETS/CASTLE.TIM", "../../../../examples/dino/assets/CASTLE.TIM").Into(castle[:])
)

var (
	// The screen: 640 by 240, dithered, its origin in the middle, with
	// frames of 1<<10 depths and room for five thousand packets.
	screen = yaroze.Screen{
		Width: 640, Height: 240,
		Depth:   10,
		Packets: 5000 * 24,
		Dither:  true,
		Center:  true,
	}

	model  yaroze.Model
	dino   yaroze.Object // the dinosaur's coordinate system
	parts  [objectMax]yaroze.Object
	objnum int

	// alpha is the speed of the walk; modeflag whether the dinosaur
	// walks by itself, and modeflagex which set of movements the buttons
	// control.
	alpha      int
	modeflag   int
	modeflagex int

	padd yaroze.Buttons

	bank yaroze.Bank
	song yaroze.Sequence
)

// The walk: which of the four leg movements is on at each of the sixteen
// steps of the cycle.
var (
	lf = [16]int{1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	rf = [16]int{0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 1, 0, 0, 0}
	ln = [16]int{0, 0, 0, 0, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	rn = [16]int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 0}
)

// blinktable is how far the eyelids are down at each frame of a two
// second cycle.
var blinktable = [120]int{
	0, 1024, 2048, 3072, 3072, 3072, 3072, 2048, 2048, 2048,
	1024, 1024, 1024, 768, 768, 512, 256, 128, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 2048, 4096, 4096, 4096, 4096, 4096,
	3072, 2048, 1024, 0, 0, 0, 2048, 4096, 2048, 0,
	0, 0, 0, 0, 512, 4096, 3072, 2048, 1024, 512,
	0, 0, 0, 512, 4096, 3072, 2048, 1024, 512, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 512, 1024, 2048, 2048, 2048, 2048, 2048,
	1024, 768, 512, 256, 0, 0, 0, 0, 0, 0,
}

func main() {
	screen.Open()

	if f := disc.LoadAll(readTimeout, &seqFile, &vh, &vb); f != nil {
		fail("could not read ", f.Name)
		return
	}
	sndSet(seqData[:], vh.Data(), vb.Data())

	if f := disc.LoadAll(readTimeout, &dinonewFile, &md0File, &vdfFile, &castleFile); f != nil {
		fail("could not read ", f.Name)
		return
	}

	modeflag = 1
	modeflagex = 0
	alpha = 32

	initTextures()
	screen.SetBackdrop(castle[:])
	modelInit()
	viewInit()
	lightInit()

	// The vertex interpolation.
	setVertexMimeData()

	// The transfer functions: the first four, the legs, with a sixteen
	// frame window; most of the rest with the longer one; and the mouth
	// with the one that overshoots.
	initCntrlarry(cnv0[:])
	for i := 4; i <= 12; i++ {
		ctlfc[i].cnv = cnv3[:]
	}
	ctlfc[9].cnv = cnv2[:]

	sndPlay()

	screen.SetVideo(yaroze.PAL)
	screen.Shift(0, 27)

	for frame := range screen.Frames() {
		n := uint32(frame.Number())
		if objInteractive() < 0 {
			break
		}

		setCntrl(n)
		// The demo turned the matrix before this frame's turn and
		// moved it after, so the frame is drawn turned as of the last
		// frame and placed as of this one.
		dino.Position.Z += ctlfc[8].out
		dino.Position.X += ctlfc[6].out
		dino.Position.Y += ctlfc[7].out

		if modeflag != 0 {
			// The walk: a step of the cycle every so many frames,
			// by alpha.
			ph2 := int(n & 0xffff)
			ph1 := ((ph2 * alpha) >> 6) % 16
			if ph1 < 0 {
				ph1 += 16
			}
			ctlfc[0].in = lf[ph1] * 4096
			ctlfc[1].in = rf[ph1] * 4096
			ctlfc[2].in = ln[ph1] * 6000
			ctlfc[3].in = rn[ph1] * 6000
		}
		mimepr[0] = ctlfc[0].out
		mimepr[1] = ctlfc[1].out
		mimepr[2] = ctlfc[2].out
		mimepr[3] = ctlfc[3].out
		mimepr[4] = ctlfc[9].out
		mimepr[5] = ctlfc[10].out
		mimepr[6] = ctlfc[11].out
		mimepr[7] = pick(n)
		mimepr[8] = ctlfc[12].out
		vertexMime()

		// Every part is in the dinosaur's coordinates, so its matrix
		// to the screen is the dinosaur's, which lights it as well.
		var world, toScreen yaroze.Matrix
		dino.Transform(&world, &toScreen)
		yaroze.SetLightMatrix(&toScreen)
		yaroze.SetScreenMatrix(&toScreen)
		for i := range objnum {
			parts[i].Sort(frame)
		}
		dino.Rotation.Y += ctlfc[4].out
		dino.Rotation.X += ctlfc[5].out

		padd = yaroze.PadButtons(0)
	}

	sndStop()
}

// pick returns how far the eyelids are down this frame.
func pick(x uint32) int {
	return blinktable[x%120]
}

// pressed reports whether any of the buttons in bits are down.
func pressed(bits yaroze.Buttons) bool { return padd.Any(bits) }

// choose returns by if up is pressed, -by if down is, and 0 otherwise.
func choose(up, down yaroze.Buttons, by int) int {
	switch {
	case pressed(up):
		return by
	case pressed(down):
		return -by
	}
	return 0
}

// onOff returns on while bits are pressed, and 0 otherwise.
func onOff(bits yaroze.Buttons, on int) int {
	if pressed(bits) {
		return on
	}
	return 0
}

// objInteractive turns the pad into the transfer functions' inputs, and
// returns -1 when the program is to quit.
func objInteractive() int {
	ctlfc[4].in = choose(yaroze.Square, yaroze.Circle, 6*4096/360)
	ctlfc[5].in = choose(yaroze.Triangle, yaroze.Cross, 6*4096/360)
	ctlfc[8].in = choose(yaroze.Select, yaroze.Start, 300)
	ctlfc[6].in = choose(yaroze.Left, yaroze.Right, 200)
	ctlfc[7].in = choose(yaroze.Down, yaroze.Up, 200)

	// L1 and R1 with another button switch modes.
	if pressed(yaroze.R1) && pressed(yaroze.L1) {
		switch {
		case pressed(yaroze.Select):
			modeflag = 0
			ctlfc[0].in, ctlfc[1].in, ctlfc[2].in, ctlfc[3].in = 0, 0, 0, 0
		case pressed(yaroze.Start):
			modeflag = 1
			ctlfc[9].in, ctlfc[10].in, ctlfc[11].in, ctlfc[12].in = 0, 0, 0, 0
		case pressed(yaroze.Up):
			modeflagex = 0
			ctlfc[0].in, ctlfc[1].in, ctlfc[2].in, ctlfc[3].in = 0, 0, 0, 0
		case pressed(yaroze.Down):
			modeflagex = 1
			ctlfc[9].in, ctlfc[10].in, ctlfc[11].in, ctlfc[12].in = 0, 0, 0, 0
		}
	}

	switch {
	case modeflag == 0 && modeflagex == 1: // the feet on the buttons
		ctlfc[0].in = onOff(yaroze.R1, 4096)
		ctlfc[1].in = onOff(yaroze.R2, 4096)
		ctlfc[2].in = onOff(yaroze.L1, 4096)
		ctlfc[3].in = onOff(yaroze.L2, 4096)
	case modeflag == 0: // the knees, and the mouth, the eyes and the head
		ctlfc[9].in = onOff(yaroze.R2, 4096)
		ctlfc[10].in = onOff(yaroze.L2, 4096)
		ctlfc[11].in = onOff(yaroze.R1, 6000)
		if pressed(yaroze.L1) {
			ctlfc[0].in = 4096
		} else {
			ctlfc[0].in = 0
			ctlfc[12].in = 0
		}
	case modeflagex == 1: // walking: the speed of the walk
		if pressed(yaroze.R1) {
			alpha++
		}
		if pressed(yaroze.R2) {
			alpha--
		}
		if pressed(yaroze.L1) {
			alpha = 32
		}
	default: // walking: the mouth, the eyes, the head and the tail
		ctlfc[9].in = onOff(yaroze.R2, 6000)
		ctlfc[10].in = onOff(yaroze.L2, 4096)
		ctlfc[11].in = onOff(yaroze.R1, 6000)
		ctlfc[12].in = onOff(yaroze.L1, 6000)
	}

	if pressed(yaroze.Start) && pressed(yaroze.Select) {
		return -1
	}
	return 0
}

func viewInit() {
	camera := yaroze.Camera{Eye: yaroze.Vector{Z: 2000}, Distance: 1000}
	camera.Set()
}

func lightInit() {
	yaroze.SetLight(0, yaroze.Light{
		Direction: yaroze.Vector{X: 20, Y: -100, Z: 100},
		Color:     yaroze.Color{R: 0xf0, G: 0xf0, B: 0xf0},
	})
	yaroze.SetLight(1, yaroze.Light{
		Direction: yaroze.Vector{X: 20, Y: -50, Z: -100},
		Color:     yaroze.Color{R: 0x80, G: 0x80, B: 0x80},
	})
	yaroze.SetLight(2, yaroze.Light{
		Direction: yaroze.Vector{X: -20, Y: 20, Z: 100},
		Color:     yaroze.Color{R: 0x80, G: 0x80, B: 0x80},
	})
	yaroze.SetAmbient(yaroze.One/4, yaroze.One/4, yaroze.One/4)

	// The fog the demo set by hand: black, from 25000 away, and never
	// complete, since its offset came out as 1.0 rather than the usual
	// 1.25.
	yaroze.SetFog(yaroze.Fog{Near: 25000})
}

// initTextures loads the dinosaur's textures into video memory: a file
// of TIMs one after another, each with its palette.
func initTextures() {
	data := vbData[:dinonewFile.Size()/4]
	for len(data) > 0 && data[0] == 0x10 {
		im := yaroze.LoadTIM(data)
		data = data[im.Len():]
	}
}

// modelInit maps the model and links a part to each of its objects, all
// in the dinosaur's coordinate system, which starts far back.
func modelInit() {
	model = yaroze.LoadTMD(md0Data[:])
	dino.Init(nil)
	dino.Position.Z = -30000
	objnum = min(model.Objects(), objectMax)
	for i := range objnum {
		parts[i].Init(&dino)
		parts[i].Link(model, i)
	}
}

// fail shows what went wrong until the program is quit.
func fail(what, detail string) {
	text := screen.Font(16, 16, 320, 32, 128)
	for range screen.Frames() {
		if yaroze.PadButtons(0).Has(yaroze.Start | yaroze.Select) {
			break
		}
		text.Print(what).Print(detail)
	}
	screen.Close()
}

// The music: the demo's snd.c.

// sndSet sends the sound bank to the sound processor and opens the
// sequence, with the volume down until it plays.
func sndSet(seq, vh, vb []uint32) {
	yaroze.SetVolume(0, 0)
	bank, _ = yaroze.OpenBank(vh, vb)
	song, _ = bank.Sequence(seq)
}

func sndPlay() {
	yaroze.SetVolume(mvol, mvol)
	song.SetVolume(svol, svol)
	yaroze.SetReverb(yaroze.ReverbStudioC, 80, 80)
	song.Loop()
}

func sndStop() {
	song.Stop()
	screen.Wait(2)
	song.Close()
	bank.Close()
}
