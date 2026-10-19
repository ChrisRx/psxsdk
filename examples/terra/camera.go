package main

// The camera: a reference view that follows the player from behind and
// above at one of three distances, turned a quarter at a time by the
// shoulder buttons, easing after the player as they move; or, in a scene,
// a script of moves between fixed points.

import "go.chrisrx.dev/psx/yaroze"

var (
	view     yaroze.GsRVIEW2
	camAngle int  // the camera's turn about the player, 4096 to the circle
	camZoom  int  // 0, 1 or 2, farthest to nearest
	camShake bool // the view jitters, as when something heavy lands

	// A scripted move: the script's keys, how many are left, and the
	// frame within the current one.
	camScripted bool
	camScript   []camKey
	camKeysLeft int
	camT        int
)

// A camKey is one move of a camera script: how the eye and the reference
// point move, from where to where, and over how many frames.
type camKey struct {
	kind    int32 // 0: both interpolate; 1: eye interpolates, looking at the player; 2: eye offset from the player
	snap    int32 // start exactly at the first point
	eyeFrom [3]int32
	eyeTo   [3]int32
	refFrom [3]int32
	refTo   [3]int32
	frames  int32
}

// sinA and cosA are the original's sine and cosine, 4096 to the circle
// and 4096 for one.
func sinA(a int) int {
	a &= 0xfff
	switch {
	case a < 0x400:
		return int(sinTable[a])
	case a < 0x800:
		return int(sinTable[0x7ff-a])
	case a < 0xc00:
		return -int(sinTable[a-0x800])
	}
	return -int(sinTable[0xfff-a])
}

func cosA(a int) int { return sinA(int(int16(a + 0x400))) }

// atanA is the original's arc tangent of y over x, in its angles, from a
// table of the first eighth of the circle.
func atanA(y, x int) int {
	y16, x16 := int(int16(y)), int(int16(x))
	if y16 == 0 {
		if x16 < 0 {
			return 0x800
		}
		return 0
	}
	if x16 == 0 {
		if y16 < 0 {
			return -0x400
		}
		return 0x400
	}
	switch {
	case x16 > 0 && y16 > 0:
		if y16 <= x16 {
			return atanRatio(y16, x16)
		}
		return 0x400 - atanRatio(x16, y16)
	case x16 > 0:
		ny := -y16
		if x16 < ny {
			return atanRatio(x16, ny) - 0x400
		}
		return -atanRatio(ny, x16)
	case y16 > 0:
		nx := -x16
		if nx < y16 {
			return atanRatio(nx, y16) + 0x400
		}
		return 0x800 - atanRatio(y16, nx)
	}
	nx, ny := -x16, -y16
	if nx < ny {
		return -0x400 - atanRatio(nx, ny)
	}
	return atanRatio(ny, nx) - 0x800
}

// atanRatio is the table's angle for n over d, with n no more than d.
func atanRatio(n, d int) int { return int(atanTable[(n<<10)/d]) }

// The distances the three zoom levels put the camera at: back from the
// player, and above.
var zoomDistance = [3][2]int{{0xe00, 0xe00}, {0xa00, 0x800}, {0x800, 0x99}}

// halfStep is half of d, pushed one further from zero, the way the
// original eased its camera.
func halfStep(d int) int {
	h := d >> 1
	if d > 0 {
		h++
	} else if d < 0 {
		h--
	}
	return h
}

// eighthStep is an eighth of d, pushed one further from zero.
func eighthStep(d int) int {
	h := d >> 3
	if d > 0 {
		h++
	} else if d < 0 {
		h--
	}
	return h
}

