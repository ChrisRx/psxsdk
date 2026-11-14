package transform_test

import (
	"testing"

	"github.com/tinygo-org/tinygo/compileopts"
	"github.com/tinygo-org/tinygo/transform"
	"tinygo.org/x/go-llvm"
)

func TestHoistOnceAllocs(t *testing.T) {
	t.Parallel()
	config := &compileopts.Config{
		Options: &compileopts.Options{},
		Target:  &compileopts.TargetSpec{GC: "leaking"},
	}
	testTransform(t, "testdata/once", func(mod llvm.Module) {
		transform.HoistOnceAllocs(mod, config)
	})
}
