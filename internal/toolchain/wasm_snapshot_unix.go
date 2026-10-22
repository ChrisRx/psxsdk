//go:build unix

package toolchain

import (
	"io/fs"
	"os"
	"strconv"
	"syscall"
)

// userSuffix tells this user's snapshot directory from other users' in the
// shared temporary directory.
func userSuffix() string { return "-" + strconv.Itoa(os.Getuid()) }

// privateToUser reports whether fi is this user's and gives no one else
// any permission, as a file that machine state is read from has to be.
func privateToUser(fi fs.FileInfo) bool {
	st, ok := fi.Sys().(*syscall.Stat_t)
	return ok && int(st.Uid) == os.Getuid() && fi.Mode().Perm()&0o077 == 0
}
