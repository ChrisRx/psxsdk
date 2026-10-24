// Package openbios holds an image of OpenBIOS, the open source BIOS from
// PCSX-Redux's nugget library, which boots everything here: the tests, the
// site's player and yaroze run, unless given Sony's. Its license is
// licenses/OPENBIOS-LICENSE.txt at the root of the repository, which the
// site's player carries beside the image.
package openbios

import _ "embed"

// Image is the BIOS image, 512KB as the console's is.
//
//go:embed openbios.bin
var Image []byte
