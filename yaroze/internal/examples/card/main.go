// A memory card, its files listed, dumped and deleted, and a number saved to
// it with an icon and a title, and loaded back.
//
// examples/card on the yaroze package's Go API: the card calls return errors
// rather than codes, the files are yaroze.CardFile values, and the icon's
// palette and pixels for the save's header come from the TIM file's words
// rather than the library's record of the loaded image.
//
// # Controls
//
// Up and down choose what to do and Cross does it. Left and right change
// the number to save. In the list, up and down choose a file. Circle goes
// back to the menu.
package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/yaroze"
)

// saveName is the save's name: B, the region (A for America, as Sony's
// FAQ has it), -NETYAROZE in place of a product code, and the program's own.
const saveName = "BA-NETYAROZEPSXSDK"

// slot is the card used, slot 1.
const slot = 0

// The menu.
const (
	doSave = iota
	doLoad
	doList
	doDelete
	doDump
	doFormat
	doTest
	menuItems
)

var menuNames = [menuItems]string{"SAVE", "LOAD", "LIST", "DELETE", "DUMP", "FORMAT", "TEST"}

var (
	iconTIM = disc.Register("ASSETS/ICON.TIM", "../../../../examples/card/assets/ICON.TIM")

	screen = yaroze.Screen{
		Background: yaroze.Color{R: 60, G: 40, B: 90},
		Depth:      1,
	}
	pad yaroze.Pad

	icon yaroze.Sprite

	// number is what is saved and loaded, and status what the card said
	// last, both kept for a test to read.
	number int
	status = -1

	menu    int
	listing bool // in the file list rather than the menu
	chosen  int
	dir     [15]yaroze.CardFile
	nfiles  int

	// message is what the last action did.
	message string

	// The save's sectors: the header, the icon and the data.
	save   [3 * yaroze.CardSector]byte
	dump   [yaroze.CardSector]byte
	dumped bool

	text yaroze.Text
)

// The save's data sector: a mark that says it is this program's, and the
// number.
const mark = "PSXSDK CARD"

func main() {
	screen.Open()
	text = screen.Font(16, 48, 288, 184, 900)

	if f := disc.LoadAll(300); f != nil {
		message = "could not read " + f.Name
	} else {
		icon = yaroze.LoadTIM(iconTIM.Data()).Sprite()
		icon.X, icon.Y = 296, 24
		icon.ScaleX, icon.ScaleY = 2*yaroze.One, 2*yaroze.One
	}

	for frame := range screen.Frames() {
		pad.Update()
		if frame.Number() == 1 {
			status = int(yaroze.CardTest(slot))
		}
		if listing {
			list()
		} else {
			chooseMenu()
		}
		draw()
		icon.Draw(frame, 0)
	}
}

// chooseMenu moves through the menu and does what is chosen.
func chooseMenu() {
	switch {
	case pad.Pressed(yaroze.Up):
		menu = (menu + menuItems - 1) % menuItems
	case pad.Pressed(yaroze.Down):
		menu = (menu + 1) % menuItems
	case pad.Duration(yaroze.Left) == 1 || pad.Duration(yaroze.Left) > 20:
		number--
	case pad.Duration(yaroze.Right) == 1 || pad.Duration(yaroze.Right) > 20:
		number++
	case pad.Pressed(yaroze.Cross):
		do(menu)
	}
}

func do(what int) {
	switch what {
	case doSave:
		message = "could not save"
		if writeSave() {
			message = "saved"
		}
	case doLoad:
		message = "no save on the card"
		if readSave() {
			message = "loaded"
		}
	case doList, doDelete, doDump:
		nfiles = yaroze.CardFiles(slot, dir[:])
		listing, chosen, dumped = true, 0, false
		message = ""
	case doFormat:
		message = "could not format"
		if yaroze.CardFormat(slot) == nil {
			message = "formatted"
		}
		status = int(yaroze.CardTest(slot))
	case doTest:
		status = int(yaroze.CardTest(slot))
		message = "tested"
	}
}

// The icon's TIM: two header words, then the palette block and the
// pixel block, each a length word, a position word, a size word and its
// data. The palette is 16 colors, 8 words, and the pixels 16 by 16 of 4
// bits, 32 words.
const (
	timCLUT   = 5
	timPixels = timCLUT + 8 + 3
)

