package serial

import (
	"context"
	"encoding/binary"
	"fmt"
)

const (
	// prompt is the monitor's prompt, which ends its reply to a command.
	prompt = ">>"

	// blockSize is how much of a binary write the monitor takes at a time.
	blockSize = 2048
)

// The bytes of the monitor's binary transfers.
const (
	interrupt     = 0x03 // Ctrl-C, which stops the monitor's current command
	transferStart = 0x01 // before the address and size
	blockStart    = 0x02 // before each block
	blockOK       = 'Y'  // the answer to a block whose sum matches
	blockNG       = 'N'  // the answer to one whose sum does not, which is sent again
	transferEnd   = '\r' // in place of 01h, which ends binary mode
)

// blockTries is how many times a block is sent, or asked for, before a
// transfer gives up on it.
const blockTries = 8

// Monitor is the monitor a Net Yaroze's boot disc runs on the console's
// serial port, the one Sony's siocons talked to. It takes a line of text at
// a time at its ">>" prompt, and uploads into RAM with its bwr command.
type Monitor struct {
	port *Port

	// Progress, if set, is called after each block a binary transfer sends
	// or takes, with the segment it is of and how many of its bytes have
	// gone, which is how SIOCONS shows a transfer going.
	Progress func(seg Segment, done int)
}

// NewMonitor interrupts whatever the monitor on the other end of port was
// doing, so that it waits at its prompt, and returns it.
func NewMonitor(ctx context.Context, port *Port) (*Monitor, error) {
	if err := port.SendByte(ctx, interrupt); err != nil {
		return nil, err
	}
	return &Monitor{port: port}, nil
}

// Command sends a line to the monitor and returns its reply: the line, as
// the monitor echoes it, and what it prints after, up to and including its
// next prompt. Waiting for the echo first means a prompt the monitor
// printed before taking the line, such as while it was starting, does not
// end the reply early.
func (m *Monitor) Command(ctx context.Context, line string) (string, error) {
	m.port.Discard()
	if _, err := m.port.Write([]byte(line + "\r")); err != nil {
		return "", err
	}
	echo, err := m.port.ReadUntil(ctx, line+"\r\n")
	if err != nil {
		return "", err
	}
	reply, err := m.port.ReadUntil(ctx, prompt)
	return echo[len(echo)-len(line)-2:] + reply, err
}

// ClearScreen clears the monitor's screen with its cls command, after an
// empty line to end any the monitor had begun.
func (m *Monitor) ClearScreen(ctx context.Context) error {
	if _, err := m.Command(ctx, ""); err != nil {
		return err
	}
	_, err := m.Command(ctx, "cls")
	return err
}

// SetRegister sets one of the registers, such as epc, gp or sp, that Run
// starts the CPU with, with the monitor's sr command.
func (m *Monitor) SetRegister(ctx context.Context, name string, v uint32) error {
	_, err := m.Command(ctx, fmt.Sprintf("sr %s %x", name, v))
	return err
}

// WriteMemory writes data into RAM at addr with the monitor's bwr command,
// to which the monitor answers "binary". Then 01h, the address and the
// size, each 32 bits big-endian, and the data in 2048-byte blocks, the last
// padded with zeros, which the monitor does not store. Each block is 02h,
// the block and its 8-bit sum, which the monitor answers with Y, or N if
// the sum is wrong, and the block is sent again. A carriage return ends
// binary mode, and the monitor answers "end binary" and goes back to its
// prompt, which WriteMemory waits for, so that the monitor is ready for
// what comes next.
func (m *Monitor) WriteMemory(ctx context.Context, addr uint32, data []byte) error {
	return m.WriteSegments(ctx, Segment{Addr: addr, Data: data})
}

// WriteSegments writes each segment into RAM as WriteMemory does, all in
// one binary session: after a segment's last block, the monitor takes
// another header.
func (m *Monitor) WriteSegments(ctx context.Context, segs ...Segment) error {
	if err := m.startTransfer(ctx, "bwr"); err != nil {
		return err
	}
	for _, seg := range segs {
		if err := m.sendHeader(ctx, seg.Addr, len(seg.Data)); err != nil {
			return err
		}
		for i := 0; i < len(seg.Data); i += blockSize {
			block := make([]byte, blockSize)
			copy(block, seg.Data[i:])
			if err := m.sendBlock(ctx, block); err != nil {
				return fmt.Errorf("serial: block at %08X: %w", seg.Addr+uint32(i), err)
			}
			if m.Progress != nil {
				m.Progress(seg, min(i+blockSize, len(seg.Data)))
			}
		}
	}
	return m.endTransfer(ctx)
}

// ReadMemory reads size bytes of RAM at addr with the monitor's brd
// command, which goes as a binary write does the other way: the host sends
// 02h for each block, and the monitor sends the block and its sum, which
// the host answers with Y, or N to have it sent again. The monitor waits
// for the host to pulse RTS after each byte, which Port does.
func (m *Monitor) ReadMemory(ctx context.Context, addr uint32, size int) ([]byte, error) {
	if err := m.startTransfer(ctx, "brd"); err != nil {
		return nil, err
	}
	if err := m.sendHeader(ctx, addr, size); err != nil {
		return nil, err
	}
	data := make([]byte, 0, size+blockSize)
	for len(data) < size {
		block, err := m.receiveBlock(ctx)
		if err != nil {
			return nil, fmt.Errorf("serial: block at %08X: %w", addr+uint32(len(data)), err)
		}
		data = append(data, block...)
		if m.Progress != nil {
			m.Progress(Segment{Addr: addr, Data: data}, min(len(data), size))
		}
	}
	return data[:size], m.endTransfer(ctx)
}

