// Command mktargets writes the TinyGo target files yaroze builds for, one
// for each way of building a Net Yaroze program (toolchain.Modes), into a
// sysroot:
//
//	mktargets SYSROOT/tinygo/yaroze
//
// The directory is the one the files of internal/toolchain/yaroze are
// copied to, which the compiler sees as /sysroot/tinygo/yaroze, or as the
// directory -as names. The tasks that make a sysroot run it; yaroze, the
// playground and the scripts that run the compiler themselves then build
// for /sysroot/tinygo/yaroze/static.json and the others.
package main

import (
	"flag"
	"fmt"
	"os"

	"go.chrisrx.dev/psx/internal/toolchain"
)

func main() {
	as := flag.String("as", "/sysroot/tinygo/"+toolchain.TargetDir, "the directory as the compiler sees it")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: mktargets [-as DIR] DIR")
		os.Exit(2)
	}
	if err := toolchain.WriteTargets(flag.Arg(0), *as); err != nil {
		fmt.Fprintln(os.Stderr, "mktargets:", err)
		os.Exit(1)
	}
}
