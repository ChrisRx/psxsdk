package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

// The game, function for function as the demo has it. Its integer types are
// kept, C's long and int as int32 and short as int16, so that values wrap
// and truncate where they did.

// Subdivision levels for GsSortObject4, in an object's attribute.
const (
	gsDIV1 = 1 << 9 // 2x2
	gsDIV2 = 2 << 9 // 4x4
	gsDIV3 = 3 << 9 // the mask of both
)

func b2i(b bool) int16 {
	if b {
		return 1
	}
	return 0
}

// collisionX reports whether moving by increment would take you too near a
// wall to the east or west.
func collisionX(position, increment yaroze.VECTOR) bool {
	r := int32(int16(position.Vz >> 10))
	c := int32(int16(position.Vx >> 10))
	nc := int32(int16(position.Vx + increment.Vx))

	if nc < border {
		return true
	}
	if mapAt(r, c)&east != 0 && (nc+border)>>10 > c {
		return true
	}
	if mapAt(r, c)&west != 0 && (nc-border)>>10 < c {
		return true
	}
	if position.Vz&1023 <= border { // near the square to the south
		if mapAt(r-1, c)&east != 0 && (nc+border)>>10 > c {
			return true
		}
		if mapAt(r-1, c)&west != 0 && (nc-border)>>10 < c {
			return true
		}
	}
	if position.Vz&1023 >= 1024-border { // near the square to the north
		if mapAt(r+1, c)&east != 0 && (nc+border)>>10 > c {
			return true
		}
		if mapAt(r+1, c)&west != 0 && (nc-border)>>10 < c {
			return true
		}
	}
	return false
}

// collisionZ is collisionX for walls to the north or south.
func collisionZ(position, increment yaroze.VECTOR) bool {
	r := int32(int16(position.Vz >> 10))
	c := int32(int16(position.Vx >> 10))
	nr := int32(int16(position.Vz + increment.Vz))

	if nr < border {
		return true
	}
	if mapAt(r, c)&north != 0 && (nr+border)>>10 > r {
		return true
	}
	if mapAt(r, c)&south != 0 && (nr-border)>>10 < r {
		return true
	}
	if position.Vx&1023 <= border {
		if mapAt(r, c-1)&north != 0 && (nr+border)>>10 > r {
			return true
		}
		if mapAt(r, c-1)&south != 0 && (nr-border)>>10 < r {
			return true
		}
	}
	if position.Vx&1023 >= 1024-border {
		if mapAt(r, c+1)&north != 0 && (nr+border)>>10 > r {
			return true
		}
		if mapAt(r, c+1)&south != 0 && (nr-border)>>10 < r {
			return true
		}
	}
	return false
}

// clearFake removes a fake block, and the walls it gave its neighbors.
func clearFake(r, c int32) {
	world[r][c] &^= fake
	if r-1 >= 0 {
		world[r-1][c] &^= north
	}
	if r+1 < maxMapRows {
		world[r+1][c] &^= south
	}
	if c-1 >= 0 {
		world[r][c-1] &^= east
	}
	if c+1 < maxMapCols {
		world[r][c+1] &^= west
	}
}

// fakeWalls removes a fake block a shot is going into.
func fakeWalls(b *projectile) {
	r := int32(int16(b.position.Vz >> 10))
	c := int32(int16(b.position.Vx >> 10))
	nr := int32(int16((b.position.Vz + b.increment.Vz) >> 10))
	nc := int32(int16((b.position.Vx + b.increment.Vx) >> 10))

	if nr > r && nr < maxMapRows && mapAt(nr, c)&fake != 0 {
		clearFake(nr, c)
	}
	if nr < r && nr >= 0 && mapAt(nr, c)&fake != 0 {
		clearFake(nr, c)
	}
	if nc > c && nc < maxMapCols && mapAt(r, nc)&fake != 0 {
		clearFake(r, nc)
	}
	if nc < c && nc >= 0 && mapAt(r, nc)&fake != 0 {
		clearFake(r, nc)
	}
}

