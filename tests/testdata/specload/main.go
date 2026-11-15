// Command specload holds loads that a test guards: the compiler must leave
// each one behind the branch that says it is safe. TestGuardedLoadsStay
// reads the machine code of guarded and field.
package main

import "go.chrisrx.dev/psx/yaroze"

type node struct {
	next  *node
	count int32
	value int32
}

var (
	table [16]int32
	nodes [2]node
	sink  int32
)

// guarded reads table only at an index the test before it allows.
//
//go:noinline
func guarded(i int) int32 {
	if uint(i) >= uint(len(table)) {
		return -1
	}
	return table[i]
}

// field reads through p only when it is not nil.
//
//go:noinline
func field(p *node) int32 {
	if p == nil {
		return -1
	}
	return p.value
}

func main() {
	for i := range 32 {
		table[i&15] = int32(i)
		nodes[i&1].value = int32(i)
		sink += guarded(i)
		sink += field(nodes[i&1].next)
		sink += field(&nodes[i&1])
	}
	for {
		yaroze.VSync(0)
	}
}
