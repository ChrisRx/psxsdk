//go:build yaroze

package main

import "go.chrisrx.dev/psx/yaroze"

// The report goes through yaroze.Print, which is the Go library's putchar
// under both libraries, since the Go library's exports win over the
// resident image's weak definitions in this program.

// check compares what the original produced with what the Go version
// did, byte for byte, and reports.
func check(name string, want, got []byte) {
	if len(want) != len(got) {
		print("FAIL ", name, ": ", len(want), " bytes against ", len(got), "\n")
		return
	}
	for i := range want {
		if want[i] != got[i] {
			print("FAIL ", name, " at byte ", i, ": want ", hex(want[i]), " got ", hex(got[i]), "\n")
			return
		}
	}
	print("ok ", name, "\n")
}

// checkInt compares two return values.
func checkInt(name string, want, got int32) {
	if want != got {
		print("FAIL ", name, ": want ", want, " got ", got, "\n")
		return
	}
	print("ok ", name, "\n")
}

// done ends the report.
func done() {
	print("twin done\n")
}

func hex(b byte) string {
	const digits = "0123456789abcdef"
	return string([]byte{digits[b>>4], digits[b&15]})
}

var _ = yaroze.Print