// bulletCollision reports whether a shot's next step goes into a wall or
// through a corner.
func bulletCollision(b *projectile) bool {
	r := int32(int16(b.position.Vz >> 10))
	c := int32(int16(b.position.Vx >> 10))
	nr := int32(int16((b.position.Vz + b.increment.Vz) >> 10))
	nc := int32(int16((b.position.Vx + b.increment.Vx) >> 10))
	m := mapAt(r, c)

	switch {
	case nr > r && m&north != 0, nr < r && m&south != 0,
		nc > c && m&east != 0, nc < c && m&west != 0:
		return true
	case nr > r && nc > c && mapAt(nr, c)&east != 0 && mapAt(r, nc)&north != 0,
		nr > r && nc < c && mapAt(nr, c)&west != 0 && mapAt(r, nc)&north != 0,
		nr < r && nc > c && mapAt(nr, c)&east != 0 && mapAt(r, nc)&south != 0,
		nr < r && nc < c && mapAt(nr, c)&west != 0 && mapAt(r, nc)&south != 0:
		return true
	}
	return false
}

// checkGoodGuyBullets moves your shots, and what they hit.
func checkGoodGuyBullets() {
	for i := range goodguy.bullet {
		b := &goodguy.bullet[i]
		if b.active != 0 {
			if bulletCollision(b) {
				fakeWalls(b)
				b.active = 0
				b.hit = 1
				b.vsync = yaroze.VSync(-1)
				b.object = &ricochetOBJ
			}

			for j := range int(numBadGuys) {
				bg := &badguy[j]
				if bg.active == 0 {
					continue
				}
				hitsize := int32(bg.hitsize >> 1)
				dx := b.position.Vx - bg.position.Vx
				dz := b.position.Vz - bg.position.Vz
				if abs(dx) < hitsize && abs(dz) < hitsize {
					yaroze.SsUtKeyOn(vab, 8, 0, 60, 0, 127, 127) // squish
					b.active = 0
					b.vsync = yaroze.VSync(-1)
					b.hit = 1
					b.object = &bloodOBJ

					if bg.health--; bg.health <= 0 {
						yaroze.SsUtKeyOn(vab, 6, 0, 60, 0, 127, 127) // scream
						bg.active = 0
						bg.object = &deadOBJ
						goodguy.kills++
					}
				}
			}

			if b.active != 0 {
				b.position.Vx += b.increment.Vx
				b.position.Vz += b.increment.Vz
			}
		}

		// A shot not ready to fire again is still in the air, or a hit
		// is still showing.
		if b.available == 0 {
			if inLineOfSight(b.position.Vx, b.position.Vz, goodguy.position.Vx, goodguy.position.Vz, int16(viewAngle)) {
				drawObject(b.object, int16(-viewAngle), b.position, &ot2[buffIdx])
			}
			if b.hit != 0 && yaroze.VSync(-1)-b.vsync > 7 {
				b.available = 1
				b.hit = 0
				b.object = &bulletOBJ
			}
		}
	}
}

