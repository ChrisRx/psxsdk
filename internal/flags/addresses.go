package flags

import (
	"fmt"
	"strconv"
	"strings"
)

// Addresses is a repeatable flag of memory addresses, each written as
// Address is: in hexadecimal with 0x, or in any base strconv recognizes.
type Addresses []uint32

func (a *Addresses) String() string {
	var specs []string
	for _, v := range *a {
		specs = append(specs, fmt.Sprintf("0x%08X", v))
	}
	return strings.Join(specs, ",")
}

func (a *Addresses) Type() string { return "address" }

func (a *Addresses) Set(s string) error {
	v, err := strconv.ParseUint(s, 0, 32)
	if err != nil {
		return fmt.Errorf("%q is not an address", s)
	}
	*a = append(*a, uint32(v))
	return nil
}
