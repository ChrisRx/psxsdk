//go:build yaroze && golibps && cleanlib

package yaroze

// With the cleanlib tag as well, the playstation-yaroze-go target links
// the clean-room library, exp/cleanlib, in place of exp/libps.
import _ "go.chrisrx.dev/psx/exp/cleanlib"
