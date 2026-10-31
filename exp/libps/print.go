//go:build yaroze

package libps

// What the library prints: its messages go through putchar, as its printf
// did. kprintf takes the directives those messages use, %d, %x with a
// width and a zero fill, %s and %c, with arguments as uint32, int32 or
// string.
//
// libps 0x80044518, printf
func kprintf(format string, args ...any) {
	n := 0
	for i := 0; i < len(format); i++ {
		c := format[i]
		if c != '%' || i+1 == len(format) {
			putchar(int32(c))
			continue
		}
		i++
		width, zero := 0, false
		if format[i] == '0' {
			zero = true
			i++
		}
		for i < len(format) && format[i] >= '0' && format[i] <= '9' {
			width = width*10 + int(format[i]-'0')
			i++
		}
		if i == len(format) {
			break
		}
		var arg any
		if n < len(args) {
			arg = args[n]
			n++
		}
		switch format[i] {
		case 'd':
			v, _ := arg.(int32)
			if u, ok := arg.(uint32); ok {
				v = int32(u)
			}
			if v < 0 {
				putchar('-')
				v = -v
			}
			kputnum(uint32(v), 10, width, zero)
		case 'x':
			v, _ := arg.(uint32)
			if s, ok := arg.(int32); ok {
				v = uint32(s)
			}
			kputnum(v, 16, width, zero)
		case 's':
			s, _ := arg.(string)
			putstr(s)
		case 'c':
			v, _ := arg.(int32)
			putchar(v)
		default:
			putchar(int32(format[i]))
		}
	}
}

// kputnum prints v in base, padded to width with spaces, or zeros.
func kputnum(v uint32, base uint32, width int, zero bool) {
	var buf [12]byte
	i := len(buf)
	for {
		i--
		d := byte(v % base)
		if d < 10 {
			buf[i] = '0' + d
		} else {
			buf[i] = 'a' + d - 10
		}
		v /= base
		if v == 0 {
			break
		}
	}
	for n := len(buf) - i; n < width; n++ {
		if zero {
			putchar('0')
		} else {
			putchar(' ')
		}
	}
	for ; i < len(buf); i++ {
		putchar(int32(buf[i]))
	}
}

// putstr prints s.
func putstr(s string) {
	for i := 0; i < len(s); i++ {
		putchar(int32(s[i]))
	}
}

// putsEnabled is whether the library's own messages are printed: the
// timeouts, the CD-ROM's errors and its initialization. Nothing in the
// original ever set the word it tested (libps 0x80046C0C), so those
// messages were never seen; what a program printed, and what printf
// printed, went through regardless.
var putsEnabled bool

// kputs prints s, and "<NULL>" for an empty one, as the library's puts
// did for a null pointer, when the messages are enabled.
//
// libps 0x800106A0, 96 bytes
func kputs(s string) {
	if !putsEnabled {
		return
	}
	if s == "" {
		s = "<NULL>"
	}
	putstr(s)
}
