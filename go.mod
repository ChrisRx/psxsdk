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
	github.com/alecthomas/chroma/v2 v2.27.0
	github.com/mewmew/mips v0.1.0
	github.com/spf13/cobra v1.10.2
	github.com/yuin/goldmark v1.8.6
	github.com/yuin/goldmark-highlighting/v2 v2.0.0-20230729083705-37449abec8cc
	go.bug.st/serial v1.0.0
	go.chrisrx.dev/x v0.0.0-20261015142902-ad113d4a3a98
)

require (
	github.com/creack/goselect v0.1.1 // indirect
	github.com/dlclark/regexp2/v2 v2.2.1 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	golang.org/x/sys v0.44.0 // indirect
)
