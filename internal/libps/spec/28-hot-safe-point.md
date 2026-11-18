# VSync stays a function

Hot reload (`internal/hot`) needs the address of the library's VSync in a program's ELF, under the symbol `VSync`: it is where every frame of the main package's code has returned, so it is where a patch
can be applied. On the static target the compiler now inlines VSync into its callers, and `yaroze disc --hot` stops with "the base has no VSync".

Keep VSync a real function, under that symbol, on the static target, and with nothing else about it changed. Its time on the resident image should not move.

## Acceptance

- `task --force build:hot`, then `go test ./internal/hot`.
- `exp/cleanlib/image/make.sh` builds, with VSync at the same cost: a probe or `TestArchiveCalls` on any program.
- `exp/cleanlib/regress.sh` without `PSX_BIOS`: FAIL stays 0, with 33 PASS and 8 TIME before.