// checkBadGuys has each monster shoot if it can see you, move toward you,
// wake up if you come near, and its shot fly.
func checkBadGuys() {
	for i := range int(numBadGuys) {
		bg := &badguy[i]
		if bg.active != 0 {
			if bg.bullet.available != 0 {
				dx := goodguy.position.Vx - bg.position.Vx
				dz := goodguy.position.Vz - bg.position.Vz
				if inLineOfSight(goodguy.position.Vx, goodguy.position.Vz, bg.position.Vx, bg.position.Vz, int16(rinvtan(dz, -dx))) {
					bg.bullet.active = 1
					bg.bullet.available = 0
					bg.bullet.vsync = yaroze.VSync(-1)
					bg.bullet.position.Vx = bg.position.Vx
					bg.bullet.position.Vz = bg.position.Vz
					bg.bullet.increment.Vx = rsin(rinvtan(dz, dx)) >> 4
					bg.bullet.increment.Vz = rcos(rinvtan(dz, dx)) >> 4
					yaroze.SsUtKeyOn(vab, 5, 0, 60, 0, 100, 100)
				}
			}
			moveBadGuy(bg, goodguy.position)
		} else if bg.health > 0 {
			if abs(bg.position.Vx-goodguy.position.Vx) < bg.activeDist &&
				abs(bg.position.Vz-goodguy.position.Vz) < bg.activeDist {
				bg.active = 1
			}
		}

		if inLineOfSight(bg.position.Vx, bg.position.Vz, goodguy.position.Vx, goodguy.position.Vz, int16(viewAngle)) {
			dx := goodguy.position.Vx - bg.position.Vx
			dz := goodguy.position.Vz - bg.position.Vz
			drawObject(bg.object, int16(2048-rinvtan(dz, -dx)), bg.position, &ot2[buffIdx])
		}

		if bg.bullet.active != 0 {
			if bulletCollision(&bg.bullet) {
				bg.bullet.active = 0
			}
			bg.bullet.position.Vx += bg.bullet.increment.Vx
			bg.bullet.position.Vz += bg.bullet.increment.Vz

			if abs(bg.bullet.position.Vx-goodguy.position.Vx) < 125 && abs(bg.bullet.position.Vz-goodguy.position.Vz) < 125 {
				yaroze.SsUtKeyOn(vab, 2, 0, 60, 0, 127, 127) // been shot
				bg.bullet.active = 0
				goodguy.health--
			} else if inLineOfSight(bg.bullet.position.Vx, bg.bullet.position.Vz, goodguy.position.Vx, goodguy.position.Vz, int16(viewAngle)) {
				drawObject(bg.bullet.object, int16(-viewAngle), bg.bullet.position, &ot2[buffIdx])
			}
		} else if bg.bullet.available == 0 && yaroze.VSync(-1)-bg.bullet.vsync > bg.pvsync {
			bg.bullet.available = 1
		}
	}
}

// moveBadGuy moves a monster toward p. It only changes direction in the
// middle of a square, preferring to turn toward p, then to keep going.
func moveBadGuy(b *badGuy, p yaroze.VECTOR) {
	if b.position.Vz&1023 == 512 && b.position.Vx&1023 == 512 {
		m := mapAt(int32(uint16(b.position.Vz>>10)), int32(uint16(b.position.Vx>>10)))
		x, z := b.position.Vx, b.position.Vz

		switch b.direction {
		case 0: // north
			switch {
			case open(m, east) && x < p.Vx:
				b.direction = 2
			case open(m, west) && x > p.Vx:
				b.direction = 3
			case open(m, north):
				b.direction = 0
			case open(m, east):
				b.direction = 2
			case open(m, west):
				b.direction = 3
			case open(m, south):
				b.direction = 1
			default:
				b.direction = -1
			}
		case 1: // south
			switch {
			case open(m, west) && x > p.Vx:
				b.direction = 3
			case open(m, east) && x < p.Vx:
				b.direction = 2
			case open(m, south):
				b.direction = 1
			case open(m, west):
				b.direction = 3
			case open(m, east):
				b.direction = 2
			case open(m, north):
				b.direction = 0
			default:
				b.direction = -1
			}
		case 2: // east
			switch {
			case open(m, south) && z > p.Vz:
				b.direction = 1
			case open(m, north) && z < p.Vz:
				b.direction = 0
			case open(m, east):
				b.direction = 2
			case open(m, south):
				b.direction = 1
			case open(m, north):
				b.direction = 0
			case open(m, west):
				b.direction = 3
			default:
				b.direction = -1
			}
		case 3: // west
			switch {
			case open(m, north) && z < p.Vz:
				b.direction = 0
			case open(m, south) && z > p.Vz:
				b.direction = 1
			case open(m, west):
				b.direction = 3
			case open(m, north):
				b.direction = 0
			case open(m, south):
				b.direction = 1
			case open(m, east):
				b.direction = 2
			default:
				b.direction = -1
			}
		default: // standing still
			switch {
			case open(m, north) && z < p.Vz:
				b.direction = 0
			case open(m, east) && x < p.Vx:
				b.direction = 2
			case open(m, west) && x > p.Vx:
				b.direction = 3
			case open(m, south) && z > p.Vz:
				b.direction = 1
			default:
				b.direction = -1
			}
		}
	}

	switch b.direction {
	case 0:
		b.position.Vz += 32
	case 1:
		b.position.Vz -= 32
	case 2:
		b.position.Vx += 32
	case 3:
		b.position.Vx -= 32
	}
}

