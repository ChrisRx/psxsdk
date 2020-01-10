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

// A Symbol represents an entry in an ECOFF local symbol table section.
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

// An ExternalSymbol represents an entry in an ECOFF external symbol table
// section.
type ExternalSymbol struct {
	// JumpTable bool
	// CobolMain bool
	// WeakExt bool
	IFD int16
	Symbol
}

// A FileDescriptor represents an ECOFF file descriptor structure.
// It is used to speed mapping ofaddress to name. It should be present in every
// file, regardless of compilation options (unverified).
type FileDescriptor struct {
	Address                   int32
	FileName                  int32
	StringsOffset             int32
	StringsLength             int32
	SymbolsOffset             int32
	SymbolsCount              int32
	OptimizationSymbolsOffset int32
	OptimizationSymbolsCount  int32
	ProceduresOffset          uint16
	ProceduresCount           int16
	AuxSymbolsOffset          int32
	AuxSymbolsCount           int32
	IndirectSymbolsOffset     int32
	IndirectSymbolsCount      int32

	// 00-05 Language
	// 05-06 File can be merged
	// 06-07 File read in after creation
	// 07-08 Compiled on big-endian host machine
	// 08-10 Level compiled with
	// 10-32 Reserved
	BitFields int32

	LineOffset int32
	LineCount  int32
}

// A ProcedureDescriptor32 represents a 32-bit ECOFF file descriptor structure.
// There should be a structure representing each text label in any given 32-bit
// ECOFF file.
type ProcedureDescriptor32 struct {
	Address                     int32
	LocalSymbolsOffset          int32
	LineNumbersOffset           int32
	RegisterMask                uint32
	RegisterOffset              int32
	OptimizationSymbolsOffset   int32
	FloatingPointRegisterMask   int32
	FloatingPointRegisterOffset int32
	FrameOffset                 int32
	FrameRegister               int16
	ProgramCounterOffest        int16
	LineBegin                   int32
	LineEnd                     int32
	LineOffset                  int32
}

// A ProcedureDescriptor64 represents a 64-bit ECOFF file descriptor structure.
// There should be a structure representing each text label in any given 64-bit
// ECOFF file.
type ProcedureDescriptor64 struct {
	ProcedureDescriptor32

	// 00-08 Size of GP prologue
	// 08-09 Procedure uses GP
	// 09-10 Register frame procedure
	// 10-11 Compiled with -pg
	// 11-24 Reserved
	// 24-32 Local variable offset from vfp
	BitFields int32
}
