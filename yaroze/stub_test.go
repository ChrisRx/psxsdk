//go:build !yaroze

package yaroze

import (
	"strings"
	"testing"

	"go.chrisrx.dev/x/assert"
)

// TestStubPanics checks that calling the library on a host fails at once,
// with a message that names the function and says where the real one is,
// rather than returning a zero or dereferencing nothing.
func TestStubPanics(t *testing.T) {
	for name, call := range map[string]func(){
		"GsInitGraph":   func() { GsInitGraph(320, 240, 0, 0, 0) },
		"FntPrint":      func() { FntPrint(nil) },
		"GsLinkObject4": func() { GsLinkObject4(nil, nil, 0) },
		"Sin":           func() { Sin(0) },
	} {
		assert.Panic(t, stub(name), call, name)
	}
	assert.Equal(t, true, strings.Contains(stub("X"), "yaroze.X: ") && strings.Contains(stub("X"), "playstation-yaroze"), stub("X"))
}
