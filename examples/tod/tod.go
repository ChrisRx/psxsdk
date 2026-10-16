package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

// The TOD player: Sony's objctl.c and todanim.c from the todview sample.
// A TOD file is a header word, a frame count and a resolution, then frames,
// each a header word (its packet count in the top half), its frame number,
// and its packets. A packet's header word holds the object's ID, the
// packet's type and flags, and its length in words.

// Packet types.
const (
	todAttr   = 0
	todCoord  = 1
	todTMDID  = 2
	todParent = 3
	todLight  = 6
	todObjCtl = 8
)

// todUndef marks a free entry in the object table.
const todUndef = 0xffffffff

// objTable is the TodOBJTABLE: objects with their coordinate systems and
// the rotation, scale and translation each was last given.
type objTable struct {
	top    *yaroze.GsDOBJ2
	nobj   int
	maxobj int
}

func (t *objTable) obj(i int) *yaroze.GsDOBJ2 {
	return (*yaroze.GsDOBJ2)(unsafe.Add(unsafe.Pointer(t.top), uintptr(i)*unsafe.Sizeof(yaroze.GsDOBJ2{})))
}

// initObjTable is TodInitObjTable.
func (t *objTable) init(objs []yaroze.GsDOBJ2, coords []yaroze.GsCOORDINATE2, params []yaroze.GsCOORD2PARAM) {
	t.nobj = 0
	t.maxobj = len(objs)
	t.top = &objs[0]
	for i := range objs {
		objs[i].Attribute = 0x80000000
		objs[i].Id = todUndef
		objs[i].Coord2 = &coords[i]
		coords[i].Param = &params[i]
		objs[i].Tmd = nil
	}
}

// search is TodSearchObjByID.
func (t *objTable) search(id uint32) *yaroze.GsDOBJ2 {
	for i := range t.nobj {
		if o := t.obj(i); o.Id == id {
			return o
		}
	}
	return nil
}

// create is TodCreateNewObj: the first free entry, or a new one at the end.
func (t *objTable) create(id uint32) *yaroze.GsDOBJ2 {
	i := 0
	for i < t.nobj && t.obj(i).Id != todUndef {
		i++
	}
	if i == t.nobj {
		if i == t.maxobj {
			return nil
		}
		t.nobj++
	}
	o := t.obj(i)
	o.Id = id
	o.Attribute = 0
	yaroze.GsInitCoordinate2(nil, o.Coord2)
	o.Tmd = nil
	return o
}

// remove is TodRemoveObj, which, as Sony wrote it, removes the first free
// entry rather than the one with the ID it is given. The TOD here never
// removes an object.
func (t *objTable) remove(id uint32) *yaroze.GsDOBJ2 {
	i := 0
	for i < t.nobj && t.obj(i).Id != todUndef {
		i++
	}
	if i == t.nobj {
		return nil
	}
	o := t.obj(i)
	o.Id = todUndef
	if i == t.nobj-1 {
		for i >= 0 && t.obj(i).Id == todUndef {
			t.nobj--
			i--
		}
	}
	return o
}

// searchTMD is TodSearchTMDByID: the address of the object in a mapped TMD
// whose position in idList holds id, or 0.
func searchTMD(tmd *uint32, idList []uint32, id uint32) uint32 {
	p := word(tmd, 1)
	n := *p
	p = word(p, 1)
	for i := 0; uint32(i) < n && i < len(idList); i++ {
		if idList[i] == id {
			return uint32(uintptr(unsafe.Pointer(p)))
		}
		p = word(p, 7)
	}
	return 0
}

// word returns the word n words on from p.
func word(p *uint32, n int) *uint32 {
	return (*uint32)(unsafe.Add(unsafe.Pointer(p), 4*n))
}

func long(p *uint32, n int) int32  { return int32(*word(p, n)) }
func short(p *uint32, n int) int16 { return *(*int16)(unsafe.Add(unsafe.Pointer(p), 2*n)) }
func byteOf(p *uint32, n int) uint8 {
	return *(*uint8)(unsafe.Add(unsafe.Pointer(p), n))
}

// setFrame is TodSetFrame: if the frame at tod is due by now, it plays its
// packets and returns the next frame; otherwise it returns tod.
func setFrame(now int, tod *uint32, t *objTable, idList []uint32, tmd *uint32) *uint32 {
	n := int(*tod >> 16)
	if int(*word(tod, 1)) > now {
		return tod
	}
	tod = word(tod, 2)
	for range n {
		tod = setPacket(tod, t, idList, tmd)
	}
	return tod
}

