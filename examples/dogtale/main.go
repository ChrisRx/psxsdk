// A Dog Tale, a platform game across a desert in parallax, to free a dog
// chained up with five padlocks.
//
// A port of Ira Rainey's Net Yaroze game "A Dog Tale" (1997), which went
// out on the cover disc of the Official UK PlayStation Magazine and on
// Sony's own Net Yaroze demo discs: a platform game in sprites, twelve
// screens wide, in which a man sets out across a desert to free his dog,
// chained up at the far end with five padlocks, by finding the five keys.
// There are platforms to climb, some of them moving, bones to collect,
// fifty of which are worth a life, and cacti, which cost one. The desert
// scrolls in parallax, the two ranges of mountains and the floor each at
// their own speed under a fixed sky, which is what the author says the
// game grew out of. The title screen scrolls a message along its foot
// under a logo that spins in, with an about box that scales in over it,
// and when the last life goes the screen is grabbed into video memory and
// spun away to nothing. The tune is Sony's sample, the one the check
// sample plays, which the author's message says was borrowed. The
// author's readme asks anyone using the code for a thank you: thanks,
// Ira. His code and graphics are under the MIT license, Copyright (c)
// 2023 Ira Rainey, which licenses/ADOGTALE-LICENSE.txt at the root of the
// repository carries.
//
// Differences from the original:
//
//   - The pictures are read off the disc, as the one file the original was
//     sent over the serial cable as, into the program's own memory rather
//     than to a fixed address. The tune comes from the SDK's own files, as
//     the check sample's does. The file's first picture, a Net Yaroze logo
//     that a function the source declares but never defines would have
//     shown, is loaded and never used.
//   - Every object has an image description of its own. The original
//     gives each a pointer to one that is never set, and so writes every
//     image's description to address 0.
//   - Objects are checked to be in use before their sizes are read, and
//     the search for the platform under the man stops at the last object.
//     The original reads the sizes of unused objects through null
//     pointers, and one past the end of its array.
//   - There are five keys. The original puts six into the world, reading
//     past the end of its table of five for the last, which lands at the
//     start of the level at whatever height the memory after the table
//     gives.
//   - The pad check reports nothing pressed when neither Select nor Start
//     is, and the title's spin starts from zero. The original leaves both
//     to whatever happened to be in a register or on the stack.
//
// Nothing allocates after startup.
//
// # Controls
//
// Left and right walk, Cross jumps, straight up or the way you are walking,
// and Triangle shows the keys found so far. Select pauses, and any direction
// or Cross goes on; Start while paused gives up and goes back to the title.
// On the title, left and right choose between About and Play and Cross
// chooses; Select closes the about box. There is no way to quit the program,
// as in the original.
package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/disc"
	"go.chrisrx.dev/psx/examples/internal/demo"
	"go.chrisrx.dev/psx/yaroze"
)

const (
	otLength = 10

	// Sprite attribute bits.
	sixteenBitOn = 1 << 25
	eightBitOn   = 1 << 24
	spriteOff    = 1 << 31

	packetMax = 2048 * 24

	mvol = 127
	svol = 127

	maxObjects = 256

	screenWidth  = 320
	screenHeight = 240

	// The size of the playing area.
	noOfScreens    = 12
	maxPoint       = screenWidth * noOfScreens
	maxLevelOneObj = 256
)

// The screens refreshScreen can draw.
const (
	logoFade = iota
	bones
	aboutBox
	startGame
	deadGeezer
	paused
)

// Kinds of world object.
const (
	bone = iota
	platform
	cactus
	key
	dog
)

// How the man is jumping.
const (
	still = iota
	jumpUp
	jumpLeft
	jumpRight
)

// The data, read into the program's own memory, in buffers of whole
// sectors. The pictures are one file, and the sprites find each at the
// offset the original had it at from the file's own address.
var (
	dogDat  [(292284 + 2047) / 2048 * 512]uint32
	seqData [(39616 + 2047) / 2048 * 512]uint32
	vhData  [(60448 + 2047) / 2048 * 512]uint32
	vbData  [(401360 + 2047) / 2048 * 512]uint32

	dogFile = disc.Register("ASSETS/DOG.DAT", "assets/DOG.DAT").Into(dogDat[:])
	seqFile = disc.Register("ASSETS/SOUND/SAMPLE1.SEQ", "assets/SAMPLE1.SEQ").Into(seqData[:])
	vh      = disc.Register("ASSETS/SOUND/STD0.VH", "assets/STD0.VH").Into(vhData[:])
	vb      = disc.Register("ASSETS/SOUND/STD0.VB", "assets/STD0.VB").Into(vbData[:])
	files   = []*disc.File{&dogFile, &seqFile, &vh, &vb}
)

// The pictures, as addresses within the file, since the sprites are set
// up from a picture's address as the original did it.
var (
	dataAddr   = uintptr(unsafe.Pointer(&dogDat[0]))
	logoData   = dataAddr + 0x02a20
	cactusData = dataAddr + 0x06fb0
	copyData   = dataAddr + 0x075d0
	bothData   = dataAddr + 0x085c4
	charData   = dataAddr + 0x0974c
	textData   = dataAddr + 0x1196c
	skyData    = dataAddr + 0x175ac
	backData   = dataAddr + 0x1d58c
	frontData  = dataAddr + 0x2676c
	floorData  = dataAddr + 0x2ff8c
	smlogoData = dataAddr + 0x315ac
	headData   = dataAddr + 0x31c2c
	bboneData  = dataAddr + 0x3215c
	boneNum    = dataAddr + 0x3243c
	platData   = dataAddr + 0x3465c
	readyData  = dataAddr + 0x34c7c
	sboneData  = dataAddr + 0x35e9c
	pauseData  = dataAddr + 0x35fbc
	manData    = dataAddr + 0x374dc
	keyData    = dataAddr + 0x39efc
	dogData    = dataAddr + 0x3a51c
	overData   = dataAddr + 0x3a97c
	endData    = dataAddr + 0x3bb9c
)

// An object is a sprite with its picture, its place and its size, and
// the rectangle it collides with.
type object struct {
	alive                     bool
	xpos, ypos, width, height int32
	image                     yaroze.GsIMAGE
	sprite                    yaroze.GsSPRITE
	collide                   yaroze.RECT
	moving                    bool
}

// A levelObject is something in the world: where it is along the level
// and how far it has moved from there, the object that draws it, and what
// it is.
type levelObject struct {
	globalPos        int32
	globalY          int32
	xOffset, yOffset int32
	typ              *object
	display          bool
	onScr            bool
	disc             int8
	moving           bool
	moveDir          bool
}

var (
	objectArray  [maxObjects]object
	spriteObject [maxObjects]*object
	worldArray   [maxObjects]levelObject

	// spare is handed out when every object is in use, which the game
	// never asks for.
	spare object

	gpuPacketArea [2][packetMax / 4]uint32
	worldOT       [2]yaroze.GsOT
	otTags        [2][1 << otLength]yaroze.GsOT_TAG
	activeBuff    int32

	overlay yaroze.GsBOXF // the about box

	whereYouAre int32 // how far along the level the screen is
	scrPos      int32

	backR, backG, backB uint8

	messPos int

	life, spinning, deadSpinning                int32
	noBones, noKeys, keyMoving, keyCounter      int32
	displayLevel, changeFrame, frameCounter     int32
	goingUp, jumping, maxBottom, onPlat, fall   int32
	breakOut, gamePaused, dead, alreadyRun      int32
	quitGame, endOfGame, whichPlat              int32
	oldPadd                                     yaroze.Buttons
	restart, rPositionX, rPositionY, rMaxBottom int32

	bank yaroze.Bank
	song yaroze.Sequence

	frameNumber uint32
)

