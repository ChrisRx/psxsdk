package nyc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.chrisrx.dev/psx/internal/hot"
)

// watchPatches builds a patch for the program whose ELF base names, built
// with --hot and running, each time the Go files of pkg change, and sends
// it on patches, until ctx ends. Each patch is linked after the one before
// it, since the program may still be running that one's code; when the
// program's region is full, or a patch cannot be built, it says so on
// standard error and waits for the next change.
func (f *buildFlags) watchPatches(ctx context.Context, pkg, base string, patches chan<- *hot.Patch) {
	data, err := os.ReadFile(base)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nyc: hot: %v\n", err)
		return
	}
	at, end, err := hot.Region(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nyc: hot: %v\n", err)
		return
	}
	tmp, err := os.MkdirTemp("", "nyc-hot-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "nyc: hot: %v\n", err)
		return
	}
	defer os.RemoveAll(tmp)
	if end == at {
		fmt.Fprintf(os.Stderr, "nyc: hot: the program is too large to leave room for patches, so a change to it needs a build and start again\n")
		return
	}
	fmt.Fprintf(os.Stderr, "nyc: hot: watching %s; a change to its Go files patches the running program, with %d KB for the patches\n", pkg, (end-at)>>10)

	last := goFilesSignature(pkg)
	for n := 0; ; n++ {
		select {
		case <-ctx.Done():
			return
		case <-time.After(300 * time.Millisecond):
		}
		sig := goFilesSignature(pkg)
		if sig == last {
			continue
		}
		// A save may come in parts: wait for the files to settle.
		for settled := goFilesSignature(pkg); settled != sig; settled = goFilesSignature(pkg) {
			sig = settled
			time.Sleep(200 * time.Millisecond)
		}
		last = sig
		if at >= end {
			fmt.Fprintf(os.Stderr, "nyc: hot: the program's region for patches is full: build and start it again\n")
			continue
		}
		out := filepath.Join(tmp, fmt.Sprintf("patch%d.elf", n))
		start := time.Now()
		if err := f.patch(ctx, pkg, base, at, out); err != nil {
			fmt.Fprintf(os.Stderr, "nyc: hot: %v\n", err)
			continue
		}
		data, err := os.ReadFile(out)
		if err != nil {
			fmt.Fprintf(os.Stderr, "nyc: hot: %v\n", err)
			continue
		}
		p, err := hot.Parse(data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "nyc: hot: %v\n", err)
			continue
		}
		if p.End > end {
			fmt.Fprintf(os.Stderr, "nyc: hot: the patch does not fit what is left of the program's region: build and start it again\n")
			at = end
			continue
		}
		fmt.Fprintf(os.Stderr, "nyc: hot: built a patch of %d bytes in %s, %d KB of the region left\n", p.Size(), time.Since(start).Round(time.Millisecond), (end-p.End)>>10)
		at = (p.End + 15) &^ 15
		select {
		case patches <- p:
		case <-ctx.Done():
			return
		}
	}
}

// goFilesSignature describes the Go files in dir: their names, sizes and
// modification times, so that a change to any shows as a different string.
func goFilesSignature(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var b strings.Builder
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		fmt.Fprintf(&b, "%s %d %d\n", e.Name(), info.Size(), info.ModTime().UnixNano())
	}
	return b.String()
}
