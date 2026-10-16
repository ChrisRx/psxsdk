---
title: How it fits together
weight: 1
section: Guide
---

```
examples/check/main.go
        |  tinygo build -target=playstation-yaroze
        v
    prog.elf  --mips1fix-->  prog.elf  --eco2exe-->  MAIN.EXE
                                                        |
                    data/... (disc.Register)  --------> mkpsxiso
                                                        v
                                               check.bin + check.cue
                                                        |
                       +--------------------------------+---------------------------+
                       v                                v                           v
              emu (go test, emutest)          web (WebAssembly, Pages)      real hardware / psxe
```

A Net Yaroze program is only half a program: the rest is `libps`, the library the Net Yaroze boot disc leaves resident in RAM. `eco2exe` combines the two into a standalone PSX-EXE, so the result boots from an ordinary disc in an emulator or on a console that plays burned discs, with no Net Yaroze hardware involved.
