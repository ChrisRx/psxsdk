package toolchain

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/ChrisRx/wazero/api"
)

// A snapshot is the compiler's memory as a build left it, written to a file
// after the first build a process makes and put back into a fresh module by
// the next process, in place of starting the compiler: the module comes up
// with the standard library parsed and type-checked, the state that is most
// of a build's time outside LLVM, and that a process keeping the module
// loaded gets for its second build. The compiler sees the working directory
// and the program through its arguments, and keys what it keeps by the
// files' contents, so the snapshot is good for any program and directory.
//
// The snapshot is about 140MB, and is a build away from being made again,
// so it lives in the temporary directory, which the system clears as it
// sees fit, rather than the cache directory. It is restored as machine
// state, so it has to be this user's alone: the directory is made with no
// permissions for anyone else and checked to be so, as is the file.
const (
	wasmPage    = 1 << 16
	snapshotMax = 1 << 30 // a memory larger than this is not kept
)

// snapshotPath is where w's snapshot is, or an error saying why there can
// be none.
func snapshotPath(w *Wasm) (string, error) {
	dir := filepath.Join(os.TempDir(), "nyc"+userSuffix())
	if err := os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return "", err
	}
	fi, err := os.Lstat(dir)
	if err != nil {
		return "", err
	}
	if !fi.IsDir() || !privateToUser(fi) {
		return "", fmt.Errorf("%s is not a directory of this user's alone", dir)
	}
	// The module cache's path is in the compiler's environment, so a
	// snapshot is for one.
	sum := sha256.Sum256([]byte(modCache()))
	return filepath.Join(dir, fmt.Sprintf("%s-%x.mem", filepath.Base(w.dir), sum[:4])), nil
}

// openSnapshot opens the snapshot at path if it is fit to restore, and
// says why not otherwise: fs.ErrNotExist when there is none.
func openSnapshot(path string) (*os.File, int64, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, 0, err
	}
	if err := checkSnapshot(fi); err != nil {
		return nil, 0, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	// Checked again as opened, in case the file changed under the name.
	if fi, err := f.Stat(); err != nil || checkSnapshot(fi) != nil {
		f.Close()
		return nil, 0, errors.New("the snapshot changed as it was opened")
	}
	return f, fi.Size(), nil
}

func checkSnapshot(fi fs.FileInfo) error {
	if !fi.Mode().IsRegular() || !privateToUser(fi) {
		return errors.New("not a regular file of this user's alone")
	}
	if fi.Size() == 0 || fi.Size()%wasmPage != 0 || fi.Size() > snapshotMax {
		return fmt.Errorf("%d bytes is not a memory's size", fi.Size())
	}
	return nil
}

// writeSnapshot writes the module's memory to path by way of a file beside
// it, so that a write cut short is never restored, and removes snapshots of
// other compilers there that have not been used for trimLimit.
func writeSnapshot(mod api.Module, path string) (int, error) {
	mem := mod.Memory()
	size := mem.Size()
	if size > snapshotMax {
		return 0, fmt.Errorf("the memory is %d MB, more than is kept", size>>20)
	}
	data, ok := mem.Read(0, size)
	if !ok {
		return 0, errors.New("the memory cannot be read")
	}
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".snapshot-*")
	if err != nil {
		return 0, err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(f.Name())
		return 0, err
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return 0, err
	}
	if err := os.Rename(f.Name(), path); err != nil {
		os.Remove(f.Name())
		return 0, err
	}
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			p := filepath.Join(dir, e.Name())
			if p == path || filepath.Ext(e.Name()) != ".mem" {
				continue
			}
			if fi, err := e.Info(); err == nil && time.Since(fi.ModTime()) > trimLimit {
				os.Remove(p)
			}
		}
	}
	return len(data), nil
}

// A restorer brings a module up with the memory a snapshot holds: context
// gives the context to instantiate the module with, and apply fills the
// memory once it is instantiated, after which the restorer is done with.
type restorer interface {
	context(ctx context.Context) context.Context
	apply(mod api.Module) error
	close()
}

// growTo has the module's memory be size bytes.
func growTo(mod api.Module, size int64) error {
	mem := mod.Memory()
	if cur := int64(mem.Size()); cur < size {
		if _, ok := mem.Grow(uint32((size - cur) / wasmPage)); !ok {
			return fmt.Errorf("the memory cannot grow to %d MB", size>>20)
		}
	}
	return nil
}

// copyRestorer reads the snapshot into the memory, which takes about 150ms
// for one of 140MB: the way where the snapshot cannot be mapped.
type copyRestorer struct {
	f    *os.File
	size int64
}

func (c *copyRestorer) context(ctx context.Context) context.Context { return ctx }

func (c *copyRestorer) apply(mod api.Module) error {
	if err := growTo(mod, c.size); err != nil {
		return err
	}
	mem := mod.Memory()
	buf := make([]byte, 8<<20)
	for at := int64(0); at < c.size; {
		n, err := c.f.ReadAt(buf, at)
		if n == 0 && err != nil {
			return err
		}
		if !mem.Write(uint32(at), buf[:n]) {
			return errors.New("the memory cannot be written")
		}
		at += int64(n)
	}
	return nil
}

func (c *copyRestorer) close() { c.f.Close() }
