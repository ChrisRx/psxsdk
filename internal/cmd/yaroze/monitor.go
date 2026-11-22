package yaroze

import (
	"bytes"
	"compress/gzip"
	"io"

	"go.chrisrx.dev/psx/serial"
)

// embeddedMonitor is the clean monitor's ELF (serial/monitor), gzipped,
// which yaroze debug --port uploads in place of Sony's.
var embeddedMonitor []byte

// EmbedMonitor builds in the clean monitor's ELF, gzipped, as task yaroze
// writes it to cmd/yaroze/wasm/monitor.elf.gz.
func EmbedMonitor(gz []byte) { embeddedMonitor = gz }

// takeOverMonitor returns what replaces Sony's monitor with the clean one
// at baud, keeping Sony's library (serial.TakeOverMonitor), or nil if no
// monitor is built in.
func takeOverMonitor(baud int) (*serial.Program, error) {
	if len(embeddedMonitor) == 0 {
		return nil, nil
	}
	r, err := gzip.NewReader(bytes.NewReader(embeddedMonitor))
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	monitor, err := serial.ReadMonitor(bytes.NewReader(data), uint32(baud))
	if err != nil {
		return nil, err
	}
	return serial.TakeOverMonitor(monitor)
}
