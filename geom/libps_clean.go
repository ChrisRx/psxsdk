//go:build yaroze && cleanlib

package geom

// A static build links the clean-room library in rather than calling the
// resident image; this import brings in the functions the
// bodyless declarations in matrix_yaroze.go name.
import _ "go.chrisrx.dev/psx/internal/libps"
