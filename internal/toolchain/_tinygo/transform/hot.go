package transform

// Hot reload for the PlayStation targets: a program built with -hot calls
// every function of its main package through a slot, a word of memory
// holding the function's address, so that a running program can be given
// new code for them. A patch, built with -hot-base against the ELF of the
// program that is running, holds the main package's functions compiled
// afresh, bound to the running program's variables and library code by
// address, and a table of the slots to point at the new functions. An
// emulator loads the patch into the memory the program reserved for it,
// and writes the table's slots.
//
// HotStubs, run in either build before the optimizer, renames each main
// package function F to F$body, which it keeps out of line, and puts a
// stub F in its place, which loads F$slot, a global initialized to F$body,
// and tail-calls what it holds. The load is volatile so that the optimizer
// does not fold it back into a direct call. Every call, function value and
// method table then goes through the slot.
//
// HotBind, run in the patch build after the optimizer, reads the base ELF's
// symbols and turns each definition the base already has into that
// address: every function but the bodies, which are what the patch is for;
// every variable that is writable, since that is the running program's
// state; and every type descriptor, since interfaces compare types by
// their addresses. Read-only data is compiled into the patch again, so
// that a changed table or string is the new one. A variable in the base
// whose size differs is an error: the running program cannot take it, and
// has to be started again. Anything the base lacks, a function added or a
// new variable, is compiled into the patch, with its own slot if it is a
// main package function. HotBind then emits __hot_patch, which the loader
// reads: the address of the library's VSync, where the program is between
// frames and safe to patch, and the slots in the base to write, each with
// the body to write there; and __hot_inits, for each bound variable whose
// initial value differs from the base's, the old and the new initial
// bytes, so that the loader can give a variable the program has not
// written since it started its new initial value, as a constant it was
// set from may have changed, while one the program has written keeps its
// state.

import (
	"debug/elf"
	"fmt"
	"strings"

	"github.com/tinygo-org/tinygo/compileopts"
	"tinygo.org/x/go-llvm"
)

// isHotFunction reports whether name is a function of the main package:
// a function, a method of one of its types, a closure or an instantiation
// of one of its generic functions.
func isHotFunction(name string) bool {
	return strings.HasPrefix(name, "main.") || strings.HasPrefix(name, "(main.") || strings.HasPrefix(name, "(*main.")
}

// HotStubs gives every function of the main package a slot and a stub, as
// described above.
func HotStubs(mod llvm.Module, config *compileopts.Config) {
	ctx := mod.Context()
	ptrType := llvm.PointerType(ctx.Int8Type(), 0)
	noinline := ctx.CreateEnumAttribute(llvm.AttributeKindID("noinline"), 0)

	var bodies []llvm.Value
	for fn := mod.FirstFunction(); !fn.IsNil(); fn = llvm.NextFunction(fn) {
		name := fn.Name()
		if fn.IsDeclaration() || !isHotFunction(name) || strings.HasSuffix(name, "$body") {
			continue
		}
		bodies = append(bodies, fn)
	}

	builder := ctx.NewBuilder()
	defer builder.Dispose()
	for _, body := range bodies {
		name := body.Name()
		fnType := body.GlobalValueType()

		stub := llvm.AddFunction(mod, name+"$stub", fnType)
		stub.SetLinkage(body.Linkage())
		stub.SetVisibility(body.Visibility())
		stub.SetUnnamedAddr(false)
		stub.SetFunctionCallConv(body.FunctionCallConv())
		AddStandardAttributes(stub, config)
		body.ReplaceAllUsesWith(stub)

		body.SetName(name + "$body")
		body.SetLinkage(llvm.InternalLinkage)
		body.SetVisibility(llvm.DefaultVisibility)
		body.RemoveEnumFunctionAttribute(llvm.AttributeKindID("alwaysinline"))
		body.RemoveEnumFunctionAttribute(llvm.AttributeKindID("inlinehint"))
		body.AddFunctionAttr(noinline)
		stub.SetName(name)

		slot := llvm.AddGlobal(mod, ptrType, name+"$slot")
		slot.SetInitializer(body)
		slot.SetLinkage(llvm.InternalLinkage)
		slot.SetAlignment(4)

		block := ctx.AddBasicBlock(stub, "entry")
		builder.SetInsertPointAtEnd(block)
		fnptr := builder.CreateLoad(ptrType, slot, "")
		fnptr.SetVolatile(true)
		call := builder.CreateCall(fnType, fnptr, stub.Params(), "")
		call.SetTailCall(true)
		call.SetInstructionCallConv(body.FunctionCallConv())
		if fnType.ReturnType().TypeKind() == llvm.VoidTypeKind {
			builder.CreateRetVoid()
		} else {
			builder.CreateRet(call)
		}
	}
}