// open reports whether a square has no wall on one side.
func open(m, wall uint16) bool { return m&wall == 0 }

// drawObject turns an object about y by ang, moves it to t, and adds it to
// an ordering table, subdividing it more the nearer it is. Nothing is drawn
// while the map is showing.
func drawObject(obj *yaroze.GsDOBJ2, ang int16, t yaroze.VECTOR, ot *yaroze.GsOT) {
	if displayMap {
		return
	}
	transform := yaroze.IDMATRIX
	yaroze.RotMatrixY(int32(ang), &transform)
	yaroze.TransMatrix(&transform, &t)

	obj.Coord2.Coord = transform
	obj.Coord2.Super = yaroze.WORLD
	obj.Coord2.Flg = 0

	var ls yaroze.MATRIX
	yaroze.GsGetLs(obj.Coord2, &ls)
	yaroze.GsSetLsMatrix(&ls)

	adx := abs(transform.T[0] - goodguy.position.Vx)
	adz := abs(transform.T[2] - goodguy.position.Vz)
	if adx < 1024 && adz < 1024 {
		obj.Attribute |= gsDIV2
	} else if adx < 2048 && adz < 2048 {
		obj.Attribute |= gsDIV1
	}

	yaroze.GsSortObject4(obj, ot, 14-int32(ot.Length), yaroze.Scratch())
	obj.Attribute &^= gsDIV3
}

// drawMap draws the walls of the squares you have seen, turned to the way
// you face, with you as a red box in the middle.
func drawMap(angle int16, pos yaroze.VECTOR) {
	box := yaroze.GsBOXF{R: 255, X: -3, Y: -3, W: 6, H: 6}
	yaroze.GsSortBoxFill(&box, &ot2[buffIdx], 0)

	l := yaroze.GsLINE{R: 255, G: 255, B: 255}
	sin, cos := rsin(int32(angle)), rcos(int32(angle))
	for row := range int32(mapRows) {
		for col := range int32(mapCols) {
			if visited[row][col] == 0 {
				continue
			}
			dx := col<<4 - pos.Vx>>6
			dz := row<<4 - pos.Vz>>6

			xsin, xcos := int16(dx*sin>>12), int16(dx*cos>>12)
			xonesin, xonecos := int16((dx+16)*sin>>12), int16((dx+16)*cos>>12)
			zsin, zcos := int16(dz*sin>>12), int16(dz*cos>>12)
			zonesin, zonecos := int16((dz+16)*sin>>12), int16((dz+16)*cos>>12)

			m := world[row][col]
			if m&north != 0 {
				l.X0, l.Y0 = xcos+zonesin, xsin-zonecos
				l.X1, l.Y1 = xonecos+zonesin, xonesin-zonecos
				yaroze.GsSortLine(&l, &ot2[buffIdx], 0)
			}
			if m&south != 0 {
				l.X0, l.Y0 = xcos+zsin, xsin-zcos
				l.X1, l.Y1 = xonecos+zsin, xonesin-zcos
				yaroze.GsSortLine(&l, &ot2[buffIdx], 0)
			}
			if m&east != 0 {
				l.X0, l.Y0 = xonecos+zsin, xonesin-zcos
				l.X1, l.Y1 = xonecos+zonesin, xonesin-zonecos
				yaroze.GsSortLine(&l, &ot2[buffIdx], 0)
			}
			if m&west != 0 {
				l.X0, l.Y0 = xcos+zsin, xsin-zcos
				l.X1, l.Y1 = xcos+zonesin, xsin-zonecos
				yaroze.GsSortLine(&l, &ot2[buffIdx], 0)
			}
		}
	}
}

