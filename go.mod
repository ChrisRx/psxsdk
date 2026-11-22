module go.chrisrx.dev/psx

go 1.27.3

require (
	github.com/ChrisRx/delve v0.0.0-20261122141123-a6c8a67bf7cf
	github.com/ChrisRx/wazero v0.0.0-20261116205937-3f122248aff2
	github.com/alecthomas/chroma/v2 v2.27.0
	github.com/google/go-dap v0.12.0
	github.com/gopikchr/gopikchr v0.0.0-20251102185413-ac35dc1606f8
	github.com/hajimehoshi/ebiten/v2 v2.10.5
	github.com/mewmew/mips v0.1.0
	github.com/spf13/cobra v1.10.2
	github.com/spf13/pflag v1.0.10
	github.com/yuin/goldmark v1.8.6
	github.com/yuin/goldmark-highlighting/v2 v2.0.0-20230729083705-37449abec8cc
	go.bug.st/serial v1.8.0
	go.chrisrx.dev/pywasi v0.4.1
	go.chrisrx.dev/x v0.0.0-20261103231709-6405755dad2b
	golang.org/x/term v0.47.0
)

require (
	github.com/cilium/ebpf v0.22.0 // indirect
	github.com/clipperhouse/stringish v0.1.1 // indirect
	github.com/clipperhouse/uax29/v2 v2.5.0 // indirect
	github.com/cosiner/argv v0.1.0 // indirect
	github.com/derekparker/trie/v3 v3.2.0 // indirect
	github.com/dlclark/regexp2/v2 v2.2.1 // indirect
	github.com/ebitengine/gomobile v0.0.0-20260820040257-d11f821a26a6 // indirect
	github.com/ebitengine/hideconsole v1.0.0 // indirect
	github.com/ebitengine/oto/v3 v3.5.0 // indirect
	github.com/ebitengine/purego v0.11.0 // indirect
	github.com/go-delve/liner v1.2.3-0.20231231155935-4726ab1d7f62 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jfreymuth/pulse v0.1.3 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.19 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	go.starlark.net v0.0.0-20231101134539-556fd59b42f6 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/arch v0.28.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/telemetry v0.0.0-20241106142447-58a1122356f5 // indirect
)

// Sony's manuals, 11MB of PDFs the docs site serves, which nothing that
// imports the module reads: left out of the module zip and of ./... patterns.
ignore ./docs/manuals