// The scrolling message's font: where each character from space on is in
// the character picture.
var (
	charYlut = [64]uint8{
		0, 0, 0, 0, 0, 0, 0, 0,
		32, 32, 32, 32, 32, 32, 32, 32,
		64, 64, 64, 64, 64, 64, 64, 64,
		96, 96, 96, 96, 96, 96, 96, 96,
		128, 128, 128, 128, 128, 128, 128, 128,
		160, 160, 160, 160, 160, 160, 160, 160,
		192, 192, 192, 192, 192, 192, 192, 192,
		224, 224, 224, 224, 224, 224, 224, 224,
	}
	charXlut = [64]uint8{
		0, 16, 32, 48, 64, 80, 96, 112,
		0, 16, 32, 48, 64, 80, 96, 112,
		0, 16, 32, 48, 64, 80, 96, 112,
		0, 16, 32, 48, 64, 80, 96, 112,
		0, 16, 32, 48, 64, 80, 96, 112,
		0, 16, 32, 48, 64, 80, 96, 112,
		0, 16, 32, 48, 64, 80, 96, 112,
		0, 16, 32, 48, 64, 80, 96, 112,
	}
)

const message = "                              A DOG TALE      A VIDEO GAME OF AMAZINGLY THIN PROPORTIONS FOR THE SONY PLAYSTATION          ALL CODE AND GRAPHICS KNOCKED TOGETHER BY IRA RAINEY, AND THE SNAZZY TUNE WAS 'BORROWED' FROM SONY - I WAS GONNA STICK AN ORIGINAL TUNE IN BUT I NEEDED TO GET THE GAME FINISHED BEFORE I MOVED HOUSE SO I USED THIS TUNE MOSTLY DUE TO TIME CONSTRAINTS, THANKS GUYS!          WELL HERE IT IS THEN MY VERY FIRST PLAYSTATION GAME DEVELOPED ON MY NET YAROZE, WHAT DO YOU RECKON?         YEAH OK I KNOW IT'S NOT EXACTLY CRASH BANDICOOT OR TOMB RAIDER BUT HEY IT'S ONLY MY FIRST ATTEMPT!      JUST THINK OF IT AS MARIO MEETS MANIC MINER            THE STORY?    WELL BASICALLY YOU'RE OUT TO RESCUE YOUR DOG WHO'S BEEN DOGNAPPED BY A NASTY NASTY GANG FOR SOME REASON, AND THEY'VE CHAINED HIM TO A RAILING WITH FIVE HUGE PADLOCKS   SO, ALL YOU NEED TO DO IS COLLECT ALL FIVE KEYS AND UNLOCK HIM   THAT'S ABOUT IT REALLY   A BIT THIN I KNOW BUT WHO GIVES A TOSS - IT'S A GAME AND I DON'T INTEND TO WRITE A BIOGRAPHY FOR A BUNCH OF PIXELS     AND WHO WOULD REALLY CARE IF I DID?      OH YEAH I FORGOT, COLLECT ALL THE BONES YOU CAN ALONG THE WAY - GET FIFTY AND YOU GET AN EXTRA LIFE!      WOW!     A BIG SHOUT HAS GOTTA GO OUT TO ALL THE FOLKS AT SONY, THANKS TO WHOM ALL THIS HAS BECOME POSSIBLE      - NET YAROZE -     PROBABLY THE BEST LUMP OF CASH I'VE EVER SPENT, WELL SINCE THAT CASE OF BEERS AND THE CHICKEN VINDALOO LAST NIGHT ANYWAY!        THANKS SONY! YOU'VE BROUGHT FUN AND ENJOYMENT BACK INTO MY MUNDANE PROGRAMMING EXISTENCE   YOU'VE TAKEN ME BACK TO THE COMMODORE 64 DAYS OF MY YOUTH - CHRIST NOW THAT I THINK ABOUT IT THAT WAS A HELL OF A LONG TIME AGO!         I DON'T WANT TO THINK ABOUT IT!         AM I REALLY THAT OLD ALREADY?   ANYONE ELSE HAPPILY REMEMBER HONG KONG PHOOEY AND INCH HIGH PRIVATE EYE? - THE FIRST TIME AROUND!        THOSE WERE THE DAYS, BACK WHEN I WAS JUST A BROKE, STRUGGLING, BEERSWILLING SEVENTEEN YEAR OLD PROGRAMMER!      AND HERE I AM NOW AS AN ADULT - A BROKE, STRUGGLING, BEERSWILLING TWENTY SEVEN YEAR OLD PROGRAMMER!         HEY NOTHING'S CHANGED!       IS THAT GOOD??         WHO CARES?      WHO'S STILL READING THIS?      YOU SHOULD BE PLAYING THE GAME BY NOW!        SHIT! WILL YOU LOOK AT THE TIME I'VE GOTTA BE UP AT SIX, I'M GONNA WRAP THIS SUCKER UP FOR THE NIGHT, SEE YOU ALL NEXT TIME FOLKS    - KEEP UP THE CODING AND LETS SEE MORE DEMOS -      IRA RAINEY - AUGUST 1997      CONTACT IRA@NETCOMUK.CO.UK   "

func main() {
	dead = 1
	backR, backG, backB = 0, 0, 0

	yaroze.PadInit()
	yaroze.SetVideoMode(yaroze.MODE_PAL)
	yaroze.GsInitGraph(screenWidth, screenHeight, 4, 0, 0)
	yaroze.GsDefDispBuff(0, 0, 0, screenHeight)

	for i := range 2 {
		worldOT[i].Length = otLength
		worldOT[i].Org = &otTags[i][0]
	}

	if missing := demo.Load(files, nil); missing != "" {
		fail("could not read ", missing)
		return
	}

	for breakOut == 0 {
		if dead != 0 {
			initObjects()
			initSound()
			playSEQ()
			breakOut = 0
			displayFrontEnd()
		}
		if breakOut == 0 {
			setupLevelOne()
		}
		if breakOut == 0 {
			playLevelOne()
		}
		if breakOut != 0 {
			stopSound()
			yaroze.ResetGraph(1)
		}
	}
}

// fail shows what went wrong until the program is quit.
func fail(what, detail string) {
	var buf [64]byte
	var text yaroze.Buffer
	text.Init(buf[:])
	yaroze.FntLoad(960, 256)
	yaroze.FntOpen(16, 16, 288, 32, 0, 128)
	for yaroze.PadRead()&(yaroze.PADstart|yaroze.PADselect) != yaroze.PADstart|yaroze.PADselect {
		activeBuff = yaroze.GsGetActiveBuff()
		yaroze.GsSetWorkBase(yaroze.Packets(&gpuPacketArea[activeBuff][0]))
		yaroze.GsClearOt(0, 0, &worldOT[activeBuff])
		yaroze.DrawSync(0)
		yaroze.VSync(0)
		yaroze.GsSwapDispBuff()
		yaroze.GsSortClear(0, 0, 0, &worldOT[activeBuff])
		yaroze.GsDrawOt(&worldOT[activeBuff])
		text.Reset()
		text.WriteString(what).WriteString(detail).Print()
		yaroze.FntFlush(-1)
	}
	yaroze.ResetGraph(3)
}

