package ecoff

import (
	"fmt"
)

type SymbolicHeader struct {
	Magic                        int16
	Version                      Version
	LineNumbersCount             int32
	LineNumbersLength            int32
	LineNumbersOffset            int32
	DenseNumbersLength           int32
	DenseNumbersOffset           int32
	ProceduresCount              int32
	ProceduresOffset             int32
	LocalSymbolsCount            int32
	LocalSymbolsOffset           int32
	OptimizationSymbolsCount     int32
	OptimizationSymbolsOffset    int32
	AuxSymbolsCount              int32
	AuxSymbolsOffset             int32
	LocalStringsLength           int32
	LocalStringsOffset           int32
	ExternalStringsLength        int32
	ExternalStringsOffset        int32
	FileDescriptorLength         int32
	FileDescriptorOffset         int32
	RelativeFileDescriptorLength int32
	RelativeFileDescriptorOffset int32
	ExternalSymbolsCount         int32
	ExternalSymbolsOffset        int32
}

type FileDescriptor struct {
	Addr                          int32
	FileName                      int32
	FileNameStringSpace           int32
	BytesInSS                     int32
	BeginningSymbols              int32
	CountFilesOfSymbols           int32
	FilesOptimizationEntries      int32
	CountFilesOptimizationEntries int32
	StartProcedure                uint16
	CountProcedure                int16
	Aux                           int32
	CountAux                      int32
	IndexIndirectTable            int32
	LangAndStuff                  uint8
	OtherJunk                     [3]uint8
	LineOffset                    int32
	Line                          int32
}

type ProcedureDescriptor struct {
	Addr         int32
	Isym         int32
	Iline        int32
	Regmask      uint32
	RegOffset    int32
	Iopt         int32
	Fregmask     int32
	FregOffset   int32
	FrameOffset  int32
	Framereg     int16
	Pcreg        int16
	LnLow        int32
	LnHigh       int32
	CbLineOffset int32
}

type Symbol struct {
	Name         string
	Index        uint32
	Value        uint32
	Type         SymbolType
	StorageClass uint32
	SectionIndex uint32
}

func (s *Symbol) String() string {
	return fmt.Sprintf("%016X st %x sc %d index=%04X\t%s", s.Value, s.Type, s.StorageClass, s.SectionIndex, s.Name)
}

type ExternalSymbol struct {
	// JumpTable bool
	// CobolMain bool lol maybe add this later for the poor misguided soul that tries to run cobol on a ps1
	// WeakExt bool
	IFD int16
	Symbol
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
