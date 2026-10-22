// SPDX-License-Identifier: GPL-2.0-only
// SPDX-FileCopyrightText: Lameguy64 and the mkpsxiso contributors
// SPDX-FileCopyrightText: 2026 Chris Marshall
//
// This file is a translation into Go of dumpsxiso, part of mkpsxiso,
// https://github.com/Lameguy64/mkpsxiso, made in 2026 by Chris Marshall
// and changed in the translation to read the image through this
// repository's format packages and to extract a directory tree rather than
// write an XML project file. See README.md in this directory. It is free
// software under the GNU General Public License version 2, the text of
// which is licenses/GPL-2.0-only.txt at the root of the repository, and it
// is not covered by the MIT license the rest of the repository is under.

// Command dumpsxiso lists and extracts the contents of a PlayStation disc
// image.
package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"go.chrisrx.dev/psx/format/iso9660"
	"go.chrisrx.dev/psx/format/psxdisc"
)

var opts struct {
	Output string
	Quiet  bool
}

func main() {
	cmd := &cobra.Command{
		Use:   "dumpsxiso [flags] <image>",
		Short: "list or extract the contents of a PlayStation disc image",
		Long: `dumpsxiso reads the filesystem of a PlayStation disc image.

With no --output it lists what the image holds, which is the quickest way to
see whether a file landed on the disc under the name a program will ask for.
With --output the tree is written to a directory, and a directory built that
way can be handed back to mkdisc.

Either a .bin of raw 2352-byte sectors or a .iso of 2048-byte blocks will do;
which one it is follows from the image itself. Reading a raw image checks each
sector's error detection, so a damaged image is reported rather than quietly
extracted.

Files whose directory record marks them as streamed media, meaning XA audio
and STR video, are written out as the 2336-byte Mode 2 sectors they are, which
is the form they went in as.`,
		Args:         cobra.ExactArgs(1),
		RunE:         run,
		SilenceUsage: true,
	}

	f := cmd.Flags()
	f.StringVarP(&opts.Output, "output", "o", "", "extract the files into this directory")
	f.BoolVarP(&opts.Quiet, "quiet", "q", false, "do not list what the image holds")

	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func run(cmd *cobra.Command, args []string) error {
	r, closer, err := psxdisc.Open(args[0])
	if err != nil {
		return err
	}
	defer closer.Close()

	if !opts.Quiet {
		describe(os.Stdout, r)
	}
	if opts.Output == "" {
		return nil
	}
	return extract(r.Root, opts.Output)
}

// describe reports what the image says about itself and what it holds.
func describe(w io.Writer, r *iso9660.Reader) {
	v := r.Volume
	fmt.Fprintf(w, "volume %s, %d blocks\n", v.VolumeID, v.Blocks)
	fmt.Fprintf(w, "  system      %s\n", v.SystemID)
	for _, f := range []struct{ name, value string }{
		{"publisher", v.Publisher},
		{"preparer", v.Preparer},
		{"application", v.Application},
	} {
		if f.value != "" {
			fmt.Fprintf(w, "  %-11s %s\n", f.name, f.value)
		}
	}
	if !v.Created.IsZero() {
		fmt.Fprintf(w, "  created     %s\n", v.Created.Format(time.RFC3339))
	}
	if v.XA {
		fmt.Fprintf(w, "  extension   CD-XA\n")
	}
	fmt.Fprintln(w)
	list(w, r.Root, "")
}

func list(w io.Writer, d *iso9660.Dir, prefix string) {
	for _, e := range d.Entries {
		note := ""
		if e.Raw {
			note = "  raw sectors"
		}
		fmt.Fprintf(w, "  %-40s %8d bytes  LBA %d%s\n", prefix+"\\"+e.Name, e.Size, e.LBA, note)
	}
	for _, sub := range d.Dirs {
		fmt.Fprintf(w, "  %-40s %8s        LBA %d\n", prefix+"\\"+sub.Name, "<dir>", sub.LBA)
		list(w, sub, prefix+"\\"+sub.Name)
	}
}

// extract writes a directory of the image to the host filesystem.
func extract(d *iso9660.Dir, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, e := range d.Entries {
		if err := extractFile(e, filepath.Join(dir, hostName(e.Name))); err != nil {
			return err
		}
	}
	for _, sub := range d.Dirs {
		if err := extract(sub, filepath.Join(dir, hostName(sub.Name))); err != nil {
			return err
		}
	}
	return nil
}

func extractFile(e *iso9660.Entry, path string) error {
	rc, err := e.Open()
	if err != nil {
		return fmt.Errorf("reading %s: %w", e.Name, err)
	}
	defer rc.Close()

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, rc); err != nil {
		f.Close()
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return err
	}
	if !e.Modified.IsZero() {
		// Ignore a failure to set the time: the contents are what matters,
		// and some filesystems will not take it.
		_ = os.Chtimes(path, e.Modified, e.Modified)
	}
	return nil
}

// hostName drops the version suffix a disc identifier carries, since nothing
// off the disc expects it and a tree extracted with it would gain a second
// suffix if it were built again.
func hostName(id string) string {
	name := strings.TrimSuffix(id, ";1")
	// Nothing in an identifier can name a parent directory, but a malformed
	// image is not bound by that, and an extracted file must not land
	// outside the directory it was asked for.
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return "_" + strings.Map(func(r rune) rune {
			if r == '/' || r == '\\' {
				return '_'
			}
			return r
		}, name)
	}
	return name
}
