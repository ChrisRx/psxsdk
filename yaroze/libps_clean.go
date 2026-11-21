//go:build yaroze && cleanlib

package yaroze

// A static build links the clean-room library in rather than calling the
// resident image; this import is what brings it in, and the
// bindings' bodyless declarations resolve to it.
import _ "go.chrisrx.dev/psx/internal/libps"
