package ps

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

// Print adds s to the window. Text past what the window holds is
// dropped.
func (t Text) Print(s string) Text {
	fntWrite(t.id, s)
	return t
}

// Int adds n in decimal.
func (t Text) Int(n int) Text {
	var buf [12]byte
	fntWriteBytes(t.id, appendInt(buf[:0], n))
	return t
}

// appendInt appends n in decimal to b, which has room for it.
func appendInt(b []byte, n int) []byte {
	neg := n < 0
	if neg {
		n = -n
	}
	start := len(b)
	for {
		b = append(b, byte('0'+n%10))
		n /= 10
		if n == 0 {
			break
		}
	}
	if neg {
		b = append(b, '-')
	}
	for i, j := start, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
	return b
}
