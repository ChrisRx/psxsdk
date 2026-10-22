package toolchain

import (
	"context"
	"errors"
	"os"
	"syscall"
	"unsafe"

	"github.com/ChrisRx/wazero/api"
	"github.com/ChrisRx/wazero/experimental"
)

// newRestorer maps the snapshot under the module's memory, copy on write,
// so that restoring it costs nothing up front and a build reads in only
// the pages it touches. head is how many bytes from the start to put back
// after instantiation, which writes the module's data segments over them.
func newRestorer(f *os.File, size, head int64) restorer {
	return &mmapRestorer{f: f, size: size, head: min(head, size)}
}

type mmapRestorer struct {
	f          *os.File
	size, head int64
	mem        *mmapMemory
	err        error
}

func (m *mmapRestorer) context(ctx context.Context) context.Context {
	return experimental.WithMemoryAllocator(ctx, experimental.MemoryAllocatorFunc(m.allocate))
}

// allocate reserves max bytes of address space, the most the memory may
// grow to, and maps the snapshot over the start of it.
func (m *mmapRestorer) allocate(cap, max uint64) experimental.LinearMemory {
	if max < uint64(m.size) {
		max = uint64(m.size)
	}
	base, _, errno := syscall.Syscall6(syscall.SYS_MMAP, 0, uintptr(max),
		syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_PRIVATE|syscall.MAP_ANONYMOUS|syscall.MAP_NORESERVE, ^uintptr(0), 0)
	if errno != 0 {
		m.err = errno
		return nil
	}
	_, _, errno = syscall.Syscall6(syscall.SYS_MMAP, base, uintptr(m.size),
		syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_PRIVATE|syscall.MAP_FIXED, m.f.Fd(), 0)
	if errno != 0 {
		syscall.Syscall(syscall.SYS_MUNMAP, base, uintptr(max), 0)
		m.err = errno
		return nil
	}
	// The address is the mapping's, not Go memory, which is what vet
	// warns of in a plain conversion.
	start := *(*unsafe.Pointer)(unsafe.Pointer(&base))
	m.mem = &mmapMemory{buf: unsafe.Slice((*byte)(start), max), base: base, max: max}
	return m.mem
}

func (m *mmapRestorer) apply(mod api.Module) error {
	if m.err != nil {
		return m.err
	}
	if m.mem == nil {
		return errors.New("the memory was not mapped")
	}
	if err := growTo(mod, m.size); err != nil {
		return err
	}
	// The data segments went over the start of the mapping: the snapshot's
	// bytes go back, read from the file rather than the mapping, which
	// now holds the segments.
	buf := make([]byte, m.head)
	if _, err := m.f.ReadAt(buf, 0); err != nil {
		return err
	}
	if !mod.Memory().Write(0, buf) {
		return errors.New("the memory cannot be written")
	}
	return nil
}

func (m *mmapRestorer) close() { m.f.Close() }

// mmapMemory is a linear memory over a reserved mapping: growing it is
// giving out more of the mapping.
type mmapMemory struct {
	buf  []byte
	base uintptr
	max  uint64
}

func (m *mmapMemory) Reallocate(size uint64) []byte {
	if size > m.max {
		return nil
	}
	return m.buf[:size]
}

func (m *mmapMemory) Free() {
	syscall.Syscall(syscall.SYS_MUNMAP, m.base, uintptr(m.max), 0)
}
