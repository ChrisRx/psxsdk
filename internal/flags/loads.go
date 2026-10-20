package flags

import (
	"fmt"
	"strings"

	"go.chrisrx.dev/psx/link"
)

// Loads is a repeatable flag of files to put in memory, each given as
// file@addr with addr in hexadecimal, as siocons' local dload did. Each
// file is read when the flag is parsed.
type Loads []link.Load

func (l *Loads) String() string {
	var specs []string
	for _, ld := range *l {
		specs = append(specs, fmt.Sprintf("%d bytes@%08x", len(ld.Data), ld.Addr))
	}
	return strings.Join(specs, ",")
}

func (l *Loads) Type() string { return "file@addr" }

func (l *Loads) Set(spec string) error {
	ld, err := link.ParseLoad(spec)
	if err != nil {
		return err
	}
	*l = append(*l, ld)
	return nil
}