// startTransfer sends a binary transfer's command and waits for the
// monitor's "binary" line.
func (m *Monitor) startTransfer(ctx context.Context, command string) error {
	m.port.Discard()
	if _, err := m.port.Write([]byte(command + "\r")); err != nil {
		return err
	}
	_, err := m.port.ReadUntil(ctx, "binary\r")
	return err
}

// sendHeader sends a transfer's address and size.
func (m *Monitor) sendHeader(ctx context.Context, addr uint32, size int) error {
	head := []byte{transferStart, 0, 0, 0, 0, 0, 0, 0, 0}
	binary.BigEndian.PutUint32(head[1:], addr)
	binary.BigEndian.PutUint32(head[5:], uint32(size))
	return m.port.Send(ctx, head)
}

// endTransfer ends binary mode and waits for the monitor's prompt.
func (m *Monitor) endTransfer(ctx context.Context) error {
	if err := m.port.SendByte(ctx, transferEnd); err != nil {
		return err
	}
	if _, err := m.port.ReadUntil(ctx, "end binary"); err != nil {
		return err
	}
	_, err := m.port.ReadUntil(ctx, prompt)
	return err
}

// sendBlock sends a block of a binary write with its sum, again for as
// long as the monitor answers N, up to blockTries times.
func (m *Monitor) sendBlock(ctx context.Context, block []byte) error {
	sum := checksum(block)
	for range blockTries {
		if err := m.port.SendByte(ctx, blockStart); err != nil {
			return err
		}
		if err := m.port.Send(ctx, block); err != nil {
			return err
		}
		if err := m.port.SendByte(ctx, sum); err != nil {
			return err
		}
		reply, err := m.port.ReceiveByte(ctx)
		if err != nil {
			return err
		}
		switch reply {
		case blockOK:
			return nil
		case blockNG:
			continue
		}
		return fmt.Errorf("serial: the monitor answered %02Xh to a block", reply)
	}
	return fmt.Errorf("serial: the monitor refused a block %d times", blockTries)
}

// receiveBlock asks for a block of a binary read and checks its sum,
// asking again for as long as it is wrong, up to blockTries times.
func (m *Monitor) receiveBlock(ctx context.Context) ([]byte, error) {
	block := make([]byte, blockSize)
	for range blockTries {
		if err := m.port.SendByte(ctx, blockStart); err != nil {
			return nil, err
		}
		for i := range block {
			b, err := m.port.ReceiveByte(ctx)
			if err != nil {
				return nil, err
			}
			block[i] = b
		}
		sum, err := m.port.ReceiveByte(ctx)
		if err != nil {
			return nil, err
		}
		if sum == checksum(block) {
			return block, m.port.SendByte(ctx, blockOK)
		}
		if err := m.port.SendByte(ctx, blockNG); err != nil {
			return nil, err
		}
	}
	return nil, fmt.Errorf("serial: a block's sum was wrong %d times", blockTries)
}

// checksum is a block's 8-bit sum.
func checksum(block []byte) uint8 {
	var sum uint8
	for _, b := range block {
		sum += b
	}
	return sum
}

// Load uploads a program as SIOCONS does: its segments in one binary
// session, each with a header of its own, then its registers, epc, gp
// unless it is 0, and sp. Run then starts it.
func (m *Monitor) Load(ctx context.Context, p *Program) error {
	if err := m.WriteSegments(ctx, p.Segments...); err != nil {
		return err
	}
	if err := m.SetRegister(ctx, "epc", p.PC); err != nil {
		return err
	}
	if p.GP != 0 {
		if err := m.SetRegister(ctx, "gp", p.GP); err != nil {
			return err
		}
	}
	return m.SetRegister(ctx, "sp", p.SP)
}

// LoadFile uploads the executable at path, of any form OpenProgram reads.
func (m *Monitor) LoadFile(ctx context.Context, path string) error {
	p, err := OpenProgram(path)
	if err != nil {
		return err
	}
	return m.Load(ctx, p)
}

// RemoteDebugger starts the monitor's remote debugger with its rdb
// command, which loads it and runs it with the stored registers, and
// returns the host's end of it once it has started.
func (m *Monitor) RemoteDebugger(ctx context.Context) (*Debugger, error) {
	m.port.Discard()
	if _, err := m.port.Write([]byte("rdb\r")); err != nil {
		return nil, err
	}
	if _, err := m.port.ReadUntil(ctx, "Exec Remote Debugger"); err != nil {
		return nil, err
	}
	if _, err := m.port.ReadUntil(ctx, "\r\n"); err != nil {
		return nil, err
	}
	return NewDebugger(m.port), nil
}

// Run starts the CPU at the registers SetRegister set, with the monitor's go
// command, and returns once the monitor has echoed it. What the program
// prints comes after, for the port to read.
func (m *Monitor) Run(ctx context.Context) error {
	m.port.Discard()
	if _, err := m.port.Write([]byte("go\r")); err != nil {
		return err
	}
	_, err := m.port.ReadUntil(ctx, "go\r\n")
	return err
}
