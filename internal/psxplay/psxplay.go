// Package psxplay runs the emulator in a window, with the keyboard or a
// gamepad as the pad in port 1 and the SPU's sound on the speakers. It is
// what yaroze run plays a disc or an executable in, and the desktop
// counterpart of the player on the site, which runs the same emulator in a
// browser.
package psxplay

import (
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/spf13/pflag"

	"go.chrisrx.dev/psx/emu"
	"go.chrisrx.dev/psx/emu/openbios"
	"go.chrisrx.dev/psx/format/psx"
	"go.chrisrx.dev/psx/internal/flags"
	"go.chrisrx.dev/psx/internal/hot"
)

// frameBudget bounds the instructions a tick runs looking for a vertical
// blank, which comes every 560,000 cycles or so. A program that turns the
// display off, or hangs, never reaches one, and the tick gives up so the
// window stays responsive.
const frameBudget = 4_000_000

// bootBudget bounds the instructions the BIOS may take to reach its shell,
// where --exe is loaded.
const bootBudget = 50_000_000

// sampleRate is the SPU's output rate, which the audio context plays at
// directly.
const sampleRate = 44100

// Options say what to run and how.
type Options struct {
	Disc  string      // the cue sheet or .zip of a disc to put in the drive, or empty for none
	BIOS  string      // a BIOS image to boot instead of the OpenBIOS built in
	EXE   string      // a PSX-EXE to load when the BIOS reaches its shell
	Card  string      // a memory card image for slot 1
	Scale int         // the window's initial size, as a multiple of 320x240
	Mute  bool        // leave the sound off
	Fast  bool        // run as fast as the host allows, showing nothing, until the first picture
	Loads flags.Loads // files to copy into RAM, with EXE
	Title string      // the window's title, yaroze and what it runs if empty
	Shots string      // the directory a screenshot is written into, on F12 or SIGUSR1

	// Breaks are addresses to print the registers at each time the program
	// reaches one.
	Breaks flags.Addresses

	// Patches, if not nil, delivers hot reload patches for the program,
	// which must have been built with --hot, each applied between frames
	// as soon as it arrives, with a line on standard output.
	Patches <-chan *hot.Patch
}

// Keys is the help on the keyboard and the gamepad, for a command's long
// description.
const Keys = `The keyboard is the pad in port 1, laid out as the site's player has it:

  arrows        d-pad          Enter      Start
  Z X A S       cross circle   Backspace  Select
                square triangle
  Q W           L1 R1          1 2        L2 R2

A gamepad with a standard layout works as itself. F11 toggles full screen
and Escape leaves it. F12 saves the picture as a PNG, at the display's own
size, into the directory --shots names, and so does a SIGUSR1 sent to the
process, for a script to take one of a running window.`

// AddFlags adds the flags for the window, the BIOS, the memory card and an
// executable to load to fs. The files to load with the executable, Loads,
// are the caller's to fill.
func (o *Options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.BIOS, "bios", "", "BIOS image to boot instead of the OpenBIOS built in")
	fs.StringVar(&o.EXE, "exe", "", "PSX-EXE to load when the BIOS reaches its shell, as psxe -x does, in place of building a package")
	fs.StringVar(&o.Card, "card", "", "memory card image for slot 1, made blank if it does not exist, and written back on exit")
	fs.IntVar(&o.Scale, "scale", 2, "the window's initial size, as a multiple of 320x240")
	fs.BoolVar(&o.Mute, "mute", false, "leave the sound off")
	fs.BoolVar(&o.Fast, "fast-start", true, "run as fast as possible until the first picture, rather than through the BIOS's start in real time")
	fs.Var(&o.Breaks, "break", "an address, hexadecimal with 0x: print the registers each time the program reaches it; repeatable")
	fs.StringVar(&o.Shots, "shots", ".", "directory to write the picture into as a PNG on F12 or SIGUSR1")
}

