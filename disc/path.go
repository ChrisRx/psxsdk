package disc

// cdPath writes name, a registered path such as "DATA/WALL0.TMD", into buf
// in the form the library's CdSearchFile and CdReadFile take, which is how
// the path reads on the disc: from the root, with backslashes, in upper case
// and with the ";1" version suffix, NUL terminated. It returns the length
// written, not counting the NUL. A name that does not fit is cut short, which
// names a file that is not there rather than overrunning buf.
//
// It is the one piece of the runtime half that has no need of the target,
// so it lives here where the host's tests can reach it. It must not
// allocate: a program loading its data may have no heap to speak of, and
// TinyGo puts a closure's captured variables on the heap, so there is none
// here.
func cdPath(buf []byte, name string) int {
	n := 0
	if len(name) == 0 || (name[0] != '/' && name[0] != '\\') {
		n = putc(buf, n, '\\')
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c == '/':
			c = '\\'
		case 'a' <= c && c <= 'z':
			c -= 'a' - 'A'
		}
		n = putc(buf, n, c)
	}
	if len(name) < 2 || name[len(name)-2:] != ";1" {
		n = putc(buf, n, ';')
		n = putc(buf, n, '1')
	}
	buf[n] = 0
	return n
}

// putc writes c at buf[n] if that leaves room for the terminator, and
// returns the next position.
func putc(buf []byte, n int, c byte) int {
	if n < len(buf)-1 {
		buf[n] = c
		n++
	}
	return n
}
