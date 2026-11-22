package tests

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"go.chrisrx.dev/x/assert"

	"go.chrisrx.dev/psx/emu"
	"go.chrisrx.dev/psx/emu/openbios"
	"go.chrisrx.dev/psx/internal/ttyout"
)

// TestBootLogLeftOut boots a disc under OpenBIOS, and Sony's BIOS when
// PSX_SONY_BIOS names one, through the writer yaroze prints the machine's
// output with: what shows starts with what the program prints, and the
// BIOS's boot log is left out.
func TestBootLogLeftOut(t *testing.T) {
	cue := debugProgram(t, "steps.cue")
	bioses := map[string][]byte{"openbios": openbios.Image}
	if path := os.Getenv("PSX_SONY_BIOS"); path != "" {
		data, err := os.ReadFile(path)
		if !assert.NoError(t, err) {
			t.FailNow()
		}
		bioses["sony"] = data
	}
	for name, bios := range bioses {
		t.Run(name, func(t *testing.T) {
			parallel(t)
			m, err := emu.New(bios)
			if !assert.NoError(t, err) {
				t.FailNow()
			}
			disc, err := emu.OpenDisc(cue)
			if !assert.NoError(t, err) {
				t.FailNow()
			}
			defer disc.Close()
			m.InsertDisc(disc)
			var out bytes.Buffer
			m.TTY = ttyout.New(&out, false)
			for frame := 0; !strings.Contains(out.String(), "debug: second 1\n"); frame++ {
				if frame > 60*60 {
					t.Fatalf("no second after a minute of frames:\n%s", out.String())
				}
				if _, err := m.RunFrame(4_000_000); err != nil {
					t.Fatal(err)
				}
			}
			t.Log(out.String())
			got := out.String()
			assert.Equal(t, false, strings.Contains(got, "\r"), got)
			for _, boot := range []string{"KERNEL SETUP", "EXEC:PC0", "S_ADDR", "Execute !"} {
				assert.Equal(t, false, strings.Contains(got, boot), boot)
			}
			assert.Equal(t, true, strings.Contains(got, "debug: start\n"), got)
		})
	}
}
