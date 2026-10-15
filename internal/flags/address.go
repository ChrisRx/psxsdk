// Package flags provides command-line flag types shared by the tools.
package flags

import (
	"fmt"
	"strconv"
)

// Address is a flag holding a memory address. It is shown in hex, as
// addresses are written everywhere else, rather than as the decimal a plain
// integer flag would print, and accepts any base strconv recognises.
type Address uint32

func (a *Address) String() string { return fmt.Sprintf("0x%08X", uint32(*a)) }
func (a *Address) Type() string   { return "address" }

func (a *Address) Set(s string) error {
	v, err := strconv.ParseUint(s, 0, 32)
	if err != nil {
		return fmt.Errorf("%q is not an address", s)
	}
	*a = Address(v)
	return nil
}