// checkForHealthPickUp picks up a first aid box you are standing on.
func checkForHealthPickUp() {
	r, c := goodguy.position.Vz>>10, goodguy.position.Vx>>10
	if mapAt(r, c)&health != 0 {
		yaroze.SsUtKeyOn(vab, 4, 0, 60, 0, 127, 127)
		goodguy.health = min(goodguy.health+30, 120)
		world[r][c] -= health
	}
}

// poundingHeart sets the heartbeat's volume from the nearest monster.
func poundingHeart() {
	var volume int32
	for i := range int(numBadGuys) {
		dx := goodguy.position.Vx - badguy[i].position.Vx
		dz := goodguy.position.Vz - badguy[i].position.Vz
		closest := max(127-(dx*dx+dz*dz)>>17, 0)
		volume = max(volume, closest)
	}
	yaroze.SsUtSetVVol(heartVoice, int16(volume), int16(volume))
}

// drawHealthBox draws the health bar beside the status panel, green over
// 100, from red to green below.
func drawHealthBox(h int32) {
	box := yaroze.GsBOXF{
		X: status.X + 120, Y: status.Y + 12,
		W: uint16(goodguy.health), H: 8,
	}
	if h > 100 {
		box.R, box.G = 0, 200
	} else {
		box.R, box.G = uint8(255-h*255/100), uint8(h*200/100)
	}
	yaroze.GsSortBoxFill(&box, &ot2[buffIdx], 0)
}

// checkLevel moves to the next level at the exit, and stops a level's
// opening music after five seconds.
func checkLevel() {
	if mapAt(goodguy.position.Vz>>10, goodguy.position.Vx>>10)&exit != 0 {
		startLevelMusic()
		goodguy.level++
		switch goodguy.level {
		case 1:
			initMap(map1Data[:])
		case 2:
			initMap(map2Data[:])
		case 3:
			initMap(map3Data[:])
		default:
			goodguy.level = 1
			initMap(map1Data[:])
		}
	}
	if playingMusic && yaroze.VSync(-1)-playVsync > 300 {
		yaroze.SsUtKeyOff(levelVoice, vab, 11, 0, 60)
		playingMusic = false
	}
}

var oldPad yaroze.Buttons

// move moves you by increment, in x and z separately, as far as walls allow.
func move(increment yaroze.VECTOR) {
	if !collisionX(goodguy.position, increment) {
		goodguy.position.Vx += increment.Vx
	}
	if !collisionZ(goodguy.position, increment) {
		goodguy.position.Vz += increment.Vz
	}
}

