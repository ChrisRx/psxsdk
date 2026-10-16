//go:build yaroze

package yaroze

// Text formatting for the on-screen font.
//
// The standard fmt package is not usable here. It pulls in reflection, which
// on a 2MB machine costs more than the program, and it allocates, which a
// frame loop must not do: with no collector running, every frame's garbage is
// permanent.
//
// A Buffer formats into storage the caller owns, so a program can keep one
// per line and reuse it forever.

// A Buffer builds a NUL terminated string in place, for passing to FntPrint.
// The zero value is not usable; give it storage with Init.
type Buffer struct {
	buf []byte
	n   int
}

// Init points the buffer at storage. One byte is reserved for the
// terminator, so the usable length is one less than len(buf).
func (b *Buffer) Init(buf []byte) {
	b.buf = buf
	b.n = 0
}

// Reset empties the buffer without touching its storage.
func (b *Buffer) Reset() {
	b.n = 0
}

// WriteString appends s, silently truncating if it will not fit. Truncating
// is the right failure here: a frame with clipped text is better than one
// that does not draw.
func (b *Buffer) WriteString(s string) *Buffer {
	for i := 0; i < len(s) && b.n < len(b.buf)-1; i++ {
		b.buf[b.n] = s[i]
		b.n++
	}
	return b
}

// WriteInt appends a signed decimal integer.
func (b *Buffer) WriteInt(v int) *Buffer {
	if v < 0 {
		b.WriteString("-")
		v = -v
	}
	// 10 digits is the most a 32-bit value needs.
	var digits [10]byte
	i := len(digits)
	for {
		i--
		digits[i] = byte('0' + v%10)
		v /= 10
		if v == 0 {
			break
		}
	}
	for ; i < len(digits); i++ {
		if b.n >= len(b.buf)-1 {
			break
		}
		b.buf[b.n] = digits[i]
		b.n++
	}
	return b
}

// Bytes returns what the buffer holds, without the terminator. It is the
// buffer's storage, good until the next write.
func (b *Buffer) Bytes() []byte {
	return b.buf[:b.n]
}

// CString terminates the buffer and returns a pointer the library can take.
func (b *Buffer) CString() *byte {
	b.buf[b.n] = 0
	return &b.buf[0]
}

// Print formats nothing and sends the buffer to the on-screen font.
func (b *Buffer) Print() {
	FntPrint(b.CString())
}

// CString returns a pointer to a NUL terminated copy of s in buf.
//
// Go string literals are not NUL terminated and the library expects C
// strings, so a path or a fixed message has to be copied once into storage
// the caller keeps alive.
func CString(buf []byte, s string) *byte {
	n := copy(buf, s)
	if n >= len(buf) {
		n = len(buf) - 1
	}
	buf[n] = 0
	return &buf[0]
}
