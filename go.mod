module go.chrisrx.dev/psx

go 1.27

tool (
	go.chrisrx.dev/psx/tools/discgen
	go.chrisrx.dev/psx/tools/dumpsxiso
	go.chrisrx.dev/psx/tools/eco2exe
	go.chrisrx.dev/psx/tools/elf2exe
	go.chrisrx.dev/psx/tools/mips1fix
	go.chrisrx.dev/psx/tools/mkdisc
	go.chrisrx.dev/psx/tools/mkpsxiso
)

require (
	github.com/ChrisRx/wazero v0.0.0-20261022122953-787716494703
	github.com/alecthomas/chroma/v2 v2.27.0
	github.com/hajimehoshi/ebiten/v2 v2.10.5
	github.com/mewmew/mips v0.1.0
	github.com/spf13/cobra v1.10.2
	github.com/spf13/pflag v1.0.9
	github.com/yuin/goldmark v1.8.6
	github.com/yuin/goldmark-highlighting/v2 v2.0.0-20230729083705-37449abec8cc
	go.bug.st/serial v1.0.0
	go.chrisrx.dev/x v0.0.0-20261015142902-ad113d4a3a98
)

require (
	github.com/creack/goselect v0.1.1 // indirect
	github.com/dlclark/regexp2/v2 v2.2.1 // indirect
	github.com/ebitengine/gomobile v0.0.0-20260820040257-d11f821a26a6 // indirect
	github.com/ebitengine/hideconsole v1.0.0 // indirect
	github.com/ebitengine/oto/v3 v3.5.0 // indirect
	github.com/ebitengine/purego v0.11.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jfreymuth/pulse v0.1.3 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
)

// Sony's manuals, 11MB of PDFs the docs site serves, which nothing that
// imports the module reads: left out of the module zip and of ./... patterns.
ignore ./docs/manuals
