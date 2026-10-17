package yaroze

// A TIM is an image file in memory, as the drive delivers it: the words of
// the file, whose header says where in video memory its pixels and palette
// go. A program reads one with package disc and loads it:
//
//	image := yaroze.TIM(textureFile.Data()).Load()
//
// The file is an ID word and then the header GsGetTimInfo reads.
type TIM []uint32

// Image is GsGetTimInfo: the image the file describes, where its pixels and
// palette go and where they are in memory, without loading it.
//
//go:inline
func (t TIM) Image() (im GsIMAGE) {
	im.Read(&t[1])
	return im
}

// Load puts the image into video memory, where its header says, with its
// palette if it has one, waits for the transfer, and returns the image for
// the texture page and palette a sprite or a model names it by.
func (t TIM) Load() GsIMAGE {
	im := t.Image()
	im.Load()
	DrawSync(0)
	return im
}
