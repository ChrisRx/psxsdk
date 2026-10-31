// Package smf2seq converts Standard MIDI Files into SEQ files, the score
// format the PlayStation's sound library plays: what Sony's smf2seq did. It
// is the smf2seq tool and yaroze's seq command.
package smf2seq

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"go.chrisrx.dev/psx/format/seq"
)

type options struct {
	Output  string
	NoBank  bool
	Verbose bool
}

// Command returns the command, as use: smf2seq as a tool of its own, and
// seq in yaroze.
func Command(use string) *cobra.Command {
	var opts options
	cmd := &cobra.Command{
		Use:   use + " [flags] <midi>...",
		Short: "convert Standard MIDI Files into SEQ files",
		Long: use + ` converts MIDI files, format 0 or 1, into SEQ files, each beside
its MIDI file with the extension .SEQ unless -o names the one output. The
tracks are merged into one stream, as Sony's SCEA smf2seq merged them, byte
for byte the same.

A SEQ keeps notes, program changes, pitch bend, pressure, tempo changes and
the control changes the library acts on: bank select (0), data entry (6),
volume (7), pan (10), expression (11) and NRPN (98, 99). The rest are
dropped; -V lists them. The first tempo and time signature go in the
header. The sequence names programs by number, so it plays with a VAB whose
programs are the instruments the MIDI file chose, such as the General MIDI
bank on the Net Yaroze boot disc, STD0.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(&opts, args)
		},
		SilenceUsage: true,
	}
	f := cmd.Flags()
	f.StringVarP(&opts.Output, "output", "o", "", "where to write the SEQ, for one MIDI file")
	f.BoolVarP(&opts.NoBank, "no-bank", "B", false, "drop bank select")
	f.BoolVarP(&opts.Verbose, "verbose", "V", false, "list what was dropped")
	return cmd
}

func run(opts *options, args []string) error {
	if opts.Output != "" && len(args) > 1 {
		return fmt.Errorf("-o names one output, for one MIDI file, not %d", len(args))
	}
	for _, path := range args {
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out, dropped, err := seq.Convert(b, seq.Options{NoBank: opts.NoBank})
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		dst := opts.Output
		if dst == "" {
			dst = strings.TrimSuffix(path, filepath.Ext(path)) + ".SEQ"
		}
		if err := os.WriteFile(dst, out, 0o644); err != nil {
			return err
		}
		h, evs, err := seq.Decode(out)
		if err != nil {
			return fmt.Errorf("%s: %w", dst, err)
		}
		fmt.Printf("created %q: %d events, %d ticks a quarter note, %.1f beats a minute\n", dst, len(evs), h.Resolution, 60e6/float64(h.Tempo))
		if opts.Verbose {
			list := func(what string, m map[byte]int) {
				for _, k := range slices.Sorted(maps.Keys(m)) {
					fmt.Printf("  dropped %s %d (%#02x): %d\n", what, k, k, m[k])
				}
			}
			list("control change", dropped.Controls)
			list("meta event", dropped.Metas)
			if dropped.SysEx > 0 {
				fmt.Printf("  dropped system exclusive: %d\n", dropped.SysEx)
			}
		}
	}
	return nil
}
