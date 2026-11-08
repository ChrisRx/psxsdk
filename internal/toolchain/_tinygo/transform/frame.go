package transform

// Allocation in the frame loop, for the PlayStation targets. Nothing
// allocated there is ever freed, since the targets use TinyGo's leaking
// collector, so an allocation the frame loop makes every time round runs
// the program out of memory in time, after minutes of play rather than at
// once. CheckFrameAllocs refuses it at the build.
//
// The frame loop is a loop that waits for the screen, one with a call that
// leads to the library's VSync: Screen.Frames', in package yaroze, or a
// loop of the program's own in package main. A loop in another package
// that waits, as disc.Wait does for a read, is not one. What runs there is
// the loop's blocks and the functions they call, directly or through a
// function value of the type called, such as the body of a range over
// Frames, and what those call. An allocation counts if it is in what runs
// there and is not guarded by a test the guarded code turns off, as a lazy
// one is:
//
//	if f.dst == nil {
//		f.dst = make([]uint32, n)
//	}
//
// A function with a //go:allocates pragma allocates on purpose, as a level
// load does, and the frame loop may call it: neither the call nor what the
// function does is reported. The compiler keeps it out of line, so that
// what it allocates is still found inside it after the optimizer is done.
//
// It is reported at the program's line, from the tinygo.pos metadata the
// compiler gives calls when there is no DWARF: the allocation itself in the
// program's code, or the call into the standard library that makes it, such
// as strconv.Itoa or a string concatenation. One in a package that is not
// the standard library's, such as archive.Entry.Load, is reported where the
// frame loop's code in package main calls it.

import (
	"fmt"
	"go/scanner"
	"slices"
	"strings"

	"tinygo.org/x/go-llvm"
)

