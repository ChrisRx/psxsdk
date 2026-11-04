//go:build yaroze && golibps && !cleanlib

package yaroze

// The playstation-yaroze-go target (yaroze --libps go) links the library
// in rather than calling the resident image; this import is what brings
// it in, and the bindings' bodyless declarations resolve to its exports.
import _ "go.chrisrx.dev/psx/exp/libps"
