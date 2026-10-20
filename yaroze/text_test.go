package yaroze

import (
	"math"
	"testing"

	"go.chrisrx.dev/x/assert"
)

func TestBufferWriteInt(t *testing.T) {
	b := NewBuffer(64)
	b.WriteInt(0).WriteString(" ").WriteInt(-7).WriteString(" ").WriteInt(int16(-300)).
		WriteString(" ").WriteInt(uint8(255)).WriteString(" ").WriteInt(uint16(65535)).
		WriteString(" ").WriteInt(int32(math.MaxInt32)).WriteString(" ").WriteInt(int32(math.MinInt32))
	assert.Equal(t, "0 -7 -300 255 65535 2147483647 -2147483648", string(b.Bytes()))
}

func TestBufferWriteIntTruncates(t *testing.T) {
	b := NewBuffer(5)
	b.WriteInt(123456)
	assert.Equal(t, "1234", string(b.Bytes()))
}
