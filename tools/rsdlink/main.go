// Command rsdlink links RSD models, the text format Sony's 3D tools wrote,
// into a TMD file the library draws: what Sony's rsdlink did, with the same
// arguments.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go.chrisrx.dev/psx/format/rsd"
	"go.chrisrx.dev/psx/format/tmd"
)

const usage = `usage: rsdlink [options] rsd-names ...

rsdlink links RSD models into one TMD file, a model an object, in the order
given. Each RSD names its PLY, MAT and GRP files and the TIM files of its
textures, which are read for where they sit in video memory; a texture is
looked for beside the RSD and then in ../TIM. A name without a directory is
looked for here and then in RSD, and its .rsd may be left off.

  -o file     the TMD to write (a.tmd)
  -s factor   scale the models that follow, by the power of two nearest
              factor, which the TMD records
  -t x y z    move the models that follow
  -v          print each model's counts and the range of its vertices
  -info       print each model's polygons
  file.arg    read more arguments from file.arg, which may not name
              another .arg file

The output is byte for byte what Sony's rsdlink wrote, for the models in the
Net Yaroze websites' archive.`

type input struct {
	path  string
	place rsd.Placement
}

func main() {
	log.SetFlags(0)
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	if err := run(args); err != nil {
		log.Fatal("rsdlink: ", err)
	}
}

func run(args []string) error {
	out := "a.tmd"
	var verbose, info bool
	var pl rsd.Placement
	var inputs []input
	var parse func(args []string, inArg bool) error
	parse = func(args []string, inArg bool) error {
		for i := 0; i < len(args); i++ {
			need := func(n int) ([]string, error) {
				if i+n >= len(args) {
					return nil, fmt.Errorf("%s needs %d value(s)", args[i], n)
				}
				v := args[i+1 : i+1+n]
				i += n
				return v, nil
			}
			switch a := args[i]; strings.ToLower(a) {
			case "-h", "-help", "--help":
				fmt.Println(usage)
				os.Exit(0)
			case "-o":
				v, err := need(1)
				if err != nil {
					return err
				}
				out = v[0]
			case "-s", "-sc":
				v, err := need(1)
				if err != nil {
					return err
				}
				f, err := strconv.ParseFloat(v[0], 64)
				if err != nil || f <= 0 {
					return fmt.Errorf("-s %q is not a positive number", v[0])
				}
				pl.Scale = f
			case "-t":
				v, err := need(3)
				if err != nil {
					return err
				}
				for k := range 3 {
					f, err := strconv.ParseFloat(v[k], 64)
					if err != nil {
						return fmt.Errorf("-t %q is not a number", v[k])
					}
					pl.Translate[k] = f
				}
			case "-v":
				verbose = true
			case "-info":
				info = true
			default:
				if strings.HasPrefix(a, "-") {
					return fmt.Errorf("unknown option %s", a)
				}
				if strings.EqualFold(filepath.Ext(a), ".arg") {
					if inArg {
						return fmt.Errorf("%s: an argument file may not name another", a)
					}
					b, err := os.ReadFile(a)
					if err != nil {
						return err
					}
					if err := parse(strings.Fields(string(b)), true); err != nil {
						return fmt.Errorf("%s: %w", a, err)
					}
					continue
				}
				p, err := locate(a)
				if err != nil {
					return err
				}
				inputs = append(inputs, input{p, pl})
			}
		}
		return nil
	}
	if err := parse(args, false); err != nil {
		return err
	}
	if len(inputs) == 0 {
		return fmt.Errorf("no RSD files given")
	}

	var t tmd.TMD
	for _, in := range inputs {
		m, err := rsd.Read(in.path)
		if err != nil {
			return err
		}
		o, err := m.Object(in.place)
		if err != nil {
			return fmt.Errorf("%s: %w", in.path, err)
		}
		if verbose {
			lo, hi := bounds(o.Vertices)
			fmt.Printf("%s: %d vertices, %d normals, %d polygons, scale 2^%d, range (%d,%d,%d)-(%d,%d,%d)\n",
				in.path, len(o.Vertices), len(o.Normals), len(o.Primitives), o.Scale, lo.X, lo.Y, lo.Z, hi.X, hi.Y, hi.Z)
		}
		if info {
			for i, p := range o.Primitives {
				fmt.Printf("  %d: mode %#02x flag %#02x, %d words, olen %d\n", i, p.Mode, p.Flag, len(p.Packet), p.Olen)
			}
		}
		t.Objects = append(t.Objects, o)
	}
	b := t.Bytes()
	if err := os.WriteFile(out, b, 0o644); err != nil {
		return err
	}
	fmt.Printf("created %q: %d object(s), %d bytes\n", out, len(t.Objects), len(b))
	return nil
}

// locate finds a model as rsdlink did: the name as given, with .rsd added
// when it has no extension, and then in RSD.
func locate(name string) (string, error) {
	if filepath.Ext(name) == "" {
		name += ".rsd"
	}
	for _, p := range []string{name, filepath.Join("RSD", name)} {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
		dir, base := filepath.Split(p)
		if dir == "" {
			dir = "."
		}
		if ents, err := os.ReadDir(dir); err == nil {
			for _, e := range ents {
				if strings.EqualFold(e.Name(), base) {
					return filepath.Join(dir, e.Name()), nil
				}
			}
		}
	}
	return "", fmt.Errorf("%s not found", name)
}

func bounds(vs []tmd.Vector) (lo, hi tmd.Vector) {
	for i, v := range vs {
		if i == 0 {
			lo, hi = v, v
			continue
		}
		lo = tmd.Vector{X: min(lo.X, v.X), Y: min(lo.Y, v.Y), Z: min(lo.Z, v.Z)}
		hi = tmd.Vector{X: max(hi.X, v.X), Y: max(hi.Y, v.Y), Z: max(hi.Z, v.Z)}
	}
	return lo, hi
}
