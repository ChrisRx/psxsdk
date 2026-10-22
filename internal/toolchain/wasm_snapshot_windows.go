//go:build !unix

package toolchain

import "io/fs"

// The temporary directory is the user's own on Windows, and what is made
// in it is theirs alone by default.
func userSuffix() string { return "" }

func privateToUser(fi fs.FileInfo) bool { return true }