// checkPad reads the pad and moves the man by it. It returns 99 for
// Select and 98 for Start, and 0 otherwise.
func checkPad() int32 {
	padd := yaroze.PadRead()

	down := padd&yaroze.PADRdown != 0
	oldDown := oldPadd&yaroze.PADRdown != 0
	jumpDude := down && !oldDown
	oldPadd = padd

	if padd&yaroze.PADselect != 0 {
		return 99
	}
	if padd&yaroze.PADstart != 0 {
		return 98
	}

	if padd&yaroze.PADRup != 0 && displayLevel == 0 && noKeys != 0 {
		keyMoving = 1
	}

	if gamePaused != 0 && padd&(yaroze.PADLleft|yaroze.PADLright|yaroze.PADLup|yaroze.PADLdown|yaroze.PADRdown) != 0 {
		gamePaused = 0
		song.Resume()
	}

	if displayLevel != 0 {
		return 0
	}
	man := spriteObject[22]

	// Right, which moves the world left.
	if padd&yaroze.PADLright != 0 && !jumpDude && jumping == still {
		man.sprite.U = 0
		changeFrame++
		moveLeft()
	}
	if padd&yaroze.PADLright != 0 && jumpDude && jumping == still {
		man.sprite.U = 0
		changeFrame++
		moveLeft()
		jumping = jumpRight
	}
	if padd&yaroze.PADLright != 0 && jumping != still {
		man.sprite.U = 0
	}

	// Left, which moves the world right.
	if padd&yaroze.PADLleft != 0 && !jumpDude && jumping == still {
		man.sprite.U = 32
		changeFrame++
		moveRight()
	}
	if padd&yaroze.PADLleft != 0 && jumpDude && jumping == still {
		man.sprite.U = 32
		changeFrame++
		moveRight()
		jumping = jumpLeft
	}
	if padd&yaroze.PADLleft != 0 && jumping != still {
		man.sprite.U = 32
	}

	if jumpDude && jumping == still {
		jumping = jumpUp
	}
	return 0
}

// createObject sets an object up as a sprite showing the picture at
// timAdd: the picture and its palette go into video memory, and the
// sprite takes its page, with extra added to the page's x, which the wide
// backgrounds use to reach their second page.
func createObject(obj *object, timAdd uintptr, extra int32) {
	im := &obj.image
	yaroze.GsGetTimInfo(yaroze.At(timAdd+4), im)

	rect := yaroze.RECT{X: im.Px, Y: im.Py, W: int16(im.Pw), H: int16(im.Ph)}
	yaroze.LoadImage(&rect, im.Pixel)

	if im.Pmode>>3&1 != 0 {
		rect = yaroze.RECT{X: im.Cx, Y: im.Cy, W: int16(im.Cw), H: int16(im.Ch)}
		yaroze.LoadImage(&rect, im.Clut)
		obj.sprite.Cx = im.Cx
		obj.sprite.Cy = im.Cy
	}

	obj.sprite.Attribute = (im.Pmode & 3) << 24
	obj.sprite.X = int16(obj.xpos)
	obj.sprite.Y = int16(obj.ypos)
	obj.sprite.W = uint16(obj.width)
	obj.sprite.H = uint16(obj.height)
	obj.sprite.Tpage = yaroze.GetTPage(int32(im.Pmode&3), 0, int32(im.Px)+extra, int32(im.Py))

	yaroze.DrawSync(0)
}

// getNextFreeObject returns the first object not in use, cleared.
func getNextFreeObject() *object {
	for i := range objectArray {
		if !objectArray[i].alive {
			clearObject(&objectArray[i])
			return &objectArray[i]
		}
	}
	clearObject(&spare)
	return &spare
}

func clearObject(obj *object) {
	*obj = object{}
	clearSprite(&obj.sprite)
}

func clearWObject(obj *levelObject) {
	*obj = levelObject{}
}

// clearSprite sets a sprite to nothing, switched off.
func clearSprite(sprite *yaroze.GsSPRITE) {
	*sprite = yaroze.GsSPRITE{
		Attribute: spriteOff,
		R:         128, G: 128, B: 128,
		Scalex: yaroze.ONE, Scaley: yaroze.ONE,
	}
}

func initObjects() {
	for i := range objectArray {
		clearObject(&objectArray[i])
		clearWObject(&worldArray[i])
	}
}

// defineObject sets sprite no up: where it is and how big, with xOff and
// yOff its place in the picture at add.
func defineObject(no int, x, y, w, h, xOff, yOff int32, add uintptr, extra int32) {
	o := spriteObject[no]
	o.xpos, o.collide.X = x, int16(x)
	o.ypos, o.collide.Y = y, int16(y)
	o.width, o.collide.W = w, int16(w)
	o.height, o.collide.H = h, int16(h)
	o.sprite.U = uint8(xOff)
	o.sprite.V = uint8(yOff)
	createObject(o, add, extra)
}

func sortFast(no int, ot *yaroze.GsOT, pri uint16) {
	yaroze.GsSortFastSprite(&spriteObject[no].sprite, ot, pri)
}

func sortSprite(no int, ot *yaroze.GsOT, pri uint16) {
	yaroze.GsSortSprite(&spriteObject[no].sprite, ot, pri)
}