// viewControl turns, walks, fires and shows the map, from the pad.
func viewControl() {
	if padStatus&yaroze.PADLleft != 0 {
		viewAngle = (viewAngle + 32) & 4095
	}
	if padStatus&yaroze.PADLright != 0 {
		viewAngle = (viewAngle - 32) & 4095
	}
	sin, cos := rsin(viewAngle), rcos(viewAngle)
	if padStatus&yaroze.PADLup != 0 { // forward
		move(yaroze.VECTOR{Vx: -sin >> 6, Vz: cos >> 6})
	}
	if padStatus&yaroze.PADLdown != 0 { // back
		move(yaroze.VECTOR{Vx: sin >> 6, Vz: -cos >> 6})
	}
	if padStatus&yaroze.PADR1 != 0 { // sidestep right
		move(yaroze.VECTOR{Vx: cos >> 6, Vz: sin >> 6})
	}
	if padStatus&yaroze.PADL1 != 0 { // sidestep left
		move(yaroze.VECTOR{Vx: -cos >> 6, Vz: -sin >> 6})
	}

	if padStatus&(yaroze.PADL1|yaroze.PADR1|yaroze.PADLup|yaroze.PADLdown) != 0 {
		goodguy.position.Vy += goodguy.bounce
		if goodguy.position.Vy > goodguy.top || goodguy.position.Vy < goodguy.bottom {
			goodguy.bounce = -goodguy.bounce
		}
	}

	if padStatus&yaroze.PADRdown != 0 && oldPad&yaroze.PADRdown == 0 { // fire, once a press
		for i := range goodguy.bullet {
			b := &goodguy.bullet[i]
			if b.available == 0 {
				continue
			}
			b.active = 1
			b.available = 0
			b.position.Vx = goodguy.position.Vx
			b.position.Vz = goodguy.position.Vz
			b.increment.Vx = -sin >> 4
			b.increment.Vz = cos >> 4
			yaroze.SsUtKeyOn(vab, 1, 0, 60, 0, 127, 127)
			break
		}
	}

	if padStatus&yaroze.PADRright != 0 {
		displayMap = true
	}
	oldPad = padStatus
}

// initMap reads a map from its text, loaded at addr: the rows and columns,
// then a line per row, top row first. X is solid, O a fake block, S the
// start, M a monster, F first aid, E the exit, and anything else floor.
func initMap(data []uint32) {
	mp := unsafe.Slice((*byte)(unsafe.Pointer(&data[0])), 4*len(data))
	i := 0

	visited = [maxMapRows][maxMapCols]uint8{}
	mapRows, mapCols, numBadGuys = 0, 0, 0

	for i < len(mp) && !isDigit(mp[i]) {
		i++
	}
	for i < len(mp) && isDigit(mp[i]) {
		mapRows = mapRows*10 + mp[i] - '0'
		i++
	}
	for i < len(mp) && !isDigit(mp[i]) {
		i++
	}
	for i < len(mp) && isDigit(mp[i]) {
		mapCols = mapCols*10 + mp[i] - '0'
		i++
	}
	for i < len(mp) && !isEOL(mp[i]) {
		i++
	}
	for i < len(mp) && isEOL(mp[i]) {
		i++
	}

	var tmap [maxMapRows][maxMapCols]uint8
	for r := int32(mapRows) - 1; r >= 0; r-- {
		for c := range int32(mapCols) {
			if i < len(mp) {
				tmap[r][c] = mp[i]
			}
			i++
		}
		for i < len(mp) && !isEOL(mp[i]) {
			i++
		}
		for i < len(mp) && isEOL(mp[i]) {
			i++
		}
	}

	rows, cols := int32(mapRows), int32(mapCols)
	for r := rows - 1; r >= 0; r-- {
		for c := range cols {
			world[r][c] = 0
			if tmap[r][c] == 'X' { // never visited
				continue
			}
			m := &world[r][c]
			// A border round the edge of the map.
			if r == 0 {
				*m |= south
			}
			if r == rows-1 {
				*m |= north
			}
			if c == 0 {
				*m |= west
			}
			if c == cols-1 {
				*m |= east
			}
			// Walls against solid squares and fake blocks.
			for _, solid := range [...]uint8{'X', 'O'} {
				if c < cols-1 && tmap[r][c+1] == solid {
					*m |= east
				}
				if c > 0 && tmap[r][c-1] == solid {
					*m |= west
				}
				if r < rows-1 && tmap[r+1][c] == solid {
					*m |= north
				}
				if r > 0 && tmap[r-1][c] == solid {
					*m |= south
				}
			}
			if c < cols-1 && tmap[r][c] == 'O' {
				*m |= fake
			}

			// The middle of the square.
			x, z := c<<10+512, r<<10+512
			switch tmap[r][c] {
			case 'S':
				initGoodGuy(&goodguy, x, z)
			case 'M':
				initBadGuy(&badguy[numBadGuys], x, z)
				numBadGuys++
			case 'F':
				*m |= health
			case 'E':
				*m |= exit
			}
		}
	}
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }
func isEOL(b byte) bool   { return b == 13 || b == 10 }

