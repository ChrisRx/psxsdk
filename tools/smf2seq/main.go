// Command smf2seq converts Standard MIDI Files into SEQ files, the score
// format the PlayStation's sound library plays: what Sony's smf2seq did. It
// is also yaroze's seq command.
package main

import (
	"log"

	"go.chrisrx.dev/psx/internal/cmd/smf2seq"
)

func main() {
	if err := smf2seq.Command("smf2seq").Execute(); err != nil {
		log.Fatal(err)
	}
}
