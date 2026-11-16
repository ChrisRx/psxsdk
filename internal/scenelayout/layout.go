// Package scenelayout holds the numbers of a scene file, SCENE.DAT: its ID,
// the most of each thing it holds, the flags of a layer's cells, and the
// sizes of its records and the offsets into its header, in bytes. Package
// format/scene, which reads and writes the file on the host and whose
// comment describes it, and package scene, which reads it on the
// console, both take them from here, so the two cannot disagree.
//
// It imports nothing, so that a program which reads a scene links none
// of what the host's package needs.
package scenelayout

// ID is the first word of a scene file, "SCN1".
const ID = 0x314E4353

// The most of each a file holds, which are the sizes of package scene's
// arrays.
const (
	MaxTextures = 16
	MaxModels   = 16
	MaxObjects  = 64
	MaxSprites  = 128
	MaxLights   = 3
	MaxName     = 15 // bytes of an object's or a sprite's name
	MaxPath     = 31 // bytes of a disc path

	// A scene has at most MaxLayers layers. A layer's map holds at most
	// MaxMapCells cells, its texture at most MaxCells, and its cells are
	// CellSize square.
	MaxLayers   = 3
	MaxMapCells = 4096
	MaxCells    = 256
	CellSize    = 16
	EmptyCell   = 0xffff

	// A room is at most MaxRoomCells across and down, as package stage
	// has it, with at most MaxLinks links, one a digit, and MaxMarkers
	// markers.
	MaxRoomCells = 32
	MaxLinks     = 10
	MaxMarkers   = 64
)

// The flags the editor names for a layer's cells; the other four bits are
// the program's own.
const (
	Solid  = 1 << 0 // nothing passes through it
	OneWay = 1 << 1 // stood on from above, passed through from below
	Hurts  = 1 << 2 // hurts what touches it
	Pickup = 1 << 3 // taken by what touches it
)

// The sizes of the records, in bytes. Each is a multiple of four, since
// the file is words.
const (
	HeaderSize    = 0x68
	PathSize      = 32
	ObjectSize    = 64
	SpriteSize    = 64
	LayerSize     = 28 // before its map
	LayerNameSize = 20 // after its map: the name, the count of flags and a pad
	RoomSize      = 36 // before its cells
	LinkSize      = 36
	MarkerSize    = 28
	NameSize      = 16 // a name, NUL-padded
	LightSize     = 16
	ObjectsAt     = HeaderSize // plus the paths
)

// The offsets of the header's fields, in bytes.
const (
	ProjectionAt = 0x0C
	AmbientAt    = 0x10
	CameraAt     = 0x1C
	LightsAt     = 0x38
)
