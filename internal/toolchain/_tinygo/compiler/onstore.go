package compiler

// The onstore experiment (-experiment=noonstore turns it off): a method
// marked //go:onstore with the names of fields of its receiver's type runs
// after each store to one of those fields, anywhere in the program, so that
// what it derives from them, such as a word laid out for a C library, is
// always up to date and need not be worked out where it is read. A field
// that is a struct or an array covers the stores to what is inside it.

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/ssa"
	"tinygo.org/x/go-llvm"
)

// onstorePackages is the import path under which a package may use
// //go:onstore, while the experiment lasts: so that changing or removing
// it touches no code but ours.
var onstorePackages = "go.chrisrx.dev/psx"

// An onstoreHook is a method marked //go:onstore and the fields it watches,
// by their index in the receiver's struct.
type onstoreHook struct {
	fn     *ssa.Function
	fields []int
}

// onstoreAllowed reports whether package path may use //go:onstore.
func onstoreAllowed(path string) bool {
	return path == onstorePackages || strings.HasPrefix(path, onstorePackages+"/")
}

// parseOnstore checks a //go:onstore pragma on f, which names the fields
// in parts after the pragma itself, and reports what is wrong with it.
func (c *compilerContext) parseOnstore(info *functionInfo, f *ssa.Function, parts []string) {
	if !c.Onstore {
		return
	}
	if f.Pkg == nil || !onstoreAllowed(f.Pkg.Pkg.Path()) {
		c.addError(f.Pos(), "//go:onstore is an experiment only packages under "+onstorePackages+" may use")
		return
	}
	st, _ := onstoreReceiver(f)
	if st == nil || f.Signature.Params().Len() != 0 || f.Signature.Results().Len() != 0 || len(f.Blocks) == 0 {
		c.addError(f.Pos(), "//go:onstore goes on a method with a pointer receiver to a struct type, no parameters and no results")
		return
	}
	if len(parts) < 2 {
		c.addError(f.Pos(), "//go:onstore names the fields whose stores call the method")
		return
	}
	for _, name := range parts[1:] {
		if onstoreField(st, name) < 0 {
			c.addError(f.Pos(), fmt.Sprintf("//go:onstore names %s, which is not a field of the receiver's type", name))
			return
		}
	}
	// It runs after every store to the fields, and is meant to be as cheap
	// as the store.
	info.onstore = true
}

// onstoreReceiver returns the struct f's receiver points to and the named
// type, or nil if f is not a method with a pointer receiver to a named
// struct type.
func onstoreReceiver(f *ssa.Function) (*types.Struct, *types.Named) {
	recv := f.Signature.Recv()
	if recv == nil {
		return nil, nil
	}
	ptr, ok := recv.Type().(*types.Pointer)
	if !ok {
		return nil, nil
	}
	named, ok := types.Unalias(ptr.Elem()).(*types.Named)
	if !ok {
		return nil, nil
	}
	st, ok := named.Underlying().(*types.Struct)
	if !ok {
		return nil, nil
	}
	return st, named
}

// onstoreField returns the index of the field of st called name, or -1.
func onstoreField(st *types.Struct, name string) int {
	for i := range st.NumFields() {
		if st.Field(i).Name() == name {
			return i
		}
	}
	return -1
}

// onstoreHookOf returns the //go:onstore method of the struct type a
// pointer of type ptr points to, or nil if it has none.
func (c *compilerContext) onstoreHookOf(ptr types.Type) *onstoreHook {
	p, ok := ptr.Underlying().(*types.Pointer)
	if !ok {
		return nil
	}
	named, ok := types.Unalias(p.Elem()).(*types.Named)
	if !ok || named.Obj().Pkg() == nil || !onstoreAllowed(named.Obj().Pkg().Path()) || named.TypeArgs().Len() != 0 {
		return nil
	}
	if hook, ok := c.onstoreHooks[named]; ok {
		return hook
	}
	if c.onstoreHooks == nil {
		c.onstoreHooks = make(map[*types.Named]*onstoreHook)
	}
	var hook *onstoreHook
	st, _ := named.Underlying().(*types.Struct)
	mset := c.program.MethodSets.MethodSet(types.NewPointer(named))
	for i := range mset.Len() {
		fn := c.program.MethodValue(mset.At(i))
		fields := onstorePragma(fn)
		if fields == nil || st == nil {
			continue
		}
		// What is wrong with the pragma is reported where the method is
		// compiled, by parseOnstore.
		hook = &onstoreHook{fn: fn}
		for _, name := range fields {
			if i := onstoreField(st, name); i >= 0 {
				hook.fields = append(hook.fields, i)
			}
		}
		break
	}
	c.onstoreHooks[named] = hook
	return hook
}

// onstorePragma returns the fields a //go:onstore pragma on fn names, or
// nil if it has none.
func onstorePragma(fn *ssa.Function) []string {
	if fn == nil || fn.Synthetic != "" {
		return nil
	}
	decl, ok := fn.Syntax().(*ast.FuncDecl)
	if !ok || decl.Doc == nil {
		return nil
	}
	for _, comment := range decl.Doc.List {
		parts := strings.Fields(comment.Text)
		if len(parts) > 0 && parts[0] == "//go:onstore" {
			return append([]string{}, parts[1:]...)
		}
	}
	return nil
}

// An onstoreCall is a hook to call after a store, on the object root
// points to.
type onstoreCall struct {
	hook *onstoreHook
	root ssa.Value
}