// Run runs the machine in a window until the window is closed.
func Run(opts Options) error {
	bios := openbios.Image
	if opts.BIOS != "" {
		var err error
		if bios, err = os.ReadFile(opts.BIOS); err != nil {
			return err
		}
	}
	m, err := emu.New(bios)
	if err != nil {
		return err
	}
	m.TTY = os.Stdout
	m.SetAudio(!opts.Mute)
	for _, addr := range opts.Breaks {
		m.SetBreak(addr, func(m *emu.Machine) bool {
			fmt.Println(describeBreak(m))
			return false
		})
	}

	title := "yaroze"
	if opts.Disc != "" {
		d, err := emu.OpenDisc(opts.Disc)
		if err != nil {
			return err
		}
		defer d.Close()
		m.InsertDisc(d)
		title += " - " + opts.Disc
	}

	var card *emu.MemoryCard
	if opts.Card != "" {
		if card, err = emu.OpenMemoryCard(opts.Card); err != nil {
			return err
		}
		m.InsertCard(1, card)
	}

	if opts.EXE != "" {
		exe, err := psx.Open(opts.EXE)
		if err != nil {
			return err
		}
		if err := m.SideloadEXE(exe, bootBudget); err != nil {
			return err
		}
		for _, l := range opts.Loads {
			if err := m.LoadRAM(l.Addr, l.Data); err != nil {
				return err
			}
		}
		title += " - " + opts.EXE
	}

	g := &game{machine: m, fast: opts.Fast, patches: opts.Patches, shots: opts.Shots}
	watchShots(&g.shot)
	if !opts.Mute {
		g.sound = newSoundBuffer()
		player, err := audio.NewContext(sampleRate).NewPlayer(g.sound)
		if err != nil {
			return err
		}
		// The player's buffer has to hold more than the sound server takes
		// at once, PipeWire's quantum being 1323 samples (30ms) at this
		// rate, or each take runs it dry and the gap plays as silence.
		player.SetBufferSize(60 * time.Millisecond)
		player.Play()
	}

	if opts.Title != "" {
		title = opts.Title
	}
	scale := max(opts.Scale, 1)
	ebiten.SetWindowTitle(title)
	ebiten.SetWindowSize(320*scale, 240*scale)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetTPS(60)
	runErr := ebiten.RunGame(g)

	if card != nil {
		if err := os.WriteFile(opts.Card, card.Data(), 0o644); err != nil {
			return errors.Join(runErr, err)
		}
	}
	return runErr
}

// describeBreak says where the program is, for a breakpoint: its frame,
// and the registers a call's arguments, return address and stack are in.
func describeBreak(m *emu.Machine) string {
	c := m.CPU
	return fmt.Sprintf("break %08x frame %d a0 %08x a1 %08x a2 %08x a3 %08x ra %08x sp %08x",
		c.PC, m.Frames(), c.R[4], c.R[5], c.R[6], c.R[7], c.R[31], c.R[29])
}

// game is the window: it runs the machine a frame a tick, reads the pad
// from the keyboard and gamepads, and shows the display.
type game struct {
	machine *emu.Machine
	sound   *soundBuffer
	frame   *image.RGBA
	image   *ebiten.Image
	samples []int16

	level  float64 // the sound buffered, averaged, for frames
	settle int     // ticks before frames may adjust again

	// fast is whether the machine is still in its fast start, and
	// fastFrames how many frames it has run so far.
	fast       bool
	fastFrames int

	// patches delivers hot reload patches, applied as they arrive.
	patches <-chan *hot.Patch

	// shots is where a screenshot goes, and shot is set, from the signal's
	// goroutine, when one is asked for; the next tick takes it.
	shots string
	shot  atomic.Bool
	// flash is the ticks left of the brighter picture shown after a
	// screenshot, so that a press of F12 is seen to have done something.
	flash int
}

// flashTicks is how long the flash lasts: three ticks is a twentieth of a
// second, a blink.
const flashTicks = 3

// saveShot writes the frame as a PNG into the shots directory, under the
// first free number, and says where.
func (g *game) saveShot() {
	if g.frame == nil {
		return
	}
	for n := 1; ; n++ {
		path := filepath.Join(g.shots, fmt.Sprintf("shot-%04d.png", n))
		if _, err := os.Stat(path); err == nil {
			continue
		}
		f, err := os.Create(path)
		if err == nil {
			err = errors.Join(png.Encode(f, g.frame), f.Close())
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "screenshot:", err)
			return
		}
		fmt.Println("screenshot:", path)
		g.flash = flashTicks
		return
	}
}

// takePatch applies a patch that has arrived, if one has.
func (g *game) takePatch() {
	select {
	case p, ok := <-g.patches:
		if ok {
			p.ApplyAt(g.machine, func() {
				fmt.Printf("hot: applied the patch at %08x, %d bytes, at frame %d\n", p.Start, p.Size(), g.machine.Frames())
			})
		}
	default:
	}
}

