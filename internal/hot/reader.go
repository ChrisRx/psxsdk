package hot

import "bytes"

// bytesReaderAt is a bytes.Reader as an io.ReaderAt, for debug/elf.
func bytesReaderAt(data []byte) *bytes.Reader { return bytes.NewReader(data) }
