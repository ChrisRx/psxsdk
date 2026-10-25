// Command wav2vag encodes sounds into VAG files, the SPU's ADPCM, which a
// program plays as sound effects or puts in a VAB as an instrument: what
// Sony's aiff2vag did. It is also yaroze's vag command.
package main

import (
	"log"

	"go.chrisrx.dev/psx/internal/wav2vag"
)

func main() {
	if err := wav2vag.Command("wav2vag").Execute(); err != nil {
		log.Fatal(err)
	}
}
