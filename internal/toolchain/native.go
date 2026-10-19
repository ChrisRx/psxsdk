package toolchain

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// fork holds the files the TinyGo fork adds, under the paths they have in
// a TinyGo root: the targets and the runtime for them. The fork changes
// nothing else, so these laid over a TinyGo of the same version are the
// fork. task nyc:fork copies them from TINYGO_SRC.
//
//go:embed _tinygo
var fork embed.FS

// Native is the tinygo command.
type Native struct {
	Path string // the command
	Root string // the TinyGo root it runs with, its own or an overlay of it
	// Overlaid is whether Root is an overlay, for a TinyGo without the
	// PlayStation targets.
	Overlaid bool
}

// NewNative finds the tinygo command, NYC_TINYGO or tinygo on the PATH,
// and when it has no PlayStation targets makes an overlay of its root that
// does.
func NewNative() (*Native, error) {
	p := os.Getenv("NYC_TINYGO")
	if p == "" {
		var err error
		if p, err = exec.LookPath("tinygo"); err != nil {
			return nil, err
		}
	}
	root, err := output(p, "env", "TINYGOROOT")
	if err != nil {
		return nil, err
	}
	n := &Native{Path: p, Root: root}
	if _, err := os.Stat(filepath.Join(root, "targets", Target+".json")); err == nil {
		return n, nil
	}
	if n.Root, err = Overlay(root); err != nil {
		return nil, fmt.Errorf("adding the PlayStation targets to %s: %w", root, err)
	}
	n.Overlaid = true
	return n, nil
}

func (n *Native) Name() string { return "tinygo" }

func (n *Native) Compile(ctx context.Context, b *Build) error {
	opt := b.Opt
	if opt == "" {
		opt = DefaultOpt
	}
	args := []string{"build", "-target=" + Target, "-opt=" + opt, "-o", b.Output}
	if len(b.Tags) > 0 {
		args = append(args, "-tags="+strings.Join(b.Tags, " "))
	}
	args = append(args, b.Package)
	cmd := exec.CommandContext(ctx, n.Path, args...)
	cmd.Stdout = b.Stderr
	cmd.Stderr = b.Stderr
	cmd.Env = os.Environ()
	if n.Overlaid {
		cmd.Env = append(cmd.Env, "TINYGOROOT="+n.Root)
		b.logf("tinygo root: %s (the fork's targets over %s)", n.Root, n.Path)
	}
	start := time.Now()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tinygo build: %w", err)
	}
	b.logf("tinygo: %s", since(start))
	return nil
}

// Overlay returns a TinyGo root that is root with the fork's files added,
// made of links: a tree in the user's cache directory whose directories on
// the way to a fork file are real, and whose other entries are links into
// root. Under src every directory is real, with links only for files, since
// TinyGo walks src when it merges it with the Go standard library, and takes
// a link to a directory for a file. It is made once for each root and
// version of the fork's files.
func Overlay(root string) (string, error) {
	h := sha256.New()
	h.Write([]byte(root))
	err := fs.WalkDir(fork, "_tinygo", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fork.ReadFile(p)
		if err != nil {
			return err
		}
		fmt.Fprintf(h, "%s %d\n", p, len(data))
		h.Write(data)
		return nil
	})
	if err != nil {
		return "", err
	}
	cache, err := cacheDir()
	if err != nil {
		return "", err
	}
	dst := filepath.Join(cache, "tinygo-"+hex.EncodeToString(h.Sum(nil))[:16])
	if _, err := os.Stat(filepath.Join(dst, ".complete")); err == nil {
		return dst, nil
	}

	tmp, err := os.MkdirTemp(cache, "tinygo-tmp-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	if err := overlay(root, tmp); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(tmp, ".complete"), nil, 0o666); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, dst); err != nil {
		// Another nyc made it first.
		if _, serr := os.Stat(filepath.Join(dst, ".complete")); serr == nil {
			return dst, nil
		}
		return "", err
	}
	return dst, nil
}

// overlay builds the overlay of root in dst, an empty directory.
func overlay(root, dst string) error {
	made := map[string]bool{}
	// materialize makes dir, a slash-separated path under dst, a real
	// directory holding links to the entries of the same directory in
	// root, along with every directory above it.
	var materialize func(dir string) error
	materialize = func(dir string) error {
		if made[dir] {
			return nil
		}
		if dir != "." {
			if err := materialize(path.Dir(dir)); err != nil {
				return err
			}
		}
		made[dir] = true
		at := filepath.Join(dst, filepath.FromSlash(dir))
		if info, err := os.Lstat(at); dir != "." && err == nil && info.IsDir() {
			// Made, with its contents, by linkTree.
			return nil
		}
		if err := os.Remove(at); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		if err := os.MkdirAll(at, 0o777); err != nil {
			return err
		}
		from := filepath.Join(root, filepath.FromSlash(dir))
		entries, err := os.ReadDir(from)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		for _, e := range entries {
			child := path.Join(dir, e.Name())
			if child == "src" || strings.HasPrefix(child, "src/") {
				if err := linkTree(filepath.Join(from, e.Name()), filepath.Join(at, e.Name())); err != nil {
					return err
				}
				continue
			}
			if err := os.Symlink(filepath.Join(from, e.Name()), filepath.Join(at, e.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	return fs.WalkDir(fork, "_tinygo", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel := strings.TrimPrefix(p, "_tinygo/")
		if err := materialize(path.Dir(rel)); err != nil {
			return err
		}
		data, err := fork.ReadFile(p)
		if err != nil {
			return err
		}
		at := filepath.Join(dst, filepath.FromSlash(rel))
		if err := os.Remove(at); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return os.WriteFile(at, data, 0o666)
	})
}

// linkTree copies the tree at from to to as cp -rs does: real directories,
// and a link for each file.
func linkTree(from, to string) error {
	return filepath.WalkDir(from, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(from, p)
		if err != nil {
			return err
		}
		at := filepath.Join(to, rel)
		if d.IsDir() {
			return os.MkdirAll(at, 0o777)
		}
		return os.Symlink(p, at)
	})
}

// cacheDir is nyc's directory in the user's cache directory.
func cacheDir() (string, error) {
	dir := os.Getenv("NYC_CACHE")
	if dir == "" {
		base, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(base, "nyc")
	}
	return dir, os.MkdirAll(dir, 0o777)
}
