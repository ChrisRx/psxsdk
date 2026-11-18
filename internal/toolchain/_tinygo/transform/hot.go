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
//
// main does not return, so a function literal written in it, such as the
// frame loop's body, is the one value main made, with what it captured laid
// out as the running program's code laid it out: a patch whose literal
// captures other variables would read them wrong, and stop the program
// with nothing said. HotStubs records how each such body unpacks its
// context, in the base as a variable F$captures, and HotBind refuses a
// patch whose body unpacks it otherwise. A literal in any other function is
// made again by that function's new code, and may change as it likes.
//
// A patch holds the main package's code alone, and the rest is the running
// program's, so the two have to be compiled with the same experiments of
// the compiler (compileopts.Experiments). With onstore, the main package's
// stores to a sprite's fields pack its words and the library's drawing
// code does not, and without it the drawing code does, so a patch built
// without it for a program built with it would leave the words stale. The
// link of a program built with -hot defines a symbol for each experiment,
// __hot_experiment_onstore say, an absolute one that takes no memory, whose
// value is 1 if the experiment is on and 0 if it is off (HotExperiments),
// and HotBind refuses a patch whose experiments differ.

import (
	"debug/elf"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/tinygo-org/tinygo/compileopts"
	"github.com/tinygo-org/tinygo/compiler/llvmutil"
	"tinygo.org/x/go-llvm"
)

// hotKeepName is the function that keeps every variable as declared.
const hotKeepName = "__hot_keep"

// isHotVariable reports whether name is a variable the keeper holds as
// declared: one of the program's own packages, main and those it imports
// by a path with a domain, such as go.chrisrx.dev/psx/yaroze. The runtime's
// and the standard library's, whose import paths have no dot in their
// first element, and the compiler's own are not.
func isHotVariable(name string) bool {
	// A name the compiler made has the package before a $, as main$pack
	// has, and a variable made of an allocation its position after an @.
	if i := strings.IndexAny(name, "$@"); i >= 0 {
		name = name[:i]
	}
	return name != "" && !isStandard(name)
}

// hotExperimentPrefix starts the name of the symbol a program built with
// -hot has for each of the compiler's experiments, the experiment's name
// after it: an absolute symbol, 1 if the experiment is on and 0 if off.
const hotExperimentPrefix = "__hot_experiment_"

// HotExperiments returns the lines of a link script that define, in a
// program built with -hot, the symbols that say which of the compiler's
// experiments it was built with, for HotBind to compare a patch's with.
func HotExperiments(config *compileopts.Config) string {
	var script strings.Builder
	for _, name := range compileopts.Experiments() {
		on := 0
		if config.Experiment(name) {
			on = 1
		}
		fmt.Fprintf(&script, "%s%s = %d;\n", hotExperimentPrefix, name, on)
	}
	return script.String()
}

// experimentList writes experiments as -experiment takes them: the names
// in order, each after "no" if it is off.
func experimentList(experiments map[string]bool) string {
	var list []string
	for name, on := range experiments {
		if !on {
			name = "no" + name
		}
		list = append(list, name)
	}
	slices.SortFunc(list, func(a, b string) int {
		return strings.Compare(strings.TrimPrefix(a, "no"), strings.TrimPrefix(b, "no"))
	})
	return strings.Join(list, ",")
}

// checkHotExperiments reports an error if the patch being built does not
// have the experiments on that the base has. A base that does not say was
// built by a compiler from before it did, and is refused: nothing tells
// which experiments its library code was compiled with, and a patch that
// guessed wrong would run and draw wrong with nothing said.
func checkHotExperiments(base map[string]hotSymbol, config *compileopts.Config) error {
	theirs := map[string]bool{}
	for name, s := range base {
		if experiment, ok := strings.CutPrefix(name, hotExperimentPrefix); ok {
			theirs[experiment] = s.addr != 0
		}
	}
	if len(theirs) == 0 {
		return fmt.Errorf("hot: the running program does not say which of the compiler's experiments it was built with, so an older compiler built it: the program has to be built and started again")
	}
	ours := map[string]bool{}
	for _, name := range compileopts.Experiments() {
		ours[name] = config.Experiment(name)
	}
	if !maps.Equal(ours, theirs) {
		return fmt.Errorf("hot: the patch is built with the experiments %s and the running program was built with %s: the program has to be built and started again", experimentList(ours), experimentList(theirs))
	}
	return nil
}

// typeLayout describes a type as far as its layout goes.
func typeLayout(t llvm.Type) string {
	switch t.TypeKind() {
	case llvm.IntegerTypeKind:
		return fmt.Sprintf("i%d", t.IntTypeWidth())
	case llvm.PointerTypeKind:
		return "ptr"
	case llvm.ArrayTypeKind:
		return fmt.Sprintf("[%d x %s]", t.ArrayLength(), typeLayout(t.ElementType()))
	case llvm.StructTypeKind:
		var fields []string
		for _, f := range t.StructElementTypes() {
			fields = append(fields, typeLayout(f))
		}
		return "{" + strings.Join(fields, ", ") + "}"
	}
	return fmt.Sprintf("kind%d", t.TypeKind())
}