// The fast start's limits: how long a tick may run frames for, which
// leaves the window its own share of the tick, and the frames it runs at
// most, half a minute of them, after which a program that shows nothing,
// one that only plays music say, runs in real time anyway.
const (
	fastTick      = 12 * time.Millisecond
	maxFastFrames = 30 * 60
)

// fastStart runs frames as fast as the host allows for a tick's worth of
// time, or until the display shows something, dropping their sound. It
// reports whether the fast start is still going.
func (g *game) fastStart() (bool, error) {
	deadline := time.Now().Add(fastTick)
	for !g.machine.Shown() && g.fastFrames < maxFastFrames {
		if time.Now().After(deadline) {
			return true, nil
		}
		if _, err := g.machine.RunFrame(frameBudget); err != nil {
			return false, err
		}
		g.samples = g.machine.Audio(g.samples[:0])
		g.fastFrames++
	}
	return false, nil
}

// keys is the keyboard's pad, laid out as the site's player has it.
var keys = map[ebiten.Key]uint16{
	ebiten.KeyArrowUp:    emu.ButtonUp,
	ebiten.KeyArrowDown:  emu.ButtonDown,
	ebiten.KeyArrowLeft:  emu.ButtonLeft,
	ebiten.KeyArrowRight: emu.ButtonRight,
	ebiten.KeyZ:          emu.ButtonCross,
	ebiten.KeyX:          emu.ButtonCircle,
	ebiten.KeyA:          emu.ButtonSquare,
	ebiten.KeyS:          emu.ButtonTriangle,
	ebiten.KeyQ:          emu.ButtonL1,
	ebiten.KeyW:          emu.ButtonR1,
	ebiten.KeyDigit1:     emu.ButtonL2,
	ebiten.KeyDigit2:     emu.ButtonR2,
	ebiten.KeyEnter:      emu.ButtonStart,
	ebiten.KeyBackspace:  emu.ButtonSelect,
}

// gamepadButtons is the standard gamepad layout's buttons as the pad's.
var gamepadButtons = map[ebiten.StandardGamepadButton]uint16{
	ebiten.StandardGamepadButtonRightBottom:      emu.ButtonCross,
	ebiten.StandardGamepadButtonRightRight:       emu.ButtonCircle,
	ebiten.StandardGamepadButtonRightLeft:        emu.ButtonSquare,
	ebiten.StandardGamepadButtonRightTop:         emu.ButtonTriangle,
	ebiten.StandardGamepadButtonFrontTopLeft:     emu.ButtonL1,
	ebiten.StandardGamepadButtonFrontTopRight:    emu.ButtonR1,
	ebiten.StandardGamepadButtonFrontBottomLeft:  emu.ButtonL2,
	ebiten.StandardGamepadButtonFrontBottomRight: emu.ButtonR2,
	ebiten.StandardGamepadButtonCenterLeft:       emu.ButtonSelect,
	ebiten.StandardGamepadButtonCenterRight:      emu.ButtonStart,
	ebiten.StandardGamepadButtonLeftStick:        emu.ButtonL3,
	ebiten.StandardGamepadButtonRightStick:       emu.ButtonR3,
	ebiten.StandardGamepadButtonLeftTop:          emu.ButtonUp,
	ebiten.StandardGamepadButtonLeftBottom:       emu.ButtonDown,
	ebiten.StandardGamepadButtonLeftLeft:         emu.ButtonLeft,
	ebiten.StandardGamepadButtonLeftRight:        emu.ButtonRight,
}

// buttons returns what is held on the keyboard and every gamepad, as the
// pad's bits. A window without focus holds nothing, so that nothing stays
// pressed when the keys are let go elsewhere.
func buttons() uint16 {
	if !ebiten.IsFocused() {
		return 0
	}
	var b uint16
	for key, bit := range keys {
		if ebiten.IsKeyPressed(key) {
			b |= bit
		}
	}
	for _, id := range ebiten.AppendGamepadIDs(nil) {
		if !ebiten.IsStandardGamepadLayoutAvailable(id) {
			continue
		}
		for button, bit := range gamepadButtons {
			if ebiten.IsStandardGamepadButtonPressed(id, button) {
				b |= bit
			}
		}
		// The left stick doubles as the d-pad, past half way.
		x := ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickHorizontal)
		y := ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickVertical)
		switch {
		case x < -0.5:
			b |= emu.ButtonLeft
		case x > 0.5:
			b |= emu.ButtonRight
		}
		switch {
		case y < -0.5:
			b |= emu.ButtonUp
		case y > 0.5:
			b |= emu.ButtonDown
		}
	}
	return b
}