// refreshScreen draws a frame of one of the screens.
func refreshScreen(tofBackground int) {
	activeBuff = yaroze.GsGetActiveBuff()
	yaroze.GsSetWorkBase(yaroze.Packets(&gpuPacketArea[activeBuff][0]))
	ot := &worldOT[activeBuff]
	yaroze.GsClearOt(0, 0, ot)

	switch tofBackground {
	case logoFade:
		sortFast(0, ot, 0)
		sortFast(1, ot, 0)

	case paused:
		sortFast(0, ot, 7)
		sortFast(1, ot, 7)
		for i := 2; i <= 5; i++ {
			sortFast(i, ot, 6)
		}
		for i := 6; i <= 9; i++ {
			sortFast(i, ot, 5)
		}
		for i := 10; i <= 12; i++ {
			sortFast(i, ot, 3)
		}
		sortFast(13, ot, 2)
		for idx := range life {
			sortSprite(14+int(idx), ot, 2)
		}
		sortFast(19, ot, 2)
		for idx := range maxLevelOneObj {
			w := &worldArray[idx]
			scrPos = w.globalPos + w.xOffset - whereYouAre
			if w.display && scrPos <= screenWidth && scrPos+w.typ.width >= 0 {
				w.typ.sprite.X = int16(scrPos)
				w.typ.collide.X = int16(scrPos)
				switch w.disc {
				case platform:
					sortFast(50+idx, ot, 3)
				case bone:
					sortFast(50+idx, ot, 2)
				default:
					sortFast(50+idx, ot, 4)
				}
			}
		}
		sortFast(20, ot, 2)
		sortFast(21, ot, 2)
		sortFast(22, ot, 1)
		sortFast(24, ot, 0)
		for idx := range noKeys {
			sortFast(26+int(idx), ot, 2)
		}

	case deadGeezer:
		sortFast(2, ot, 0)

	case startGame:
		if endOfGame == 0 {
			sortSprite(0, ot, 1)
			sortSprite(1, ot, 1)
		}
		sortFast(2, ot, 0)

	case bones:
		scrollText()
		sortFast(0, ot, 4) // the sky
		sortFast(1, ot, 4)
		sortFast(2, ot, 3) // the back mountains
		sortFast(3, ot, 3)
		sortFast(4, ot, 2) // the front
		sortFast(5, ot, 2)
		sortFast(6, ot, 1) // the floor
		sortFast(7, ot, 1)
		sortSprite(8, ot, 0) // the bones
		sortSprite(9, ot, 0)
		sortSprite(10, ot, 0)       // the logo
		for i := 11; i <= 32; i++ { // the message
			sortFast(i, ot, 0)
		}
		sortFast(34, ot, 0)

	case aboutBox:
		scrollText()
		sortFast(0, ot, 4)
		sortFast(1, ot, 4)
		sortFast(2, ot, 3)
		sortFast(3, ot, 3)
		sortFast(4, ot, 2)
		sortFast(5, ot, 2)
		sortFast(6, ot, 1)
		sortFast(7, ot, 1)
		sortSprite(8, ot, 1)
		sortSprite(9, ot, 1)
		sortSprite(10, ot, 1)
		sortFast(34, ot, 1)
		yaroze.GsSortBoxFill(&overlay, ot, 0) // the box and the text in it
		sortSprite(33, ot, 0)
		for i := 11; i <= 27; i++ { // the message
			sortFast(i, ot, 2)
		}
		for i := 28; i <= 32; i++ {
			sortFast(i, ot, 1)
		}
	}

	yaroze.DrawSync(0)
	yaroze.VSync(0)
	yaroze.GsSwapDispBuff()
	yaroze.GsSortClear(backR, backG, backB, ot)
	yaroze.GsDrawOt(ot)
	frameNumber++
}

// setupLevelOne sets the sprites up for the level: the scenery, the
// panel along the bottom, the keys, the man, and the world's objects.
// After a life is lost only their places are set again.
func setupLevelOne() {
	if dead != 0 {
		for idx := range maxObjects {
			spriteObject[idx] = getNextFreeObject()
			clearObject(spriteObject[idx])
			spriteObject[idx].alive = true
		}

		life = 4
		restart = 0
		rPositionX = 30
		rPositionY = 148
		rMaxBottom = 148
		spinning = 0
		noBones = 0
		noKeys = 0
		endOfGame = 0
	}

	whereYouAre = restart
	gamePaused = 0
	whichPlat = -1

	// The sky.
	defineObject(0, 0, 0, 160, 75, 0, 0, skyData, 0)
	defineObject(1, 160, 0, 160, 75, 32, 0, skyData, 80)

	// The back mountains.
	defineObject(2, 0, 30, 160, 115, 0, 0, backData, 0)
	defineObject(3, 160, 30, 160, 115, 32, 0, backData, 80)
	defineObject(4, 320, 30, 160, 115, 0, 0, backData, 0)
	defineObject(5, 480, 30, 160, 115, 32, 0, backData, 80)

	// The front mountains.
	defineObject(6, 0, 70, 160, 120, 0, 0, frontData, 0)
	defineObject(7, 160, 70, 160, 120, 32, 0, frontData, 80)
	defineObject(8, 320, 70, 160, 120, 0, 0, frontData, 0)
	defineObject(9, 480, 70, 160, 120, 32, 0, frontData, 80)

	// The floor.
	defineObject(10, 0, 180, 160, 32, 0, 0, floorData, 0)
	defineObject(11, 160, 180, 160, 32, 0, 0, floorData, 0)
	defineObject(12, 320, 180, 160, 32, 0, 0, floorData, 0)

	// The small logo.
	defineObject(13, 10, 212, 40, 28, 0, 0, smlogoData, 0)

	// The lives, heads that can spin about their centers.
	for i := range 5 {
		defineObject(14+i, int32(80+28*i), 227, 28, 28, 40, 0, headData, 0)
		spriteObject[14+i].sprite.Mx = 14
		spriteObject[14+i].sprite.My = 14
		spriteObject[14+i].sprite.Rotate = yaroze.ONE
	}

	// The big bone, and the count of bones beside it.
	defineObject(19, 222, 212, 48, 28, 0, 28, bboneData, 0)
	defineObject(20, 273, 211, 16, 32, int32(charXlut[noBones/10+16]), int32(charYlut[noBones/10+16])+108, boneNum, 0)
	defineObject(21, 288, 211, 16, 32, int32(charXlut[noBones%10+16]), int32(charYlut[noBones%10+16])+108, boneNum, 0)

	// The keys collected, above the screen until shown.
	for i := range 5 {
		defineObject(26+i, int32(96+24*i), -32, 32, 32, 0, 120, keyData, 0)
	}
	keyMoving = 0
	keyCounter = 0

	// The man.
	defineObject(22, rPositionX, rPositionY, 32, 32, 0, 0, manData, 0)
	spriteObject[22].sprite.U = 0
	spriteObject[22].sprite.V = 64

	goingUp = 1
	jumping = still
	maxBottom = rMaxBottom
	onPlat, fall = 0, 0
	changeFrame = 0
	frameCounter = 3
	spriteObject[22].moving = false
	spriteObject[22].collide.H = int16(spriteObject[22].sprite.H) - 6

	// "Level one", and "game paused".
	defineObject(23, 96, 104, 128, 32, 0, 108, readyData, 0)
	displayLevel = 1
	defineObject(24, 84, 104, 152, 32, 0, 0, pauseData, 0)

	setupWorld()
}

// The world's layout: where the platforms, bones, cacti and keys are
// along the level, from the original's world.h.
const (
	totalPlatforms = 25
	totalBones     = 64
	totalCactus    = 65
	totalKeys      = 5
)