// inLineOfSight reports whether x1,z1 can be seen from x2,z2 facing ang:
// within 45 degrees either side, near enough, and with no wall where a line
// between them crosses each square.
func inLineOfSight(x1, z1, x2, z2 int32, ang int16) bool {
	dx, dz := x1-x2, z1-z2

	if abs(dx) > 3072 || abs(dz) > 3072 {
		return false
	}
	if x1>>10 == x2>>10 && z1>>10 == z2>>10 {
		return true
	}
	if d := abs(rinvtan(dz, -dx) - int32(ang)); d > 512 && d < 3584 {
		return false
	}
	if dx*dx+dz*dz > 3072*3072 {
		return false
	}

	r, c := z2>>10, x2>>10
	if abs(z1>>10-r) > abs(x1>>10-c) {
		var az int32
		if dz < 0 {
			az = -(z2 & 1023)
		} else {
			az = 1024 - z2&1023
		}
		ax := x2 + az*dx/dz
		xinc := 1024 * dx / abs(dz)
		for {
			for ax>>10 > c {
				c++
				if mapAt(r, c-1)&east != 0 {
					return false
				}
			}
			for ax>>10 < c {
				c--
				if mapAt(r, c+1)&west != 0 {
					return false
				}
			}
			if dz > 0 && mapAt(r, c)&north != 0 || dz < 0 && mapAt(r, c)&south != 0 {
				return false
			}
			if dz < 0 {
				r--
			} else {
				r++
			}
			ax += xinc
			if !(dz < 0 && r > z1>>10 || dz > 0 && r < z1>>10) {
				break
			}
		}
		if x1>>10 > c && mapAt(r, c)&east != 0 || x1>>10 < c && mapAt(r, c)&west != 0 {
			return false
		}
	} else {
		var ax int32
		if dx < 0 {
			ax = -(x2 & 1023)
		} else {
			ax = 1024 - x2&1023
		}
		az := z2 + ax*dz/dx
		zinc := 1024 * dz / abs(dx)
		for {
			for az>>10 > r {
				r++
				if mapAt(r-1, c)&north != 0 {
					return false
				}
			}
			for az>>10 < r {
				r--
				if mapAt(r+1, c)&south != 0 {
					return false
				}
			}
			if dx > 0 && mapAt(r, c)&east != 0 || dx < 0 && mapAt(r, c)&west != 0 {
				return false
			}
			if dx < 0 {
				c--
			} else {
				c++
			}
			az += zinc
			if !(dx < 0 && c > x1>>10 || dx > 0 && c < x1>>10) {
				break
			}
		}
		if z1>>10 > r && mapAt(r, c)&north != 0 || z1>>10 < r && mapAt(r, c)&south != 0 {
			return false
		}
	}
	return true
}

func initGoodGuy(g *goodGuy, x, z int32) {
	g.position = yaroze.VECTOR{Vx: x, Vy: -512, Vz: z}
	g.health = 100
	g.kills = 0
	g.top, g.bottom, g.bounce = -480, -544, -8
	for i := range g.bullet {
		initProjectile(&g.bullet[i], &bulletOBJ)
	}
}

func initBadGuy(b *badGuy, x, z int32) {
	b.active = 0
	b.activeDist = 5120
	b.direction = -1
	b.health = 5
	b.hitsize = 500
	b.facing = 0
	b.pvsync = 60
	b.position = yaroze.VECTOR{Vx: x, Vz: z}
	initProjectile(&b.bullet, &bullet2OBJ)
	b.object = &monster1OBJ
}

