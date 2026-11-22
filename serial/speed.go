package serial

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	bugst "go.bug.st/serial"
)

// Speeds are the line speeds a monitor can be at, the likeliest first:
// what yaroze raises it to, and the boot disc's default.
var Speeds = []int{115200, 9600, 57600, 38400, 19200, 4800, 2400}

// FastSpeed is the fastest speed both monitors' baud command takes.
const FastSpeed = 115200

// probeTimeout is how long DetectSpeed waits for a prompt at each speed: a
// prompt is 57 bytes, which at 2400 bps take a quarter of a second.
const probeTimeout = 600 * time.Millisecond

func (d device) SetBaud(baud int) error {
	return d.SetMode(&bugst.Mode{BaudRate: baud})
}

// SetBaud changes the host's speed, on a line that can.
func (p *Port) SetBaud(baud int) error {
	l, ok := p.line.(interface{ SetBaud(int) error })
	if !ok {
		return errors.New("serial: the line's speed cannot be changed")
	}
	return l.SetBaud(baud)
}

// DetectSpeed finds the speed of the monitor waiting at its prompt at the
// other end, trying each of Speeds: a Control-C, which ends a line the
// monitor has begun with nothing in it, has it print its prompt again,
// which only comes back whole at the right speed. At a wrong one the
// monitor reads garbage, which the next Control-C clears. It leaves the
// port at the speed found.
//
// It isn't for a monitor running a program: the clean monitor resets the
// console for a Control-C then.
func (p *Port) DetectSpeed(ctx context.Context) (int, error) {
	timeout := p.Timeout
	defer func() { p.Timeout = timeout }()
	p.Timeout = probeTimeout
	for _, baud := range Speeds {
		if err := p.SetBaud(baud); err != nil {
			return 0, err
		}
		for range 2 {
			p.Discard()
			if _, err := p.Write([]byte{0x03}); err != nil {
				return 0, err
			}
			_, err := p.ReadUntil(ctx, prompt)
			if err == nil {
				return baud, nil
			}
			if ctx.Err() != nil {
				return 0, ctx.Err()
			}
		}
	}
	return 0, errors.New("serial: no monitor answered at any speed; is it at its prompt?")
}

// RaiseSpeed changes the monitor's speed, and the host's with it, with
// the monitor's baud command, which both monitors have: it switches once it
// has said so, waits for a Return at the new speed, and asks whether to
// keep the speed on the memory card, which is answered no. The console goes
// back to the card's speed when it is reset.
func (m *Monitor) RaiseSpeed(ctx context.Context, baud int) error {
	m.port.Discard()
	if _, err := m.port.Write(fmt.Appendf(nil, "baud %d\r", baud)); err != nil {
		return err
	}
	reply, err := m.port.ReadUntil(ctx, "RETURN")
	if err != nil {
		return fmt.Errorf("serial: the monitor did not change its speed: %w", err)
	}
	if strings.Contains(reply, "Invalid") {
		return fmt.Errorf("serial: the monitor does not take %d bps", baud)
	}
	// The rest of the line, sent at the old speed, then the new one.
	if err := m.port.pause(ctx, 50*time.Millisecond); err != nil {
		return err
	}
	if err := m.port.SetBaud(baud); err != nil {
		return err
	}
	if err := m.port.pause(ctx, 50*time.Millisecond); err != nil {
		return err
	}
	// A Return the monitor reads before it has switched is garbage to it,
	// so it is sent again until the question comes. A Return it reads
	// after that answers the question, no, and then the prompt comes
	// instead, with nothing more to send.
	timeout := m.port.Timeout
	defer func() { m.port.Timeout = timeout }()
	m.port.Timeout = probeTimeout
	m.port.Discard()
	asked := false
	tries := 0
	for ; tries < 10 && !asked; tries++ {
		if _, err := m.port.Write([]byte("\r")); err != nil {
			return err
		}
		_, err := m.port.ReadUntil(ctx, "( y/n ) ")
		asked = err == nil
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	if !asked {
		return fmt.Errorf("serial: the monitor did not answer at %d bps", baud)
	}
	if tries > 1 {
		if _, err := m.port.ReadUntil(ctx, prompt); err == nil {
			return nil
		}
	}
	if _, err := m.port.Write([]byte("n")); err != nil {
		return err
	}
	m.port.Timeout = timeout
	_, err = m.port.ReadUntil(ctx, prompt)
	return err
}
