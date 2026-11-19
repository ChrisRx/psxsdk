package serial

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"time"
)

// Loopback is what a serial adapter did with its transmit line joined to its
// receive line and its RTS to its CTS, with no console attached.
type Loopback struct {
	// Sent and Received count the bytes of a burst of every value, and Wrong
	// how many of those that came back differ from what went.
	Sent, Received, Wrong int

	// Follows reports whether CTS followed each change of RTS. CTSLag is how
	// long CTS took to show a change, the median and the longest, which is
	// how old a reading of CTS can be.
	Follows        bool
	CTSLag, CTSMax time.Duration

	// CTSRead and Pulse are the median time to read CTS and to turn RTS off
	// and on. RoundTrip is the median time for a byte to go out and return.
	CTSRead, Pulse, RoundTrip time.Duration

	// BlockTime is how long a block took to go a byte at a time, waiting
	// for CTS before each as an upload does, and BlockOK whether all of it
	// came back right.
	BlockTime time.Duration
	BlockOK   bool
}

// OK reports whether the adapter does what an upload and a binary read need.
func (l Loopback) OK() bool {
	return l.Sent == l.Received && l.Wrong == 0 && l.Follows && l.BlockOK
}

// RunLoopback measures the adapter behind p, whose lines must be joined as
// Loopback says. It stops at the first check the adapter can't complete, with
// what it has measured so far and the reason.
func RunLoopback(ctx context.Context, p *Port) (Loopback, error) {
	var l Loopback

	// Every value, over and over: no pacing, as fast as the adapter takes it.
	burst := make([]byte, 0, 4096)
	for len(burst) < cap(burst) {
		burst = append(burst, byte(len(burst)))
	}
	l.Sent = len(burst)
	back, err := loopWrite(ctx, p, burst)
	l.Received = len(back)
	for i := range min(len(back), len(burst)) {
		if back[i] != burst[i] {
			l.Wrong++
		}
	}
	if err != nil {
		return l, fmt.Errorf("serial: the burst: %w", err)
	}

	// CTS must follow RTS each way.
	var lags []time.Duration
	on := true
	for range 40 {
		on = !on
		lag, err := ctsLag(ctx, p, on)
		if err != nil {
			return l, fmt.Errorf("serial: CTS did not follow RTS going to %v: %w", on, err)
		}
		lags = append(lags, lag)
	}
	if !on {
		if _, err := ctsLag(ctx, p, true); err != nil {
			return l, fmt.Errorf("serial: CTS did not follow RTS going to true: %w", err)
		}
	}
	l.Follows = true
	l.CTSLag, l.CTSMax = median(lags), slices.Max(lags)

	reads := make([]time.Duration, 2000)
	for i := range reads {
		start := time.Now()
		if _, err := p.line.CTS(); err != nil {
			return l, err
		}
		reads[i] = time.Since(start)
	}
	l.CTSRead = median(reads)

	pulses := make([]time.Duration, 500)
	for i := range pulses {
		start := time.Now()
		if err := p.pulse(); err != nil {
			return l, err
		}
		pulses[i] = time.Since(start)
	}
	l.Pulse = median(pulses)

	trips := make([]time.Duration, 100)
	for i := range trips {
		start := time.Now()
		if _, err := p.line.Write([]byte{byte(i)}); err != nil {
			return l, err
		}
		if _, err := p.ReceiveByte(ctx); err != nil {
			return l, fmt.Errorf("serial: a byte's round trip: %w", err)
		}
		trips[i] = time.Since(start)
	}
	l.RoundTrip = median(trips)

	// A block as an upload sends it: CTS before every byte. What comes back
	// is read after, so a block must fit the operating system's buffer.
	block := burst[:blockSize]
	start := time.Now()
	if err := p.Send(ctx, block); err != nil {
		return l, fmt.Errorf("serial: a paced block: %w", err)
	}
	l.BlockTime = time.Since(start)
	back, err = loopRead(ctx, p, len(block))
	if err != nil {
		return l, fmt.Errorf("serial: a paced block's return: %w", err)
	}
	l.BlockOK = bytes.Equal(back, block)
	return l, nil
}

// loopWrite sends data whole, and returns what comes back.
func loopWrite(ctx context.Context, p *Port, data []byte) ([]byte, error) {
	if _, err := p.line.Write(data); err != nil {
		return nil, err
	}
	return loopRead(ctx, p, len(data))
}

// loopRead reads n bytes, or as many as arrive before the port's Timeout.
func loopRead(ctx context.Context, p *Port, n int) ([]byte, error) {
	got := make([]byte, 0, n)
	for len(got) < n {
		b, err := p.ReceiveByte(ctx)
		if err != nil {
			return got, err
		}
		got = append(got, b)
	}
	return got, nil
}

// ctsLag sets RTS and returns how long until CTS reads the same.
func ctsLag(ctx context.Context, p *Port, on bool) (time.Duration, error) {
	if err := p.line.SetRTS(on); err != nil {
		return 0, err
	}
	start := time.Now()
	for {
		cts, err := p.line.CTS()
		if err != nil {
			return 0, err
		}
		if cts == on {
			return time.Since(start), nil
		}
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		if time.Since(start) > p.Timeout {
			return 0, ErrTimeout
		}
	}
}

// median is the middle of ds, which it sorts.
func median(ds []time.Duration) time.Duration {
	slices.Sort(ds)
	return ds[len(ds)/2]
}