// writeSave writes the number to the card, with a header and the icon.
func writeSave() bool {
	tim := iconTIM.Data()
	h := yaroze.SaveHeader{Frames: 1, Blocks: 1}
	h.SetTitle("PSXSDK CARD EXAMPLE")
	copy(h.CLUT[:], unsafe.Slice((*uint16)(unsafe.Pointer(&tim[timCLUT])), 16))
	h.Put(save[:yaroze.CardSector])
	copy(save[yaroze.CardSector:], unsafe.Slice((*byte)(unsafe.Pointer(&tim[timPixels])), yaroze.CardSector))
	data := save[2*yaroze.CardSector:]
	clear(data)
	copy(data, mark)
	putInt(data[16:], number)
	return yaroze.CardWrite(slot, saveName, save[:]) == nil
}

// readSave reads the number back, if the save is there and is this
// program's.
func readSave() bool {
	if n, err := yaroze.CardRead(slot, saveName, save[:]); err != nil || n != len(save) {
		return false
	}
	data := save[2*yaroze.CardSector:]
	if !hasMark(data) {
		return false
	}
	number = getInt(data[16:])
	return true
}

func putInt(b []byte, v int) {
	b[0], b[1], b[2], b[3] = byte(v), byte(v>>8), byte(v>>16), byte(v>>24)
}

func getInt(b []byte) int {
	return int(int32(uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24))
}

// list chooses a file, and deletes or dumps it if that is what the menu
// asked for.
func list() {
	switch {
	case pad.Pressed(yaroze.Circle):
		listing = false
	case pad.Pressed(yaroze.Up) && chosen > 0:
		chosen--
		dumped = false
	case pad.Pressed(yaroze.Down) && chosen < nfiles-1:
		chosen++
		dumped = false
	case pad.Pressed(yaroze.Cross) && nfiles > 0:
		name := dir[chosen].Name()
		switch menu {
		case doDelete:
			message = "could not delete"
			if yaroze.CardDelete(slot, name) == nil {
				message = "deleted"
			}
			nfiles = yaroze.CardFiles(slot, dir[:])
			chosen = min(chosen, max(nfiles-1, 0))
		case doDump:
			n, err := yaroze.CardRead(slot, name, dump[:])
			dumped = err == nil && n == len(dump)
		}
	}
}

var cardStates = [5]string{"NO CARD", "CARD", "NEW CARD", "CARD ERROR", "UNFORMATTED"}

const hexDigits = "0123456789ABCDEF"

// hex is a byte of the dump as text, kept here so that it is not made anew.
var hex [3]byte

func draw() {
	text.Print("SLOT 1: ")
	if status >= 0 && status < len(cardStates) {
		text.Print(cardStates[status])
	}
	text.Print("\nNUMBER: ").Int(number).Print("\n\n")
	if !listing {
		for i := range menuItems {
			name := menuNames[i]
			if i == menu {
				text.Print("~c990> ")
			} else {
				text.Print("~c999  ")
			}
			text.Print(name).Print("\n")
		}
		text.Print("~c999\n").Print(message).Print("\n")
		return
	}
	text.Print(menuNames[menu]).Print(": ").Int(nfiles).Print(" FILE(S)\n")
	for i := range nfiles {
		if i == chosen {
			text.Print("~c990> ")
		} else {
			text.Print("~c999  ")
		}
		f := &dir[i]
		text.Print(f.Name()).Print(" ").Int(f.Size() / yaroze.CardBlock).Print("\n")
	}
	text.Print("~c999")
	if dumped {
		text.Print("\n")
		for row := range 6 {
			for col := range 8 {
				b := dump[row*8+col]
				hex[0], hex[1], hex[2] = hexDigits[b>>4], hexDigits[b&15], ' '
				text.Print(unsafe.String(&hex[0], 3))
			}
			text.Print("\n")
		}
	}
	text.Print("\n").Print(message).Print("\n")
}

// hasMark reports whether data starts with mark, compared a byte at a time:
// string(data[:len(mark)]) would copy the bytes into a new string, which
// with no collector running is never freed.
func hasMark(data []byte) bool {
	for i := range len(mark) {
		if data[i] != mark[i] {
			return false
		}
	}
	return true
}
