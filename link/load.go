package link

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// A Load is a file to put in memory at an address, as a Net Yaroze's
// siocons did for a program's data with its "local dload" command: the
// program's executable expected its data at the addresses its batch file
// named, and running one anywhere else means putting it there first,
// which Embed does in an image and an emulator can do in its RAM.
type Load struct {
	Addr uint32
	Data []byte
}

// ParseLoad reads a load given as file@addr, with addr in hexadecimal:
// "data1.arc@80090000".
func ParseLoad(spec string) (Load, error) {
	name, addr, ok := strings.Cut(spec, "@")
	if !ok {
		return Load{}, fmt.Errorf("load %q: want file@addr", spec)
	}
	a, err := strconv.ParseUint(strings.TrimPrefix(addr, "0x"), 16, 32)
	if err != nil {
		return Load{}, fmt.Errorf("load %q: bad address: %v", spec, err)
	}
	data, err := os.ReadFile(name)
	if err != nil {
		return Load{}, err
	}
	return Load{Addr: uint32(a), Data: data}, nil
}
