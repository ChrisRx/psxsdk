package transform

// Allocations made once, for the PlayStation targets: with the leaking
// collector nothing allocated is ever freed, so an object a program
// allocates at most once might as well be a variable. HoistOnceAllocs gives
// each such allocation a zeroed global of its own, in .bss, where the
// linker places it and counts it against the program's memory, rather
// than taking it from the heap at run time. A program can then set up
// what it needs as locals in main, and pass their addresses to the
// library, and they are laid out as if they had been package-level
// variables.
//
// An allocation is made at most once if it is in a block of a function
// that runs at most once and the block is in no loop. main.main runs once,
// called by the runtime and nothing else. A function runs at most once if
// its only use is a call from such a block: a helper main calls to set
// things up. A function with any other use, a second call, a function
// value or a method table, may run any number of times.
//
// It runs after OptimizeAllocs, so what is left is what escapes or is too
// big for the stack, and only constant sizes are hoisted. Each global is
// named by its function, its size and where the allocation is, as
// main.main$once.64@main.go:12:9, for a program built with -hot: a patch is
// compiled apart from the program it is bound to and binds its variables
// to the program's by name, refusing one whose size has changed. An edit
// elsewhere leaves the name as it was, and an allocation that changed is a
// new variable rather than an error. Only main and the functions it calls
// once use these globals, and a patch does not run them again.

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/tinygo-org/tinygo/compileopts"
	"tinygo.org/x/go-llvm"
)

// HoistOnceAllocs turns allocations made at most once into globals, as
// described above. -print-allocs lists those of the functions it matches.
func HoistOnceAllocs(mod llvm.Module, config *compileopts.Config) {
	if gc := config.GC(); gc != "leaking" && gc != "none" {
		return
	}
	main := mod.NamedFunction("main.main")
	if main.IsNil() || main.IsDeclaration() || len(getUses(main)) != 1 {
		return
	}
	// In a -hot build a main package function F is a stub that calls F$body
	// through a slot (transform/hot.go): the body is what runs.
	body := func(fn llvm.Value) llvm.Value {
		if b := mod.NamedFunction(fn.Name() + "$body"); !b.IsNil() && !b.IsDeclaration() {
			return b
		}
		return fn
	}
	main = body(main)

	// The blocks that run at most once, function by function.
	once := make(map[llvm.BasicBlock]bool)
	seen := map[llvm.Value]bool{main: true}
	for work := []llvm.Value{main}; len(work) > 0; {
		fn := work[len(work)-1]
		work = work[:len(work)-1]
		for _, bb := range acyclicBlocks(fn) {
			once[bb] = true
			for inst := bb.FirstInstruction(); !inst.IsNil(); inst = llvm.NextInstruction(inst) {
				if inst.IsACallInst().IsNil() {
					continue
				}
				callee := inst.CalledValue()
				if callee.IsAFunction().IsNil() || callee.IsDeclaration() || seen[callee] || len(getUses(callee)) != 1 {
					continue
				}
				seen[callee] = true
				work = append(work, body(callee))
			}
		}
	}

	targetData := llvm.NewTargetData(mod.DataLayout())
	defer targetData.Dispose()
	ctx := mod.Context()
	complex128Type := ctx.StructType([]llvm.Type{ctx.DoubleType(), ctx.DoubleType()}, false)
	maxAlign := targetData.ABITypeAlignment(complex128Type)

	used := make(map[string]bool)
	for _, name := range []string{"runtime.alloc", "runtime.alloc_noheap"} {
		allocator := mod.NamedFunction(name)
		if allocator.IsNil() {
			continue
		}
		for _, call := range getUses(allocator) {
			if call.IsACallInst().IsNil() || call.CalledValue() != allocator || !once[call.InstructionParent()] {
				continue
			}
			if call.Operand(0).IsAConstantInt().IsNil() {
				continue
			}
			size := call.Operand(0).ZExtValue()
			if size == 0 {
				continue // OptimizeAllocs gives these runtime.zeroSizedAlloc
			}
			align := maxAlign
			if attr := call.GetCallSiteEnumAttribute(0, llvm.AttributeKindID("align")); !attr.IsNil() {
				align = int(attr.GetEnumValue())
			}
			fn := call.InstructionParent().Parent()
			global := llvm.AddGlobal(mod, llvm.ArrayType(ctx.Int8Type(), int(size)), onceName(fn, call, size, used))
			global.SetInitializer(llvm.ConstNull(global.GlobalValueType()))
			global.SetLinkage(llvm.InternalLinkage)
			global.SetAlignment(align)
			if re := config.Options.PrintAllocs; re != nil && !config.Options.PrintAllocsCover && re.MatchString(fn.Name()) {
				fmt.Fprintf(os.Stderr, "%s: object allocated once, made the global %s\n", getPosition(call), global.Name())
			}
			call.ReplaceAllUsesWith(global)
			call.EraseFromParentAsInstruction()
		}
	}
}

// onceName names the global for an allocation of size bytes made once by
// call in fn: by where the call is, from its tinygo.pos metadata or its
// debug information, or else by its order in fn. A second allocation the
// optimizer has copied from the same place is told apart by a count.
func onceName(fn, call llvm.Value, size uint64, used map[string]bool) string {
	base := fmt.Sprintf("%s$once.%d", fn.Name(), size)
	if pos := keepsError(call, "").Pos; pos.Line > 0 {
		base += fmt.Sprintf("@%s:%d:%d", filepath.Base(pos.Filename), pos.Line, pos.Column)
	}
	name := base
	for n := 1; used[name]; n++ {
		name = fmt.Sprintf("%s#%d", base, n)
	}
	used[name] = true
	return name
}

// acyclicBlocks returns the blocks of fn that are reachable from its entry
// and in no cycle: those that run at most once each time fn runs.
func acyclicBlocks(fn llvm.Value) []llvm.BasicBlock {
	var result []llvm.BasicBlock
	for _, c := range blockComponents(fn) {
		if !c.cyclic {
			result = append(result, c.blocks[0])
		}
	}
	return result
}

// A component is a strongly connected component of a function's blocks,
// and whether it is a cycle: more than one block, or one that branches to
// itself.
type component struct {
	blocks []llvm.BasicBlock
	cyclic bool
}

// blockComponents returns the strongly connected components of the blocks
// of fn that are reachable from its entry, by Tarjan's algorithm.
func blockComponents(fn llvm.Value) []component {
	index := make(map[llvm.BasicBlock]int)
	low := make(map[llvm.BasicBlock]int)
	onStack := make(map[llvm.BasicBlock]bool)
	var stack []llvm.BasicBlock
	var result []component
	var visit func(bb llvm.BasicBlock)
	visit = func(bb llvm.BasicBlock) {
		index[bb] = len(index)
		low[bb] = index[bb]
		stack = append(stack, bb)
		onStack[bb] = true
		self := false
		term := bb.LastInstruction()
		for i := 0; i < term.SuccessorsCount(); i++ {
			next := term.Successor(i)
			if next == bb {
				self = true
			}
			if _, ok := index[next]; !ok {
				visit(next)
				low[bb] = min(low[bb], low[next])
			} else if onStack[next] {
				low[bb] = min(low[bb], index[next])
			}
		}
		if low[bb] != index[bb] {
			return
		}
		var blocks []llvm.BasicBlock
		for {
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			onStack[top] = false
			blocks = append(blocks, top)
			if top == bb {
				break
			}
		}
		result = append(result, component{blocks, len(blocks) > 1 || self})
	}
	visit(fn.EntryBasicBlock())
	return result
}
