// Package openbios holds an image of OpenBIOS, the open source BIOS from
// PCSX-Redux's nugget library, which boots everything here: the tests, the
// site's player and psxplay, unless given Sony's. OPENBIOS-LICENSE.txt is
// its license.
package openbios

import _ "embed"

// Image is the BIOS image, 512KB as the console's is.
//
//go:embed openbios.bin
var Image []byte