func (g *game) Update() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyF11) {
		ebiten.SetFullscreen(!ebiten.IsFullscreen())
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) && ebiten.IsFullscreen() {
		ebiten.SetFullscreen(false)
	}

	pad := g.machine.Pad(1)
	pad.Release(0xffff)
	pad.Press(buttons())
	if g.patches != nil {
		g.takePatch()
	}

	if g.fast {
		fast, err := g.fastStart()
		if err != nil {
			return fmt.Errorf("the emulator stopped: %w", err)
		}
		g.fast = fast
		if fast {
			return nil
		}
		// Real time from here: the sound's pacing starts afresh.
		g.level, g.settle = 0, 0
	}

	// A tick runs to the next vertical blank. Without one in the budget,
	// the display is whatever it was, and the next tick tries again.
	for range g.frames() {
		if _, err := g.machine.RunFrame(frameBudget); err != nil {
			return fmt.Errorf("the emulator stopped: %w", err)
		}
		if g.sound != nil {
			g.samples = g.machine.Audio(g.samples[:0])
			g.sound.write(g.samples)
		}
	}
	g.frame = g.machine.DisplayInto(g.frame)
	if inpututil.IsKeyJustPressed(ebiten.KeyF12) || g.shot.Swap(false) {
		g.saveShot()
	}
	return nil
}

// frames is how many frames a tick runs. The ticks come at 60 a second
// and the machine's frames at a little under, so with sound, the sound
// paces it: a frame is left out while too much is buffered, and an extra
// one run while too little is, which keeps the buffer near soundStart
// rather than filling until it drops what is in it. Without sound, every
// tick runs a frame.
//
// The player takes its buffer's worth at once, so the level is a sawtooth;
// it is averaged over half a second or so, and after a frame is left out
// or added, the average is given that long to follow before another.
func (g *game) frames() int {
	if g.sound == nil {
		return 1
	}
	n := float64(g.sound.buffered())
	if g.level == 0 {
		g.level = n
	}
	g.level += (n - g.level) / 32
	if g.settle > 0 {
		g.settle--
		return 1
	}
	const slack = 2 * soundFrame
	switch {
	case g.level > soundStart+slack:
		g.settle = 32
		return 0
	case g.level < soundStart-slack:
		g.settle = 32
		return 2
	}
	return 1
}

func (g *game) Draw(screen *ebiten.Image) {
	// Nothing is shown during the fast start, before the first frame.
	if g.frame == nil {
		return
	}
	w, h := g.frame.Rect.Dx(), g.frame.Rect.Dy()
	if w == 0 || h == 0 {
		return
	}
	if g.image == nil || g.image.Bounds().Dx() != w || g.image.Bounds().Dy() != h {
		g.image = ebiten.NewImage(w, h)
	}
	g.image.WritePixels(g.frame.Pix)

	// The display fills a 4:3 picture, as a television would show it,
	// whatever the display mode's resolution, as large as fits.
	sw, sh := float64(screen.Bounds().Dx()), float64(screen.Bounds().Dy())
	pw, ph := sw, sw*3/4
	if ph > sh {
		pw, ph = sh*4/3, sh
	}
	var op ebiten.DrawImageOptions
	op.GeoM.Scale(pw/float64(w), ph/float64(h))
	op.GeoM.Translate((sw-pw)/2, (sh-ph)/2)
	op.Filter = ebiten.FilterLinear
	screen.DrawImage(g.image, &op)
	if g.flash > 0 {
		// Drawn over itself added, the picture is twice as bright: the
		// flash after a screenshot.
		g.flash--
		op.Blend = ebiten.BlendLighter
		screen.DrawImage(g.image, &op)
	}
}

// Layout is never called, since LayoutF is there, but the Game interface
// asks for it.
func (g *game) Layout(w, h int) (int, int) { return w, h }

// LayoutF draws at the window's own resolution, so that the picture is as
// sharp as the screen allows.
func (g *game) LayoutF(w, h float64) (float64, float64) {
	s := ebiten.Monitor().DeviceScaleFactor()
	return w * s, h * s
}