// CheckFrameAllocs returns an error at each allocation the frame loop
// makes, as described above.
func CheckFrameAllocs(mod llvm.Module) []error {
	vsync := mod.NamedFunction("VSync")
	alloc := mod.NamedFunction("runtime.alloc")
	if vsync.IsNil() || alloc.IsNil() {
		return nil
	}

	// The functions that wait for the screen, and those that allocate, by
	// way of direct calls.
	waits := callersOf(vsync)
	allocates := callersOf(alloc)

	// Function values, by type, for the calls through one.
	values := make(map[llvm.Type][]llvm.Value)
	for fn := mod.FirstFunction(); !fn.IsNil(); fn = llvm.NextFunction(fn) {
		if fn.IsDeclaration() {
			continue
		}
		for _, use := range getUses(fn) {
			if use.IsACallInst().IsNil() || use.CalledValue() != fn {
				values[fn.GlobalValueType()] = append(values[fn.GlobalValueType()], fn)
				break
			}
		}
	}

	// The blocks of each frame loop, and then the functions that run in
	// one, each with the call that first reaches it, for the message.
	frameBlocks := make(map[llvm.BasicBlock]bool)
	for fn := mod.FirstFunction(); !fn.IsNil(); fn = llvm.NextFunction(fn) {
		if fn.IsDeclaration() {
			continue
		}
		for _, c := range blockComponents(fn) {
			if c.cyclic && slices.ContainsFunc(c.blocks, func(bb llvm.BasicBlock) bool { return waitsHere(bb, waits) }) {
				for _, bb := range c.blocks {
					frameBlocks[bb] = true
				}
			}
		}
	}
	via := make(map[llvm.Value]llvm.Value) // a function that runs every frame, and the call that reaches it
	var work []llvm.Value
	visit := func(call llvm.Value) {
		var callees []llvm.Value
		if callee := call.CalledValue(); !callee.IsAFunction().IsNil() {
			callees = []llvm.Value{callee}
		} else {
			callees = values[call.CalledFunctionType()]
		}
		for _, callee := range callees {
			if _, ok := via[callee]; !ok && !callee.IsDeclaration() && !allocatesOnPurpose(callee) {
				via[callee] = call
				work = append(work, callee)
			}
		}
	}
	for fn := mod.FirstFunction(); !fn.IsNil(); fn = llvm.NextFunction(fn) {
		for bb := fn.FirstBasicBlock(); !bb.IsNil(); bb = llvm.NextBasicBlock(bb) {
			if frameBlocks[bb] {
				eachCall(bb, visit)
			}
		}
	}
	for len(work) > 0 {
		fn := work[0]
		work = work[1:]
		for bb := fn.FirstBasicBlock(); !bb.IsNil(); bb = llvm.NextBasicBlock(bb) {
			eachCall(bb, visit)
		}
	}

	var errs []error
	reported := make(map[string]bool)
	check := func(call llvm.Value) {
		callee := call.CalledValue()
		if callee.IsAFunction().IsNil() || !allocates[callee] && callee != alloc || allocatesOnPurpose(callee) {
			return
		}
		fn := call.InstructionParent().Parent()
		if isStandard(fn.Name()) || !isStandard(callee.Name()) && callee != alloc {
			return // reported where the program's code calls it, or inside
		}
		if guarded(call) {
			return
		}
		what := "this"
		if callee != alloc {
			what = allocName(callee.Name())
		}
		site, in := call, ""
		if callPackage(call) != "main" {
			// In a package of its own: report it at the call in package
			// main that leads to it, if there is one, and otherwise say
			// whose code it is in. Code inlined into package main's has
			// lost the call it came from.
			in = ", in " + funcName(fn)
			if !strings.Contains(fn.Name(), "$") {
				in = ", in " + shortName(fn.Name())
			}
			for c := via[fn]; !c.IsNil() && packageOf(fn.Name()) != "main"; c = via[c.InstructionParent().Parent()] {
				if callPackage(c) == "main" {
					what = "this call"
					if callee := c.CalledValue(); !callee.IsAFunction().IsNil() {
						what = shortName(callee.Name())
					}
					site, in = c, ""
					break
				}
			}
		}
		msg := fmt.Sprintf("%s allocates each time the frame loop runs it%s, and nothing is ever freed: make what it makes once, before the loop, and reuse it", what, in)
		err := keepsError(site, msg)
		// One line's allocations are one report: a closure and the
		// variables it captures, say.
		key := fmt.Sprintf("%s:%d %s", err.Pos.Filename, err.Pos.Line, msg)
		if !reported[key] {
			reported[key] = true
			errs = append(errs, err)
		}
	}
	for fn := mod.FirstFunction(); !fn.IsNil(); fn = llvm.NextFunction(fn) {
		_, every := via[fn]
		for bb := fn.FirstBasicBlock(); !bb.IsNil(); bb = llvm.NextBasicBlock(bb) {
			if every || frameBlocks[bb] {
				eachCall(bb, check)
			}
		}
	}
	sortErrors(errs)
	return errs
}

// sortErrors sorts errors, each a scanner.Error, by file and line.
func sortErrors(errs []error) {
	slices.SortStableFunc(errs, func(a, b error) int {
		pa, pb := a.(scanner.Error).Pos, b.(scanner.Error).Pos
		if c := strings.Compare(pa.Filename, pb.Filename); c != 0 {
			return c
		}
		if pa.Line != pb.Line {
			return pa.Line - pb.Line
		}
		return pa.Column - pb.Column
	})
}

// allocatesOnPurpose reports whether fn has a //go:allocates pragma, which
// says it allocates as a load does, and that the frame loop may call it.
func allocatesOnPurpose(fn llvm.Value) bool {
	return !fn.GetStringAttributeAtIndex(-1, "tinygo-allocates").IsNil()
}

// callersOf returns fn and the functions that call it, directly or through
// functions that do.
func callersOf(fn llvm.Value) map[llvm.Value]bool {
	set := map[llvm.Value]bool{fn: true}
	work := []llvm.Value{fn}
	for len(work) > 0 {
		f := work[len(work)-1]
		work = work[:len(work)-1]
		for _, use := range getUses(f) {
			if use.IsACallInst().IsNil() || use.CalledValue() != f {
				continue
			}
			caller := use.InstructionParent().Parent()
			if !set[caller] {
				set[caller] = true
				work = append(work, caller)
			}
		}
	}
	return set
}

// eachCall calls f with each call instruction in bb.
func eachCall(bb llvm.BasicBlock, f func(llvm.Value)) {
	for inst := bb.FirstInstruction(); !inst.IsNil(); inst = llvm.NextInstruction(inst) {
		if !inst.IsACallInst().IsNil() {
			f(inst)
		}
	}
}

