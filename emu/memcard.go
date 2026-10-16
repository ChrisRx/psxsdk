package emu

import "fmt"

// MemoryCardSize is the size of a memory card: 1024 sectors of 128 bytes.
const MemoryCardSize = 128 << 10

// MemoryCard is a memory card, as psxe's mcd.c models one. psxe disables its
// cards, so a machine starts with none; InsertCard puts one in a slot.
//
// It departs from psxe where psxe's card would not work: psxe stores 127 of
// a write's 128 bytes, and answers every byte after them one early; its
// sector address is 16 bits wide, so sectors from 512 on wrap to the first
// half of the card; and it has no answer to the get ID command.
type MemoryCard struct {
	data [MemoryCardSize]byte

	flag     uint8
	msb, lsb uint8
	addr     uint32
	rx       uint8
	pending  int
	mode     uint8
	state    int
	tx       uint8
	ready    bool
	checksum uint8
}

// NewMemoryCard returns a card holding data, which is copied, or a blank
// card if data is nil.
func NewMemoryCard(data []byte) (*MemoryCard, error) {
	c := &MemoryCard{flag: 0x08}
	if data != nil {
		if len(data) != MemoryCardSize {
			return nil, fmt.Errorf("a memory card image is %d bytes, not %d", MemoryCardSize, len(data))
		}
		copy(c.data[:], data)
	}
	return c, nil
}

// Data returns what the card holds, as a program last wrote it. It is the
// card's own, not a copy.
func (c *MemoryCard) Data() []byte { return c.data[:] }

// Transfer states, as psxe numbers them: the bytes every command starts
// with, then those of a read, a write and a get ID in turn. A state is the
// byte the card sends next.
const (
	cardHiZ = iota
	cardFlag
	cardID1
	cardID2

	cardReadRxMSB
	cardReadRxLSB
	cardReadAck1
	cardReadAck2
	cardReadMSB
	cardReadLSB
	cardReadData
	cardReadChecksum
	cardReadEnd

	cardWriteRxMSB
	cardWriteRxLSB
	cardWriteData
	cardWriteChecksum
	cardWriteAck1
	cardWriteAck2
	cardWriteEnd

	cardIDAck1
	cardIDAck2
	cardIDData0
	cardIDData1
	cardIDData2
	cardIDData3
)

// read clocks out the next byte of the card's reply, after write has taken
// the byte the host sent with it.
func (c *MemoryCard) read() uint8 {
	switch c.state {
	case cardHiZ:
		c.tx = 0xff
	case cardFlag:
		c.tx = c.flag
		c.flag = 0
		if c.mode != 'R' && c.mode != 'W' && c.mode != 'S' {
			// psxe goes on answering 5Dh to a command it does not know;
			// a card stops answering after the flag.
			c.ready = false
			c.state = cardHiZ
			return c.tx
		}
	case cardID1:
		c.tx = 0x5a
	case cardID2:
		c.ready = true
		switch c.mode {
		case 'R':
			c.state = cardReadRxMSB
		case 'W':
			c.state = cardWriteRxMSB
		case 'S':
			c.state = cardIDAck1
		}
		c.tx = 0x5d
		return c.tx

	case cardReadRxMSB:
		c.tx = 0x00
	case cardReadRxLSB:
		c.tx = c.msb
	case cardReadAck1:
		c.tx = 0x5c
	case cardReadAck2:
		c.tx = 0x5d
	case cardReadMSB:
		c.tx = c.msb
		c.checksum = c.msb
	case cardReadLSB:
		c.tx = c.lsb
		c.checksum ^= c.lsb
		c.pending = 128
	case cardReadData:
		c.pending--
		v := c.data[c.addr]
		c.addr = (c.addr + 1) % MemoryCardSize
		c.checksum ^= v
		if c.pending != 0 {
			return v
		}
		c.tx = v
	case cardReadChecksum:
		c.tx = c.checksum
	case cardReadEnd:
		c.ready = false
		c.state = cardHiZ
		return 'G'

	case cardWriteRxMSB:
		c.tx = 0x00
	case cardWriteRxLSB:
		c.tx = c.msb
		c.pending = 128 // psxe's 127 drops the last byte
	case cardWriteData:
		c.pending--
		c.data[c.addr] = c.rx
		c.addr = (c.addr + 1) % MemoryCardSize
		if c.pending != 0 {
			return c.rx
		}
		c.tx = c.rx
	case cardWriteChecksum:
		c.tx = c.rx
	case cardWriteAck1:
		c.tx = 0x5c
	case cardWriteAck2:
		c.tx = 0x5d
	case cardWriteEnd:
		c.ready = false
		c.state = cardHiZ
		return 'G'

	// The get ID command, which psxe leaves out: two acknowledges, then
	// the card's size in sectors as psxe's card has it.
	case cardIDAck1:
		c.tx = 0x5c
	case cardIDAck2:
		c.tx = 0x5d
	case cardIDData0:
		c.tx = 0x04
	case cardIDData1:
		c.tx = 0x00
	case cardIDData2:
		c.tx = 0x00
	case cardIDData3:
		c.ready = false
		c.state = cardHiZ
		return 0x80
	}
	c.ready = true
	c.state++
	return c.tx
}

// write takes a byte the host sends.
func (c *MemoryCard) write(v uint8) {
	switch c.state {
	case cardFlag:
		c.mode = v
	case cardReadRxMSB, cardWriteRxMSB:
		c.msb = v
	case cardReadRxLSB, cardWriteRxLSB:
		c.lsb = v
		// psxe keeps the address in 16 bits.
		c.addr = (uint32(c.msb)<<8 | uint32(c.lsb)) << 7 % MemoryCardSize
	case cardWriteData:
		c.rx = v
	}
}

// reset ends a transfer, as deselecting the card does.
func (c *MemoryCard) reset() { c.state = cardHiZ }
