package serial

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	bugst "go.bug.st/serial"
	"go.chrisrx.dev/x/errors"
)

const (
	// DefaultTimeout is how long a Port waits for the console to send
	// something, unless its Timeout says otherwise.
	DefaultTimeout = 5 * time.Second

	// pollInterval is how long a read of the device waits before it returns
	// with nothing, so that a Port can see its context canceled or its
	// timeout passed.
	pollInterval = 50 * time.Millisecond
)

// ErrTimeout is the error a Port gives when the console has not answered in
// time.
var ErrTimeout = errors.New("serial: timed out waiting for the console")

// Line is the host's end of the cable, as a Port uses it. Read returns 0 and
// no error when nothing arrives for a while, as a serial device with a read
// timeout does, CTS reports whether the console is ready for a byte, and
// SetRTS turns the host's RTS, the console's CTS, on or off.
type Line interface {
	io.Reader
	io.Writer
	CTS() (bool, error)
	SetRTS(on bool) error
}

// Port is a serial connection to a PlayStation. Write sends bytes as they
// are, for text the console reads at its own pace. SendByte and Send wait
// for the console's CTS before each byte, for binary data it reads a byte at
// a time.
type Port struct {
	// Output receives everything the console sends, as it is read. Open and
	// NewPort set it to io.Discard.
	Output io.Writer

	// Timeout is how long a read waits for the console before it fails with
	// ErrTimeout. Open and NewPort set it to DefaultTimeout.
	Timeout time.Duration

	line    Line
	closer  io.Closer
	pending []byte // read from the line and not yet returned
}

// Devices returns the names of the host's serial devices.
func Devices() ([]string, error) {
	return bugst.GetPortsList()
}

// Open opens the serial device name, such as /dev/ttyUSB0 or COM3, at baud
// bits a second, and raises RTS to tell the console the host is there.
func Open(name string, baud int) (*Port, error) {
	d, err := bugst.Open(name, &bugst.Mode{BaudRate: baud})
	if err != nil {
		return nil, err
	}
	if err := d.SetReadTimeout(pollInterval); err != nil {
		d.Close()
		return nil, err
	}
	if err := d.SetRTS(true); err != nil {
		d.Close()
		return nil, err
	}
	p := NewPort(device{d})
	p.closer = d
	return p, nil
}

// NewPort returns a Port over line, which Close leaves open.
func NewPort(line Line) *Port {
	return &Port{Output: io.Discard, Timeout: DefaultTimeout, line: line}
}

// Close closes the device Open opened.
func (p *Port) Close() error {
	if p.closer == nil {
		return nil
	}
	return p.closer.Close()
}

// Discard drops what has been read from the console and not yet returned.
func (p *Port) Discard() {
	p.pending = p.pending[:0]
}

// Write sends data without waiting for CTS.
func (p *Port) Write(data []byte) (int, error) {
	return p.line.Write(data)
}

// SendByte sends a byte once the console raises CTS, and fails if it has not
// within the port's Timeout.
func (p *Port) SendByte(ctx context.Context, b byte) error {
	start := time.Now()
	for {
		cts, err := p.line.CTS()
		if err != nil {
			return err
		}
		if cts {
			break
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if time.Since(start) > p.Timeout {
			return fmt.Errorf("serial: sending %02Xh: %w", b, ErrTimeout)
		}
	}
	_, err := p.line.Write([]byte{b})
	return err
}

// Send sends data a byte at a time, as SendByte does.
func (p *Port) Send(ctx context.Context, data []byte) error {
	for _, b := range data {
		if err := p.SendByte(ctx, b); err != nil {
			return err
		}
	}
	return nil
}

// ReceiveByte returns the next byte the console sends.
func (p *Port) ReceiveByte(ctx context.Context) (byte, error) {
	deadline := time.Now().Add(p.Timeout)
	for len(p.pending) == 0 {
		if err := p.fill(ctx, deadline); err != nil {
			return 0, err
		}
	}
	b := p.pending[0]
	p.pending = p.pending[1:]
	return b, nil
}

// ReadUntil reads what the console sends up to and including seq, and
// returns it. What follows seq is kept for the next read.
func (p *Port) ReadUntil(ctx context.Context, seq string) (string, error) {
	deadline := time.Now().Add(p.Timeout)
	for {
		if i := bytes.Index(p.pending, []byte(seq)); i >= 0 {
			s := string(p.pending[:i+len(seq)])
			p.pending = p.pending[i+len(seq):]
			return s, nil
		}
		if err := p.fill(ctx, deadline); err != nil {
			return "", fmt.Errorf("serial: waiting for %q: %w", seq, err)
		}
	}
}

// Poll reads what the console has sent since, if anything, copying it to
// Output, waiting no longer than a read of the line does: on a serial
// device, a few tens of milliseconds. It is for a loop that does other
// things between reads, such as reading the keyboard.
func (p *Port) Poll() error {
	var buf [1024]byte
	n, err := p.line.Read(buf[:])
	if n > 0 {
		p.Output.Write(buf[:n])
		p.pending = append(p.pending, buf[:n]...)
		if err := p.pulse(); err != nil {
			return err
		}
	}
	if err == io.EOF {
		return io.ErrUnexpectedEOF
	}
	return err
}

// pulse turns RTS off and on again, as SIOCONS's receive interrupt does for
// each byte: the Net Yaroze's monitor waits for that pulse before it sends
// the next byte of a binary read.
func (p *Port) pulse() error {
	if err := p.line.SetRTS(false); err != nil {
		return err
	}
	return p.line.SetRTS(true)
}

// Stream copies what the console sends to Output until ctx is done, which
// is how a host shows what a running program prints. What was read before
// and not returned has been copied already, and is dropped. It returns
// ctx's error, or the line's if it fails first.
func (p *Port) Stream(ctx context.Context) error {
	for {
		p.Discard()
		if err := p.fill(ctx, time.Now().Add(p.Timeout)); err != nil && !errors.Is(err, ErrTimeout) {
			return err
		}
	}
}

// fill reads what the console has sent onto pending, copying it to Output.
// It fails once ctx is done or deadline passes with nothing read. Having
// read something, it pulses RTS.
func (p *Port) fill(ctx context.Context, deadline time.Time) error {
	var buf [1024]byte
	for {
		n, err := p.line.Read(buf[:])
		if n > 0 {
			p.Output.Write(buf[:n])
			p.pending = append(p.pending, buf[:n]...)
			return p.pulse()
		}
		if err == io.EOF {
			return io.ErrUnexpectedEOF
		}
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if time.Now().After(deadline) {
			return ErrTimeout
		}
	}
}

// device is a serial device as a Line.
type device struct {
	bugst.Port
}

func (d device) CTS() (bool, error) {
	s, err := d.GetModemStatusBits()
	if err != nil {
		return false, err
	}
	return s.CTS, nil
}