// waitsHere reports whether bb is a frame loop's, by a call in it to one of
// the functions that wait for the screen, made by the program's code, in
// package main, or by the screen's own loop in package yaroze, Frames. A
// loop elsewhere that waits, such as disc.Wait polling a read once a frame,
// is a wait and not a frame loop.
func waitsHere(bb llvm.BasicBlock, waits map[llvm.Value]bool) bool {
	for inst := bb.FirstInstruction(); !inst.IsNil(); inst = llvm.NextInstruction(inst) {
		if !inst.IsACallInst().IsNil() && waits[inst.CalledValue()] {
			if pkg := callPackage(inst); pkg == "main" || pkg == "yaroze" || strings.HasSuffix(pkg, "/yaroze") {
				return true
			}
		}
	}
	return false
}

// successors returns the blocks bb's terminator may branch to.
func successors(bb llvm.BasicBlock) []llvm.BasicBlock {
	term := bb.LastInstruction()
	var succs []llvm.BasicBlock
	for i := 0; i < term.SuccessorsCount(); i++ {
		succs = append(succs, term.Successor(i))
	}
	return succs
}

// guarded reports whether call is only reached through a test of memory
// that the guarded code stores to, as a lazy allocation is, made the first
// time and kept: the test of a pointer against nil, or of a flag, that the
// code after the test sets. The blocks are followed back through each one's
// only predecessor, past the conditional branches that test anything else,
// and the store must be in a block only the tested branch leads to.
func guarded(call llvm.Value) bool {
	fn := call.InstructionParent().Parent()
	preds := make(map[llvm.BasicBlock][]llvm.BasicBlock)
	for bb := fn.FirstBasicBlock(); !bb.IsNil(); bb = llvm.NextBasicBlock(bb) {
		for _, succ := range successors(bb) {
			preds[succ] = append(preds[succ], bb)
		}
	}
	bb := call.InstructionParent()
	for range 16 {
		p := preds[bb]
		if len(p) != 1 {
			return false
		}
		taken := bb // the branch's side the call is on, reached only from it
		bb = p[0]
		term := bb.LastInstruction()
		if term.IsABranchInst().IsNil() || term.OperandsCount() != 3 {
			continue // not a conditional branch
		}
		cond := term.Operand(0)
		if ptr := loadedFrom(cond); !ptr.IsNil() && turnsOff(dominated(fn, taken), ptr, cond, taken == term.Successor(0)) {
			return true
		}
	}
	return false
}

// dominated returns the blocks of fn that every path from its entry to
// them goes through start: those reachable from start, less those
// reachable from the entry without it.
func dominated(fn llvm.Value, start llvm.BasicBlock) []llvm.BasicBlock {
	reach := func(from llvm.BasicBlock, avoid llvm.BasicBlock) map[llvm.BasicBlock]bool {
		seen := map[llvm.BasicBlock]bool{}
		work := []llvm.BasicBlock{from}
		for len(work) > 0 {
			bb := work[len(work)-1]
			work = work[:len(work)-1]
			if bb == avoid || seen[bb] {
				continue
			}
			seen[bb] = true
			work = append(work, successors(bb)...)
		}
		return seen
	}
	around := reach(fn.EntryBasicBlock(), start)
	var blocks []llvm.BasicBlock
	for bb := range reach(start, llvm.BasicBlock{}) {
		if !around[bb] {
			blocks = append(blocks, bb)
		}
	}
	return blocks
}

// turnsOff reports whether blocks store to the memory at ptr, at an address
// computed the same way, if not by the same instruction, and every value
// they store there makes cond come out the other way from want, the way
// that led into them: the test is turned off, as a lazy allocation's is
// when the pointer it tests is set, and not set again, as the state a
// range over a function's body keeps is, made -1 while the body runs and
// 0 again after.
func turnsOff(blocks []llvm.BasicBlock, ptr, cond llvm.Value, want bool) bool {
	stored := false
	for _, bb := range blocks {
		for inst := bb.FirstInstruction(); !inst.IsNil(); inst = llvm.NextInstruction(inst) {
			if inst.IsAStoreInst().IsNil() || !samePointer(inst.Operand(1), ptr) {
				continue
			}
			got, ok := evalCond(cond, storedValue(inst.Operand(0)))
			if !ok || got == want {
				return false
			}
			stored = true
		}
	}
	return stored
}

