package serial

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// The remote debugger the monitor's rdb command starts talks HSS, the
// protocol of the gdb that came with the Net Yaroze, whose "target hss"
// speaks it: packets of a 12-byte header, the magic "HSs", a code, an
// address, a length and a checksum, all little-endian, then the data,
// padded to an even length. Every packet but an ACK or NAK is answered with
// one, and sent again after a NAK.

// The codes of HSS packets.
const (
	HSSAck    = 0
	HSSNak    = 1
	HSSOK     = 2  // a request's reply, with its data
	HSSNG     = 3  // a request failed, with a word saying why
	HSSRdReg  = 4  // the register numbered by the address
	HSSWrReg  = 5  // the register numbered by the address set to the word in the data
	HSSRdMem  = 6  // the data's word of bytes at the address
	HSSWrMem  = 7  // the data written at the address
	HSSCont   = 8  // the program carries on
	HSSStep   = 9  // the program runs one instruction
	HSSSignal = 10 // the program stopped, for the signal in the data
	HSSQuery  = 11
	HSSSend   = 12 // text the program printed, for the host to show
	HSSUnimp  = 13 // a request the target does not know
)

// The registers HSS numbers, beyond the 32 general ones.
const (
	HSSRegSR  = 32
	HSSRegLO  = 33
	HSSRegHI  = 34
	HSSRegCR  = 36
	HSSRegEPC = 37
)

// hssMax is the most data a packet carries.
const hssMax = 1500

// HSSPacket is an HSS packet, as a Debugger sends and receives them.
type HSSPacket struct {
	Code uint8
	Addr uint32
	Data []byte
}

// encode makes p's bytes, with its checksum.
func (p HSSPacket) encode() []byte {
	n := len(p.Data)
	b := make([]byte, 12+n+n&1)
	copy(b, "HSs")
	b[3] = p.Code
	binary.LittleEndian.PutUint32(b[4:], p.Addr)
	binary.LittleEndian.PutUint16(b[8:], uint16(n))
	copy(b[12:], p.Data)
	binary.LittleEndian.PutUint16(b[10:], ^hssSum(b))
	return b
}

// hssSum is the ones' complement sum of b's half-words.
func hssSum(b []byte) uint16 {
	var v uint32
	for i := 0; i+1 < len(b); i += 2 {
		v += uint32(binary.LittleEndian.Uint16(b[i:]))
	}
	for v>>16 != 0 {
		v = v&0xffff + v>>16
	}
	return uint16(v)
}

// Debugger is the host's end of the remote debugger, on a port with the
// monitor's rdb running at the other end.
type Debugger struct {
	port *Port

	// Output, if set, receives the text the program prints, which comes in
	// SEND packets while it runs.
	Output io.Writer

	// Signal is the signal the program last stopped for.
	Signal uint32
}

// NewDebugger returns the host's end of the remote debugger on port.
func NewDebugger(port *Port) *Debugger { return &Debugger{port: port} }

// receive reads a packet, answering it with an ACK, or a NAK if its sum is
// wrong, and reading the next. Text the program sends, in SEND packets,
// goes to Output.
func (d *Debugger) receive(ctx context.Context) (HSSPacket, error) {
	for {
		var head [12]byte
		for i := 0; i < len(head); {
			c, err := d.port.ReceiveByte(ctx)
			if err != nil {
				return HSSPacket{}, err
			}
			if i < 3 && c != "HSs"[i] {
				i = 0
				if c == 'H' {
					i = 1
				}
				head[0] = c
				continue
			}
			head[i] = c
			i++
		}
		n := int(binary.LittleEndian.Uint16(head[8:]))
		if n > hssMax {
			continue
		}
		b := append(head[:], make([]byte, n+n&1)...)
		for i := 12; i < len(b); i++ {
			c, err := d.port.ReceiveByte(ctx)
			if err != nil {
				return HSSPacket{}, err
			}
			b[i] = c
		}
		p := HSSPacket{Code: b[3], Addr: binary.LittleEndian.Uint32(b[4:]), Data: b[12 : 12+n]}
		if p.Code == HSSAck || p.Code == HSSNak {
			return p, nil
		}
		if hssSum(b) != 0xffff {
			if err := d.ack(HSSNak); err != nil {
				return HSSPacket{}, err
			}
			continue
		}
		if err := d.ack(HSSAck); err != nil {
			return HSSPacket{}, err
		}
		if p.Code == HSSSend {
			if d.Output != nil {
				text := p.Data
				if i := bytes.IndexByte(text, 0); i >= 0 {
					text = text[:i]
				}
				d.Output.Write(text)
			}
			continue
		}
		return p, nil
	}
}

