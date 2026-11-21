package toolchain

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// GDB for WebAssembly, gdb.wasm, debugs C programs (yaroze debug): it is
// built by the gdb-wasi project, under GDB's license, the GPL, so it is a
// separate program that yaroze runs rather than part of yaroze. It is
// downloaded the first time it is needed, from where the file embedded with
// EmbedGDB says, and kept in the cache directory like the full compiler.
// YAROZE_GDB names one on disk instead.
var embeddedGDB []byte

// EmbedGDB builds in the manifest of gdb.wasm to download: its URL, the
// SHA-256 and size of the gzipped module.
func EmbedGDB(manifest []byte) { embeddedGDB = manifest }

// ErrNoGDB is GDB's error when this yaroze does not say where to get it.
var ErrNoGDB = fmt.Errorf("this yaroze does not say where to get GDB for WebAssembly; set YAROZE_GDB to a gdb.wasm")

// GDB returns the path of gdb.wasm, and a directory for wazero's compiled
// code of it, downloading it the first time. stderr is told of the
// download.
func GDB(ctx context.Context, stderr io.Writer) (module, cache string, err error) {
	if path := os.Getenv("YAROZE_GDB"); path != "" {
		module, err := filepath.Abs(path)
		if err != nil {
			return "", "", err
		}
		dir, err := cacheDir()
		if err != nil {
			return "", "", err
		}
		return module, filepath.Join(dir, "wasm-gdb-local", "wazero"), nil
	}
	var m fullManifest
	if len(embeddedGDB) > 0 {
		if err := json.Unmarshal(embeddedGDB, &m); err != nil {
			return "", "", fmt.Errorf("GDB's manifest: %w", err)
		}
	}
	if m.URL == "" {
		return "", "", ErrNoGDB
	}
	dir, err := fetch(ctx, m, "wasm-gdb-", "gdb.wasm", "GDB for WebAssembly", stderr, func(string, ...any) {})
	if err != nil {
		return "", "", err
	}
	return filepath.Join(dir, "gdb.wasm"), filepath.Join(dir, "wazero"), nil
}