var (
	platxLut = [totalPlatforms]int32{192, 300, 376, 440, 538, 762, 864, 944, 1048, 1152,
		1256, 1360, 1416, 1708, 1796, 2512, 2688, 2864, 3024, 3224,
		3304, 3378, 3560, 3670, 3680}
	platyLut = [totalPlatforms]int32{160, 128, 142, 96, 110, 124, 142, 100, 153, 121,
		89, 57, 121, 121, 130, 148, 148, 148, 148, 121,
		148, 110, 110, 80, 155}

	bonexLut = [totalBones]int32{64, 92, 120, 192, 224, 300, 332, 300, 328, 356, 384, 412,
		440, 440, 472, 538, 570, 602, 634, 666, 698, 944, 976,
		810, 842, 1048, 1080, 1152, 1184, 1256, 1288, 1360, 1392,
		1708, 1740, 1796, 1828, 2000, 2032, 2064, 2128, 2160, 2192,
		2256, 2288, 2320, 2384, 2416, 2448, 2512, 2544, 2688, 2720,
		2864, 2896, 3224, 3256, 3384, 3416, 3528, 3560, 3592,
		3808, 3840}
	boneyLut = [totalBones]int32{165, 165, 165, 96, 96, 64, 64, 165, 165, 165, 165, 165,
		165, 32, 32, 60, 60, 60, 60, 60, 60, 165, 165,
		165, 165, 121, 121, 89, 89, 57, 57, 25, 25, 89, 89, 66, 66,
		165, 165, 165, 165, 165, 165, 165, 165, 165, 165, 165, 165,
		116, 116, 116, 116, 116, 116, 89, 89, 165, 165, 165, 165, 165,
		165, 165}

	cactusxLut = [totalCactus]int32{148, 268, 392, 472, 506, 538, 570, 602, 634, 666,
		698, 730, 762, 880, 976, 1008, 1120, 1208, 1240, 1304,
		1336, 1368, 1400, 1432, 1464, 1496, 1528, 1560, 1592, 1624,
		1656, 1688, 1720, 1796, 1828, 1772, 1968, 2096, 2224, 2352,
		2480, 2580, 2616, 2652, 2756, 2792, 2828, 2928, 2960, 2992,
		3024, 3056, 3088, 3120, 3224, 3256, 3304, 3410, 3474, 3560,
		3648, 3712, 3702, 3880, 3996}
	cactusyLut = [totalCactus]int32{153, 153, 110, 148, 150, 149, 150, 148, 148, 149,
		148, 150, 148, 162, 68, 153, 153, 148, 148, 148,
		148, 148, 148, 150, 148, 151, 148, 148, 153, 148,
		148, 148, 148, 104, 104, 148, 153, 153, 153, 153,
		153, 150, 148, 150, 150, 148, 150, 153, 151, 154,
		162, 162, 162, 162, 153, 153, 116, 78, 150, 78,
		162, 123, 48, 150, 149}

	keyxLut = [totalKeys]int32{616, 944, 1424, 3090, 3752}
	keyyLut = [totalKeys]int32{24, 68, 89, 50, 148}
)

// setupWorld puts the world's objects in place: the dog at the end, the
// platforms, five of them moving, the bones and the keys, which are only
// put back for a new game, and the cacti.
func setupWorld() {
	worldArray[0].globalPos = maxPoint + screenWidth - 18
	worldArray[0].globalY = 149
	worldArray[0].typ = spriteObject[50]
	worldArray[0].display = true
	worldArray[0].disc = dog
	defineObject(50, worldArray[0].globalPos, worldArray[0].globalY, 18, 32, 0, 0, dogData, 0)

	for idx := 1; idx < totalPlatforms+1; idx++ {
		w := &worldArray[idx]
		w.globalPos = platxLut[idx-1]
		w.globalY = platyLut[idx-1]
		w.typ = spriteObject[50+idx]
		w.display = true
		w.disc = platform
		defineObject(50+idx, w.globalPos, w.globalY, 64, 16, 0, 92, platData, 0)
	}
	worldArray[5].moving = true
	worldArray[13].moving = true
	worldArray[19].moving = true
	worldArray[24].moving = true
	worldArray[152].moving = true

	if dead != 0 {
		for idx := totalPlatforms + 1; idx < totalPlatforms+1+totalBones; idx++ {
			w := &worldArray[idx]
			w.globalPos = bonexLut[idx-(totalPlatforms+1)]
			w.globalY = boneyLut[idx-(totalPlatforms+1)]
			w.typ = spriteObject[50+idx]
			w.display = true
			w.disc = bone
			defineObject(50+idx, w.globalPos, w.globalY, 28, 16, 136, 0, sboneData, 0)
		}
		for idx := 200; idx < 200+totalKeys; idx++ {
			w := &worldArray[idx]
			w.globalPos = keyxLut[idx-200]
			w.globalY = keyyLut[idx-200]
			w.typ = spriteObject[50+idx]
			w.display = true
			w.disc = key
			defineObject(50+idx, w.globalPos, w.globalY, 32, 32, 0, 120, keyData, 0)
		}
		dead = 0
	}

	first := totalPlatforms + 1 + totalBones
	for idx := first; idx < first+totalCactus; idx++ {
		w := &worldArray[idx]
		w.globalPos = cactusxLut[idx-first]
		w.globalY = cactusyLut[idx-first]
		w.typ = spriteObject[50+idx]
		w.display = true
		w.disc = cactus
		defineObject(50+idx, w.globalPos, w.globalY, 32, 32, 0, 0, cactusData, 0)
		o := spriteObject[50+idx]
		o.collide.X += 4
		o.collide.Y += 4
		o.collide.W -= 4
		o.collide.H -= 4
	}
}

// displayFrontEnd shows the title: the scenery, the logo spinning in,
// the message scrolling along the bottom, and two bones to choose
// between, About and Play, the chosen one swelling and shrinking.
func displayFrontEnd() {
	var play, offset int
	if alreadyRun == 0 {
		play, offset = 0, 0
	} else {
		play, offset = 1, 1
	}
	alreadyRun = 1

	for idx := range 35 {
		spriteObject[idx] = getNextFreeObject()
		clearObject(spriteObject[idx])
		spriteObject[idx].alive = true
	}

	defineObject(0, 0, 0, 160, 75, 0, 0, skyData, 0)
	defineObject(1, 160, 0, 160, 75, 32, 0, skyData, 80)
	defineObject(2, 0, 30, 160, 115, 0, 0, backData, 0)
	defineObject(3, 160, 30, 160, 115, 32, 0, backData, 80)
	defineObject(4, 0, 70, 160, 120, 0, 0, frontData, 0)
	defineObject(5, 160, 70, 160, 120, 32, 0, frontData, 80)
	defineObject(6, 0, 180, 160, 32, 0, 0, floorData, 0)
	defineObject(7, 160, 180, 160, 32, 0, 0, floorData, 0)

	// The About bone and the Play bone, scaled about their centers.
	defineObject(8, 39, 25, 58, 34, 58, 0, bothData, 0)
	spriteObject[8].sprite.Mx = 29
	spriteObject[8].sprite.My = 17
	defineObject(9, 281, 25, 58, 34, 0, 0, bothData, 0)
	spriteObject[9].sprite.Mx = 29
	spriteObject[9].sprite.My = 17

	// The logo, starting eight times its size, which a sprite's scale
	// cannot hold and wraps, as the original's does.
	defineObject(10, 154, 112, 166, 104, 0, 0, logoData, 0)
	spriteObject[10].sprite.Mx = 83
	spriteObject[10].sprite.My = 52
	big := int32(yaroze.ONE * 8)
	spriteObject[10].sprite.Scalex = int16(big)
	spriteObject[10].sprite.Scaley = int16(big)

	// The message, a sprite a character.
	messPos = 0
	for idx := range 22 {
		o := spriteObject[idx+11]
		o.width = 16
		o.height = 32
		o.xpos = int32(idx * 15)
		o.ypos = 208
		c := message[messPos]
		messPos++
		o.sprite.U = charXlut[c-32]
		o.sprite.V = charYlut[c-32]
		createObject(o, charData, 0)
	}
	messPos--

	// Scale in.
	for idx := big; idx > yaroze.ONE; idx -= yaroze.ONE / 5 {
		spriteObject[10].sprite.Scalex = int16(idx)
		spriteObject[10].sprite.Scaley = int16(idx)
		spriteObject[10].sprite.Rotate += yaroze.ONE * 10
		refreshScreen(bones)
	}

	defineObject(34, 42, 164, 236, 15, 0, 180, copyData, 0)

	scaleValue := int32(yaroze.ONE)
	scaleCount := 0
	for startGame := false; !startGame; {
		padd := yaroze.PadRead()

		if padd&yaroze.PADLright != 0 && play == 0 {
			play = 1
		}
		if padd&yaroze.PADLleft != 0 && play != 0 {
			play = 0
		}
		if padd&yaroze.PADRdown != 0 && play == 0 {
			gameAbout()
		}
		if padd&yaroze.PADRdown != 0 && play != 0 {
			startGame = true
		}

		// The chosen bone swells and shrinks.
		scaleCount = (scaleCount + 1) % 25
		if scaleCount < 12 {
			scaleValue += yaroze.ONE / 32
		} else if scaleCount > 12 {
			scaleValue -= yaroze.ONE / 32
		}
		if play != 0 && scaleCount == 0 {
			offset = 1
		} else if play == 0 && scaleCount == 0 {
			offset = 0
		}
		spriteObject[8+offset].sprite.Scalex = int16(scaleValue)
		spriteObject[8+offset].sprite.Scaley = int16(scaleValue)

		refreshScreen(bones)
	}

	for idx := range 35 {
		clearObject(spriteObject[idx])
	}
}

