package transform

// What the library keeps, for the PlayStation targets. TinyGo takes every
// //export declaration to keep none of the pointers it is given, cgo's rule,
// so a variable whose address is passed to one stays on the stack. Sony's
// library breaks the rule in a few places, which its bindings say with
// //go:keeps: GsSetWorkBase builds packets where it was told to until it is
// told otherwise, and LoadImage goes on reading the pixels after it returns,
// until DrawSync. The compiler marks each such parameter with a tinygo-keeps
// attribute, holding its name and the function after which it is no longer
// kept, if any, and leaves one kept for good able to escape, so that what is
// passed to it is allocated on the heap, or made a global by HoistOnceAllocs
// if it is allocated once.
//
// CheckKeeps then refuses what would still go wrong, at the call:
//
//   - A variable on the stack passed to a parameter kept until a later call,
//     when the function it belongs to can return before making that call:
//     the library would go on with memory the next call reuses. main is
//     exempt, since it does not return while the program runs.
//   - A variable allocated each time a loop goes round, or each time a
//     function that may run every frame runs, passed to a parameter kept
//     for good, and going nowhere else: nothing is ever freed, so the
//     program would run out of memory in time. One the program also stores
//     or returns is its own, which it may keep and use again, as a pool
//     does, and is left to it. A function may run every frame if it is called from a
//     loop, if it is a function value, such as the body of the frame loop,
//     or if such a function calls it.
//   - A variable on the stack passed to a parameter kept for good, which the
//     compiler should not leave there, outside main.
//
// It follows a pointer back through address arithmetic, conversions, phis,
// selects and the calls that return their argument, to the variable, and
// from a parameter to each call of its function, so that a variable passed
// through a wrapper such as RECT.Load is found where it is made. The call it
// checks and reports is the one in the variable's own function, the
// program's line, from the tinygo.pos metadata the compiler gives it when
// there is no DWARF. A pointer that comes from anywhere else, a load, is not
// checked.

import (
	"fmt"
	"go/scanner"
	"go/token"
	"slices"
	"strings"

	"github.com/tinygo-org/tinygo/compileopts"
	"github.com/tinygo-org/tinygo/compiler/llvmutil"
	"tinygo.org/x/go-llvm"
)

// CheckKeeps returns an error at each call that passes the library a
// variable it would lose, as described above.
func CheckKeeps(mod llvm.Module, config *compileopts.Config) []error {
	kind := "tinygo-keeps"
	var errs []error
	var repeated map[llvm.Value]bool
	cyclic := make(map[llvm.Value]map[llvm.BasicBlock]bool)
	inLoop := func(bb llvm.BasicBlock) bool {
		fn := bb.Parent()
		blocks, ok := cyclic[fn]
		if !ok {
			blocks = make(map[llvm.BasicBlock]bool)
			for b := fn.FirstBasicBlock(); !b.IsNil(); b = llvm.NextBasicBlock(b) {
				blocks[b] = true
			}
			for _, b := range acyclicBlocks(fn) {
				delete(blocks, b)
			}
			cyclic[fn] = blocks
		}
		return blocks[bb]
	}

	reported := make(map[string]bool)
	report := func(site llvm.Value, msg string) {
		err := keepsError(site, msg)
		if key := err.Error(); !reported[key] {
			reported[key] = true
			errs = append(errs, err)
		}
	}
	for fn := mod.FirstFunction(); !fn.IsNil(); fn = llvm.NextFunction(fn) {
		if !fn.IsDeclaration() {
			continue
		}
		for i := 0; i < fn.ParamsCount(); i++ {
			attr := fn.GetStringAttributeAtIndex(i+1, kind)
			if attr.IsNil() {
				continue
			}
			fields := strings.Fields(attr.GetStringValue())
			if len(fields) == 0 {
				continue
			}
			param, until := fields[0], ""
			if len(fields) > 1 {
				until = fields[1]
			}
			for _, call := range getUses(fn) {
				if call.IsACallInst().IsNil() || call.CalledValue() != fn {
					continue
				}
				for _, o := range pointerOrigins(call.Operand(i), call, nil, nil, 0) {
					base, site := o.base, o.site
					owner := base.InstructionParent().Parent()
					inMain := owner.Name() == "main.main"
					who := funcName(owner)
					what := fn.Name()
					if callee := site.CalledValue(); callee != fn {
						what += ", called through " + shortName(callee.Name()) + ","
					}
					switch {
					case !base.IsAAllocaInst().IsNil() && until != "":
						if !inMain && !o.synced(call, until) {
							report(site, fmt.Sprintf("%s goes on with %s until %s, but the variable passed is on the stack of %s, which can return first: call %s before it does, or make the variable a package-level one", what, param, until, who, until))
						}
					case !base.IsAAllocaInst().IsNil():
						if !inMain {
							report(site, fmt.Sprintf("%s keeps %s, but the variable passed is on the stack of %s, which returns: make it a package-level variable", what, param, who))
						}
					case until == "" && isHeapAlloc(base):
						if repeated == nil {
							repeated = repeatedFunctions(mod, inLoop)
						}
						if (inLoop(base.InstructionParent()) || repeated[owner]) && !escapesElsewhere(base, site, nil) {
							report(site, fmt.Sprintf("%s keeps %s, so the variable passed is allocated each time %s runs here, and never freed: make it a package-level variable", what, param, who))
						}
					}
				}
			}
		}
	}
	return errs
}

