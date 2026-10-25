// Package mkscene cooks a scene, written as JSON by the playground's scene
// editor or by hand, into the file a program reads with package scene. It is
// the mkscene tool and yaroze's scene command.
package mkscene

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"go.chrisrx.dev/psx/format/scene"
)

type options struct {
	Output string
	Read   bool
}

// Command returns the command, as use: mkscene as a tool of its own, and
// scene in yaroze.
func Command(use string) *cobra.Command {
	var opts options
	cmd := &cobra.Command{
		Use:   use + " [flags] <scene.json>",
		Short: "cook a scene into the file package scene reads",
		Long: use + ` cooks a scene, scene.json, into SCENE.DAT beside it unless -o
names the output: the file a program reads off its disc and draws with
package scene. The JSON is what the playground's scene editor writes, and
what format/scene describes: a camera, the projection, the ambient light,
up to three lights, the TIMs and TMDs the scene needs by their paths on the
disc, the objects placed from those models and the sprites cut from those
textures, each with a name the program finds it by; or a room of cells for
package stage, with its links to other scenes and its markers.

-r reads a cooked scene and prints it as JSON, or writes it to -o.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(&opts, args)
		},
		SilenceUsage: true,
	}
	f := cmd.Flags()
	f.StringVarP(&opts.Output, "output", "o", "", "where to write the scene")
	f.BoolVarP(&opts.Read, "read", "r", false, "read a cooked scene and print it as JSON")
	return cmd
}

func run(opts *options, args []string) error {
	in, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}
	if opts.Read {
		s, err := scene.Decode(in)
		if err != nil {
			return fmt.Errorf("%s: %w", args[0], err)
		}
		b, err := json.MarshalIndent(s, "", "  ")
		if err != nil {
			return err
		}
		b = append(b, '\n')
		if opts.Output == "" {
			_, err = os.Stdout.Write(b)
			return err
		}
		return os.WriteFile(opts.Output, b, 0o644)
	}
	var s scene.Scene
	if err := json.Unmarshal(in, &s); err != nil {
		return fmt.Errorf("%s: %w", args[0], err)
	}
	b, err := s.Bytes()
	if err != nil {
		return fmt.Errorf("%s: %w", args[0], err)
	}
	out := opts.Output
	if out == "" {
		out = filepath.Join(filepath.Dir(args[0]), "SCENE.DAT")
	}
	if err := os.WriteFile(out, b, 0o644); err != nil {
		return err
	}
	camera := fmt.Sprintf("camera at %d,%d,%d looking at %d,%d,%d", s.Camera.From[0], s.Camera.From[1], s.Camera.From[2], s.Camera.At[0], s.Camera.At[1], s.Camera.At[2])
	if s.Camera == (scene.Camera{}) {
		camera = "no camera, the program's own"
	}
	fmt.Printf("created %q: %d bytes, %d object%s, %d sprite%s, %d light%s, %s\n",
		out, len(b), len(s.Objects), plural(len(s.Objects)), len(s.Sprites), plural(len(s.Sprites)), len(s.Lights), plural(len(s.Lights)), camera)
	for _, o := range s.Objects {
		fmt.Printf("  %s: %s object %d at %d,%d,%d\n", o.Name, strings.TrimPrefix(o.Model, "DATA/"), o.Index, o.Position[0], o.Position[1], o.Position[2])
	}
	if r := s.Room; r != nil {
		fmt.Printf("  a room of %d by %d cells, %d link%s, %d marker%s\n", len(r.Cells[0]), len(r.Cells), len(r.Links), plural(len(r.Links)), len(r.Markers), plural(len(r.Markers)))
		for _, l := range r.Links {
			fmt.Printf("    door %d leads to %s\n", l.Door, l.Scene)
		}
		for _, m := range r.Markers {
			fmt.Printf("    %s on cell %d, %d\n", m.Name, m.Cell[0], m.Cell[1])
		}
	}
	return nil
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
