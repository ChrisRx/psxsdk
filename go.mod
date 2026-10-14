module go.chrisrx.dev/psx

go 1.27

tool (
	go.chrisrx.dev/psx/tools/dumpsxiso
	go.chrisrx.dev/psx/tools/eco2exe
	go.chrisrx.dev/psx/tools/elf2exe
	go.chrisrx.dev/psx/tools/mips1fix
	go.chrisrx.dev/psx/tools/mkpsxiso
)

require (
	github.com/mewmew/mips v0.1.0
	github.com/spf13/cobra v1.10.2
	go.bug.st/serial v1.0.0
	go.chrisrx.dev/x v0.0.0-20260830171909-b2f0655227fa
)

require (
	github.com/creack/goselect v0.1.1 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	golang.org/x/sys v0.44.0 // indirect
)
