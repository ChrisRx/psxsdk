package rsd

import (
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"go.chrisrx.dev/x/assert"

	"go.chrisrx.dev/psx/format/tmd"
)

// Command is an rsdlink command line: the TMD it writes and the models
// that go in it, each with its placement.
type command struct {
	out    string
	models []string
	places []Placement
}

// parseCommand reads the arguments of an rsdlink command line.
func parseCommand(args []string) (command, error) {
	c := command{out: "a.tmd"}
	var pl Placement
	for i := 0; i < len(args); i++ {
		switch a := strings.ToLower(args[i]); a {
		case "-o":
			i++
			c.out = args[i]
		case "-s", "-sc":
			i++
			f, err := strconv.ParseFloat(args[i], 64)
			if err != nil {
				return c, err
			}
			pl.Scale = f
		case "-t":
			for k := range 3 {
				i++
				f, err := strconv.ParseFloat(args[i], 64)
				if err != nil {
					return c, err
				}
				pl.Translate[k] = f
			}
		case "-v", "-info":
		default:
			if strings.HasPrefix(a, "-") || strings.HasSuffix(a, ".arg") {
				return c, fmt.Errorf("unsupported argument %q", a)
			}
			c.models = append(c.models, args[i])
			c.places = append(c.places, pl)
		}
	}
	return c, nil
}

// edited are the TMDs whose sources changed after they were linked, by the
// last two directories and the name.
var edited = map[string]string{
	"3DRA/MAP/TREE.TMD": "its MAT colours polygon 14 red, where the TMD has it green",
}

// TestArchive links the RSD models in the Net Yaroze websites' archive
// whose TMD sits beside them, with the rsdlink command a batch file or
// makefile there gives, or alone at scale 1 when none does, and compares
// the TMD byte for byte. PSX_WEBARCHIVE is the archive's site directory.
func TestArchive(t *testing.T) {
	root := os.Getenv("PSX_WEBARCHIVE")
	if root == "" {
		t.Skip("PSX_WEBARCHIVE is not set")
	}
	var tried, same, excused int
	err := filepath.WalkDir(root, func(dir string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return err
		}
		ents, _ := os.ReadDir(dir)
		names := map[string]string{}
		for _, e := range ents {
			names[strings.ToLower(e.Name())] = e.Name()
		}
		// The commands written down in the directory, by output.
		cmds := map[string]command{}
		for _, e := range ents {
			ext := strings.ToLower(filepath.Ext(e.Name()))
			if ext != ".bat" && !strings.EqualFold(e.Name(), "makefile") {
				continue
			}
			b, _ := os.ReadFile(filepath.Join(dir, e.Name()))
			for _, line := range strings.Split(strings.ReplaceAll(string(b), "\r", ""), "\n") {
				f := strings.Fields(line)
				if len(f) < 2 || !strings.EqualFold(filepath.Base(strings.ReplaceAll(f[0], `\`, "/")), "rsdlink") && !strings.EqualFold(f[0], "rsdlink.exe") {
					continue
				}
				c, err := parseCommand(f[1:])
				if err != nil {
					continue
				}
				cmds[strings.ToLower(c.out)] = c
			}
		}
		// Where commands are written down, they say which TMD came of
		// which models; elsewhere a model and a TMD of the same name are
		// taken to be one, linked at the scale the TMD records.
		type job struct {
			golden string
			c      command
			infer  bool
		}
		var jobs []job
		if len(cmds) > 0 {
			for out, c := range cmds {
				if golden, ok := names[out]; ok {
					jobs = append(jobs, job{golden, c, false})
				}
			}
		} else {
			for lower, name := range names {
				if stem, ok := strings.CutSuffix(lower, ".rsd"); ok {
					if golden, ok := names[stem+".tmd"]; ok {
						jobs = append(jobs, job{golden, command{models: []string{name}, places: []Placement{{}}}, true})
					}
				}
			}
		}
		for _, j := range jobs {
			golden, c := j.golden, j.c
			want, err := os.ReadFile(filepath.Join(dir, golden))
			if err != nil {
				return err
			}
			if j.infer {
				w, err := tmd.Decode(want)
				if err != nil || len(w.Objects) != 1 {
					continue
				}
				c.places[0].Scale = math.Ldexp(1, int(w.Objects[0].Scale))
			}
			// A command whose models are not there linked something
			// else, shipped without its sources.
			paths, ok := modelPaths(dir, c.models)
			if !ok {
				continue
			}
			tried++
			got, err := link(paths, c.places)
			if err != nil {
				t.Logf("%s: %v", filepath.Join(dir, golden), err)
				continue
			}
			if string(got) == string(want) {
				same++
				continue
			}
			if reason, ok := edited[filepath.ToSlash(filepath.Join(filepath.Base(filepath.Dir(dir)), filepath.Base(dir), golden))]; ok {
				t.Logf("%s: differs, as expected: %s", filepath.Join(dir, golden), reason)
				excused++
				continue
			}
			i := 0
			for i < len(got) && i < len(want) && got[i] == want[i] {
				i++
			}
			t.Logf("%s: differs at %#x of %d (Sony's %d): % x, Sony's % x", filepath.Join(dir, golden), i, len(got), len(want),
				got[i&^3:min(i&^3+16, len(got))], want[i&^3:min(i&^3+16, len(want))])
		}
		return nil
	})
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	t.Logf("%d of %d TMDs link byte for byte, and %d differ because their sources were edited", same, tried, excused)
	if tried == 0 || same+excused < tried {
		t.Errorf("%d of %d TMDs differ", tried-same-excused, tried)
	}
}

// modelPaths finds the RSD files a command names, as rsdlink did: in the
// directory, then in RSD under it.
func modelPaths(dir string, models []string) ([]string, bool) {
	var out []string
	for _, name := range models {
		if !strings.HasSuffix(strings.ToLower(name), ".rsd") {
			name += ".rsd"
		}
		p, err := find(dir, name)
		if err != nil {
			if p, err = find(filepath.Join(dir, "RSD"), name); err != nil {
				return nil, false
			}
		}
		out = append(out, p)
	}
	return out, true
}

// link links models into a TMD, one object each.
func link(paths []string, places []Placement) ([]byte, error) {
	var out tmd.TMD
	for i, p := range paths {
		m, err := Read(p)
		if err != nil {
			return nil, err
		}
		o, err := m.Object(places[i])
		if err != nil {
			return nil, err
		}
		out.Objects = append(out.Objects, o)
	}
	return out.Bytes(), nil
}
