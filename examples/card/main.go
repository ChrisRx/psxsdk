// A memory card, its files listed, dumped and deleted, and a number saved to
// it with an icon and a title, and loaded back.
//
// A port of Sony's card sample (sample/graphics/card in the Net Yaroze
// samples), which lists the files on a card, creates one, deletes one,
// dumps one and tests whether a card is there, through the kernel's file
// functions. This one does those to the card in slot 1, and to them adds a
// save that the console's own card screen can show: the file has a name
// in Sony's form, BA-NETYAROZE and a name of the program's own, and begins
// with the header Sony's convention sets out (docs/manuals/savefile.md):
// "SC", the icon's frames and the file's blocks, its title in full-width
// Shift-JIS, and the icon's palette, then the icon, 16 by 16 pixels of 4
// bits, and then the program's data. The icon is a TIM made by png2tim from
// gen/icon.png, whose pixels and palette are already in the form the header
// wants, and the program shows it at the top of the screen.
//
// A card holds 15 blocks of 8 KB, and a file is a whole number of them; it
// is read and written in sectors of 128 bytes. A card is formatted once,
// which writes its directory. TestCard says whether a card is there and
// whether it is formatted; it talks to the card over the port the
// controller shares, so the program calls it when it starts and when asked,
// as the sample did, not every frame.
//
// Sony's checklist (docs/manuals/requirements.md) told Net Yaroze programs
// on magazine discs not to touch memory cards at all, since a save's name
// should carry the game's product code and a Net Yaroze program had none.
// The program here is for learning how cards work; the name it saves under
// is the one Sony's member FAQ suggested.
//
// Differences from the sample: it uses slot 1 rather than 2; its "create"
// is the save, which writes a proper header and a number rather than an
// empty file; it adds load and format; and it writes with yaroze.CardWrite
// and reads with CardRead and CardFiles, which keep the kernel's calls in
// one place.
//
// # Controls
//
// Up and down choose what to do and Cross does it. Left and right change
// the number to save. In the list, up and down choose a file. Circle goes
// back to the menu.
package main

//go:generate go run ./gen gen/icon.png
//go:generate go run go.chrisrx.dev/psx/tools/png2tim --at 960,0 --clut 0,481 -o assets/ICON.TIM gen/icon.png
//go:generate go run go.chrisrx.dev/psx/tools/discgen assets

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
	screen = yaroze.Screen{Background: yaroze.CVECTOR{R: 60, G: 40, B: 90}, Depth: 1}
	pad    yaroze.Pad

	icon      yaroze.GsSPRITE
	iconImage yaroze.GsIMAGE

	// number is what is saved and loaded, and status what the card said
	// last, both kept for a test to read.
	number int32
	status int32 = -1

	menu    int32
	listing bool // in the file list rather than the menu
	chosen  int32
	dir     [15]yaroze.DIRENTRY
	nfiles  int32

	// message is what the last action did.
	message string

	// The save's sectors: the header, the icon and the data.
	save   [3 * yaroze.CardSector]byte
	dump   [yaroze.CardSector]byte
	dumped bool

	text    yaroze.Buffer
	textBuf [1024]byte
)

// The save's data sector: a mark that says it is this program's, and the
// number.
const mark = "PSXSDK CARD"

func main() {
	screen.Open()
	screen.Font(16, 48, 288, 184, 900)
	text.Init(textBuf[:])

	if f := disc.LoadAll(300, files...); f != nil {
		message = "could not read " + f.Name
	} else {
		iconImage = yaroze.TIM(iconTIM.Data()).Load()
		icon = iconImage.Sprite()
		icon.X, icon.Y = 296, 24
		icon.Scalex, icon.Scaley = 2*yaroze.ONE, 2*yaroze.ONE
	}

	for ot := range screen.Frames() {
		pad.Update()
		if screen.Frame() == 1 {
			status = yaroze.TestCard(slot)
		}
		if listing {
			list()
		} else {
			chooseMenu()
		}
		draw()
		icon.Sort(ot, 0)
	}
}

// chooseMenu moves through the menu and does what is chosen.
func chooseMenu() {
	switch {
	case pad.Pressed(yaroze.PADLup):
		menu = (menu + menuItems - 1) % menuItems
	case pad.Pressed(yaroze.PADLdown):
		menu = (menu + 1) % menuItems
	case pad.Duration(yaroze.PADLleft) == 1 || pad.Duration(yaroze.PADLleft) > 20:
		number--
	case pad.Duration(yaroze.PADLright) == 1 || pad.Duration(yaroze.PADLright) > 20:
		number++
	case pad.Pressed(yaroze.PADRdown):
		do(menu)
	}
}

