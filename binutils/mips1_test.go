package binutils

import (
	"debug/elf"
	"encoding/binary"
	"testing"

	"go.chrisrx.dev/x/assert"
)

// mipsObject returns a little-endian MIPS ELF with one executable section,
// .text, holding words, and one that is not, .rodata, holding them again.
func mipsObject(words ...uint32) []byte {
	var text []byte
	for _, w := range words {
		text = binary.LittleEndian.AppendUint32(text, w)
	}
	shstrtab := []byte("\x00.text\x00.rodata\x00.shstrtab\x00")
	const ehsize, shentsize = 52, 40
	textOff := ehsize
	rodataOff := textOff + len(text)
	strOff := rodataOff + len(text)
	shOff := strOff + len(shstrtab)

	le := binary.LittleEndian
	var b []byte
	b = append(b, 0x7f, 'E', 'L', 'F', byte(elf.ELFCLASS32), byte(elf.ELFDATA2LSB), byte(elf.EV_CURRENT))
	b = append(b, make([]byte, 9)...)
	b = le.AppendUint16(b, uint16(elf.ET_REL))
	b = le.AppendUint16(b, uint16(elf.EM_MIPS))
	b = le.AppendUint32(b, uint32(elf.EV_CURRENT))
	b = le.AppendUint32(b, 0)             // entry
	b = le.AppendUint32(b, 0)             // phoff
	b = le.AppendUint32(b, uint32(shOff)) // shoff
	b = le.AppendUint32(b, 0)             // flags
	b = le.AppendUint16(b, ehsize)
	b = le.AppendUint16(b, 0) // phentsize
	b = le.AppendUint16(b, 0) // phnum
	b = le.AppendUint16(b, shentsize)
	b = le.AppendUint16(b, 4) // shnum
	b = le.AppendUint16(b, 3) // shstrndx
	b = append(b, text...)
	b = append(b, text...)
	b = append(b, shstrtab...)
	section := func(name uint32, typ elf.SectionType, flags elf.SectionFlag, off, size int) {
		for _, v := range []uint32{name, uint32(typ), uint32(flags), 0, uint32(off), uint32(size), 0, 0, 4, 0} {
			b = le.AppendUint32(b, v)
		}
	}
	section(0, elf.SHT_NULL, 0, 0, 0)
	section(1, elf.SHT_PROGBITS, elf.SHF_ALLOC|elf.SHF_EXECINSTR, textOff, len(text))
	section(7, elf.SHT_PROGBITS, elf.SHF_ALLOC, rodataOff, len(text))
	section(15, elf.SHT_STRTAB, 0, strOff, len(shstrtab))
	return b
}

func TestMIPS1Traps(t *testing.T) {
	const (
		div  = 0x0082001a // div  $zero, $a0, $v0
		teq  = 0x004001f4 // teq  $v0, $zero, 7
		mfhi = 0x00001010 // mfhi $v0
		tne  = 0x00a001f6 // tne  $a1, $zero, 7
	)
	data := mipsObject(div, teq, mfhi, tne)
	traps, err := MIPS1Traps(data)
	traps = assert.Ok(t, traps, err)
	assert.Equal(t, []Trap{
		{Section: ".text", Offset: 4, Pos: 52 + 4, Name: "teq", Word: teq},
		{Section: ".text", Offset: 12, Pos: 52 + 12, Name: "tne", Word: tne},
	}, traps)

	RemoveTraps(data, traps)
	traps, err = MIPS1Traps(data)
	assert.Equal(t, 0, len(assert.Ok(t, traps, err)))
	text := data[52 : 52+16]
	assert.Equal(t, []uint32{div, 0, mfhi, 0}, []uint32{
		binary.LittleEndian.Uint32(text[0:]), binary.LittleEndian.Uint32(text[4:]),
		binary.LittleEndian.Uint32(text[8:]), binary.LittleEndian.Uint32(text[12:]),
	})
	// The same words outside executable code are data, and stay.
	rodata := data[52+16 : 52+32]
	assert.Equal(t, uint32(teq), binary.LittleEndian.Uint32(rodata[4:]))

	_, err = MIPS1Traps([]byte("not an object, but long enough to have a header"))
	assert.Error(t, "bad magic", err)
}
