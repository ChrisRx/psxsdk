//go:build yaroze

package libps

// Startup does what the resident library's startup routine did before a
// program ran, for a build that lays this library out as the resident
// image: the callbacks reset, the display masked and the GPU reset, the
// drive, the sequencer and the memory card driver started in the order
// that routine had them, which takes it some five frames. The pads are
// left for their first use, as the rest of this library leaves them.
//
// libps 0x80010030, 152 bytes
func Startup() {
	ResetCallback()
	SetDispMask(0)
	ResetGraph(0)
	SetGraphDebug(0)
	sndStart()
	cardStart()
}
