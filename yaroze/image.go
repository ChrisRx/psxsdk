package yaroze

// An Image is a picture in video memory, loaded from a TIM file: its
// pixels, and its palette if it has one. A sprite shows it, or part of
// it.
type Image struct {
	depth   uint32 // 0 for 4 bits a pixel, 1 for 8, 2 for 16
	w, h    uint16 // in pixels
	texture Texture
	words   int // the file's length
}

// LoadTIM loads the TIM file in words, as a disc file is read, into video
// memory where its header says, and returns the image. The transfer is
// waited for.
func LoadTIM(words []uint32) Image { return loadTIM(words, false, 0, 0) }

// LoadTIMAt is LoadTIM with the pixels put at x, y in video memory
// rather than where the file says, for a file whose placement a
// program chooses for itself. The palette goes where the file says.
func LoadTIMAt(words []uint32, x, y int) Image { return loadTIM(words, true, x, y) }

// Width and Height are the image's size in pixels.
func (im Image) Width() int  { return int(im.w) }
func (im Image) Height() int { return int(im.h) }

// Len is the TIM file's length in words, for walking a file of TIMs one
// after another.
func (im Image) Len() int { return im.words }

// Video memory as rectangles, 16-bit units wide: 1024 by 512 of them,
// the display buffers among them.

// CopyRect copies the rectangle src of video memory to x, y in it, as
// the GPU does, queued with the drawing.
func CopyRect(src Rect, x, y int) { copyRect(src, x, y) }

// ClearRect fills the rectangle of video memory with a color, queued
// with the drawing.
func ClearRect(r Rect, c Color) { clearRect(r, c) }

// LoadRect puts the words into the rectangle of video memory, row by
// row, queued with the drawing: the words stay in use until the GPU
// has them, which the frame's end waits for.
func LoadRect(r Rect, words []uint32) { loadRect(r, words) }

// StoreRect reads the rectangle of video memory into the words, row by
// row, and waits for it.
func StoreRect(r Rect, words []uint32) { storeRect(r, words) }

// Texture is how a sprite names the image: its page, its corner within
// it, and its palette.
func (im Image) Texture() Texture { return im.texture }

// Sprite returns a sprite showing the whole image at its own size and
// colors, upright and unscaled, with its pivot at its middle. A 24-bit
// image cannot be a sprite.
func (im Image) Sprite() Sprite {
	return Sprite{
		Mode:    Mode(im.depth) << 24,
		W:       im.w,
		H:       im.h,
		Texture: im.texture,
		Color:   Color{128, 128, 128},
		Pivot:   Point{int16(im.w / 2), int16(im.h / 2)},
		ScaleX:  One,
		ScaleY:  One,
	}
}
