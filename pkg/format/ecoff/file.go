package ecoff

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
)

// A FileHeader represents an ECOFF file header.
type FileHeader struct {
	Magic                [2]byte
	NumSections          uint16
	Timestamp            int32
	SymbolicHeaderOffset uint32
	SymbolicHeaderSize   int32
	OptionalHeader       uint16
	Flags                uint16
}

// An ObjectHeader represents an ECOFF object header. This is the header
// referred to in the OptionalHeader field of FileHeader.
type ObjectHeader struct {
	Magic     int16
	Vstamp    int16
	TextSize  int32
	DataSize  int32
	BssSize   int32
	Entry     uint32
	TextStart uint32
	DataStart uint32
	BssStart  uint32
	GprMask   uint32
	CprMask   [4]uint32
	GpValue   uint32
}

// A File represents an ECOFF file.
type File struct {
	FileHeader
	ObjectHeader

	ExternalSymbols []*ExternalSymbol
	LocalSymbols    []*Symbol
	Sections        []*Section

	byteOrder binary.ByteOrder
	closer    io.Closer
}

// A SectionHeader represents an ECOFF section header.
type SectionHeader struct {
	Name              [8]uint8
	PhysicalAddress   uint32
	VirtualAddress    uint32
	Size              int32
	Offset            uint32
	RelocationsOffset uint32
	LineNumbersOffset int32
	NumRelocations    uint16
	NumLineNumbers    uint16
	Flags             int32
}

// A Section represents a single section in an ECOFF file.
type Section struct {
	SectionHeader

	io.ReaderAt
	sr *io.SectionReader
}

// Data reads and returns the contents of the ECOFF section.
func (s *Section) Data() ([]byte, error) {
	data := make([]byte, s.Size)
	n, err := io.ReadFull(s.Open(), data)
	return data[0:n], err
}

// Open returns a new ReadSeeker reading the ECOFF section.
func (s *Section) Open() io.ReadSeeker {
	return io.NewSectionReader(s.sr, 0, 1<<63-1)
}

func (s *Section) String() string {
	return fmt.Sprintf("%-10s len=%-4d offset=%-4d 0x%08X 0x%08X", s.Name, s.Size, s.Offset, s.PhysicalAddress, s.VirtualAddress)
}

func Open(name string) (*File, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	fi, err := NewFile(f)
	if err != nil {
		return nil, err
	}
	fi.closer = f
	return fi, nil
}