// A hotSymbol is a definition in the base ELF.
type hotSymbol struct {
	addr, size uint32
	function   bool // code, or an address the library's table gives, rather than a variable
	writable   bool
	global     bool   // a global symbol, which a local one does not replace
	data       []byte // a writable variable's initial bytes in the base, nil when all zero
}

// readHotBase reads the definitions of a base ELF, by name.
func readHotBase(path string) (map[string]hotSymbol, error) {
	f, err := elf.Open(path)
	if err != nil {
		return nil, fmt.Errorf("hot: reading the base: %w", err)
	}
	defer f.Close()
	syms, err := f.Symbols()
	if err != nil {
		return nil, fmt.Errorf("hot: reading the base's symbols: %w", err)
	}
	out := make(map[string]hotSymbol, len(syms))
	sections := map[elf.SectionIndex][]byte{}
	for _, s := range syms {
		if s.Name == "" || s.Section == elf.SHN_UNDEF || elf.ST_TYPE(s.Info) == elf.STT_SECTION || elf.ST_TYPE(s.Info) == elf.STT_FILE {
			continue
		}
		h := hotSymbol{addr: uint32(s.Value), size: uint32(s.Size), global: elf.ST_BIND(s.Info) != elf.STB_LOCAL}
		switch typ := elf.ST_TYPE(s.Info); {
		case s.Section == elf.SHN_ABS:
			// The resident library's entry points, and its variables,
			// which the target's address table marks as objects.
			h.function = typ != elf.STT_OBJECT
		case int(s.Section) < len(f.Sections):
			sec := f.Sections[s.Section]
			h.writable = sec.Flags&elf.SHF_WRITE != 0
			h.function = typ == elf.STT_FUNC || typ == elf.STT_NOTYPE && sec.Flags&elf.SHF_EXECINSTR != 0
			if h.writable && !h.function && sec.Type != elf.SHT_NOBITS && s.Size > 0 {
				data, ok := sections[s.Section]
				if !ok {
					if data, err = sec.Data(); err != nil {
						return nil, fmt.Errorf("hot: reading the base's %s: %w", sec.Name, err)
					}
					sections[s.Section] = data
				}
				if off := s.Value - sec.Addr; off+s.Size <= uint64(len(data)) {
					h.data = data[off : off+s.Size]
				}
			}
		default:
			continue
		}
		if old, ok := out[s.Name]; ok && old.global && !h.global {
			continue
		}
		out[s.Name] = h
	}
	return out, nil
}

// hotSlotsName and hotInitsName are the globals HotBind emits for the
// loader.
const (
	hotSlotsName = "__hot_patch"
	hotInitsName = "__hot_inits"
)

