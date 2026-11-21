package ecoffelf

import (
	"strconv"
	"strings"
)

// demangle undoes the commonest of GCC 2's C++ name mangling, which current
// debuggers no longer read: a member function name__<n>Class<args> is
// Class::name, a constructor __<n>Class<args> Class::Class, a destructor
// _$_<n>Class Class::~Class, a function overloaded at file level
// name__F<args> is name, and a few operators have their names. Anything
// else is left as it is.
func demangle(s string) (string, bool) {
	if rest, ok := strings.CutPrefix(s, "_$_"); ok {
		if class, ok := className(rest); ok {
			return class + "::~" + class, true
		}
		return s, false
	}
	if rest, ok := strings.CutPrefix(s, "_._"); ok {
		if class, ok := className(rest); ok {
			return class + "::~" + class, true
		}
		return s, false
	}
	// A constructor: __ and the class.
	if rest, ok := strings.CutPrefix(s, "__"); ok && rest != "" && rest[0] >= '0' && rest[0] <= '9' {
		if class, ok := className(rest); ok {
			return class + "::" + class, true
		}
	}
	// The name is up to the last "__" before a class or F.
	for i := len(s) - 2; i > 0; i-- {
		if s[i] != '_' || s[i+1] != '_' {
			continue
		}
		name, rest := s[:i], s[i+2:]
		if op, ok := operators[name]; ok {
			name = op
		}
		switch {
		case strings.HasPrefix(rest, "F"):
			return name, true
		case rest != "" && rest[0] >= '0' && rest[0] <= '9':
			if class, ok := className(rest); ok {
				return class + "::" + name, true
			}
		}
	}
	return s, false
}

// className reads <n>Class from the start of s.
func className(s string) (string, bool) {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	n, err := strconv.Atoi(s[:i])
	if err != nil || n <= 0 || i+n > len(s) {
		return "", false
	}
	return s[i : i+n], true
}

var operators = map[string]string{
	"__nw": "operator new", "__dl": "operator delete",
	"__vn": "operator new[]", "__vd": "operator delete[]",
	"__as": "operator=", "__eq": "operator==", "__ne": "operator!=",
	"__pl": "operator+", "__mi": "operator-", "__ml": "operator*",
	"__dv": "operator/", "__lt": "operator<", "__gt": "operator>",
	"__vc": "operator[]", "__cl": "operator()",
}
