//go:build yaroze

// Package lpge holds what L. Evans's Net Yaroze tutorials share, from the
// modules each of them carries a copy of: object.c, which keeps every object
// in the game in one array and ties each to a model or a sprite and to a
// coordinate system; tmd.c, which links an object to a model in a TMD; and
// sincos.c, the trigonometry by table. They are signed LPGE.
//
// The copies differ between the tutorials only in how much the object
// structure holds; this is the largest, the tunnel's. The assertions are
// left out, and so are the functions no port calls.
//
// Like the examples, nothing here allocates.
package lpge

import (
	"unsafe"

	"go.chrisrx.dev/psx/yaroze"
)

// None marks an ID, type or mode as not set, and as an object's super
// coordinate system, asks for one of its own. TheWorld asks for the world's.
const (
	None     = 1001
	TheWorld = 1002
)

// How an object is drawn.
const (
	TMD    = 0
	Sprite = 1
)

// Kinds of shot.
const (
	FireSprite       = 0
	TetrahedronModel = 1
)

const (
	MaxFollowers = 24
	MaxObjects   = 1024
)

// A Handler is an ObjectHandler: one object in the game, drawn either as a
// TMD model through its GsDOBJ2 or as a sprite.
type Handler struct {
	ID      int32
	Alive   bool
	Type    int32
	SubType int32
	Which   int32

	DisplayFlag int32 // TMD or Sprite

	ImageInfo      *yaroze.GsIMAGE
	Sprite         yaroze.GsSPRITE
	ScaleX, ScaleY int32

	Handler      yaroze.GsDOBJ2
	ModelAddress uintptr
	WhichModel   int32

	ScalingFlag   bool
	ScalingVector yaroze.VECTOR

	MovementTypeFlag int32

	Position             yaroze.VECTOR
	Velocity             yaroze.VECTOR
	MovementMomentumFlag bool
	MovementSpeed        int32

	Rotate               yaroze.SVECTOR
	Twist                yaroze.VECTOR
	RotationMomentumFlag bool
	RotationSpeed        int32

	Coord                   yaroze.GsCOORDINATE2
	SuperCoordinateObjectID int32
	Matrix                  yaroze.MATRIX // relation to the super coordinate system

	InOrbit        bool
	CentralBodyID  int32
	Angle          int32
	AngleIncrement int32
	WhichWay       int32
	Radius         int32
	FirstVector    yaroze.VECTOR
	SecondVector   yaroze.VECTOR

	SpecialMovement int32

	LifeTime         int32
	SpecialTimeLimit int32
	SecondPeriod     int32
	ThirdPeriod      int32

	InitialHealth int32
	CurrentHealth int32

	StrategyFlag             int32
	CurrentActionFlag        int32
	NumberFramesOfThisAction int32

	MeritRating int32

	Allegiance int32

	CanFire             bool
	FiringRate          int32
	FramesSinceLastFire int32
	ShotSpeed           int32
	ShotType            int32

	ActionState int32

	ObjectIDList [12]int32

	LeaderFlag      bool
	NumberFollowers int32
	ListOfFollowers [MaxFollowers]int32
	LeaderID        int32
}

// Objects is ObjectArray, every object in the game by ID.
var Objects [MaxObjects]*Handler

// InitialiseObjectClass empties Objects.
func InitialiseObjectClass() {
	for i := range Objects {
		Objects[i] = nil
	}
}

// Register puts an object into the first free place in Objects, which is
// its ID from then on. It returns false if there is none.
func Register(o *Handler) bool {
	for i := range Objects {
		if Objects[i] == nil {
			Objects[i] = o
			o.ID = int32(i)
			return true
		}
	}
	return false
}

// LinkAllObjectsToModelsOrSprites links each living object to its model or
// its sprite. Every object must be registered first.
func LinkAllObjectsToModelsOrSprites() {
	for _, o := range &Objects {
		if o == nil || !o.Alive {
			continue
		}
		switch o.DisplayFlag {
		case TMD:
			LinkObjectHandlerToTmdObject(&o.Handler, o.WhichModel, o.ModelAddress)
		case Sprite:
			LinkObjectToSprite(o)
		}
	}
}

