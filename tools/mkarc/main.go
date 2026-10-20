// Command mkarc packs files into one archive for the disc, which a
// program reads with package archive.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"go.chrisrx.dev/psx/archive"
	farchive "go.chrisrx.dev/psx/format/archive"
)

var opts struct {
	Output string
	List   bool
	Store  []string
}

func main() {
	cmd := &cobra.Command{
		Use:   "mkarc [flags] <file or directory>...",
		Short: "pack files into one archive for the disc",
		Long: `mkarc packs the files named, and every file in each directory named,
into one archive, which a program registers with package disc as one file
and reads with package archive, unpacking the entries it wants into memory of
its own. Each entry is packed with deflate, unless that would save less
than a tenth of it, as it does a sound bank or a movie, when it is stored
as it is and the program can use it where it lies.

Entries are named as the disc names files, in capitals, eight and three,
from each file's name, so two files of a name cannot go in one archive
whatever their directories.

-l lists an archive's entries, with how each is held.`,
		Args:         cobra.MinimumNArgs(1),
		RunE:         run,
		SilenceUsage: true,
	}
	f := cmd.Flags()
	f.StringVarP(&opts.Output, "output", "o", "", "where to write the archive (default: the first directory's name, in capitals, with .ARC, under assets/)")
	f.BoolVarP(&opts.List, "list", "l", false, "list an archive's entries")
	f.StringArrayVarP(&opts.Store, "store", "s", nil, "an entry name to store as it is, whatever it would pack to; may repeat")
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func run(cmd *cobra.Command, args []string) error {
	if opts.List {
		for _, p := range args {
			if err := list(p); err != nil {
				return err
			}
		}
		return nil
	}
	var paths []string
	for _, a := range args {
		info, err := os.Stat(a)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			paths = append(paths, a)
			continue
		}
		entries, err := os.ReadDir(a)
		if err != nil {
			return err
		}
		var names []string
		for _, e := range entries {
			if !e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				names = append(names, filepath.Join(a, e.Name()))
			}
		}
		sort.Strings(names)
		paths = append(paths, names...)
	}
	if len(paths) == 0 {
		return fmt.Errorf("nothing to pack")
	}
	store := map[string]bool{}
	for _, s := range opts.Store {
		store[strings.ToUpper(s)] = true
	}
	var entries []farchive.Entry
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		name := farchive.Name(p)
		entries = append(entries, farchive.Entry{Name: name, Data: data, Store: store[name]})
	}
	b, err := farchive.Pack(entries)
	if err != nil {
		return err
	}
	out := opts.Output
	if out == "" {
		stem := filepath.Base(args[0])
		if info, err := os.Stat(args[0]); err == nil && !info.IsDir() {
			stem = strings.TrimSuffix(stem, filepath.Ext(stem))
		}
		out = filepath.Join("assets", strings.TrimSuffix(farchive.Name(stem+".x"), ".X")+".ARC")
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(out, b, 0o644); err != nil {
		return err
	}
	a, err := archive.OpenBytes(b)
	if err != nil {
		return err
	}
	raw := 0
	largest := archive.Entry{}
	for i := 0; i < a.Len(); i++ {
		e := a.At(i)
		raw += e.Size
		if e.Size > largest.Size {
			largest = e
		}
	}
	fmt.Printf("created %q: %d bytes from %d (%d%%), %d file%s, the largest %s at %d bytes\n", out, len(b), raw, pct(len(b), raw), a.Len(), plural(a.Len()), largest.Name, largest.Size)
	printEntries(a)
	return nil
}

func list(p string) error {
	b, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	a, err := archive.OpenBytes(b)
	if err != nil {
		return fmt.Errorf("%s: %w", p, err)
	}
	raw := 0
	for i := 0; i < a.Len(); i++ {
		raw += a.At(i).Size
	}
	fmt.Printf("%s: %d bytes from %d (%d%%), %d file%s\n", p, len(b), raw, pct(len(b), raw), a.Len(), plural(a.Len()))
	printEntries(a)
	return nil
}

func printEntries(a archive.Archive) {
	for i := 0; i < a.Len(); i++ {
		e := a.At(i)
		how := "stored"
		if e.Method == archive.Deflate {
			how = fmt.Sprintf("deflate, %d%%", pct(len(e.Packed()), e.Size))
		}
		fmt.Printf("  %-14s %8d bytes, %s\n", e.Name, e.Size, how)
	}
}

func pct(a, b int) int {
	if b == 0 {
		return 100
	}
	return (100*a + b/2) / b
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
