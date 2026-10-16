---
title: hdr2go
weight: 9
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

$ hdr2go --layouts layouts.txt --header libps.h --package libps \
    --out libps/zz_generated.go < ast.json
30 structs, 2 aliases, 166 functions (named=31 known=33 sizes=35)
```

Padding the C compiler inserted implicitly is written out explicitly, and
each struct's size is asserted against clang's number:

```go
var _ [unsafe.Sizeof(GsSPRITE{}) - 36]byte
var _ [36 - unsafe.Sizeof(GsSPRITE{})]byte
```

An array cannot have negative length, so the pair fails to compile unless the
size is exactly right. This is deliberately not a test: a host toolchain will
not compile the bodyless function declarations, and a bare-metal target has
nowhere to run one, so an assertion inside the generated file is the only
check that runs during the build that matters.

Names stay as the header spells them, so the library's documentation reads
across directly. The one departure is that a C name beginning with a
lowercase letter is capitalised, since Go would otherwise put it out of reach
of every other package; `//export` carries the real symbol either way.

Anything the type mapping does not recognise is reported rather than guessed
at, and the report names what has to be bound by hand:

```
1 declarations with unmapped types, skipped:
  GsOT_TAG: uses bitfields, bind by hand
8 unprototyped or variadic, bind by hand:
  FntPrint int ()
  ...
```

An empty parameter list is the common case there. In this vintage of C it
means "unspecified", not "none", so generating a zero-argument binding would
produce something that compiles and then passes nothing.