// funcName names fn as a message should: a closure, main.main$1, as the
// function literal in main.main.
func funcName(fn llvm.Value) string {
	name := fn.Name()
	if outer, _, ok := strings.Cut(name, "$"); ok {
		return "the function literal in " + outer
	}
	return name
}

// escapesElsewhere reports whether ptr, or a pointer made from it, goes
// anywhere but the call site: stored, returned, or passed to a function that
// may keep it.
func escapesElsewhere(ptr, site llvm.Value, seen map[llvm.Value]bool) bool {
	if seen == nil {
		seen = make(map[llvm.Value]bool)
	}
	if seen[ptr] {
		return false
	}
	seen[ptr] = true
	for _, use := range getUses(ptr) {
		switch {
		case use == site:
		case !use.IsALoadInst().IsNil(), !use.IsAICmpInst().IsNil():
		case !use.IsAStoreInst().IsNil():
			if use.Operand(0) == ptr {
				return true // the pointer itself is stored
			}
		case !use.IsAGetElementPtrInst().IsNil(), !use.IsABitCastInst().IsNil(), !use.IsAPHINode().IsNil(), !use.IsASelectInst().IsNil():
			if escapesElsewhere(use, site, seen) {
				return true
			}
		case !use.IsACallInst().IsNil():
			called := use.CalledValue()
			if called.IsAFunction().IsNil() {
				return true
			}
			kind := llvm.AttributeKindID(llvmutil.NoCaptureAttrName())
			for i := 0; i < use.OperandsCount()-1; i++ {
				if use.Operand(i) == ptr && (i >= called.ParamsCount() || !llvmutil.IsNoCapture(called.GetEnumAttributeAtIndex(i+1, kind))) {
					return true
				}
			}
		default:
			return true // returned, or converted to an integer
		}
	}
	return false
}

// shortName drops the import path from a function's name:
// (*go.chrisrx.dev/psx/yaroze.RECT).Load is (*yaroze.RECT).Load.
func shortName(name string) string {
	open := strings.LastIndex(name, "(") + 1
	if open > 0 && open < len(name) && name[open] == '*' {
		open++
	}
	if i := strings.LastIndex(name, "/"); i >= open {
		return name[:open] + name[i+1:]
	}
	return name
}

// keepsError is an error at call: where its tinygo.pos metadata says, the
// file, line and column, or where the debug information does.
func keepsError(call llvm.Value, msg string) scanner.Error {
	pos := getPosition(call)
	if md := call.Metadata(call.Type().Context().MDKindID("tinygo.pos")); !md.IsNil() && md.OperandsCount() >= 3 {
		pos = token.Position{
			Filename: md.Operand(0).ConstGetAsString(),
			Line:     int(md.Operand(1).ZExtValue()),
			Column:   int(md.Operand(2).ZExtValue()),
		}
	}
	return scanner.Error{Pos: pos, Msg: msg}
}

func isHeapAlloc(v llvm.Value) bool {
	if v.IsACallInst().IsNil() {
		return false
	}
	name := v.CalledValue().Name()
	return name == "runtime.alloc" || name == "runtime.alloc_noheap"
}

// An origin is an allocation, stack or heap, that a pointer passed to the
// library may point into, and the call in the allocation's function that
// passes it on: the call of the library itself, or of a function that leads
// to it.
type origin struct {
	base, site llvm.Value
	via        []llvm.Value // the calls between the library's and site, innermost first
}

// synced reports whether a call of until follows, before its function
// returns, the library's call or one of the calls leading to it.
func (o origin) synced(call llvm.Value, until string) bool {
	if callFollows(call, until) || callFollows(o.site, until) {
		return true
	}
	for _, c := range o.via {
		if callFollows(c, until) {
			return true
		}
	}
	return false
}

