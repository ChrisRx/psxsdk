//go:build js

package main

import (
	"fmt"
	"syscall/js"

	"go.chrisrx.dev/psx/internal/hot"
)

// The running program's region for hot reload patches, when it was built
// with -hot: its bounds, where the next patch goes, and whether a patch
// was loaded since the last run reported it.
var (
	hotStart, hotEnd, hotAt uint32
	patched                 bool
)

// setHotRegion reads the region from the program's ELF, or clears it.
func setHotRegion(elfData []byte) {
	hotStart, hotEnd, hotAt = 0, 0, 0
	if elfData == nil {
		return
	}
	if start, end, err := hot.Region(elfData); err == nil {
		hotStart, hotEnd, hotAt = start, end, start
	}
}

// hotRegion returns {start, end, at}, or null when the program running
// was not built with -hot.
func hotRegion(js.Value, []js.Value) any {
	if machine == nil || hotStart == 0 {
		return nil
	}
	return map[string]any{"start": int(hotStart), "end": int(hotEnd), "at": int(hotAt)}
}

// patch loads a hot reload patch, an ELF built with -hot-base at the
// region's at, the next time the program is between frames, and returns
// {start, end, at}, the memory it took and where the next goes, or
// {error}.
func patch(_ js.Value, args []js.Value) any {
	if machine == nil || len(args) < 1 {
		return map[string]any{"error": "nothing is running"}
	}
	if hotStart == 0 {
		return map[string]any{"error": "the program was not built for hot reload"}
	}
	p, err := hot.Parse(bytesFromJS(args[0]))
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	if p.Start < hotAt || p.End > hotEnd {
		return map[string]any{"error": fmt.Sprintf("the patch, %08x to %08x, does not fit what is left of the program's region, %08x to %08x: build and run it again", p.Start, p.End, hotAt, hotEnd)}
	}
	p.ApplyAt(machine, func() { patched = true })
	hotAt = (p.End + 15) &^ 15
	return map[string]any{"start": int(p.Start), "end": int(p.End), "at": int(hotAt), "slots": len(p.Slots)}
}
