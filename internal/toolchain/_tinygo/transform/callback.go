package transform

// The functions the library calls from an interrupt, for the PlayStation
// targets. VSyncCallback installs a function that the library calls at the
// start of every vertical blank, with interrupts off, and yaroze.OnVSync
// installs one that calls the Go func it is given. Such a function must not
// allocate, since the allocator is not made to be entered from an
// interrupt that may have stopped it halfway, and must not wait, for the
// vertical blank or the GPU or the drive, which with interrupts off never
// comes. CheckCallbacks refuses either, in the function and in what it
// calls.
//
// The functions are found from the address passed to VSyncCallback, and
// from there through calls: a call through a func value loaded from memory
// calls each function stored there, and a function's parameter is
// whatever each of its calls passes it, as OnVSync's is.

import (
	"fmt"

	"tinygo.org/x/go-llvm"
)

// blocking is the library's functions that wait when their first argument
// is 0: for the vertical blank, the GPU, the drive's command and its read.
var blocking = []string{"VSync", "DrawSync", "CdSync", "CdReadSync"}

// CheckCallbacks returns an error at each allocation, and each wait, in a
// function the library calls from an interrupt.
func CheckCallbacks(mod llvm.Module) []error {
	install := mod.NamedFunction("VSyncCallback")
	if install.IsNil() {
		return nil
	}
	alloc := mod.NamedFunction("runtime.alloc")
	var allocates map[llvm.Value]bool
	if !alloc.IsNil() {
		allocates = callersOf(alloc, nil)
	}

	// The functions the interrupt runs, and the call that first reaches
	// each, for the message.
	via := make(map[llvm.Value]llvm.Value)
	var work []llvm.Value
	add := func(fn, call llvm.Value) {
		if _, ok := via[fn]; !ok && !fn.IsDeclaration() {
			via[fn] = call
			work = append(work, fn)
		}
	}
	for _, use := range getUses(install) {
		if use.IsACallInst().IsNil() || use.CalledValue() != install {
			continue
		}
		for _, fn := range functionsFrom(use.Operand(0), 0) {
			add(fn, use)
		}
	}
	for len(work) > 0 {
		fn := work[0]
		work = work[1:]
		for bb := fn.FirstBasicBlock(); !bb.IsNil(); bb = llvm.NextBasicBlock(bb) {
			eachCall(bb, func(call llvm.Value) {
				for _, callee := range functionsFrom(call.CalledValue(), 0) {
					add(callee, call)
				}
			})
		}
	}

	var errs []error
	reported := make(map[string]bool)
	report := func(site llvm.Value, msg string) {
		err := keepsError(site, msg)
		if key := err.Error(); !reported[key] {
			reported[key] = true
			errs = append(errs, err)
		}
	}
	for fn := range via {
		if isStandard(fn.Name()) {
			continue // reported where the program's code calls it
		}
		for bb := fn.FirstBasicBlock(); !bb.IsNil(); bb = llvm.NextBasicBlock(bb) {
			eachCall(bb, func(call llvm.Value) {
				callee := call.CalledValue()
				if callee.IsAFunction().IsNil() {
					return
				}
				switch {
				case callee == alloc:
					report(call, fmt.Sprintf("this allocates in %s, which the vertical blank interrupt runs: the allocator is not to be entered from an interrupt, so allocate before installing it", funcName(fn)))
				case allocates[callee] && isStandard(callee.Name()):
					report(call, fmt.Sprintf("%s allocates in %s, which the vertical blank interrupt runs: the allocator is not to be entered from an interrupt, so allocate before installing it", allocName(callee.Name()), funcName(fn)))
				case callee.IsDeclaration() && waits(call):
					report(call, fmt.Sprintf("%s waits, in %s, which the vertical blank interrupt runs with interrupts off, so what it waits for never comes: wait in the frame loop instead", callee.Name(), funcName(fn)))
				}
			})
		}
	}
	sortErrors(errs)
	return errs
}