func do(what int32) {
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
		nfiles = int32(yaroze.CardFiles(slot, dir[:]))
		listing, chosen, dumped = true, 0, false
		message = ""
	case doFormat:
		var path [8]byte
		message = "could not format"
		if yaroze.Format(yaroze.CardPath(path[:], slot, "")) == 1 {
			message = "formatted"
		}
		status = yaroze.TestCard(slot)
	case doTest:
		status = yaroze.TestCard(slot)
		message = "tested"
	}
}

// writeSave writes the number to the card, with a header and the icon.
func writeSave() bool {
	h := yaroze.SaveHeader{Frames: 1, Blocks: 1}
	h.SetTitle("PSXSDK CARD EXAMPLE")
	clut := unsafe.Slice((*uint16)(unsafe.Pointer(iconImage.Clut)), 16)
	copy(h.CLUT[:], clut)
	h.Put(save[:yaroze.CardSector])
	pixels := unsafe.Slice((*byte)(unsafe.Pointer(iconImage.Pixel)), yaroze.CardSector)
	copy(save[yaroze.CardSector:], pixels)
	data := save[2*yaroze.CardSector:]
	clear(data)
	copy(data, mark)
	putInt(data[16:], number)
	return yaroze.CardWrite(slot, saveName, save[:])
}

// readSave reads the number back, if the save is there and is this
// program's.
func readSave() bool {
	if yaroze.CardRead(slot, saveName, save[:]) != len(save) {
		return false
	}
	data := save[2*yaroze.CardSector:]
	if string(data[:len(mark)]) != mark {
		return false
	}
	number = getInt(data[16:])
	return true
}

func putInt(b []byte, v int32) {
	b[0], b[1], b[2], b[3] = byte(v), byte(v>>8), byte(v>>16), byte(v>>24)
}

func getInt(b []byte) int32 {
	return int32(b[0]) | int32(b[1])<<8 | int32(b[2])<<16 | int32(b[3])<<24
}

// list chooses a file, and deletes or dumps it if that is what the menu
// asked for.
func list() {
	switch {
	case pad.Pressed(yaroze.PADRright):
		listing = false
	case pad.Pressed(yaroze.PADLup) && chosen > 0:
		chosen--
		dumped = false
	case pad.Pressed(yaroze.PADLdown) && chosen < nfiles-1:
		chosen++
		dumped = false
	case pad.Pressed(yaroze.PADRdown) && nfiles > 0:
		name := dir[chosen].FileName()
		switch menu {
		case doDelete:
			var path [32]byte
			message = "could not delete"
			if yaroze.Delete(yaroze.CardPath(path[:], slot, name)) == 1 {
				message = "deleted"
			}
			nfiles = int32(yaroze.CardFiles(slot, dir[:]))
			chosen = min(chosen, max(nfiles-1, 0))
		case doDump:
			dumped = yaroze.CardRead(slot, name, dump[:]) == len(dump)
		}
	}
}

var cardStates = [5]string{"NO CARD", "CARD", "NEW CARD", "CARD ERROR", "UNFORMATTED"}

const hexDigits = "0123456789ABCDEF"

// hex is a byte of the dump as text, kept here so that it is not made anew.
var hex [3]byte

func draw() {
	text.Reset()
	text.WriteString("SLOT 1: ")
	if status >= 0 && int(status) < len(cardStates) {
		text.WriteString(cardStates[status])
	}
	text.WriteString("\nNUMBER: ").WriteInt(number).WriteString("\n\n")
	if !listing {
		for i := range int32(menuItems) {
			name := menuNames[i]
			if i == menu {
				text.WriteString("~c990> ")
			} else {
				text.WriteString("~c999  ")
			}
			text.WriteString(name).WriteString("\n")
		}
		text.WriteString("~c999\n").WriteString(message).WriteString("\n")
		text.Print()
		return
	}
	text.WriteString(menuNames[menu]).WriteString(": ").WriteInt(nfiles).WriteString(" FILE(S)\n")
	for i := range nfiles {
		if i == chosen {
			text.WriteString("~c990> ")
		} else {
			text.WriteString("~c999  ")
		}
		f := &dir[i]
		text.WriteString(f.FileName()).WriteString(" ").WriteInt(f.Size / yaroze.CardBlock).WriteString("\n")
	}
	text.WriteString("~c999")
	if dumped {
		text.WriteString("\n")
		for row := range 6 {
			for col := range 8 {
				b := dump[row*8+col]
				hex[0], hex[1], hex[2] = hexDigits[b>>4], hexDigits[b&15], ' '
				text.WriteString(unsafe.String(&hex[0], 3))
			}
			text.WriteString("\n")
		}
	}
	text.WriteString("\n").WriteString(message).WriteString("\n")
	text.Print()
}