// updateView moves the view after the player: the reference point halfway
// toward them, and the eye an eighth of the way toward its place behind
// them at the zoom level, or all the way with snap. The eye never goes
// below 153.
func updateView(zoom int, snap bool) {
	dist, height := zoomDistance[zoom][0], zoomDistance[zoom][1]
	px, py, pz := playerPosition()
	py, pz = -py, -pz
	if snap {
		view.Vrx, view.Vry, view.Vrz = int32(px), int32(py), int32(pz)
	} else {
		dx := int(int16(int32(px) - view.Vrx))
		dy := int(int16(int32(py) - view.Vry))
		dz := int(int16(int32(pz) - view.Vrz))
		view.Vrx += int32(halfStep(dx))
		view.Vry += int32(halfStep(dy))
		view.Vrz += int32(halfStep(dz))
	}
	ex := int16(px - int16(dist*sinA(camAngle)>>12))
	ey := int16(py - int16(height))
	if ey > 153 {
		ey = 0x99
	}
	ez := int16(pz - int16(dist*cosA(camAngle)>>12))
	if snap {
		view.Vpx, view.Vpy, view.Vpz = int32(ex), int32(ey), int32(ez)
		return
	}
	if !camShake {
		dx := int(int16(int32(ex) - view.Vpx))
		dy := int(int16(int32(ey) - view.Vpy))
		dz := int(int16(int32(ez) - view.Vpz))
		view.Vpx += int32(eighthStep(dx))
		view.Vpy += int32(eighthStep(dy))
		view.Vpz += int32(eighthStep(dz))
		return
	}
	f := int32(0x1000 - yaroze.Rand()%0x2aa)
	view.Vpx = view.Vrx + (int32(ex)-view.Vrx)*f/0x1000
	view.Vpy = view.Vry + (int32(ey)-view.Vry)*f/0x1000
	view.Vpz = view.Vrz + (int32(ez)-view.Vrz)*f/0x1000
}

// cameraFrame moves the camera for the frame: along its script if one is
// playing, and otherwise under the shoulder buttons, L1 and R1 turning it
// and L2 and R2 zooming, and then sets the view.
func cameraFrame() {
	if camScripted {
		scriptedCamera()
	} else {
		if pad.Pressed(yaroze.PADL1) {
			camAngle = (camAngle - 0x200) & 0xfff
			playEffect(2)
		} else if pad.Pressed(yaroze.PADR1) {
			camAngle = (camAngle + 0x200) & 0xfff
			playEffect(2)
		}
		if pad.Pressed(yaroze.PADL2) {
			if camZoom > 0 {
				camZoom--
				playEffect(2)
			}
		} else if pad.Pressed(yaroze.PADR2) && camZoom < 2 {
			camZoom++
			playEffect(2)
		}
		updateView(camZoom, false)
	}
	view.Set()
}

// resetView snaps the camera behind the player at the zoom level.
func resetView() {
	updateView(camZoom, true)
}

// startCameraScript starts camera script n.
func startCameraScript(n int) {
	camScripted = true
	camT = 0
	camScript = cameraScripts[n]
	camKeysLeft = len(camScript)
}

// nextCameraKey moves the script on to its next key, or ends it.
func nextCameraKey() {
	camKeysLeft--
	if camKeysLeft == 0 {
		camScripted = false
		return
	}
	camScript = camScript[1:]
	camT = 0
}

// scriptedCamera moves the camera along the current key of its script:
// the points it is between, by its kind, and then the eye a 32nd of the
// way toward its point each frame, the reference point as fast, or a
// quarter of the way when it looks at the player.
func scriptedCamera() {
	k := &camScript[0]
	t, n := int32(camT), k.frames
	px, py, pz := playerPosition()
	py, pz = -py, -pz
	var ex, ey, ez, rx, ry, rz int16
	switch k.kind {
	case 1:
		ex, ey, ez = lerpKey3(&k.eyeFrom, &k.eyeTo, t, n)
		rx, ry, rz = px, py, pz
	case 0:
		ex, ey, ez = lerpKey3(&k.eyeFrom, &k.eyeTo, t, n)
		rx, ry, rz = lerpKey3(&k.refFrom, &k.refTo, t, n)
	case 2:
		ex, ey, ez = lerpKey3(&k.eyeFrom, &k.eyeTo, t, n)
		ex, ey, ez = px+ex, py+ey, pz+ez
		rx, ry, rz = px, py, pz
	}
	if camT == 0 && k.snap != 0 {
		view.Vpx, view.Vpy, view.Vpz = int32(ex), int32(ey), int32(ez)
		view.Vrx, view.Vry, view.Vrz = int32(rx), int32(ry), int32(rz)
	} else {
		view.Vpx += (int32(ex) - view.Vpx) >> 5
		view.Vpy += (int32(ey) - view.Vpy) >> 5
		view.Vpz += (int32(ez) - view.Vpz) >> 5
		if k.kind == 0 {
			view.Vrx += (int32(rx) - view.Vrx) >> 5
			view.Vry += (int32(ry) - view.Vry) >> 5
			view.Vrz += (int32(rz) - view.Vrz) >> 5
		} else {
			view.Vrx += (int32(rx) - view.Vrx) >> 2
			view.Vry += (int32(ry) - view.Vry) >> 2
			view.Vrz += (int32(rz) - view.Vrz) >> 2
		}
	}
	camT++
	if int32(camT) > n {
		camT = int(n)
	}
}

