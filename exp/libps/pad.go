//go:build yaroze

package libps

// The controllers. The kernel's pad service reads both ports at every
// vertical blank into two 34-byte buffers the library gives it: the
// first byte is 0 while a controller answers, the second its type and the
// count of halfwords after, and the rest what it sent, the buttons first.

// libps 0x8004EAEC and 0x8004EB10
var padBuf0, padBuf1 padBuffer

// A padBuffer is the 34 bytes the kernel writes a port into, on a word's
// boundary, as the original's were: programs read the first bytes as a
// word.
type padBuffer struct {
	_ [0]uint32
	b [34]uint8
}

// padStarted is whether the kernel has been given the buffers and told to
// read the ports. The resident library's startup gave the buffers right
// after its GPU reset, and the kernel read the ports every vertical blank
// from then on (its card driver, started in the same routine, reads them
// too), before the member's program ran. This library has no routine
// before the program, so it starts the service in the program's first
// ResetGraph, where the rest of that startup's work is done, and a
// program that asks for the pads before any ResetGraph starts it there.
var padStarted bool

// padStart gives the kernel the buffers and starts the service, once.
func padStart() {
	if padStarted {
		return
	}
	initPAD(&padBuf0.b[0], uint32(len(padBuf0.b)), &padBuf1.b[0], uint32(len(padBuf1.b)))
	startPAD()
	// Starting the service has the kernel's pad handler acknowledge the
	// vertical blank itself again, which would take the interrupt from
	// this library's handler; the boot disc started the service before
	// the library told the kernel not to.
	changeClearPAD(0)
	padStarted = true
}

// GetPadBuf gives the program the two buffers, cleared to 0xff, which is
// what a port with nothing on it reads as.
//
// libps 0x800100C8, 100 bytes
//
//export GetPadBuf
func GetPadBuf(a0, a1 **uint8) {
	if !padStarted {
		// The service leaves the vertical blank for this library's
		// interrupt handler to acknowledge, which the resident library
		// installed at boot and this one installs on the first
		// ResetGraph: a program that asks for the pads first needs it now,
		// or the kernel's own handler, with nothing acknowledging the
		// blank, runs the pad service without end.
		ResetCallback()
		padStart()
	}
	for i := range padBuf0.b {
		padBuf0.b[i] = 0xff
		padBuf1.b[i] = 0xff
	}
	*a0 = &padBuf0.b[0]
	*a1 = &padBuf1.b[0]
}
