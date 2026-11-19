package yaroze

// A Text is a window of the console's font, which Screen.Font opened:
// what is printed to it during a frame is drawn at the frame's end. The
// methods return the window, so prints chain:
//
//	text.Print("Score: ").Int(score).Print("\n")
//
// Nothing is formatted: the standard fmt package costs more than a
// program here, so a window takes strings and integers as they are.
type Text struct {
	id int32
}

// ID is the window's stream, as FntOpen returned it, for the library's
// own font calls.
func (t Text) ID() int { return int(t.id) }

// Print adds s to the window. Text past what the window holds is
// dropped.
func (t Text) Print(s string) Text {
	fntWrite(t.id, s)
	return t
}

// Int adds n in decimal.
func (t Text) Int(n int) Text {
	fntWriteBytes(t.id, decimal(&intDigits, n))
	return t
}

// intDigits holds the digits Int prints, a package variable so that a
// frame loop that prints numbers allocates nothing.
var intDigits [20]byte

// decimal writes n in decimal at the end of buf and returns the digits.
func decimal(buf *[20]byte, n int) []byte {
	u := uint(n)
	if n < 0 {
		u = -u
	}
	i := len(buf)
	for {
		i--
		buf[i] = byte('0' + u%10)
		u /= 10
		if u == 0 {
			break
		}
	}
	if n < 0 {
		i--
		buf[i] = '-'
	}
	return buf[i:]
}