// The dummy object takes the packets for an object that is not in the
// table. Sony's leaves its coordinate system an uninitialised pointer.
var (
	dummyObj    yaroze.GsDOBJ2
	dummyCoord  yaroze.GsCOORDINATE2
	dummyParams yaroze.GsCOORD2PARAM
)

// setPacket is TodSetPacket: it plays one packet and returns the next.
func setPacket(packet *uint32, t *objTable, idList []uint32, tmd *uint32) *uint32 {
	hdr := *packet
	id := hdr & 0xffff
	typ := hdr >> 16 & 0xf
	flag := hdr >> 20 & 0xf
	length := int(hdr >> 24)
	d := word(packet, 1)

	o := t.search(id)
	if o == nil {
		o = &dummyObj
		dummyObj.Coord2 = &dummyCoord
		dummyCoord.Param = &dummyParams
	} else {
		o.Coord2.Flg = 0
	}
	coord := &o.Coord2.Coord
	p := o.Coord2.Param

	switch typ {
	case todAttr:
		o.Attribute = o.Attribute&*d | *word(d, 1)

	case todCoord:
		if flag&1 != 0 {
			// Differences from the frame before.
			if flag&2 != 0 {
				p.Rotate.Vx += int16(long(d, 0) / 360)
				p.Rotate.Vy += int16(long(d, 1) / 360)
				p.Rotate.Vz += int16(long(d, 2) / 360)
				d = word(d, 3)
			}
			if flag&4 != 0 {
				p.Scale.Vx = p.Scale.Vx * int32(short(d, 0)) / 4096
				p.Scale.Vy = p.Scale.Vy * int32(short(d, 1)) / 4096
				p.Scale.Vz = p.Scale.Vz * int32(short(d, 2)) / 4096
				d = word(d, 2)
			}
			if flag&8 != 0 {
				p.Trans.Vx += long(d, 0)
				p.Trans.Vy += long(d, 1)
				p.Trans.Vz += long(d, 2)
			}
			yaroze.RotMatrix(&p.Rotate, coord)
			yaroze.ScaleMatrix(coord, &p.Scale)
			yaroze.TransMatrix(coord, &p.Trans)
		} else {
			// Values outright.
			if flag&2 != 0 {
				p.Rotate.Vx = int16(long(d, 0) / 360)
				p.Rotate.Vy = int16(long(d, 1) / 360)
				p.Rotate.Vz = int16(long(d, 2) / 360)
				d = word(d, 3)
				yaroze.RotMatrix(&p.Rotate, coord)
			}
			if flag&4 != 0 {
				p.Scale.Vx = int32(short(d, 0))
				p.Scale.Vy = int32(short(d, 1))
				p.Scale.Vz = int32(short(d, 2))
				d = word(d, 2)
				if flag&2 == 0 {
					yaroze.RotMatrix(&p.Rotate, coord)
				}
				yaroze.ScaleMatrix(coord, &p.Scale)
			}
			if flag&8 != 0 {
				p.Trans.Vx = long(d, 0)
				p.Trans.Vy = long(d, 1)
				p.Trans.Vz = long(d, 2)
				yaroze.TransMatrix(coord, &p.Trans)
			}
		}

	case todTMDID:
		if tmd != nil {
			yaroze.GsLinkObject4(searchTMD(tmd, idList, *d&0xffff), o, 0)
		}

	case todParent:
		if *d == 0 || *d == 0xffff {
			o.Coord2.Super = nil
		} else if parent := t.search(*d); parent != nil {
			o.Coord2.Super = parent.Coord2
		}

	case todObjCtl:
		switch flag {
		case 0:
			t.create(id)
		case 1:
			t.remove(id)
		}

	case todLight:
		l := &lights[id%3]
		if flag&2 != 0 {
			if flag&1 != 0 {
				l.Vx += long(d, 0)
				l.Vy += long(d, 1)
				l.Vz += long(d, 2)
			} else {
				l.Vx, l.Vy, l.Vz = long(d, 0), long(d, 1), long(d, 2)
			}
			d = word(d, 3)
		}
		if flag&4 != 0 {
			if flag&1 != 0 {
				l.R += byteOf(d, 0)
				l.G += byteOf(d, 1)
				l.B += byteOf(d, 2)
			} else {
				l.R, l.G, l.B = byteOf(d, 0), byteOf(d, 1), byteOf(d, 2)
			}
		}
		yaroze.GsSetFlatLight(int32(id%3), l)
	}
	return word(packet, length)
}
