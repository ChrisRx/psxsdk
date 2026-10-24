package yaroze

import "unsafe"

// GPU packets a program builds itself, in the frame's packet space next to
// the ones the library's sort functions build, and links into an ordering
// table as they do. A packet is words: a tag the GPU reads the packet's
// length and the next packet's address from, and then the command and its
// arguments as the GPU takes them, laid out as the SDK's libgpu.h has them.
//
//	p := yaroze.AllocPacket(4)
//	p[0] = 3 << 24          // three words follow the tag
//	p[1] = 0x60<<24 | color // a filled rectangle
//	p[2] = uint32(uint16(x)) | uint32(uint16(y))<<16
//	p[3] = uint32(w) | uint32(h)<<16
//	ot.Link(0, p)

// AllocPacket takes n words of the frame's packet space, from where
// PacketPointer stands, and returns them. Nothing checks that the space has
// room: a frame that builds more than Screen.Packets holds writes past it.
func AllocPacket(n int) []uint32 {
	p := (*uint32)(unsafe.Pointer(PacketPointer()))
	SetPacketPointer((*PACKET)(unsafe.Add(unsafe.Pointer(p), 4*n)))
	return unsafe.Slice(p, n)
}

// Link puts the packet, whose first word is its tag, at the head of the
// table's entry for depth, keeping the tag's length and the entry's. Depth
// 0 is drawn last, in front of everything, and 1<<Length-1 first.
func (ot *GsOT) Link(depth int, packet []uint32) {
	tag := (*uint32)(unsafe.Add(unsafe.Pointer(ot.Org), 4*depth))
	packet[0] = packet[0]&0xff000000 | *tag&0xffffff
	*tag = *tag&0xff000000 | uint32(uintptr(unsafe.Pointer(&packet[0])))&0xffffff
}

// LinkTag is Link on a tag held somewhere other than a table: the tag of a
// packet already linked, to put the packet after it, or the word a program
// keeps as a list of its own.
func LinkTag(tag *uint32, packet []uint32) {
	packet[0] = packet[0]&0xff000000 | *tag&0xffffff
	*tag = *tag&0xff000000 | uint32(uintptr(unsafe.Pointer(&packet[0])))&0xffffff
}

// Entry returns the word at depth in the table, for LinkTag.
func (ot *GsOT) Entry(depth int) *uint32 {
	return (*uint32)(unsafe.Add(unsafe.Pointer(ot.Org), 4*depth))
}