func initProjectile(p *projectile, o *yaroze.GsDOBJ2) {
	*p = projectile{available: 1, object: o}
}

var frameVSync int32

// animateBadGuys swaps every moving monster's walking frame, every ten
// frames.
func animateBadGuys() {
	if yaroze.VSync(-1)-frameVSync <= 10 {
		return
	}
	for i := range int(numBadGuys) {
		bg := &badguy[i]
		if bg.active == 0 {
			continue
		}
		if bg.object == &monster1OBJ {
			bg.object = &monster2OBJ
		} else {
			bg.object = &monster1OBJ
		}
		frameVSync = yaroze.VSync(-1)
	}
}

// wallFor is which wall a square shows: the exit's, a fake block's if the
// square beyond is one, or plain.
func wallFor(m uint16, neighborFake bool) *yaroze.GsDOBJ2 {
	switch {
	case m&exit != 0:
		return &ewallOBJ
	case neighborFake:
		return &fwallOBJ
	}
	return &wallOBJ
}

// drawWalls draws the floor, roof, walls and any first aid box of every
// square within three of you that is in sight, judged from eight points
// round its edge, and marks it seen for the map.
func drawWalls() {
	for r := range int32(mapRows) {
		for c := range int32(mapCols) {
			x, z := c<<10, r<<10
			if abs(goodguy.position.Vx>>10-c) > 3 || abs(goodguy.position.Vz>>10-r) > 3 {
				continue
			}
			gx, gz := goodguy.position.Vx, goodguy.position.Vz
			seen := false
			for _, p := range [8][2]int32{
				{1, 16}, {1, 1008}, {16, 1}, {16, 1023},
				{1008, 1}, {1008, 1023}, {1023, 16}, {1023, 1008},
			} {
				if inLineOfSight(x+p[0], z+p[1], gx, gz, int16(viewAngle)) {
					seen = true
					break
				}
			}
			if !seen {
				continue
			}
			visited[r][c] = 1
			m := world[r][c]

			if m&north != 0 {
				drawObject(wallFor(m, r < maxMapRows-1 && mapAt(r+1, c)&fake != 0), 0,
					yaroze.VECTOR{Vx: x + 512, Vy: -512, Vz: z + 1024}, &ot2[buffIdx])
			}
			if m&south != 0 {
				drawObject(wallFor(m, r > 0 && mapAt(r-1, c)&fake != 0), 2048,
					yaroze.VECTOR{Vx: x + 512, Vy: -512, Vz: z}, &ot2[buffIdx])
			}
			if m&east != 0 {
				drawObject(wallFor(m, c < maxMapCols-1 && mapAt(r, c+1)&fake != 0), 1024,
					yaroze.VECTOR{Vx: x + 1024, Vy: -512, Vz: z + 512}, &ot2[buffIdx])
			}
			if m&west != 0 {
				drawObject(wallFor(m, c > 0 && mapAt(r, c-1)&fake != 0), 3072,
					yaroze.VECTOR{Vx: x, Vy: -512, Vz: z + 512}, &ot2[buffIdx])
			}

			floor, roof := &floorOBJ, &roofOBJ
			if m&exit != 0 {
				floor, roof = &efloorOBJ, &eroofOBJ
			}
			drawObject(floor, 0, yaroze.VECTOR{Vx: x + 512, Vz: z + 512}, &ot1[buffIdx])
			drawObject(roof, 0, yaroze.VECTOR{Vx: x + 512, Vy: -1024, Vz: z + 512}, &ot1[buffIdx])

			if m&health != 0 {
				drawObject(&healthOBJ, int16(-viewAngle), yaroze.VECTOR{Vx: x + 512, Vz: z + 512}, &ot2[buffIdx])
			}
		}
	}
}
