package psx

import (
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/pkg/errors"
)

var (
	ExecutableSignature = [8]byte{'P', 'S', '-', 'X', ' ', 'E', 'X', 'E'}
)

type FileHeader struct {
	Magic     [8]byte
	Text      uint32
	Data      uint32
	PC0       uint32
	GP0       uint32
	TextAddr  uint32
	TextSize  uint32
	DataAddr  uint32
	DataSize  uint32
	BSSAddr   uint32
	BSSSize   uint32
	StackAddr uint32
	StackSize uint32

	Reserved [20]byte

	ASCIIMarker [1972]byte
}

type File struct {
	FileHeader
	Sections []*Section
}

func Open(name string) (*File, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	ff, err := parseFile(f, binary.LittleEndian)
	if err != nil {
		return nil, err
	}
	return ff, nil
}

func ParseFile(r io.Reader) (*File, error) {
	return parseFile(r, binary.LittleEndian)
}

func parseFile(r io.Reader, bo binary.ByteOrder) (*File, error) {
	var f File
	if err := binary.Read(r, bo, &f.FileHeader); err != nil {
		return nil, err
	}
	if f.Magic != ExecutableSignature {
		return nil, errors.New("file magic invalid")
	}
	if f.TextSize%2048 != 0 {
		return nil, errors.New("text section must be aligned to 2048")
	}
	if f.TextSize != 0 {
		s := &Section{
			Name: "text",
			Addr: f.TextAddr,
		}
		s.Data = make([]byte, f.TextSize)
		_, err := io.ReadAtLeast(r, s.Data, len(s.Data))
		if err != nil {
			return nil, err
		}
		f.Sections = append(f.Sections, s)
	}
	return &f, nil
}

func (f *File) Bytes() []byte {
	data := make([]byte, 2048)
	copy(data, f.Magic[:])
	binary.LittleEndian.PutUint32(data[8:], f.Text)
	binary.LittleEndian.PutUint32(data[12:], f.Data)
	binary.LittleEndian.PutUint32(data[16:], f.PC0)
	binary.LittleEndian.PutUint32(data[20:], f.GP0)
	binary.LittleEndian.PutUint32(data[24:], f.TextAddr)
	binary.LittleEndian.PutUint32(data[28:], f.TextSize)
	binary.LittleEndian.PutUint32(data[32:], f.DataAddr)
	binary.LittleEndian.PutUint32(data[36:], f.DataSize)
	binary.LittleEndian.PutUint32(data[40:], f.BSSAddr)
	binary.LittleEndian.PutUint32(data[44:], f.BSSSize)
	binary.LittleEndian.PutUint32(data[48:], f.StackAddr)
	binary.LittleEndian.PutUint32(data[52:], f.StackSize)
	copy(data[56:], f.Reserved[:])
	copy(data[76:], f.ASCIIMarker[:])
	for _, s := range f.Sections {
		data = append(data, s.Data...)
	}
	return data
}

func (f *File) Size() int64 {
	n := binary.Size(&f.FileHeader)
	for _, s := range f.Sections {
		n += len(s.Data)
	}
	return int64(n)
}

func (f *File) SetMarker(s string) int {
	copy(f.FileHeader.ASCIIMarker[0:], make([]byte, 1972))
	return copy(f.FileHeader.ASCIIMarker[0:], []byte(s))
}

func (f *File) Section(name string) *Section {
	for _, s := range f.Sections {
		if s.Name == name {
			return s
		}
	}
	return &Section{Name: name, Data: make([]byte, 0)}
}

func (f *File) String() string {
	return fmt.Sprintf("PSX-EXE executable - sections=%d addr=0x%X size=%d", len(f.Sections), f.TextAddr, f.Size())
}

func (f *File) WriteFile(path string) error {
	return ioutil.WriteFile(path, f.Bytes(), 0755)
}

type Section struct {
	Name string
	Addr uint32
	Data []byte
}

func (s *Section) String() string {
	return fmt.Sprintf("name=%s addr=0x%X len=%d", s.Name, s.Addr, len(s.Data))
}