// pointerOrigins returns the origins of ptr, which site passes on, by way of
// the calls in via.
func pointerOrigins(ptr, site llvm.Value, via []llvm.Value, seen map[llvm.Value]bool, depth int) []origin {
	if seen == nil {
		seen = make(map[llvm.Value]bool)
	}
	if seen[ptr] || depth > 16 {
		return nil
	}
	seen[ptr] = true
	switch {
	case !ptr.IsAAllocaInst().IsNil():
		return []origin{{ptr, site, via}}
	case !ptr.IsAArgument().IsNil():
		// A parameter: what each call of the function passes.
		fn := ptr.ParamParent()
		index := -1
		for i := 0; i < fn.ParamsCount(); i++ {
			if fn.Param(i) == ptr {
				index = i
			}
		}
		var origins []origin
		for _, call := range getUses(fn) {
			if index < 0 || call.IsACallInst().IsNil() || call.CalledValue() != fn {
				continue
			}
			origins = append(origins, pointerOrigins(call.Operand(index), call, append(slices.Clone(via), site), seen, depth+1)...)
		}
		return origins
	case !ptr.IsACallInst().IsNil():
		if isHeapAlloc(ptr) {
			return []origin{{ptr, site, via}}
		}
		// A call that returns one of its arguments, as a conversion does.
		called := ptr.CalledValue()
		if called.IsAFunction().IsNil() {
			return nil
		}
		returned := llvm.AttributeKindID("returned")
		for i := 0; i < called.ParamsCount(); i++ {
			if !called.GetEnumAttributeAtIndex(i+1, returned).IsNil() {
				return pointerOrigins(ptr.Operand(i), site, via, seen, depth)
			}
		}
		return nil
	case !ptr.IsAGetElementPtrInst().IsNil(), !ptr.IsABitCastInst().IsNil():
		return pointerOrigins(ptr.Operand(0), site, via, seen, depth)
	case !ptr.IsAPHINode().IsNil():
		var origins []origin
		for i := 0; i < ptr.IncomingCount(); i++ {
			origins = append(origins, pointerOrigins(ptr.IncomingValue(i), site, via, seen, depth)...)
		}
		return origins
	case !ptr.IsASelectInst().IsNil():
		return append(pointerOrigins(ptr.Operand(1), site, via, seen, depth), pointerOrigins(ptr.Operand(2), site, via, seen, depth)...)
	}
	return nil
}

// callFollows reports whether every way on from call, in its function,
// calls the function named until before the function returns.
func callFollows(call llvm.Value, until string) bool {
	calls := func(inst llvm.Value) bool {
		return !inst.IsACallInst().IsNil() && inst.CalledValue().Name() == until
	}
	for inst := llvm.NextInstruction(call); !inst.IsNil(); inst = llvm.NextInstruction(inst) {
		if calls(inst) {
			return true
		}
	}
	// Every block reachable from here without passing such a call must not
	// return.
	start := call.InstructionParent()
	if !start.LastInstruction().IsAReturnInst().IsNil() {
		return false
	}
	seen := map[llvm.BasicBlock]bool{}
	var work []llvm.BasicBlock
	push := func(bb llvm.BasicBlock) {
		term := bb.LastInstruction()
		for i := 0; i < term.SuccessorsCount(); i++ {
			if next := term.Successor(i); !seen[next] {
				seen[next] = true
				work = append(work, next)
			}
		}
	}
	push(start)
	for len(work) > 0 {
		bb := work[len(work)-1]
		work = work[:len(work)-1]
		found := false
		for inst := bb.FirstInstruction(); !inst.IsNil(); inst = llvm.NextInstruction(inst) {
			if calls(inst) {
				found = true
				break
			}
		}
		if found {
			continue
		}
		if !bb.LastInstruction().IsAReturnInst().IsNil() {
			return false
		}
		push(bb)
	}
	return true
}

// repeatedFunctions returns the functions that may run every frame: those
// called from a loop, those used as values, and those such functions call.
func repeatedFunctions(mod llvm.Module, inLoop func(llvm.BasicBlock) bool) map[llvm.Value]bool {
	repeated := make(map[llvm.Value]bool)
	var work []llvm.Value
	add := func(fn llvm.Value) {
		if fn.IsDeclaration() || repeated[fn] {
			return
		}
		repeated[fn] = true
		work = append(work, fn)
	}
	for fn := mod.FirstFunction(); !fn.IsNil(); fn = llvm.NextFunction(fn) {
		if fn.IsDeclaration() {
			continue
		}
		for _, use := range getUses(fn) {
			if use.IsACallInst().IsNil() || use.CalledValue() != fn {
				add(fn) // a function value
				break
			}
			if inLoop(use.InstructionParent()) {
				add(fn)
				break
			}
		}
	}
	for len(work) > 0 {
		fn := work[len(work)-1]
		work = work[:len(work)-1]
		for bb := fn.FirstBasicBlock(); !bb.IsNil(); bb = llvm.NextBasicBlock(bb) {
			for inst := bb.FirstInstruction(); !inst.IsNil(); inst = llvm.NextInstruction(inst) {
				if !inst.IsACallInst().IsNil() {
					if callee := inst.CalledValue(); !callee.IsAFunction().IsNil() {
						add(callee)
					}
				}
			}
		}
	}
	return repeated
}
