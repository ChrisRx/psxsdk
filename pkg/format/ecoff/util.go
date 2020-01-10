package ecoff

import (
	"fmt"
	"strings"
)

func Print(f *File) error {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Magic:                %x\n", f.FileHeader.Magic))
	sb.WriteString(fmt.Sprintf("NumSections:          %x\n", f.FileHeader.NumSections))
	sb.WriteString(fmt.Sprintf("Timestamp:            %x\n", f.FileHeader.Timestamp))
	sb.WriteString(fmt.Sprintf("SymbolicHeaderOffset: %x\n", f.FileHeader.SymbolicHeaderOffset))
	sb.WriteString(fmt.Sprintf("SymbolicHeaderSize:   %x\n", f.FileHeader.SymbolicHeaderSize))
	sb.WriteString(fmt.Sprintf("OptionalHeader:       %x\n", f.FileHeader.OptionalHeader))
	sb.WriteString(fmt.Sprintf("Flags:                %x\n", f.FileHeader.Flags))
	sb.WriteString("\n")

	sb.WriteString(fmt.Sprintf("Magic:     %x\n", f.ObjectHeader.Magic))
	sb.WriteString(fmt.Sprintf("Vstamp:    %x\n", f.ObjectHeader.Vstamp))
	sb.WriteString(fmt.Sprintf("TextSize:  %v\n", f.ObjectHeader.TextSize))
	sb.WriteString(fmt.Sprintf("DataSize:  %v\n", f.ObjectHeader.DataSize))
	sb.WriteString(fmt.Sprintf("BssSize:   %v\n", f.ObjectHeader.BssSize))
	sb.WriteString(fmt.Sprintf("Entry:     %x\n", f.ObjectHeader.Entry))
	sb.WriteString(fmt.Sprintf("TextStart: %x\n", f.ObjectHeader.TextStart))
	sb.WriteString(fmt.Sprintf("DataStart: %x\n", f.ObjectHeader.DataStart))
	sb.WriteString(fmt.Sprintf("BssStart:  %x\n", f.ObjectHeader.BssStart))
	sb.WriteString(fmt.Sprintf("GprMask:   %X\n", f.ObjectHeader.GprMask))
	sb.WriteString(fmt.Sprintf("CprMask:   %x\n", f.ObjectHeader.CprMask))
	sb.WriteString(fmt.Sprintf("GpValue:   %x\n", f.ObjectHeader.GpValue))
	sb.WriteString("\n")

	for _, s := range f.Sections {
		sb.WriteString(s.String())
		sb.WriteString("\n")
	}
	fmt.Printf("%s\n", sb.String())
	return nil
}

func PrintSymbolHeader(s *SymbolicHeader) error {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Magic: %X\n", s.Magic))
	sb.WriteString(fmt.Sprintf("Version: %v\n", s.Version))
	sb.WriteString(fmt.Sprintf("LineNumbersCount: %+v\n", s.LineNumbersCount))
	sb.WriteString(fmt.Sprintf("LineNumbersLength: %+v\n", s.LineNumbersLength))
	sb.WriteString(fmt.Sprintf("LineNumbersOffset: %+v\n", s.LineNumbersOffset))
	sb.WriteString(fmt.Sprintf("DenseNumbersLength: %+v\n", s.DenseNumbersLength))
	sb.WriteString(fmt.Sprintf("DenseNumbersOffset: %+v\n", s.DenseNumbersOffset))
	sb.WriteString(fmt.Sprintf("ProceduresCount: %+v\n", s.ProceduresCount))
	sb.WriteString(fmt.Sprintf("ProceduresOffset: %+v\n", s.ProceduresOffset))
	sb.WriteString(fmt.Sprintf("LocalSymbolsCount: %+v\n", s.LocalSymbolsCount))
	sb.WriteString(fmt.Sprintf("LocalSymbolsOffset: %+v\n", s.LocalSymbolsOffset))
	sb.WriteString(fmt.Sprintf("OptimizationSymbolsCount: %+v\n", s.OptimizationSymbolsCount))
	sb.WriteString(fmt.Sprintf("OptimizationSymbolsOffset: %+v\n", s.OptimizationSymbolsOffset))
	sb.WriteString(fmt.Sprintf("AuxSymbolsCount: %+v\n", s.AuxSymbolsCount))
	sb.WriteString(fmt.Sprintf("AuxSymbolsOffset: %+v\n", s.AuxSymbolsOffset))
	sb.WriteString(fmt.Sprintf("LocalStringsLength: %+v\n", s.LocalStringsLength))
	sb.WriteString(fmt.Sprintf("LocalStringsOffset: %+v\n", s.LocalStringsOffset))
	sb.WriteString(fmt.Sprintf("ExternalStringsLength: %+v\n", s.ExternalStringsLength))
	sb.WriteString(fmt.Sprintf("ExternalStringsOffset: %+v\n", s.ExternalStringsOffset))
	sb.WriteString(fmt.Sprintf("FileDescriptorLength: %+v\n", s.FileDescriptorLength))
	sb.WriteString(fmt.Sprintf("FileDescriptorOffset: %+v\n", s.FileDescriptorOffset))
	sb.WriteString(fmt.Sprintf("RelativeFileDescriptorLength: %+v\n", s.RelativeFileDescriptorLength))
	sb.WriteString(fmt.Sprintf("RelativeFileDescriptorOffset: %+v\n", s.RelativeFileDescriptorOffset))
	sb.WriteString(fmt.Sprintf("ExternalSymbolsCount: %+v\n", s.ExternalSymbolsCount))
	sb.WriteString(fmt.Sprintf("ExternalSymbolsOffset: %+v\n", s.ExternalSymbolsOffset))
	fmt.Printf("%s\n", sb.String())
	return nil
}

func extractBits(num uint32, offset, n int) uint32 {
	return ((1 << n) - 1) & (num >> offset)
}

func reverse(data []byte) []byte {
	for left, right := 0, len(data)-1; left < right; left, right = left+1, right-1 {
		data[left], data[right] = data[right], data[left]
	}
	return data
}

func splitNull(s string) []string {
	return strings.Split(s, "\000")
}