func (d *Debugger) ack(code uint8) error {
	_, err := d.port.Write(HSSPacket{Code: code}.encode())
	return err
}

// send sends a packet until the target acknowledges it, up to four times,
// as gdb does: again after a NAK, or after the port's Timeout with no
// answer, as for a packet the target missed while it started.
func (d *Debugger) send(ctx context.Context, p HSSPacket) error {
	b := p.encode()
	for range 4 {
		if _, err := d.port.Write(b); err != nil {
			return err
		}
		r, err := d.receive(ctx)
		if errors.Is(err, ErrTimeout) {
			continue
		}
		if err != nil {
			return err
		}
		if r.Code == HSSAck {
			return nil
		}
	}
	return fmt.Errorf("serial: the debugger did not take a %d packet", p.Code)
}

// request sends a request and returns the target's reply.
func (d *Debugger) request(ctx context.Context, p HSSPacket) (HSSPacket, error) {
	if err := d.send(ctx, p); err != nil {
		return HSSPacket{}, err
	}
	r, err := d.receive(ctx)
	if err != nil {
		return r, err
	}
	switch r.Code {
	case HSSOK:
		return r, nil
	case HSSNG:
		var why uint32
		if len(r.Data) >= 4 {
			why = binary.LittleEndian.Uint32(r.Data)
		}
		return r, fmt.Errorf("serial: the debugger failed: %08x", why)
	}
	return r, fmt.Errorf("serial: the debugger answered %d", r.Code)
}

func word(v uint32) []byte { return binary.LittleEndian.AppendUint32(nil, v) }

// ReadRegister reads one of the stopped program's registers, as HSS
// numbers them.
func (d *Debugger) ReadRegister(ctx context.Context, n int) (uint32, error) {
	r, err := d.request(ctx, HSSPacket{Code: HSSRdReg, Addr: uint32(n)})
	if err != nil {
		return 0, err
	}
	if len(r.Data) < 4 {
		return 0, fmt.Errorf("serial: the debugger answered %d bytes for a register", len(r.Data))
	}
	return binary.LittleEndian.Uint32(r.Data), nil
}

// WriteRegister sets one of the stopped program's registers.
func (d *Debugger) WriteRegister(ctx context.Context, n int, v uint32) error {
	_, err := d.request(ctx, HSSPacket{Code: HSSWrReg, Addr: uint32(n), Data: word(v)})
	return err
}

// ReadMemory reads memory, at most hssMax bytes a packet.
func (d *Debugger) ReadMemory(ctx context.Context, addr uint32, n int) ([]byte, error) {
	var out []byte
	for len(out) < n {
		k := min(n-len(out), hssMax)
		r, err := d.request(ctx, HSSPacket{Code: HSSRdMem, Addr: addr + uint32(len(out)), Data: word(uint32(k))})
		if err != nil {
			return nil, err
		}
		out = append(out, r.Data[:min(k, len(r.Data))]...)
	}
	return out, nil
}

// WriteMemory writes memory, at most hssMax bytes a packet.
func (d *Debugger) WriteMemory(ctx context.Context, addr uint32, data []byte) error {
	for i := 0; i < len(data); i += hssMax {
		if _, err := d.request(ctx, HSSPacket{Code: HSSWrMem, Addr: addr + uint32(i), Data: data[i:min(i+hssMax, len(data))]}); err != nil {
			return err
		}
	}
	return nil
}

// Continue starts the program going again, and Step runs one instruction
// of it. Wait waits for it to stop.
func (d *Debugger) Continue(ctx context.Context) error { return d.send(ctx, HSSPacket{Code: HSSCont}) }
func (d *Debugger) Step(ctx context.Context) error     { return d.send(ctx, HSSPacket{Code: HSSStep}) }

// Wait waits for the program to stop, and returns the signal it stopped
// for, which it keeps in Signal too.
func (d *Debugger) Wait(ctx context.Context) (uint32, error) {
	for {
		r, err := d.receive(ctx)
		if err != nil {
			return 0, err
		}
		if r.Code == HSSSignal && len(r.Data) >= 4 {
			d.Signal = binary.LittleEndian.Uint32(r.Data)
			return d.Signal, nil
		}
	}
}

// Request sends any packet, for what the methods above do not cover, and
// returns the reply.
func (d *Debugger) Request(ctx context.Context, p HSSPacket) (HSSPacket, error) {
	return d.request(ctx, p)
}
