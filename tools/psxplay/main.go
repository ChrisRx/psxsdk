// Command psxplay runs the emulator in a window, with the keyboard or a
// gamepad as the pad in port 1 and the SPU's sound on the speakers. It is
// the desktop counterpart of the player on the site, which runs the same
// emulator in a browser.
package main

import (
	"errors"
	"fmt"
	"image"
	"io/fs"
	"log"
	"os"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/spf13/cobra"

	"go.chrisrx.dev/psx/binutils"
	"go.chrisrx.dev/psx/emu"
	"go.chrisrx.dev/psx/emu/openbios"
	"go.chrisrx.dev/psx/format/psx"
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

var opts struct {
	BIOS  string
	EXE   string
	Card  string
	Scale int
	Mute  bool
	Loads []string
}

func main() {
	cmd := &cobra.Command{
		Use:   "psxplay [flags] [disc.cue]",
		Short: "run the emulator in a window",
		Long: `psxplay boots OpenBIOS, or the BIOS given with --bios, in a window, with
the disc whose cue sheet is given in the drive, or with --exe loaded when
the BIOS reaches its shell. What the BIOS prints to the TTY goes to
standard output.

The keyboard is the pad in port 1, laid out as the site's player has it:

  arrows        d-pad          Enter      Start
  Z X A S       cross circle   Backspace  Select
                square triangle
  Q W           L1 R1          1 2        L2 R2

A gamepad with a standard layout works as itself. F11 toggles full screen
and Escape leaves it.`,
		Args:         cobra.MaximumNArgs(1),
		RunE:         run,
		SilenceUsage: true,
	}
	f := cmd.Flags()
	f.StringVar(&opts.BIOS, "bios", "", "BIOS image to boot instead of the OpenBIOS built in")
	f.StringVar(&opts.EXE, "exe", "", "PSX-EXE to load when the BIOS reaches its shell, as psxe -x does")
	f.StringArrayVar(&opts.Loads, "load", nil, "file@addr: copy a file into RAM at a hexadecimal address, with --exe, as siocons' local dload did")
	f.StringVar(&opts.Card, "card", "", "memory card image for slot 1, made blank if it does not exist, and written back on exit")
	f.IntVar(&opts.Scale, "scale", 2, "the window's initial size, as a multiple of 320x240")
	f.BoolVar(&opts.Mute, "mute", false, "leave the sound off")

	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func run(cmd *cobra.Command, args []string) error {
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

	title := "psxplay"
	if len(args) == 1 {
		d, err := emu.OpenDisc(args[0])
		if err != nil {
			return err
		}
		defer d.Close()
		m.InsertDisc(d)
		title += " - " + args[0]
	}

	var card *emu.MemoryCard
	if opts.Card != "" {
		data, err := os.ReadFile(opts.Card)
		if errors.Is(err, fs.ErrNotExist) {
			data, err = nil, nil
		}
		if err != nil {
			return err
		}
		if card, err = emu.NewMemoryCard(data); err != nil {
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
		for _, spec := range opts.Loads {
			l, err := binutils.ParseLoad(spec)
			if err != nil {
				return err
			}
			if err := m.LoadRAM(l.Addr, l.Data); err != nil {
				return err
			}
		}
		title += " - " + opts.EXE
	}

	g := &game{machine: m}
	if !opts.Mute {
		g.sound = newSoundBuffer()
		player, err := audio.NewContext(sampleRate).NewPlayer(g.sound)
		if err != nil {
			return err
		}
		player.SetBufferSize(20 * time.Millisecond)
		player.Play()
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

// game is the window: it runs the machine a frame a tick, reads the pad
// from the keyboard and gamepads, and shows the display.
type game struct {
	machine *emu.Machine
	sound   *soundBuffer
	frame   *image.RGBA
	image   *ebiten.Image
	samples []int16
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

	// A tick runs to the next vertical blank. Without one in the budget,
	// the display is whatever it was, and the next tick tries again.
	if _, err := g.machine.RunFrame(frameBudget); err != nil {
		return fmt.Errorf("the emulator stopped: %w", err)
	}
	g.frame = g.machine.DisplayInto(g.frame)

	if g.sound != nil {
		g.samples = g.machine.Audio(g.samples[:0])
		g.sound.write(g.samples)
	}
	return nil
}

func (g *game) Draw(screen *ebiten.Image) {
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