// HotBind binds the module to the base ELF named by -hot-base, as described
// above, and emits __hot_patch.
func HotBind(mod llvm.Module, config *compileopts.Config) error {
	base, err := readHotBase(config.Options.HotBase)
	if err != nil {
		return err
	}
	safe, ok := base["VSync"]
	if !ok {
		return fmt.Errorf("hot: the base has no VSync: it is not a Net Yaroze program")
	}

	ctx := mod.Context()
	i32 := ctx.Int32Type()
	ptrType := llvm.PointerType(ctx.Int8Type(), 0)
	address := func(addr uint32) llvm.Value {
		return llvm.ConstIntToPtr(llvm.ConstInt(i32, uint64(addr), false), ptrType)
	}
	td := llvm.NewTargetData(mod.DataLayout())
	defer td.Dispose()

	// The lists of what to keep refer to globals by value, which binding
	// cannot express, so they go.
	for _, name := range []string{"llvm.used", "llvm.compiler.used"} {
		if g := mod.NamedGlobal(name); !g.IsNil() {
			g.EraseFromParentAsGlobal()
		}
	}

	// Functions: the bodies are the patch; the rest, when the base has
	// them, are the base's.
	var bodies, bound []llvm.Value
	for fn := mod.FirstFunction(); !fn.IsNil(); fn = llvm.NextFunction(fn) {
		name := fn.Name()
		if strings.HasSuffix(name, "$body") {
			if fn.IsDeclaration() {
				continue
			}
			fn.SetLinkage(llvm.ExternalLinkage)
			fn.SetVisibility(llvm.DefaultVisibility)
			bodies = append(bodies, fn)
			continue
		}
		s, ok := base[name]
		if !ok {
			continue // compiled into the patch, or for the linker
		}
		if !s.function {
			return fmt.Errorf("hot: %s is a function here and a variable in the base", name)
		}
		fn.ReplaceAllUsesWith(address(s.addr))
		bound = append(bound, fn)
	}
	for _, fn := range bound {
		fn.EraseFromParentAsFunction()
	}

	// Variables: the base's where they hold state or name a type. A bound
	// variable whose initial value changed is noted for the loader, with
	// the base's bytes and the new ones, the latter as a constant of the
	// patch, which the linker lays out.
	initType := ctx.StructType([]llvm.Type{ptrType, i32, ptrType, ptrType}, false)
	var inits []llvm.Value
	bound = bound[:0]
	for g := mod.FirstGlobal(); !g.IsNil(); g = llvm.NextGlobal(g) {
		name := g.Name()
		s, ok := base[name]
		if !ok || name == hotSlotsName || name == hotInitsName {
			continue
		}
		if s.function {
			if g.IsDeclaration() {
				// An entry point the program takes the address of.
				g.ReplaceAllUsesWith(address(s.addr))
				bound = append(bound, g)
				continue
			}
			return fmt.Errorf("hot: %s is a variable here and a function in the base", name)
		}
		if !s.writable && !g.IsDeclaration() && !strings.HasPrefix(name, "reflect/types.") {
			continue // read-only: compiled into the patch again
		}
		if !g.IsDeclaration() && s.size != 0 {
			if size := td.TypeAllocSize(g.GlobalValueType()); uint32(size) != s.size {
				return fmt.Errorf("hot: %s is %d bytes and the running program's is %d: the program has to be started again", name, size, s.size)
			}
			if init := g.Initializer(); s.writable && !strings.HasSuffix(name, "$slot") && !init.IsNil() && !(init.IsNull() && s.data == nil) {
				was := make([]byte, s.size)
				copy(was, s.data)
				old := llvm.AddGlobal(mod, llvm.ArrayType(ctx.Int8Type(), int(s.size)), name+"$was")
				old.SetInitializer(ctx.ConstString(string(was), false))
				old.SetLinkage(llvm.InternalLinkage)
				old.SetGlobalConstant(true)
				now := llvm.AddGlobal(mod, g.GlobalValueType(), name+"$init")
				now.SetInitializer(init)
				now.SetLinkage(llvm.InternalLinkage)
				now.SetGlobalConstant(true)
				now.SetAlignment(g.Alignment())
				inits = append(inits, ctx.ConstStruct([]llvm.Value{address(s.addr), llvm.ConstInt(i32, uint64(s.size), false), old, now}, false))
			}
		}
		g.ReplaceAllUsesWith(address(s.addr))
		bound = append(bound, g)
	}
	for _, g := range bound {
		g.EraseFromParentAsGlobal()
	}
	initsTable := llvm.AddGlobal(mod, ctx.StructType([]llvm.Type{i32, llvm.ArrayType(initType, len(inits))}, false), hotInitsName)
	initsTable.SetInitializer(ctx.ConstStruct([]llvm.Value{
		llvm.ConstInt(i32, uint64(len(inits)), false),
		llvm.ConstArray(initType, inits),
	}, false))
	initsTable.SetLinkage(llvm.ExternalLinkage)
	initsTable.SetAlignment(4)

	// The table: the slots in the base to point at the bodies. A body whose
	// slot the base lacks is a new function, whose slot the patch holds,
	// initialized to the body already.
	entryType := ctx.StructType([]llvm.Type{ptrType, ptrType}, false)
	var entries []llvm.Value
	for _, body := range bodies {
		slot, ok := base[strings.TrimSuffix(body.Name(), "$body")+"$slot"]
		if !ok {
			continue
		}
		entries = append(entries, ctx.ConstStruct([]llvm.Value{address(slot.addr), body}, false))
	}
	tableType := llvm.ArrayType(entryType, len(entries))
	patchType := ctx.StructType([]llvm.Type{i32, i32, tableType}, false)
	patch := llvm.AddGlobal(mod, patchType, hotSlotsName)
	patch.SetInitializer(ctx.ConstStruct([]llvm.Value{
		llvm.ConstInt(i32, uint64(safe.addr), false),
		llvm.ConstInt(i32, uint64(len(entries)), false),
		llvm.ConstArray(entryType, entries),
	}, false))
	patch.SetLinkage(llvm.ExternalLinkage)
	patch.SetAlignment(4)
	return nil
}