// inMain matches the function literals written in main itself.
var inMain = regexp.MustCompile(`^main\.main\$[0-9]+$`)

// captureLayout describes how fn unpacks the context its last parameter
// holds, before the optimizer: the types it reads through it, or "" if it
// uses none. A closure's captured variables are packed there, a single
// pointer as the context itself.
func captureLayout(fn llvm.Value) string {
	if fn.ParamsCount() == 0 {
		return ""
	}
	var parts []string
	for _, use := range getUses(fn.LastParam()) {
		switch {
		case !use.IsAGetElementPtrInst().IsNil():
			parts = append(parts, "gep "+typeLayout(use.GEPSourceElementType()))
		case !use.IsALoadInst().IsNil():
			parts = append(parts, "load "+typeLayout(use.Type()))
		default:
			parts = append(parts, "ptr")
		}
	}
	slices.Sort(parts)
	return strings.Join(slices.Compact(parts), "; ")
}

// isHotFunction reports whether name is a function of the main package:
// a function, a method of one of its types, a closure or an instantiation
// of one of its generic functions.
func isHotFunction(name string) bool {
	return strings.HasPrefix(name, "main.") || strings.HasPrefix(name, "(main.") || strings.HasPrefix(name, "(*main.")
}

// HotStubs gives every function of the main package a slot and a stub, as
// described above, and keeps every variable as it was declared.
func HotStubs(mod llvm.Module, config *compileopts.Config) {
	ctx := mod.Context()
	ptrType := llvm.PointerType(ctx.Int8Type(), 0)
	noinline := ctx.CreateEnumAttribute(llvm.AttributeKindID("noinline"), 0)

	// The optimizer narrows a variable to what the program stores in it,
	// a halfword stored 0 or 4 to a byte say, and folds one stored once
	// into its readers. A patch is built from a program that has changed,
	// so its view of what is stored, and so the width it would choose,
	// can differ from the base's, and a variable has to be the same size
	// in both for the patch to use the running program's. A volatile load
	// of a variable puts it beyond the optimizer's analysis, so a function
	// that is never called, kept through the link, loads a byte of every
	// one. In a patch build the function is in the base already, and is
	// bound away with the rest. The variables of the runtime and the
	// standard library are left to the optimizer. It links what it does
	// by proving some of them never set: the main task's defer frame,
	// without which the panic unwinding it never does would need a
	// longjmp the target has not, and the type errors.Is and errors.As
	// compare with, which kept would keep internal/reflectlite, and with
	// it the code that prints a panic's value of any type, whose errors'
	// messages allocate, and a loop the compiler makes a call of strlen,
	// which the target has not either. A patch that changes what the
	// program stores in one of them, the first defer of a program say,
	// is refused for the size where the variable is in both, and the
	// program started again. Where the base's optimizer folded the
	// variable away, the patch has one of its own, which the base's code
	// does not read.
	keeper := llvm.AddFunction(mod, hotKeepName, llvm.FunctionType(ctx.VoidType(), nil, false))
	keeper.SetLinkage(llvm.ExternalLinkage)
	keep := ctx.NewBuilder()
	defer keep.Dispose()
	keep.SetInsertPointAtEnd(ctx.AddBasicBlock(keeper, "entry"))
	for g := mod.FirstGlobal(); !g.IsNil(); g = llvm.NextGlobal(g) {
		if g.IsDeclaration() || g.IsGlobalConstant() || !isHotVariable(g.Name()) {
			continue
		}
		keep.CreateLoad(ctx.Int8Type(), g, "").SetVolatile(true)
	}
	keep.CreateRetVoid()
	llvmutil.AppendToGlobal(mod, "llvm.used", keeper)

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

		if inMain.MatchString(name) {
			// Never all zero, so that it goes in .data, which the patch reads.
			layout := "captures: " + captureLayout(body)
			if config.Options.HotBase != "" {
				body.AddFunctionAttr(ctx.CreateStringAttribute("tinygo-captures", layout))
			} else {
				captures := llvm.AddGlobal(mod, llvm.ArrayType(ctx.Int8Type(), len(layout)+1), name+"$captures")
				captures.SetInitializer(ctx.ConstString(layout, true))
				captures.SetLinkage(llvm.InternalLinkage)
				llvmutil.AppendToGlobal(mod, "llvm.used", captures)
			}
		}

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
	if err := checkHotExperiments(base, config); err != nil {
		return err
	}
	for fn := mod.FirstFunction(); !fn.IsNil(); fn = llvm.NextFunction(fn) {
		attr := fn.GetStringAttributeAtIndex(-1, "tinygo-captures")
		if attr.IsNil() {
			continue
		}
		name := strings.TrimSuffix(fn.Name(), "$body")
		was, ok := base[name+"$captures"]
		if !ok || was.data == nil {
			continue // new here, or a base built before the layouts were kept
		}
		if strings.TrimRight(string(was.data), "\x00") != attr.GetStringValue() {
			return fmt.Errorf("hot: %s captures other variables than the running program's, which made it once and goes on calling it: the program has to be started again", funcName(fn))
		}
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