// LinkAllObjectsToTheirCoordinateSystems points each living object's
// GsDOBJ2 at its coordinate system: its own, the world's, or another
// object's.
func LinkAllObjectsToTheirCoordinateSystems() {
	for _, o := range &Objects {
		if o == nil || !o.Alive {
			continue
		}
		switch o.SuperCoordinateObjectID {
		case None:
			o.Handler.Coord2 = &o.Coord
		case TheWorld:
			o.Handler.Coord2 = nil
		default:
			o.Handler.Coord2 = &Objects[o.SuperCoordinateObjectID].Coord
		}
	}
}

// BringToLife makes an object alive, of a type, with a model.
func (o *Handler) BringToLife(typ int32, modelAddress uintptr, whichModel int32, superCoordinateObjectID int32) {
	o.Alive = true
	o.Type = typ
	o.ModelAddress = modelAddress
	o.WhichModel = whichModel
	o.SuperCoordinateObjectID = superCoordinateObjectID
}

// Init is InitSingleObject: every field to its starting value, with a
// coordinate system of its own.
func (o *Handler) Init() {
	o.ID = None
	o.Alive = false
	o.Type = None
	o.SubType = None
	o.Which = None
	o.DisplayFlag = TMD
	o.ImageInfo = nil
	InitGsSprite(&o.Sprite)
	o.ScaleX, o.ScaleY = yaroze.ONE, yaroze.ONE
	o.ModelAddress = 0
	o.WhichModel = None
	o.ScalingFlag = false
	o.ScalingVector = yaroze.VECTOR{Vx: yaroze.ONE, Vy: yaroze.ONE, Vz: yaroze.ONE}
	o.MovementTypeFlag = 1 // dynamic

	o.Position = yaroze.VECTOR{}
	o.Velocity = yaroze.VECTOR{}
	o.MovementMomentumFlag = false
	o.MovementSpeed = 0
	o.Rotate = yaroze.SVECTOR{}
	o.Twist = yaroze.VECTOR{}
	o.RotationMomentumFlag = false
	o.RotationSpeed = 0

	yaroze.GsInitCoordinate2(nil, &o.Coord)
	o.Handler.Coord2 = &o.Coord
	o.SuperCoordinateObjectID = None
	o.Matrix = identity

	o.InOrbit = false
	o.CentralBodyID = None
	o.Angle, o.AngleIncrement = 0, 0
	o.WhichWay = None
	o.Radius = 0
	o.FirstVector, o.SecondVector = yaroze.VECTOR{}, yaroze.VECTOR{}
	o.SpecialMovement = None
	o.LifeTime, o.SpecialTimeLimit = 0, 0
	o.SecondPeriod, o.ThirdPeriod = 0, 0

	o.InitialHealth, o.CurrentHealth = 0, 0
	o.StrategyFlag, o.CurrentActionFlag = None, None
	o.NumberFramesOfThisAction = 0
	o.MeritRating = 0
	o.Allegiance = None
	o.CanFire = false
	o.FiringRate, o.FramesSinceLastFire = 0, 0
	o.ShotSpeed = 0
	o.ShotType = -1
	o.ActionState = -1
	o.clearLists()
}

// Revive is ReviveObject: alive again, at the origin, still and unturned,
// with its health restored. Its coordinate system is left as it was.
func (o *Handler) Revive() {
	o.Alive = true
	o.Position = yaroze.VECTOR{}
	o.Velocity = yaroze.VECTOR{}
	o.Rotate = yaroze.SVECTOR{}
	o.Twist = yaroze.VECTOR{}
	o.Matrix = identity
	o.InOrbit = false
	o.CentralBodyID = None
	o.Angle, o.AngleIncrement = 0, 0
	o.WhichWay = None
	o.Radius = 0
	o.FirstVector, o.SecondVector = yaroze.VECTOR{}, yaroze.VECTOR{}
	o.SpecialMovement = None
	o.LifeTime, o.SpecialTimeLimit = 0, 0
	o.SecondPeriod, o.ThirdPeriod = 0, 0
	o.CurrentHealth = o.InitialHealth
	o.StrategyFlag, o.CurrentActionFlag = None, None
	o.NumberFramesOfThisAction = 0
	o.MeritRating = 0
	o.ActionState = -1
	o.clearLists()
}

func (o *Handler) clearLists() {
	for i := range o.ObjectIDList {
		o.ObjectIDList[i] = -1
	}
	o.LeaderFlag = false
	o.NumberFollowers = 0
	for i := range o.ListOfFollowers {
		o.ListOfFollowers[i] = -1
	}
	o.LeaderID = -1
}

var identity = yaroze.MATRIX{M: [3][3]int16{
	{yaroze.ONE, 0, 0},
	{0, yaroze.ONE, 0},
	{0, 0, yaroze.ONE},
}}

