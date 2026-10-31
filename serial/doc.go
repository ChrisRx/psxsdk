// Package serial talks to a PlayStation over a serial cable from the host.
//
// It comes in two layers. A Port is the cable: the host's serial device,
// with the hardware flow control the console's serial port expects done by
// hand, and what the console sends kept until it is read. A protocol is
// spoken over a Port by a type of its own. So far that is Monitor, for the
// monitor a Net Yaroze's boot disc runs, which Sony's siocons talked to and
// which takes text commands and binary uploads into RAM, and Debugger, for
// the remote debugger its rdb command starts. A retail console's
// loader would be another type over the same Port.
package serial