// waits reports whether call is of one of the library's functions that
// wait, with 0, its waiting mode.
func waits(call llvm.Value) bool {
	name := call.CalledValue().Name()
	for _, b := range blocking {
		if name == b && call.OperandsCount() > 1 {
			arg := call.Operand(0)
			return !arg.IsAConstantInt().IsNil() && arg.SExtValue() == 0
		}
	}
	return false
}

// functionsFrom returns the functions v may be: a function, a conversion of
// one, a phi or select of them, a parameter, which is what each call of its
// function passes, or a load, which is what is stored at the same address.
func functionsFrom(v llvm.Value, depth int) []llvm.Value {
	if depth > 8 || v.IsNil() {
		return nil
	}
	switch {
	case !v.IsAFunction().IsNil():
		return []llvm.Value{v}
	case !v.IsAConstantExpr().IsNil(), !v.IsACastInst().IsNil():
		return functionsFrom(v.Operand(0), depth+1)
	case !v.IsAPHINode().IsNil():
		var fns []llvm.Value
		for i := 0; i < v.IncomingCount(); i++ {
			fns = append(fns, functionsFrom(v.IncomingValue(i), depth+1)...)
		}
		return fns
	case !v.IsASelectInst().IsNil():
		return append(functionsFrom(v.Operand(1), depth+1), functionsFrom(v.Operand(2), depth+1)...)
	case !v.IsAArgument().IsNil():
		fn := v.ParamParent()
		index := -1
		for i := 0; i < fn.ParamsCount(); i++ {
			if fn.Param(i) == v {
				index = i
			}
		}
		var fns []llvm.Value
		for _, use := range getUses(fn) {
			if !use.IsACallInst().IsNil() && use.CalledValue() == fn && index < use.OperandsCount()-1 {
				fns = append(fns, functionsFrom(use.Operand(index), depth+1)...)
			}
		}
		return fns
	case !v.IsALoadInst().IsNil():
		ptr := v.Operand(0)
		var fns []llvm.Value
		for _, store := range storesInto(ptr) {
			fns = append(fns, functionsFrom(store.Operand(0), depth+1)...)
		}
		return fns
	}
	return nil
}

// storesInto returns the stores anywhere in the module to the address ptr
// loads from: a global, or the same constant offsets into one.
func storesInto(ptr llvm.Value) []llvm.Value {
	base := ptr
	for !base.IsAGetElementPtrInst().IsNil() || !base.IsAConstantExpr().IsNil() && base.OperandsCount() > 0 {
		base = base.Operand(0)
	}
	if base.IsAGlobalVariable().IsNil() {
		return nil
	}
	var stores []llvm.Value
	var visit func(user llvm.Value, depth int)
	visit = func(user llvm.Value, depth int) {
		if depth > 4 {
			return
		}
		if !user.IsAStoreInst().IsNil() {
			if samePointer(user.Operand(1), ptr) || sameConstantPointer(user.Operand(1), ptr) {
				stores = append(stores, user)
			}
			return
		}
		if !user.IsAGetElementPtrInst().IsNil() || !user.IsAConstantExpr().IsNil() {
			for _, u := range getUses(user) {
				visit(u, depth+1)
			}
		}
	}
	for _, use := range getUses(base) {
		visit(use, 0)
	}
	return stores
}

// sameConstantPointer reports whether a and b are the same constant
// address, which LLVM gives one value, or a global and a constant offset
// of zero into it.
func sameConstantPointer(a, b llvm.Value) bool {
	strip := func(v llvm.Value) llvm.Value {
		for !v.IsAConstantExpr().IsNil() && v.OperandsCount() > 0 && v.Opcode() == llvm.GetElementPtr && allZero(v) {
			v = v.Operand(0)
		}
		return v
	}
	return strip(a) == strip(b)
}

// allZero reports whether a constant address's offsets are all zero.
func allZero(gep llvm.Value) bool {
	for i := 1; i < gep.OperandsCount(); i++ {
		if c := gep.Operand(i); c.IsAConstantInt().IsNil() || c.ZExtValue() != 0 {
			return false
		}
	}
	return true
}
