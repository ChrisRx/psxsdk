package tests

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.chrisrx.dev/x/assert"

	"go.chrisrx.dev/psx/internal/cmd/siocons"
)

// TestSiocons runs a batch file through SIOCONS against each monitor: data
// loaded with dload, a program with load, memory saved with dsave, lines
// typed to the monitor once it shows its prompt, the program run and its
// output shown, and SIOCONS told to quit once the monitor is back at its
// prompt after the program returns. A local command does not wait for the
// prompt, and one that talks to the monitor takes what the monitor is still
// printing as it starts, as SIOCONS's did, so the file waits for the
// prompt after dr.
func TestSiocons(t *testing.T) {
	eachMonitor(t, func(t *testing.T, boot boot) {
		program := serialProgram(t, "exit")
		m, port, mon := boot(t)
		dir := t.TempDir()
		data := make([]byte, 3000)
		for i := range data {
			data[i] = byte(i*11 + 7)
		}
		dataFile := filepath.Join(dir, "data.bin")
		saved := filepath.Join(dir, "saved.bin")
		assert.NoError(t, os.WriteFile(dataFile, data, 0o644))
		batch := filepath.Join(dir, "batch")
		assert.NoError(t, os.WriteFile(batch, []byte(fmt.Sprintf(`local echo starting
local dload %s 80100000
local load %s
dr
local wait-prompt
local dsave %s 80100010 20
go
dr
local wait-prompt
local quit
`, dataFile, program, saved)), 0o644))

		var screen bytes.Buffer
		c := siocons.New(port, mon, &screen, nil)
		ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
		defer cancel()
		assert.NoError(t, c.Run(ctx, batch))
		out := screen.String()
		t.Logf("%q", out)
		for _, want := range []string{"starting", "dload [" + dataFile + "]", "PC=80090000", "pc(epc)=80090000", "returning", "Communication baud rate 115200 bps"} {
			assert.Equal(t, true, strings.Contains(out, want), want)
		}
		got := make([]byte, len(data))
		for i := range got {
			got[i] = m.Bus.Read8(0x80100000 + uint32(i))
		}
		assert.Equal(t, data, got, "dload should have put the data in RAM")
		s, err := os.ReadFile(saved)
		assert.NoError(t, err)
		assert.Equal(t, data[0x10:0x30], s, "dsave should have saved 20h bytes from 80100010")
	})
}
