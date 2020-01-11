package yaroze

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ChrisRx/psxsdk/pkg/format/ecoff"
	"github.com/pkg/errors"
	"go.bug.st/serial"
)

const (
	ECOFF_PSX_SP uint64 = 0x801fff00
	Prompt              = ">>"
)

type PortConfig struct {
	BaudRate   int
	DeviceName string
	Output     io.Writer
}

type Port struct {
	serial.Port
	w io.Writer
}

func OpenPort(cfg *PortConfig) (*Port, error) {
	port, err := serial.Open(cfg.DeviceName, &serial.Mode{BaudRate: cfg.BaudRate})
	if err != nil {
		return nil, err
	}
	p := &Port{
		Port: port,
		w:    cfg.Output,
	}
	if err := p.Port.SetRTS(true); err != nil {
		return nil, err
	}
	if err := p.Clear(); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *Port) Clear() error {
	return p.WriteByte(0x03)
}

func (p *Port) ClearScreen() error {
	return p.SendCommand("\rcls")
}

func (p *Port) Go() error {
	if _, err := p.Port.Write([]byte("go\r")); err != nil {
		return err
	}
	time.Sleep(2000 * time.Millisecond)
	return nil
}

func (p *Port) Bwr() error {
	_, err := p.Port.Write([]byte("bwr\x0d"))
	if err != nil {
		return err
	}
	return p.ReadUntil("binary")
}

func (p *Port) Write(data []byte) error {
	for _, b := range data {
		if err := p.WriteByte(b); err != nil {
			return err
		}
	}
	return nil
}

func (p *Port) Handshake(addr uint32, size int32) error {
	if err := p.WriteByte(0x01); err != nil {
		return err
	}
	data := make([]byte, 8)
	binary.BigEndian.PutUint32(data[0:], addr)
	binary.BigEndian.PutUint32(data[4:], uint32(size))
	return p.Write(data)
}

func (p *Port) SendCommand(command string) error {
	_, err := p.Port.Write([]byte(fmt.Sprintf("%s\r", command)))
	if err != nil {
		return err
	}
	return p.ReadUntil(Prompt)
}

func (p *Port) ReadByte() (byte, error) {
	var b bytes.Buffer
	buf := make([]byte, 1024)
	for {
		status, err := p.Port.GetModemStatusBits()
		if err != nil {
			return 0, err
		}
		if !status.CTS {
			continue
		}
		n, err := p.Port.Read(buf)
		time.Sleep(100 * time.Millisecond)
		if err != nil {
			if err == io.EOF {
				break
			}
			return 0, err
		}
		p.w.Write(buf[:n])
		if n == 0 {
			return 0, errors.New("oh no")
		}
		b.Write(buf[:n])
		return buf[0], nil
	}
	return 0, errors.New("oh nos")
}

func (p *Port) ReadUntil(seq string) error {
	var b bytes.Buffer
	buf := make([]byte, 1024)
	for {
		n, err := p.Port.Read(buf)
		time.Sleep(100 * time.Millisecond)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		p.w.Write(buf[:n])
		b.Write(buf[:n])
		//fmt.Printf("b.String() = %+v\n", b.String())

		if strings.Contains(b.String(), seq) {
			break
		}
	}
	return nil
}

func (p *Port) WriteByte(b byte) error {
	st := time.Now()
	for {
		status, err := p.Port.GetModemStatusBits()
		if err != nil {
			return err
		}
		if status.CTS {
			break
		}
		if time.Since(st) > 500*time.Millisecond {
			return errors.New("WriteByte timeout")
		}
	}
	_, err := p.Port.Write([]byte{b})
	if err != nil {
		return err
	}
	return nil
}

func (p *Port) Load(f *ecoff.File) error {
	time.Sleep(500 * time.Millisecond)
	if err := p.SendCommand(fmt.Sprintf("sr epc %x", f.Entry)); err != nil {
		return err
	}
	if err := p.SendCommand(fmt.Sprintf("sr gp %x", f.GpValue)); err != nil {
		return err
	}
	if err := p.SendCommand(fmt.Sprintf("sr sp %x", ECOFF_PSX_SP)); err != nil {
		return err
	}
	for _, s := range f.Sections {
		data, err := s.Data()
		if err != nil {
			return err
		}
		if len(data) == 0 {
			continue
		}
		if err := p.Bwr(); err != nil {
			return err
		}

		if err := p.Handshake(s.VirtualAddress, s.Size); err != nil {
			return err
		}

		// if this isn't here it will fail with WriteByte timeout
		time.Sleep(100 * time.Millisecond)
		if err := p.SendData(data, 2048); err != nil {
			return err
		}
	}
	return nil
}

func (p *Port) SendData(data []byte, batch int) error {
	for i := 0; i < len(data); i += batch {
		j := i + batch
		if j > len(data) {
			j = len(data)
		}

		if err := p.WriteByte(0x02); err != nil {
			return err
		}
		chunk := data[i:j]
		if len(chunk)%2048 != 0 {
			pad := make([]byte, 2048-len(chunk))
			chunk = append(chunk, pad...)
		}

		// if this isn't here it will fail and hang
		time.Sleep(1 * time.Millisecond)
		var sum uint8
		for _, b := range chunk {
			err := p.WriteByte(b)
			if err != nil {
				return errors.Wrap(err, "SendData")
			}
			sum += b
		}
		if err := p.WriteByte(sum); err != nil {
			return err
		}
		resp, err := p.ReadByte()
		if err != nil {
			return err
		}
		if resp != 0x59 {
			return errors.Errorf("end: %d", resp)
		}
	}
	if err := p.WriteByte(0x0d); err != nil {
		return err
	}
	if err := p.ReadUntil("end binary"); err != nil {
		return err
	}
	return nil
}