// A value is what a store leaves in memory, as far as a test of it goes:
// a constant, or a pointer known not to be nil, such as an allocation.
type value struct {
	known   bool
	n       uint64
	nonNull bool
}

func storedValue(v llvm.Value) value {
	switch {
	case !v.IsAConstantInt().IsNil():
		return value{known: true, n: v.ZExtValue()}
	case !v.IsAConstantPointerNull().IsNil():
		return value{known: true}
	case v.Type().TypeKind() == llvm.PointerTypeKind:
		return value{nonNull: isHeapAlloc(v) || !v.IsAGlobalValue().IsNil()}
	}
	return value{}
}

// evalCond evaluates cond, a test of a load loadedFrom found, for the load
// giving v, and reports whether it could.
func evalCond(cond llvm.Value, v value) (bool, bool) {
	x, ok := evalInt(cond, v)
	return x.n&1 != 0, ok && x.known
}

func evalInt(e llvm.Value, v value) (value, bool) {
	switch {
	case !e.IsALoadInst().IsNil():
		return v, true
	case !e.IsATruncInst().IsNil(), !e.IsAZExtInst().IsNil():
		x, ok := evalInt(e.Operand(0), v)
		if ok && x.known && !e.IsATruncInst().IsNil() {
			if bits := e.Type().IntTypeWidth(); bits < 64 {
				x.n &= 1<<bits - 1
			}
		}
		return x, ok
	case !e.IsABinaryOperator().IsNil() && e.InstructionOpcode() == llvm.Xor:
		x, ok := evalInt(e.Operand(0), v)
		c := e.Operand(1)
		if !ok || !x.known || c.IsAConstantInt().IsNil() {
			return value{}, false
		}
		return value{known: true, n: x.n ^ c.ZExtValue()}, true
	case !e.IsAICmpInst().IsNil():
		a, b := e.Operand(0), e.Operand(1)
		pred := e.IntPredicate()
		if !a.IsAConstant().IsNil() {
			a, b = b, a
			pred = swapped(pred)
		}
		x, ok := evalInt(a, v)
		if !ok {
			return value{}, false
		}
		nullTest := !b.IsAConstantPointerNull().IsNil()
		if nullTest && !x.known && x.nonNull {
			x = value{known: true, n: 1} // not nil: any nonzero will do
		}
		if !x.known {
			return value{}, false
		}
		var c uint64
		switch {
		case nullTest:
		case !b.IsAConstantInt().IsNil():
			c = b.ZExtValue()
		default:
			return value{}, false
		}
		width := 64
		if !nullTest {
			width = b.Type().IntTypeWidth()
		}
		r, ok := compare(pred, x.n, c, width)
		if !ok {
			return value{}, false
		}
		if r {
			return value{known: true, n: 1}, true
		}
		return value{known: true}, true
	}
	return value{}, false
}

// swapped is pred with its operands swapped.
func swapped(pred llvm.IntPredicate) llvm.IntPredicate {
	switch pred {
	case llvm.IntSGT:
		return llvm.IntSLT
	case llvm.IntSLT:
		return llvm.IntSGT
	case llvm.IntSGE:
		return llvm.IntSLE
	case llvm.IntSLE:
		return llvm.IntSGE
	case llvm.IntUGT:
		return llvm.IntULT
	case llvm.IntULT:
		return llvm.IntUGT
	case llvm.IntUGE:
		return llvm.IntULE
	case llvm.IntULE:
		return llvm.IntUGE
	}
	return pred
}

// compare compares a and b, integers of width bits, by pred.
func compare(pred llvm.IntPredicate, a, b uint64, width int) (bool, bool) {
	sa, sb := int64(a<<(64-width))>>(64-width), int64(b<<(64-width))>>(64-width)
	switch pred {
	case llvm.IntEQ:
		return a == b, true
	case llvm.IntNE:
		return a != b, true
	case llvm.IntUGT:
		return a > b, true
	case llvm.IntUGE:
		return a >= b, true
	case llvm.IntULT:
		return a < b, true
	case llvm.IntULE:
		return a <= b, true
	case llvm.IntSGT:
		return sa > sb, true
	case llvm.IntSGE:
		return sa >= sb, true
	case llvm.IntSLT:
		return sa < sb, true
	case llvm.IntSLE:
		return sa <= sb, true
	}
	return false, false
}

