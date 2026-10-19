// gen makes the benchmark's data: each raw file packed as a deflate
// stream and as an LZ4 block, each with an eight byte header of the raw
// and packed sizes, and checks both unpack back on the host.
//
//	go run ./internal/compressbench/gen internal/compressbench/data FILE...
package main

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.chrisrx.dev/psx/arc"
	"go.chrisrx.dev/psx/internal/compressbench/lz4"
)

func main() {
	out := os.Args[1]
	if err := os.MkdirAll(out, 0o755); err != nil {
		panic(err)
	}
	for _, src := range os.Args[2:] {
		raw, err := os.ReadFile(src)
		if err != nil {
			panic(err)
		}
		stem := strings.ToUpper(strings.TrimSuffix(filepath.Base(src), filepath.Ext(src)))
		if len(stem) > 8 {
			stem = stem[:8]
		}
		var b bytes.Buffer
		w, _ := flate.NewWriter(&b, flate.BestCompression)
		w.Write(raw)
		w.Close()
		dfl := b.Bytes()
		block := lz4.Encode(raw)
		check := func(name string, packed []byte, f func(dst, src []byte) int) {
			dst := make([]byte, len(raw))
			if n := f(dst, packed); n != len(raw) || !bytes.Equal(dst, raw) {
				panic(fmt.Sprintf("%s: %s does not unpack (%d of %d)", src, name, n, len(raw)))
			}
		}
		check("deflate", dfl, arc.Inflate)
		check("lz4", block, lz4.Decode)
		write := func(ext string, packed []byte) {
			var hdr [8]byte
			binary.LittleEndian.PutUint32(hdr[0:], uint32(len(raw)))
			binary.LittleEndian.PutUint32(hdr[4:], uint32(len(packed)))
			if err := os.WriteFile(filepath.Join(out, stem+ext), append(hdr[:], packed...), 0o644); err != nil {
				panic(err)
			}
			fmt.Printf("%s%s: %d -> %d (%.0f%%)\n", stem, ext, len(raw), len(packed), 100*float64(len(packed))/float64(len(raw)))
		}
		write(".DFL", dfl)
		write(".LZ4", block)
	}
}
