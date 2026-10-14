package psx

import (
	"fmt"
	"strings"
)

func AlignTextData(exe *File, size int64) {
	if exe.Size()%size == 0 {
		return
	}
	pad := make([]byte, size-(exe.Size()%size))
	exe.Section("text").Data = append(exe.Section("text").Data, pad...)
	exe.TextSize += uint32(len(pad))
}

func Print(f *File) error {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Magic:     %s\n", f.Magic))
	sb.WriteString(fmt.Sprintf("Text:      %x\n", f.Text))
	sb.WriteString(fmt.Sprintf("Data:      %x\n", f.Data))
	sb.WriteString(fmt.Sprintf("PC0:       %x\n", f.PC0))
	sb.WriteString(fmt.Sprintf("GP0:       %x\n", f.GP0))
	sb.WriteString(fmt.Sprintf("TextAddr:  %x\n", f.TextAddr))
	sb.WriteString(fmt.Sprintf("TextSize:  %v\n", f.TextSize))
	sb.WriteString(fmt.Sprintf("DataAddr:  %x\n", f.DataAddr))
	sb.WriteString(fmt.Sprintf("DataSize:  %v\n", f.DataSize))
	sb.WriteString(fmt.Sprintf("BSSAddr:   %x\n", f.BSSAddr))
	sb.WriteString(fmt.Sprintf("BSSSize:   %v\n", f.BSSSize))
	sb.WriteString(fmt.Sprintf("StackAddr: %x\n", f.StackAddr))
	sb.WriteString(fmt.Sprintf("StackSize: %v\n", f.StackSize))
	for _, section := range f.Sections {
		sb.WriteString(section.String())
		sb.WriteString("\n")
	}
	fmt.Printf("%s\n", sb.String())
	return nil
}