// samePointer reports whether a and b are the same pointer, or the same
// constant offsets from the same pointer.
func samePointer(a, b llvm.Value) bool {
	if a == b {
		return true
	}
	if a.IsAGetElementPtrInst().IsNil() || b.IsAGetElementPtrInst().IsNil() ||
		a.OperandsCount() != b.OperandsCount() || a.GEPSourceElementType() != b.GEPSourceElementType() ||
		!samePointer(a.Operand(0), b.Operand(0)) {
		return false
	}
	for i := 1; i < a.OperandsCount(); i++ {
		if a.Operand(i) != b.Operand(i) || a.Operand(i).IsAConstant().IsNil() {
			return false
		}
	}
	return true
}

// loadedFrom returns the pointer a branch condition tests the memory at:
// the load itself, a comparison of it with a constant, or its negation.
func loadedFrom(cond llvm.Value) llvm.Value {
	for range 4 {
		switch {
		case !cond.IsALoadInst().IsNil():
			return cond.Operand(0)
		case !cond.IsAICmpInst().IsNil():
			switch {
			case !cond.Operand(1).IsAConstant().IsNil():
				cond = cond.Operand(0)
			case !cond.Operand(0).IsAConstant().IsNil():
				cond = cond.Operand(1)
			default:
				return llvm.Value{}
			}
		case !cond.IsATruncInst().IsNil(), !cond.IsAZExtInst().IsNil():
			cond = cond.Operand(0)
		case !cond.IsABinaryOperator().IsNil() && cond.InstructionOpcode() == llvm.Xor && !cond.Operand(1).IsAConstant().IsNil():
			cond = cond.Operand(0)
		default:
			return llvm.Value{}
		}
	}
	return llvm.Value{}
}

// callPackage returns the package of the code a call is in, from its
// tinygo.pos metadata, which code inlined from another package keeps, or
// else from its function's name.
func callPackage(call llvm.Value) string {
	if md := call.Metadata(call.Type().Context().MDKindID("tinygo.pos")); !md.IsNil() && md.OperandsCount() >= 4 {
		return md.Operand(3).ConstGetAsString()
	}
	return packageOf(call.InstructionParent().Parent().Name())
}

// packageOf returns the package path of a function's name, as TinyGo gives
// it: main.main$1, (*strings.Builder).WriteString,
// go.chrisrx.dev/psx/archive.Entry.Load.
func packageOf(name string) string {
	name = strings.TrimLeft(name, "(*")
	if i := strings.IndexByte(name, '['); i >= 0 {
		name = name[:i]
	}
	dir := ""
	if i := strings.LastIndexByte(name, '/'); i >= 0 {
		dir, name = name[:i+1], name[i+1:]
	}
	if i := strings.IndexByte(name, '.'); i >= 0 {
		name = name[:i]
	}
	return dir + name
}

// isStandard reports whether a function is the runtime's or the standard
// library's, whose code has no positions and is reported where the
// program calls it.
func isStandard(name string) bool {
	pkg := packageOf(name)
	first, _, _ := strings.Cut(pkg, "/")
	return pkg != "main" && !strings.Contains(first, ".")
}

// allocName names what a call into the runtime or the standard library
// that allocates is, as the program wrote it.
func allocName(name string) string {
	switch name {
	case "runtime.stringConcat":
		return "joining strings with +"
	case "runtime.sliceAppend":
		return "an append that grows the slice"
	case "runtime.stringFromBytes", "runtime.stringFromRunes", "runtime.stringFromUnicode":
		return "a conversion to string"
	case "runtime.stringToBytes", "runtime.stringToRunes":
		return "a conversion of a string to a slice"
	case "runtime.hashmapMake", "runtime.hashmapSet", "runtime.hashmapStringSet", "runtime.hashmapBinarySet", "runtime.hashmapInterfaceSet":
		return "a map that grows"
	case "runtime.chanMake":
		return "a channel"
	}
	return shortName(name)
}
