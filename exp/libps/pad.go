//go:build yaroze

package libps

// The controllers. The kernel's pad service reads both ports at every
// vertical blank into two 34-byte buffers the library gives it: the
// first byte is 0 while a controller answers, the second its type and the
// count of halfwords after, and the rest what it sent, the buttons first.

// libps 0x8004EAEC and 0x8004EB10
var padBuf0, padBuf1 [34]uint8

// padStarted is whether the kernel has been given the buffers and told to
// read the ports. The resident library gave the buffers in its startup
// and never started the service: the Net Yaroze boot disc's own program
// had, before it loaded the member's. With nothing before this library,
// it starts the service itself, the first time the buffers are asked for.
var padStarted bool

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
		initPAD(&padBuf0[0], uint32(len(padBuf0)), &padBuf1[0], uint32(len(padBuf1)))
		startPAD()
		// Starting the service has the kernel's pad handler acknowledge
		// the vertical blank itself again, which would take the
		// interrupt from this library's handler; the boot disc started
		// the service before the library told the kernel not to.
		changeClearPAD(0)
		padStarted = true
	}
	for i := range padBuf0 {
		padBuf0[i] = 0xff
		padBuf1[i] = 0xff
	}
	*a0 = &padBuf0[0]
	*a1 = &padBuf1[0]
}
