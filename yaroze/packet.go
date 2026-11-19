package yaroze

import (
	"unsafe"

	"go.chrisrx.dev/psx/gpu"
)

// GPU packets a program builds itself, in the frame's packet space
// beside the ones the library's drawing builds, and links into the frame
// at a depth as they are. The gpu package has the packet types;
// Frame.Packet takes room for one, sets its tag, has it set its command
// code, links it in, and returns it to fill in:
//
//	sky := frame.Packet[gpu.ShadedQuad](frame.Depths() - 1)
//	sky.Color0, sky.XY0 = top, gpu.At(-160, -120)
//	sky.Color1, sky.XY1 = top, gpu.At(160, -120)
//	sky.Color2, sky.XY2 = bottom, gpu.At(-160, 120)
//	sky.Color3, sky.XY3 = bottom, gpu.At(160, 120)

// Packet takes room for a T, one of the gpu package's packet types,
// from the frame's packet space, sets its tag to its length, has it set
// its command code, links it into the frame at depth, and returns it to
// fill in. T is all a call names, frame.Packet[gpu.Tile](depth); P
// follows from it. Nothing checks that the space has room: a frame that
// builds more than Screen.Packets holds writes past it.
func (f *Frame) Packet[T any, P gpu.Packet[T]](depth int) *T {
	p := newPacket[T, P]()
	linkPacket(f, depth, (*gpu.Tag)(unsafe.Pointer(p)))
	return p
}

// PacketAfter is Packet with the packet linked after the packet prev,
// of any type, rather than into the frame: it is drawn right after prev
// wherever prev was linked. A primitive goes after the drawing mode it
// needs this way; at one depth, Packet puts each packet in front of the
// last, so the one linked last is drawn first.
func (f *Frame) PacketAfter[T any, P gpu.Packet[T]](prev *gpu.Tag) *T {
	p := newPacket[T, P]()
	linkAfter(prev, (*gpu.Tag)(unsafe.Pointer(p)))
	return p
}

// newPacket takes the room for a T, sets the tag's length, and has the
// packet set its code. The tag is written first: the store tells the
// compiler the packet is there, and Init's then costs nothing more than
// its own.
func newPacket[T any, P gpu.Packet[T]]() *T {
	var zero T
	words := int(unsafe.Sizeof(zero) / 4)
	p := (*T)(unsafe.Pointer(allocPacket(words)))
	*(*gpu.Tag)(unsafe.Pointer(p)) = gpu.Tag(words-1) << 24
	P(p).Init()
	return p
}

// DrawMode returns the word a gpu.DrawMode's Page takes to draw from the
// texture's page with the blend, which the packets after it use when
// their Code has gpu.CodeTranslucent. A drawing mode always holds one of
// the four blends, so BlendNone, for packets that do not blend, gives
// the same word as BlendAverage.
func (t Texture) DrawMode(blend BlendMode) uint32 {
	return 0xe1<<24 | uint32(t.Page)&^0x60 | uint32(blend&3)<<5
}