// SetScaling sets how much SortSize scales the object along each axis, ONE
// being its own size.
func (o *Handler) SetScaling(x, y, z int32) {
	o.ScalingFlag = true
	o.ScalingVector = yaroze.VECTOR{Vx: x, Vy: y, Vz: z}
}

// SortSize is SortObjectSize: it scales the object's coordinate system by
// its scaling, if it has any. Objects in coordinate systems under it are
// scaled along with it.
func (o *Handler) SortSize() {
	if !o.ScalingFlag {
		return
	}
	yaroze.ScaleMatrix(&o.Coord.Coord, &o.ScalingVector)
	o.Coord.Flg = 0
}

// InitGsSprite sets a sprite to nothing in particular, at full brightness
// and its own size.
func InitGsSprite(s *yaroze.GsSPRITE) {
	*s = yaroze.GsSPRITE{
		R: 128, G: 128, B: 128,
		Scalex: yaroze.ONE, Scaley: yaroze.ONE,
	}
}

// FindTopLeftOfTexturePage returns the texture page an image is in, by the
// page's top left corner in VRAM, and the image's place in it. Pages are
// taken to start every 320 pixels across, as the tutorials have them.
func FindTopLeftOfTexturePage(img *yaroze.GsIMAGE) (x, y, u, v int32) {
	x, y = int32(img.Px), int32(img.Py)
	for x%320 != 0 {
		x--
		u++
	}
	for y%256 != 0 {
		y--
		v++
	}
	return x, y, u, v
}

// Sprite attribute bits for the color depth of the texture.
const (
	sixteenBit = 1 << 25
	eightBit   = 1 << 24
)

// LinkSpriteToImageInfo sets a sprite up to show the whole of an image
// loaded into VRAM.
func LinkSpriteToImageInfo(s *yaroze.GsSPRITE, img *yaroze.GsIMAGE) {
	InitGsSprite(s)
	linkSprite(s, img)
}

// LinkObjectToSprite sets an object's sprite up to show its image.
func LinkObjectToSprite(o *Handler) {
	linkSprite(&o.Sprite, o.ImageInfo)
}

func linkSprite(s *yaroze.GsSPRITE, img *yaroze.GsIMAGE) {
	x, y, u, v := FindTopLeftOfTexturePage(img)
	var widthCompression, tpageType int32
	switch img.Pmode {
	case 0: // 16-bit
		s.Attribute |= sixteenBit
		widthCompression, tpageType = 1, 2
	case 8: // 4-bit
		widthCompression, tpageType = 4, 0
		s.Cx, s.Cy = img.Cx, img.Cy
	case 9: // 8-bit
		s.Attribute |= eightBit
		widthCompression, tpageType = 2, 1
		s.Cx, s.Cy = img.Cx, img.Cy
	}
	s.Tpage = yaroze.GetTPage(tpageType, 0, x, y)
	s.W = uint16(int32(img.Pw) * widthCompression)
	s.H = uint16(img.Ph)
	s.U = uint8(u)
	s.V = uint8(v)
}

// LinkObjectHandlerToTmdObject links a GsDOBJ2 to one of the objects in the
// TMD at tmdAddress, mapping its offsets to addresses first.
func LinkObjectHandlerToTmdObject(h *yaroze.GsDOBJ2, which int32, tmdAddress uintptr) {
	p := (*uint32)(unsafe.Pointer(tmdAddress + 4)) // past the file's ID word
	yaroze.GsMapModelingData(p)
	yaroze.GsLinkObject4(uint32(tmdAddress+12), h, which)
}

// ProperInitialiseTexture loads a TIM at address into VRAM, with its
// palette if it has one, and describes it in img.
func ProperInitialiseTexture(address uintptr, img *yaroze.GsIMAGE) {
	yaroze.GsGetTimInfo((*uint32)(unsafe.Pointer(address+4)), img)
	rect := yaroze.RECT{X: img.Px, Y: img.Py, W: int16(img.Pw), H: int16(img.Ph)}
	yaroze.LoadImage(&rect, img.Pixel)
	if img.Pmode>>3&1 != 0 {
		rect = yaroze.RECT{X: img.Cx, Y: img.Cy, W: int16(img.Cw), H: int16(img.Ch)}
		yaroze.LoadImage(&rect, img.Clut)
	}
	yaroze.DrawSync(0)
}