// onstoreCalls returns the hooks a store to addr calls: one for each
// field on the way from the object addr is in to addr that a hook of the
// object's type watches. The way goes through fields and the elements of
// arrays, which are inside the object, and stops at a pointer or a slice,
// which lead outside it.
func (c *compilerContext) onstoreCalls(addr ssa.Value) []onstoreCall {
	var calls []onstoreCall
	for {
		switch a := addr.(type) {
		case *ssa.FieldAddr:
			if hook := c.onstoreHookOf(a.X.Type()); hook != nil && slices.Contains(hook.fields, a.Field) {
				calls = append(calls, onstoreCall{hook, a.X})
			}
			addr = a.X
		case *ssa.IndexAddr:
			if _, ok := a.X.Type().Underlying().(*types.Pointer); !ok {
				return calls // a slice's element
			}
			addr = a.X
		default:
			return calls
		}
	}
}

// createOnstoreCalls calls the hooks a store watches after it, unless the
// next store in the block calls the same ones on the same objects with
// nothing between them but the address arithmetic for it: then that store
// calls them, once for both.
func (b *builder) createOnstoreCalls(store *ssa.Store) {
	if !b.Onstore {
		return
	}
	calls := b.onstoreCalls(store.Addr)
	if len(calls) == 0 {
		return
	}
	if next := nextStore(store); next != nil && sameOnstoreCalls(calls, b.onstoreCalls(next.Addr)) {
		return
	}
	for _, call := range calls {
		if call.hook.fn == b.fn {
			continue // the hook's own stores
		}
		fnType, fn := b.getFunction(call.hook.fn)
		recv := b.getValue(call.root, getPos(store))
		b.createInvokeWithAnalysis(fnType, fn, []llvm.Value{recv, llvm.Undef(b.dataPtrType)}, "", true, false)
	}
}

// nextStore returns the store that follows store in its block with
// nothing but address arithmetic between them, or nil.
func nextStore(store *ssa.Store) *ssa.Store {
	instrs := store.Block().Instrs
	i := slices.Index(instrs, ssa.Instruction(store))
	for _, instr := range instrs[i+1:] {
		switch instr := instr.(type) {
		case *ssa.FieldAddr, *ssa.IndexAddr, *ssa.DebugRef:
			continue
		case *ssa.Store:
			return instr
		}
		return nil
	}
	return nil
}

// sameOnstoreCalls reports whether two stores call the same hooks on the
// same objects.
func sameOnstoreCalls(a, b []onstoreCall) bool {
	return slices.EqualFunc(a, b, func(x, y onstoreCall) bool {
		return x.hook == y.hook && sameAddr(x.root, y.root)
	})
}

// sameAddr reports whether x and y are the same address: the same value,
// or the same field or element of the same address, made again.
func sameAddr(x, y ssa.Value) bool {
	if x == y {
		return true
	}
	switch x := x.(type) {
	case *ssa.FieldAddr:
		y, ok := y.(*ssa.FieldAddr)
		return ok && x.Field == y.Field && sameAddr(x.X, y.X)
	case *ssa.IndexAddr:
		y, ok := y.(*ssa.IndexAddr)
		if !ok || !sameAddr(x.X, y.X) {
			return false
		}
		if x.Index == y.Index {
			return true
		}
		cx, okx := x.Index.(*ssa.Const)
		cy, oky := y.Index.(*ssa.Const)
		return okx && oky && cx.Value != nil && cy.Value != nil && constant.Compare(cx.Value, token.EQL, cy.Value)
	}
	return false
}

// checkOnstoreAddrs refuses an address of a watched field that is used for
// anything but to store to it or load from it: a store through it
// elsewhere would not call the hook.
func (b *builder) checkOnstoreAddrs() {
	if !b.Onstore {
		return
	}
	for _, block := range b.fn.Blocks {
		for _, instr := range block.Instrs {
			fa, ok := instr.(*ssa.FieldAddr)
			if !ok {
				continue
			}
			hook := b.onstoreHookOf(fa.X.Type())
			if hook == nil || !slices.Contains(hook.fields, fa.Field) {
				continue
			}
			use := onstoreEscape(fa)
			if use == nil {
				continue
			}
			pos := use.Pos()
			if !pos.IsValid() {
				pos = fa.Pos()
			}
			named := types.Unalias(fa.X.Type().Underlying().(*types.Pointer).Elem()).(*types.Named)
			field := named.Underlying().(*types.Struct).Field(fa.Field).Name()
			b.addError(pos, fmt.Sprintf("the address of %s.%s, or of something in it, is used for more than a store or a load, and a store through it would not call %s, which //go:onstore runs after each store to the field: store to the field directly, or build with -experiment=noonstore", named.Obj().Name(), field, hook.fn.Name()))
		}
	}
}

// onstoreEscape returns a use of address v, or of an address inside it,
// that is neither a store to it nor a load from it, or nil if there is
// none.
func onstoreEscape(v ssa.Value) ssa.Instruction {
	for _, ref := range *v.Referrers() {
		switch ref := ref.(type) {
		case *ssa.Store:
			if ref.Addr == v && ref.Val != v {
				continue
			}
		case *ssa.UnOp:
			if ref.Op == token.MUL {
				continue
			}
		case *ssa.FieldAddr:
			if use := onstoreEscape(ref); use != nil {
				return use
			}
			continue
		case *ssa.IndexAddr:
			if use := onstoreEscape(ref); use != nil {
				return use
			}
			continue
		case *ssa.DebugRef:
			continue
		}
		return ref
	}
	return nil
}