// gameAbout scales the about box in over the title, waits for Select,
// and scales it out again.
func gameAbout() {
	spriteObject[33] = getNextFreeObject()
	clearObject(spriteObject[33])
	spriteObject[33].alive = true

	defineObject(33, 160, 120, 256, 184, 0, 0, textData, 0)
	spriteObject[33].sprite.Mx = 128
	spriteObject[33].sprite.My = 92
	spriteObject[33].sprite.Scalex = 0
	spriteObject[33].sprite.Scaley = 0

	overlay = yaroze.GsBOXF{Attribute: 1 << 30, X: 160, Y: 120, W: 0, H: 0, R: 0, G: 0, B: 0}

	for range 21 {
		overlay.X -= 7
		overlay.Y -= 5
		overlay.W += 14
		overlay.H += 10
		spriteObject[33].sprite.Scaley += yaroze.ONE / 21
		spriteObject[33].sprite.Scalex = spriteObject[33].sprite.Scaley
		refreshScreen(aboutBox)
	}

	for yaroze.PadRead()&yaroze.PADselect == 0 {
		refreshScreen(aboutBox)
	}

	for range 19 {
		refreshScreen(aboutBox)
		overlay.X += 7
		overlay.Y += 5
		overlay.W -= 14
		overlay.H -= 10
		spriteObject[33].sprite.Scaley -= yaroze.ONE / 21
		spriteObject[33].sprite.Scalex = spriteObject[33].sprite.Scaley
	}

	clearObject(spriteObject[33])
}

// moveRight moves the world right, which is the man walking left: he
// walks across the screen at its ends, and otherwise the scenery scrolls
// past him, the floor fastest and the far mountains slowest.
func moveRight() {
	man := spriteObject[22]
	if (man.sprite.X < 124 && whereYouAre == 0) || (man.sprite.X > 123 && whereYouAre >= maxPoint) {
		if man.sprite.X > 0 {
			man.sprite.X -= 3
		}
		checkUnderAgain()
		return
	}
	if whereYouAre == 0 {
		return
	}
	checkUnderAgain()
	whereYouAre -= 4
	if spriteObject[6].sprite.X != 0 {
		for i := 6; i <= 9; i++ {
			spriteObject[i].sprite.X += 2
		}
	} else {
		spriteObject[6].sprite.X = -318
		spriteObject[7].sprite.X = -158
		spriteObject[8].sprite.X = 2
		spriteObject[9].sprite.X = 162
	}
	if spriteObject[10].sprite.X != 0 {
		for i := 10; i <= 12; i++ {
			spriteObject[i].sprite.X += 4
		}
	} else {
		spriteObject[10].sprite.X = -156
		spriteObject[11].sprite.X = 4
		spriteObject[12].sprite.X = 164
	}
	if spriteObject[2].sprite.X != 0 {
		for i := 2; i <= 5; i++ {
			spriteObject[i].sprite.X += 1
		}
	} else {
		spriteObject[2].sprite.X = -319
		spriteObject[3].sprite.X = -159
		spriteObject[4].sprite.X = 1
		spriteObject[5].sprite.X = 161
	}
}

// moveLeft moves the world left, which is the man walking right. Reaching
// the dog with all five keys ends the game.
func moveLeft() {
	man := spriteObject[22]
	if (man.sprite.X != 123 && whereYouAre < maxPoint) || (man.sprite.X >= 123 && whereYouAre >= maxPoint) {
		if man.sprite.X > 266 && noKeys == 5 {
			endOfGame = 1
		} else if man.sprite.X < 272 {
			man.sprite.X += 3
		}
		checkUnderAgain()
		return
	}
	if whereYouAre >= maxPoint {
		return
	}
	checkUnderAgain()
	whereYouAre += 4
	if spriteObject[8].sprite.X != 0 {
		for i := 6; i <= 9; i++ {
			spriteObject[i].sprite.X -= 2
		}
	} else {
		spriteObject[6].sprite.X = -2
		spriteObject[7].sprite.X = 158
		spriteObject[8].sprite.X = 318
		spriteObject[9].sprite.X = 478
	}
	if spriteObject[12].sprite.X != 160 {
		for i := 10; i <= 12; i++ {
			spriteObject[i].sprite.X -= 4
		}
	} else {
		spriteObject[10].sprite.X = -4
		spriteObject[11].sprite.X = 156
		spriteObject[12].sprite.X = 316
	}
	if spriteObject[4].sprite.X != 0 {
		for i := 2; i <= 5; i++ {
			spriteObject[i].sprite.X -= 1
		}
	} else {
		spriteObject[2].sprite.X = -1
		spriteObject[3].sprite.X = 159
		spriteObject[4].sprite.X = 319
		spriteObject[5].sprite.X = 479
	}
}

// scrollText moves the message's characters along three pixels, and when
// they have gone a character's width puts them back and shifts the
// characters down one, with the next from the message at the end.
func scrollText() {
	for idx := range 22 {
		spriteObject[idx+11].sprite.X -= 3
	}
	if spriteObject[12].sprite.X < 1 {
		for idx := range 22 {
			spriteObject[idx+11].sprite.X = int16(idx * 15)
			spriteObject[idx+11].sprite.U = spriteObject[idx+12].sprite.U
			spriteObject[idx+11].sprite.V = spriteObject[idx+12].sprite.V
		}
		messPos++
		if messPos == len(message) {
			messPos = 0
		}
		c := message[messPos]
		spriteObject[32].sprite.U = charXlut[c-32]
		spriteObject[32].sprite.V = charYlut[c-32]
	}
}