// lerpKey3 is a point t frames of n from one point toward another.
func lerpKey3(from, to *[3]int32, t, n int32) (x, y, z int16) {
	x = int16(from[0]) + int16((to[0]-from[0])*t/n)
	y = int16(from[1]) + int16((to[1]-from[1])*t/n)
	z = int16(from[2]) + int16((to[2]-from[2])*t/n)
	return
}

// cameraScripts are the scripted moves: the opening's tour of the port,
// and the scenes.
var cameraScripts = [6][]camKey{
	{
		{0, 1, [3]int32{14336, -3072, -16384}, [3]int32{13312, -768, -10240}, [3]int32{12800, -512, -5120}, [3]int32{6656, -1536, -15360}, 210},
		{0, 0, [3]int32{13312, -768, -10240}, [3]int32{12288, -1024, -11776}, [3]int32{6656, -1536, -15360}, [3]int32{10240, -768, -13312}, 60},
		{0, 0, [3]int32{12288, -1024, -11776}, [3]int32{9728, -1024, -11264}, [3]int32{10240, -768, -13312}, [3]int32{12800, -768, -15360}, 30},
		{1, 1, [3]int32{5120, -3072, -5120}, [3]int32{11776, -1024, -11776}, [3]int32{}, [3]int32{}, 60},
		{2, 1, [3]int32{512, -512, -1536}, [3]int32{512, -1024, -2560}, [3]int32{}, [3]int32{}, 30},
		{1, 1, [3]int32{9216, -1536, -13312}, [3]int32{10240, -3584, -10240}, [3]int32{}, [3]int32{}, 90},
		{2, 1, [3]int32{0, 0, 2560}, [3]int32{2560, -2048, -2560}, [3]int32{}, [3]int32{}, 150},
		{1, 1, [3]int32{14336, -512, -1024}, [3]int32{14336, -1536, -2560}, [3]int32{}, [3]int32{}, 60},
	},
	{
		{0, 1, [3]int32{2048, -4096, -7168}, [3]int32{3584, -102, -5120}, [3]int32{0, -1024, -4096}, [3]int32{4608, -614, -4096}, 90},
	},
	{
		{2, 1, [3]int32{1024, -256, 1536}, [3]int32{-1024, -768, 1024}, [3]int32{}, [3]int32{}, 180},
	},
	{
		{0, 1, [3]int32{14592, -1280, -14336}, [3]int32{11520, -1280, -14336}, [3]int32{13056, -768, -12288}, [3]int32{12032, -768, -12288}, 150},
		{2, 1, [3]int32{-2560, -204, -1024}, [3]int32{-1024, -51, 512}, [3]int32{}, [3]int32{}, 120},
		{0, 1, [3]int32{13056, -512, -11520}, [3]int32{13056, -1024, -10752}, [3]int32{12032, -1280, -12032}, [3]int32{12032, 0, -11264}, 10},
		{2, 1, [3]int32{768, -256, 1024}, [3]int32{768, -256, 1024}, [3]int32{}, [3]int32{}, 30},
		{2, 0, [3]int32{768, -256, 1024}, [3]int32{1024, 0, 2048}, [3]int32{}, [3]int32{}, 300},
	},
	{
		{0, 1, [3]int32{10240, -1280, -14336}, [3]int32{11264, -1280, -13824}, [3]int32{9216, -768, -12288}, [3]int32{12032, -768, -12288}, 60},
		{2, 1, [3]int32{768, 0, -512}, [3]int32{768, -256, -1024}, [3]int32{}, [3]int32{}, 150},
		{2, 0, [3]int32{768, -256, -1024}, [3]int32{1536, -256, -2048}, [3]int32{}, [3]int32{}, 360},
	},
	{
		{0, 1, [3]int32{9728, -1024, -13312}, [3]int32{10240, -1024, -13312}, [3]int32{10752, -1024, -12288}, [3]int32{10752, -1024, -12288}, 60},
		{0, 1, [3]int32{10752, -1280, -11264}, [3]int32{13312, -1024, -10240}, [3]int32{10752, -1024, -12288}, [3]int32{10752, -1024, -12288}, 480},
	},
}
