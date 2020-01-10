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
	ECOFF_PSX_SP = 0x801fff00
	Prompt       = ">>"
)

type Conn struct {
	serial.Port
	w io.Writer
}

func NewConn(w io.Writer, baudRate int) (*Conn, error) {
	//serial.GetPortsList()
	port, err := serial.Open("/dev/ttyUSB0", &serial.Mode{BaudRate: baudRate})
	if err != nil {
		return nil, err
	}
	c := &Conn{
		Port: port,
		w:    w,
	}
	if err := c.Port.SetRTS(true); err != nil {
		return nil, err
	}
	if err := c.Clear(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Conn) Clear() error {
	return c.WriteByte(0x03)
}

func (c *Conn) ClearScreen() error {
	return c.SendCommand("\rcls")
}

func (c *Conn) Go() error {
	if _, err := c.Port.Write([]byte("go\r")); err != nil {
		return err
	}
	time.Sleep(2000 * time.Millisecond)
	return nil
}

func (c *Conn) Bwr() error {
	_, err := c.Port.Write([]byte("bwr\x0d"))
	if err != nil {
		return err
	}
	return c.ReadUntil("binary")
}

func (c *Conn) Write(data []byte) error {
	for _, b := range data {
		if err := c.WriteByte(b); err != nil {
			return err
		}
	}
	return nil
}

func (c *Conn) Handshake(addr uint32, size int32) error {
	if err := c.WriteByte(0x01); err != nil {
		return err
	}
	data := make([]byte, 8)
	binary.BigEndian.PutUint32(data[0:], addr)
	binary.BigEndian.PutUint32(data[4:], uint32(size))
	return c.Write(data)
}

func (c *Conn) SendCommand(command string) error {
	_, err := c.Port.Write([]byte(fmt.Sprintf("%s\r", command)))
	if err != nil {
		return err
	}
	return c.ReadUntil(Prompt)
}

func (c *Conn) ReadByte() (byte, error) {
	var b bytes.Buffer
	buf := make([]byte, 1024)
	for {
		status, err := c.Port.GetModemStatusBits()
		if err != nil {
			return 0, err
		}
		if !status.CTS {
			continue
		}
		n, err := c.Port.Read(buf)
		time.Sleep(100 * time.Millisecond)
		if err != nil {
			if err == io.EOF {
				break
			}
			return 0, err
		}
		c.w.Write(buf[:n])
		if n == 0 {
			return 0, errors.New("oh no")
		}
		b.Write(buf[:n])
		return buf[0], nil
	}
	return 0, errors.New("oh nos")
}

func (c *Conn) ReadUntil(seq string) error {
	var b bytes.Buffer
	buf := make([]byte, 1024)
	for {
		n, err := c.Port.Read(buf)
		time.Sleep(100 * time.Millisecond)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		c.w.Write(buf[:n])
		b.Write(buf[:n])
		//fmt.Printf("b.String() = %+v\n", b.String())

		if strings.Contains(b.String(), seq) {
			break
		}
	}
	return nil
}

func (c *Conn) WriteByte(b byte) error {
	st := time.Now()
	for {
		status, err := c.Port.GetModemStatusBits()
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
	_, err := c.Port.Write([]byte{b})
	if err != nil {
		return err
	}
	return nil
}

func (c *Conn) Load(f *ecoff.File) error {
	time.Sleep(500 * time.Millisecond)
	if err := c.SendCommand(fmt.Sprintf("sr epc %x", f.Entry)); err != nil {
		return err
	}
	if err := c.SendCommand(fmt.Sprintf("sr gp %x", f.GpValue)); err != nil {
		return err
	}
	if err := c.SendCommand(fmt.Sprintf("sr sp %x", ECOFF_PSX_SP)); err != nil {
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
		if err := c.Bwr(); err != nil {
			return err
		}

		if err := c.Handshake(s.VirtualAddress, s.Size); err != nil {
			return err
		}

		// if this isn't here it will fail with WriteByte timeout
		time.Sleep(100 * time.Millisecond)
		if err := c.SendData(data, 2048); err != nil {
			return err
		}
	}
	return nil
}

func (c *Conn) SendData(data []byte, batch int) error {
	for i := 0; i < len(data); i += batch {
		j := i + batch
		if j > len(data) {
			j = len(data)
		}

		if err := c.WriteByte(0x02); err != nil {
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
			err := c.WriteByte(b)
			if err != nil {
				return errors.Wrap(err, "SendData")
			}
			sum += b
		}
		if err := c.WriteByte(sum); err != nil {
			return err
		}
		resp, err := c.ReadByte()
		if err != nil {
			return err
		}
		if resp != 0x59 {
			return errors.Errorf("end: %d", resp)
		}
	}
	if err := c.WriteByte(0x0d); err != nil {
		return err
	}
	if err := c.ReadUntil("end binary"); err != nil {
		return err
	}
	return nil
}