// playLevelOne plays the level until the man gives up, loses a life or
// frees the dog.
func playLevelOne() {
	var timer uint32
	var rotateTitle int32
	quitGame, deadSpinning = 0, 0

	for quitGame == 0 {
		if deadSpinning == 0 {
			if checkPad() == 99 {
				gamePaused = 1
				song.Pause()
			}
			for gamePaused != 0 {
				if checkPad() == 98 {
					quitGame = 1
					gamePaused = 0
					dead = 1
					stopSound()
				}
				if displayLevel != 0 {
					displayLevel = 0
					clearObject(spriteObject[23])
				}
				refreshScreen(paused)
			}
		}

		// Freeing the dog ends the game the same way as losing the last
		// life, with a different picture.
		if endOfGame != 0 {
			life = 0
			alreadyRun = 0
		}

		if life == 0 {
			pause(30)
			quitGame = 1
			dead = 1

			if endOfGame == 0 {
				// Grab the screen into the frame buffer beside it.
				grabRect := yaroze.RECT{X: 0, Y: 0, W: 320, H: 240}
				if activeBuff != 0 {
					grabRect.Y = 240
				}
				yaroze.MoveImage(&grabRect, 320, 0)
				yaroze.DrawSync(0)
			}

			for idx := range 30 {
				clearObject(spriteObject[idx])
			}

			if endOfGame == 0 {
				// Two sprites showing the grab's halves, to spin and
				// shrink about the screen's center.
				spriteObject[0] = getNextFreeObject()
				clearObject(spriteObject[0])
				spriteObject[0].alive = true
				s := &spriteObject[0].sprite
				s.Attribute = sixteenBitOn
				s.X, s.Y = 160, 120
				s.W, s.H = 160, 240
				s.Mx, s.My = 160, 120
				s.Tpage = 5

				spriteObject[1] = getNextFreeObject()
				clearObject(spriteObject[1])
				spriteObject[1].alive = true
				s = &spriteObject[1].sprite
				s.Attribute = sixteenBitOn
				s.X, s.Y = 160, 120
				s.W, s.H = 160, 240
				s.Mx, s.My = 0, 120
				s.U, s.V = 32, 0
				s.Tpage = 7
			}

			if endOfGame == 0 {
				defineObject(2, 96, 104, 128, 32, 0, 0, overData, 0)
			} else {
				defineObject(2, 32, 28, 256, 184, 0, 0, endData, 0)
			}

			left, right := int32(127), int32(127)
			for shrink := int32(yaroze.ONE); shrink > -(yaroze.ONE / 56); shrink -= yaroze.ONE / 64 {
				right -= 2
				left = right
				if endOfGame == 0 {
					rotateTitle = (rotateTitle + yaroze.ONE*5) % (yaroze.ONE * 360)
					spriteObject[0].sprite.Rotate = rotateTitle
					spriteObject[1].sprite.Rotate = rotateTitle
					spriteObject[0].sprite.Scalex, spriteObject[0].sprite.Scaley = int16(shrink), int16(shrink)
					spriteObject[1].sprite.Scalex, spriteObject[1].sprite.Scaley = int16(shrink), int16(shrink)
				}
				refreshScreen(startGame)
				yaroze.SsSetMVol(int16(left), int16(right))
			}

			if endOfGame == 0 {
				clearObject(spriteObject[0])
				clearObject(spriteObject[1])
				for range 60 {
					refreshScreen(deadGeezer)
				}
			} else {
				for range 1200 {
					if checkPad() == 99 {
						break
					}
					refreshScreen(deadGeezer)
				}
			}

			stopSound()
		}

		if dead != 0 {
			continue
		}

		timer++
		if timer > 75 {
			displayLevel = 0
		}

		activeBuff = yaroze.GsGetActiveBuff()
		yaroze.GsSetWorkBase(yaroze.Packets(&gpuPacketArea[activeBuff][0]))
		ot := &worldOT[activeBuff]
		yaroze.GsClearOt(0, 0, ot)

		sortFast(0, ot, 5) // the sky
		sortFast(1, ot, 5)
		for i := 2; i <= 5; i++ { // the back mountains
			sortFast(i, ot, 4)
		}
		for i := 6; i <= 9; i++ { // the front mountains
			sortFast(i, ot, 3)
		}
		for i := 10; i <= 12; i++ { // the floor
			sortFast(i, ot, 1)
		}
		sortFast(13, ot, 0) // the small logo
		sortFast(19, ot, 0) // the big bone
		if displayLevel != 0 {
			sortFast(23, ot, 0)
		}

		// The world's objects, the moving platforms carrying the man
		// along with them.
		for idx := range maxLevelOneObj {
			w := &worldArray[idx]
			if w.moving && w.moveDir {
				w.xOffset += 4
				if spriteObject[22].moving && jumping == still && int32(idx) == whichPlat {
					moveLeft()
				}
				if w.xOffset > 128 {
					w.moveDir = false
				}
			} else if w.moving && !w.moveDir {
				w.xOffset -= 4
				if spriteObject[22].moving && jumping == still && int32(idx) == whichPlat {
					moveRight()
				}
				if w.xOffset < 1 {
					w.moveDir = true
				}
			}
			scrPos = w.globalPos + w.xOffset - whereYouAre
			if w.display && scrPos <= screenWidth && scrPos+w.typ.width >= 0 {
				w.onScr = true
				w.typ.sprite.X = int16(scrPos)
				w.typ.collide.X = int16(scrPos)
				if w.disc != platform {
					if deadSpinning == 0 {
						checkBones(50 + idx)
					}
					if w.disc == bone {
						sortFast(50+idx, ot, 0)
					} else {
						sortFast(50+idx, ot, 2)
					}
				} else {
					sortFast(50+idx, ot, 1)
				}
			} else {
				w.onScr = false
			}
		}

		// The count of bones.
		spriteObject[20].sprite.U = charXlut[noBones/10+16]
		spriteObject[20].sprite.V = charYlut[noBones/10+16] + 108
		spriteObject[21].sprite.U = charXlut[noBones%10+16]
		spriteObject[21].sprite.V = charYlut[noBones%10+16] + 108
		sortFast(20, ot, 0)
		sortFast(21, ot, 0)

		if keyMoving != 0 {
			moveKeys()
		}
		for idx := range noKeys {
			sortFast(26+int(idx), ot, 2)
		}

		// The man: his jump or fall, and his walk, a frame every other
		// step.
		if deadSpinning == 0 {
			if jumping != still && displayLevel == 0 {
				jump()
			} else if jumping != still && displayLevel != 0 {
				jumping = still
			}
			if fall != 0 {
				falling()
			}
			if changeFrame > 1 {
				changeFrame = 0
				man := &spriteObject[22].sprite
				switch frameCounter {
				case 1, 7:
					man.V = 32
					frameCounter++
				case 2, 6:
					man.V = 64
					frameCounter++
				case 3, 5:
					man.V = 96
					frameCounter++
				case 4:
					man.V = 128
					frameCounter++
				case 8:
					man.V = 0
					frameCounter = 1
				}
			}
		}
		sortFast(22, ot, 0)

		// The lives, which spin when one is gained or lost. Losing one
		// past the middle of the level restarts from there.
		if spinning != 0 || deadSpinning != 0 {
			for idx := range life {
				s := &spriteObject[14+int(idx)].sprite
				s.Rotate += yaroze.ONE * 10
				if s.Rotate >= yaroze.ONE*350 {
					s.Rotate = yaroze.ONE
					spinning = 0
					if whereYouAre > 1700 {
						restart = 1584
						rPositionX = 123
						rPositionY = 89
						rMaxBottom = 89
					}
					if deadSpinning != 0 {
						quitGame = 1
					}
				}
				sortSprite(14+int(idx), ot, 0)
			}
		} else {
			for idx := range life {
				sortSprite(14+int(idx), ot, 0)
			}
		}

		yaroze.DrawSync(0)
		yaroze.VSync(0)
		yaroze.GsSwapDispBuff()
		yaroze.GsSortClear(backR, backG, backB, ot)
		yaroze.GsDrawOt(ot)
		frameNumber++
	}
}