func NewFile(r io.ReaderAt) (*File, error) {
	sr := io.NewSectionReader(r, 0, 1<<63-1)
	f := new(File)
	if _, err := r.ReadAt(f.FileHeader.Magic[:], 0); err != nil {
		return nil, err
	}

	switch f.FileHeader.Magic {
	case MIPSBE_MAGIC, MIPSEL_BE_MAGIC:
		f.byteOrder = binary.BigEndian
	case MIPSEL_MAGIC, MIPSBE_EL_MAGIC:
		f.byteOrder = binary.LittleEndian
	default:
		return nil, errors.New("file magic invalid")
	}

	if err := binary.Read(sr, f.byteOrder, &f.FileHeader); err != nil {
		return nil, err
	}

	// TODO: should check for OptionalHeader to determine if ObjectHeader is
	// included
	if err := binary.Read(sr, f.byteOrder, &f.ObjectHeader); err != nil {
		return nil, err
	}

	// Read section headers
	for i := uint16(0); i < f.FileHeader.NumSections; i++ {
		s := new(Section)
		if err := binary.Read(sr, f.byteOrder, &s.SectionHeader); err != nil {
			return nil, err
		}
		if s.Offset == 0 {
			s.Size = 0
		}
		s.sr = io.NewSectionReader(r, int64(s.Offset), int64(s.Size))
		s.ReaderAt = s.sr
		f.Sections = append(f.Sections, s)
	}

	// Read symbolic headers
	sr.Seek(int64(f.FileHeader.SymbolicHeaderOffset), os.SEEK_SET)
	shdr := new(SymbolicHeader)
	if err := binary.Read(sr, f.byteOrder, shdr); err != nil {
		return nil, err
	}

	sr.Seek(int64(shdr.ProceduresOffset), os.SEEK_SET)
	for i := 0; i < int(shdr.ProceduresCount); i++ {
		// NOTE: Support only planned for 32-bit files.
		pd := new(ProcedureDescriptor32)
		if err := binary.Read(sr, f.byteOrder, pd); err != nil {
			return nil, err
		}
	}

	// TODO: read additional headers, such as FileDescriptor

	// Parse local strings
	ls := make([]byte, shdr.LocalStringsLength)
	sr.Seek(int64(shdr.LocalStringsOffset), os.SEEK_SET)
	if _, err := sr.Read(ls); err != nil {
		return nil, err
	}

	// Parse local symbols
	sr.Seek(int64(shdr.LocalSymbolsOffset), os.SEEK_SET)
	for i := 0; i < int(shdr.LocalSymbolsCount); i++ {
		var s [3]uint32
		if err := binary.Read(sr, f.byteOrder, &s); err != nil {
			return nil, err
		}
		sym := &Symbol{
			Index: s[0],
			Value: s[1],
		}
		switch f.byteOrder {
		case binary.LittleEndian:
			sym.Type = SymbolType(extractBits(s[2], 0, 6))
			sym.StorageClass = extractBits(s[2], 6, 5)
			sym.SectionIndex = extractBits(s[2], 12, 20)
		case binary.BigEndian:
			sym.Type = SymbolType(extractBits(s[2], 26, 6))
			sym.StorageClass = extractBits(s[2], 21, 5)
			sym.SectionIndex = extractBits(s[2], 0, 20)
		}
		sym.Name, _ = getString(ls, int(sym.Index))
		f.LocalSymbols = append(f.LocalSymbols, sym)
	}

	// Parse external strings
	es := make([]byte, shdr.ExternalStringsLength)
	sr.Seek(int64(shdr.ExternalStringsOffset), os.SEEK_SET)
	if _, err := sr.Read(es); err != nil {
		return nil, err
	}

	// Parse external symbols
	sr.Seek(int64(shdr.ExternalSymbolsOffset), os.SEEK_SET)
	for i := 0; i < int(shdr.ExternalSymbolsCount); i++ {
		var s struct {
			Unused int16
			IFD    int16
			S      [3]uint32
		}
		if err := binary.Read(sr, f.byteOrder, &s); err != nil {
			return nil, err
		}
		sym := &ExternalSymbol{
			IFD: s.IFD,
			Symbol: Symbol{
				Index: s.S[0],
				Value: s.S[1],
			},
		}
		switch f.byteOrder {
		case binary.LittleEndian:
			sym.Type = SymbolType(extractBits(s.S[2], 0, 6))
			sym.StorageClass = extractBits(s.S[2], 6, 5)
			sym.SectionIndex = extractBits(s.S[2], 12, 20)
		case binary.BigEndian:
			sym.Type = SymbolType(extractBits(s.S[2], 26, 6))
			sym.StorageClass = extractBits(s.S[2], 21, 5)
			sym.SectionIndex = extractBits(s.S[2], 0, 20)
		}
		sym.Name, _ = getString(es, int(sym.Index))
		f.ExternalSymbols = append(f.ExternalSymbols, sym)
	}

	return f, nil
}

func (f *File) Close() error {
	var err error
	if f.closer != nil {
		err = f.closer.Close()
		f.closer = nil
	}
	return err
}

// Data returns a byte slice representing all contiguous section data.
func (f *File) Data() []byte {
	data := make([]byte, 0)
	for _, s := range f.Sections {
		sdata, _ := s.Data()
		data = append(data, sdata...)
	}
	return data
}

// Size returns the number of bytes for data in all sections.
func (f *File) Size() uint32 {
	var n int
	for _, s := range f.Sections {
		n += int(s.Size)
	}
	return uint32(n)
}

func (f *File) String() string {
	var name string
	switch f.FileHeader.Magic {
	case MIPSBE_MAGIC:
		name = "MIPSBE ECOFF"
	case MIPSEL_BE_MAGIC:
		name = "MIPSEL-BE ECOFF"
	case MIPSEL_MAGIC:
		name = "MIPSEL ECOFF"
	case MIPSBE_EL_MAGIC:
		name = "MIPSBE-EL ECOFF"
	default:
		panic("invalid file magic")
	}
	return fmt.Sprintf("%s executable - start=0x%08X size=%d sections=%d", name, f.Entry, f.Size(), len(f.Sections))
}

// Symbols returns a slice of Symbols from the combined local and external
// symbol tables.
func (f *File) Symbols() []*Symbol {
	symbols := make([]*Symbol, 0)
	for _, s := range f.LocalSymbols {
		symbols = append(symbols, s)
	}
	for _, s := range f.ExternalSymbols {
		symbols = append(symbols, &s.Symbol)
	}
	return symbols
}

// Symbols returns a map of Symbols to start addresses for the given type.
func (f *File) SymbolsByType(st SymbolType) map[uint32]*Symbol {
	symbols := make(map[uint32]*Symbol)
	for _, s := range f.Symbols() {
		if s.Type != st {
			continue
		}
		symbols[s.Value] = s
	}
	return symbols
}

// getString extracts a string from an ECOFF string table.
func getString(section []byte, start int) (string, bool) {
	if start < 0 || start >= len(section) {
		return "", false
	}

	for end := start; end < len(section); end++ {
		if section[end] == 0 {
			return string(section[start:end]), true
		}
	}
	return "", false
}
