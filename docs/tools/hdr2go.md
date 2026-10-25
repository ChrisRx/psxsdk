---
title: hdr2go
weight: 10
---

`hdr2go` generates Go bindings from a C header. It was written to bind the Net
Yaroze library for TinyGo, where the alternative was 166 functions and 30
structs transcribed by hand.

Transcription is not safe here. Every struct has to match the C layout byte
for byte, because the library reads and writes them directly, and a field at
the wrong offset does not fail to compile. It draws nothing. So rather than
read the header, `hdr2go` asks the compiler that will build the code, twice:

```bash
clang -target mipsel-none-elf -march=mips1 -msoft-float -std=gnu89 -fsyntax-only \
    -Xclang -ast-dump=json                  hdr.c   # structure and types
clang ... -Xclang -fdump-record-layouts-complete hdr.c  # offsets and sizes

$ hdr2go --layouts layouts.txt --header libps.h --package yaroze \
    --out yaroze --doc yaroze/libps.txt --skip GsLinkObject4 < ast.json
30 structs, 2 aliases, 165 functions (named=31 known=33 sizes=35)
```

Three files come out of it, because the library is only there on the
console and a program should still compile everywhere else.
`zz_generated.go` holds the structs, which every build shares.
`zz_generated_yaroze.go`, built only under the `yaroze` tag that the
`playstation-yaroze` target sets, holds the functions as bodyless
declarations carrying `//export`, which in TinyGo imports a C symbol rather
than exporting a Go one, and the size assertions. `zz_generated_stub.go`,
built everywhere else, holds the same functions with bodies that panic,
naming themselves and saying where the real one is, so that an editor, `go
vet` and `go build` all work on a program written against the library, and a
call into it fails at run time rather than at the build. `--tag` names the
tag, and `--out` the directory the three go into.

Padding the C compiler inserted implicitly is written out explicitly, and
each struct's size is asserted against clang's number in the console's file:

```go
var _ [unsafe.Sizeof(GsSPRITE{}) - 36]byte
var _ [36 - unsafe.Sizeof(GsSPRITE{})]byte
```

An array cannot have negative length, so the pair fails to compile unless the
size is exactly right. This is deliberately not a test: a host toolchain will
not compile the bodyless function declarations, and a bare-metal target has
nowhere to run one, so an assertion inside the generated file is the only
check that runs during the build that matters. On a host, where a pointer is
wider, the structs that hold one are a different size, and nothing there
depends on it.

The comments come from the file `--doc` names, so that the documentation an
editor shows is kept apart from what the generator derives. It is plain
text: a name on a line of its own, the C one, or `Struct.field` for one
field, opens an entry, and the tab-indented lines after it are the comment,
written as the Go one, with a blank line between paragraphs. An entry that
matches nothing generated is reported, which is how a mistyped name shows
up. The library's is `yaroze/libps.txt`, condensed from Sony's library
reference.

Names stay exactly as the header writes them, so the library's documentation
applies unchanged. The one exception is that a C name beginning with a
lowercase letter is capitalized, since Go would otherwise put it out of reach
of every other package; `//export` carries the real symbol either way.

Anything the type mapping does not recognize is reported rather than guessed
at, and the report names what has to be bound by hand:

```
1 declarations with unmapped types, skipped:
  GsOT_TAG: uses bitfields, bind by hand
9 unprototyped, variadic or skipped, bind by hand:
  FntPrint int ()
  GsLinkObject4 void (unsigned long, GsDOBJ2 *, int) (--skip)
  ...
```

An empty parameter list is the common case there. In this vintage of C it
means "unspecified", not "none", so generating a zero-argument binding would
produce something that compiles and then passes nothing. `--skip` leaves out
a function whose header signature is not the one to offer: `GsLinkObject4`
takes a pointer typed as an integer, and the package binds it by hand with a
Go signature that inlines to the same call.