// jump moves the man through a jump: up four pixels a frame until he is
// 55 above where he started, then down until he lands, which is on a
// platform if one is under him, walking along if the jump is sideways.
// The original works out a speed for each frame and then resets it, so
// there is no gravity.
func jump() {
	man := spriteObject[22]
	const objVel = 8

	switch jumping {
	case jumpUp:
		if goingUp != 0 {
			onPlat = 0
			man.sprite.Y -= objVel >> 1
			if man.sprite.Y < int16(maxBottom-55) {
				goingUp = 0
			}
		} else {
			checkUnder()
			man.sprite.Y += objVel >> 1
			if man.sprite.Y >= int16(maxBottom) {
				land()
			}
		}
	case jumpLeft:
		if goingUp != 0 {
			onPlat = 0
			man.sprite.Y -= objVel >> 1
			if man.sprite.Y < int16(maxBottom-55) {
				goingUp = 0
			}
			moveRight()
		} else {
			checkUnder()
			man.sprite.Y += objVel >> 1
			if man.sprite.Y >= int16(maxBottom) {
				land()
			} else {
				moveRight()
			}
		}
	case jumpRight:
		if goingUp != 0 {
			onPlat = 0
			man.sprite.Y -= objVel >> 1
			if man.sprite.Y < int16(maxBottom-55) {
				goingUp = 0
			}
			moveLeft()
		} else {
			checkUnder()
			man.sprite.Y += objVel >> 1
			if man.sprite.Y >= int16(maxBottom) {
				land()
			} else {
				moveLeft()
			}
		}
	}
}

// land ends a jump where the man has come down, unless there is nothing
// under him and he goes on falling.
func land() {
	checkUnderAgain()
	goingUp = 1
	jumping = still
	if fall == 0 {
		spriteObject[22].sprite.Y = int16(maxBottom)
	}
}

// checkBones checks the man against one world object: a bone is
// collected, with a life every fifty while there are fewer than five; a
// cactus costs a life; a key is collected and the keys are shown.
func checkBones(thing int) {
	man := spriteObject[22]
	man.collide.W = 16
	man.collide.X = man.sprite.X + 8
	man.collide.Y = man.sprite.Y + 3

	w := &worldArray[thing-50]
	if !rectanglesOverlap(&man.collide, &spriteObject[thing].collide) {
		return
	}
	switch w.disc {
	case bone:
		w.display = false
		if (noBones < 50 && life < 5) || (noBones < 25 && life == 5) {
			noBones++
		}
		if noBones == 50 && life < 5 {
			life++
			spinning = 1
			noBones = 0
		}
	case cactus:
		life--
		deadSpinning = 1
	case key:
		noKeys++
		w.display = false
		if keyMoving == 0 {
			keyMoving = 1
		} else {
			for idx := range 5 {
				spriteObject[26+idx].sprite.Y = -32
			}
			keyCounter = 0
			keyMoving = 1
		}
	}
}

// checkUnder looks for a platform under the man, by a narrow rectangle
// under his feet, and settles him on it, or on a new one with a short
// drop. Finding none under him, he falls.
func checkUnder() {
	man := spriteObject[22]
	man.collide.W = 8
	if man.sprite.U == 32 {
		man.collide.X = man.sprite.X + 15
	} else {
		man.collide.X = man.sprite.X + 9
	}
	man.collide.Y = man.sprite.Y + 3

	for idx := int32(0); idx < maxObjects; idx++ {
		w := &worldArray[idx]
		if w.disc != platform || !w.onScr {
			continue
		}
		plat := spriteObject[idx+50]
		if xOverlap(&man.collide, &plat.collide) && man.sprite.Y+int16(man.sprite.H) <= plat.sprite.Y {
			if whichPlat == -1 {
				whichPlat = idx
			}
			if whichPlat != idx {
				if jumping != still && goingUp == 0 {
					maxBottom = w.globalY - int32(man.sprite.H)
				}
				man.moving = w.moving
				if jumping == still {
					jumping = jumpUp
					goingUp = 0
				}
				whichPlat = idx
			} else {
				onPlat = 1
				if jumping != still && goingUp == 0 {
					maxBottom = w.globalY - int32(man.sprite.H)
				}
				man.moving = w.moving
			}
			return
		} else if onPlat == 0 {
			fall = 1
		}
	}
}

// checkUnderAgain checks whether the man has walked off his platform,
// by looking for one a little above where he is.
func checkUnderAgain() {
	man := spriteObject[22]
	onPlat = 0
	man.sprite.Y -= 10
	checkUnder()
	if (onPlat == 0 && goingUp == 0) || (onPlat == 0 && jumping == still) {
		maxBottom = 148
		fall = 1
	}
	man.sprite.Y += 10
}

// falling drops the man six pixels a frame until he lands.
func falling() {
	man := spriteObject[22]
	if jumping != still {
		return
	}
	man.sprite.Y += 6
	if onPlat == 0 {
		man.moving = false
	}
	if man.sprite.Y >= int16(maxBottom) {
		goingUp = 1
		jumping = still
		man.sprite.Y = int16(maxBottom)
		fall = 0
	}
}

func rectanglesOverlap(first, second *yaroze.RECT) bool {
	return xOverlap(first, second) && yOverlap(first, second)
}

func xOverlap(first, second *yaroze.RECT) bool {
	if first.X >= second.X {
		return first.X < second.X+second.W
	}
	return first.X+first.W > second.X
}

func yOverlap(first, second *yaroze.RECT) bool {
	if first.Y >= second.Y {
		return first.Y < second.Y+second.H
	}
	return first.Y+first.H > second.Y
}

// pause waits duration frames.
func pause(duration int) {
	for range duration {
		yaroze.VSync(0)
	}
}

// The sound, which the author says was taken from Sony's check sample.

func initSound() {
	bank, _ = yaroze.OpenBank(vh.Data(), vb.Data())
	song, _ = bank.Sequence(seqData[:])
}

func playSEQ() {
	if !song.Valid() {
		return
	}
	yaroze.SsSetMVol(mvol, mvol)
	song.SetVolume(svol, svol)
	song.Play(yaroze.SSPLAY_INFINITY)
}

func stopSound() {
	if song.Valid() {
		song.Stop()
		yaroze.VSync(0)
		yaroze.VSync(0)
		song.Close()
	}
	bank.Close()
}

// moveKeys brings the keys found down from above the screen, holds them
// a hundred frames, and takes them back up.
func moveKeys() {
	switch keyCounter {
	case 0:
		for idx := range noKeys {
			spriteObject[26+int(idx)].sprite.Y += 2
		}
	case 100:
		for idx := range noKeys {
			spriteObject[26+int(idx)].sprite.Y -= 2
		}
	}
	if spriteObject[26].sprite.Y == 0 {
		keyCounter++
	} else if spriteObject[26].sprite.Y == -32 && keyCounter == 100 {
		keyMoving = 0
		keyCounter = 0
	}
}
