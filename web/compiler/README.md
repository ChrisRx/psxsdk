# The compiler in the browser

The player's "Write your own" panel builds a program for the Net Yaroze
in the browser and runs it in the emulator. The compiler behind it is
TinyGo itself, compiled to WebAssembly, with LLVM, clang and lld linked
into it the way a release build of TinyGo links them, running against a
file system held in memory.

This directory holds the recipe for building it. `build.sh` is the whole
of it; the other files are what it feeds to LLVM.

## How the pieces fit

TinyGo is a Go program bound to LLVM through cgo, and Go's own WebAssembly
ports have no cgo. TinyGo's does: its wasip1 target compiles the C parts
with clang and links them with wasm-ld. So TinyGo compiles TinyGo, and
the C++ it links against is LLVM built for wasm32-wasi:

- `llvm/configure.sh` configures LLVM, clang and lld as static libraries
  for wasm32-wasi with only the Mips backend, no threads and no tools,
  using nixpkgs' wasi cross toolchain. Clang goes into one of two builds
  of the compiler. A release TinyGo compiles a target's C and assembly
  with an in-process clang, and that is half of the module, so the shipped
  `tinygo.wasm` leaves it out and relies on TinyGo's cache instead, while
  `tinygo-full.wasm` keeps it and never leaves the build machine:
  `mkcache.sh` runs it, under wasmtime, over every example against the
  same sysroot paths the browser uses, and the cache ships in the sysroot
  tarball keyed by the shipped compiler's hash. That also gives a first
  build in the browser the standard library ready-compiled. `llvm/llvm-wasi.patch` and
  `llvm/wasi-compat/` are what LLVM's Support library needs to compile
  without signals, processes, sockets or users: a few declarations and
  stubs, and the code paths that catch crashes or run subprocesses cut
  out. `llvm/libcxx.sh` builds libc++ with its threading API on, because
  LLVM's headers use `std::mutex` whatever `LLVM_ENABLE_THREADS` says;
  nothing ever starts a thread.
- `glue/` is the C and C++ that is not LLVM: go-llvm's bindings, an lld
  driver with only the ELF linker, TinyGo's clang driver for the full
  build, and `wasi-stubs.c`, the POSIX functions LLVM links against and
  WASI does not have, each failing the way a system without the feature
  would.
- `target/gen.sh` writes a TinyGo target, wasip1 plus every library above
  on the link line and 32MB stacks. TinyGo runs goroutines, the main one
  included, on heap-allocated stacks of the target's default size, and
  LLVM's recursion needs far more than the usual 64K.

The compiler itself is TinyGo with `tinygo-wasm.patch` applied, which
`prepare.sh` does to a copy of the fork. The fork carries only the
PlayStation targets, as a pull request to TinyGo would; the patch holds
what running the compiler as a WebAssembly program needs: `cmd/tinygo-wasm`,
the build command alone, and under the `wasm` build tag a loader that
lists packages with go/build instead of running `go list`, takes GOROOT
and GOVERSION from the environment instead of `go env`, uses a GOROOT the
host merged in advance, and runs lld in-process instead of as a
subprocess. `cmd/tinygo-sysroot`, also in the patch, gathers that merged
GOROOT, the psx packages and the target files into the tree the compiler
reads, which the `playground` task packs, with the cache, as
`sysroot.tar.gz`. `prepare.sh` says how to regenerate the patch.

In the page, `compile-worker.js` fetches all of that once and
`pipeline.js` runs each build: the compiler, then `mips1fix`, `eco2exe`
and `mkpsxiso` compiled to wasip1 by the Go toolchain, through `wasi.js`,
a WASI host over an in-memory file system. The output is a disc the
player boots like an example. The data files the page holds beside the
program (`assets.js`), uploaded or copied from an example's data, which
the `site` task publishes under `examples/NAME/data/` and lists in
`examples.json`, go into the build's `data/` for mkpsxiso to put on the
disc where the program registers them with package disc.

## In the workflows

The Compiler workflow (`.github/workflows/compiler.yml`) builds the two
modules when this directory, the flake or the fork's revision changes, or
on request, caches them under a key made of those, less the Markdown
(`key.sh`), and
publishes them as a release named by the key (`release.sh`), which is kept
where the cache drops an entry unused for a week, then runs the Pages
workflow. The Pages workflow restores that cache or downloads the release
when it misses, through the same composite action
(`.github/actions/compiler`), and never builds the compiler itself: with
neither, a push to main skips the deploy until the Compiler workflow has
published it, and a pull request builds the site without the playground.
A release published from a local build (`release.sh .build/compiler`)
before the change is pushed saves the Compiler workflow the build. The
action always
prepares the patched TinyGo tree, then runs `task playground` in the
`playground` shell, which builds that tree's `tinygo-sysroot` natively and
runs the full module under wasmtime to fill the cache. The fork is checked out at the revision `flake.nix` pins, so
the compiler in the browser is the TinyGo the examples were built with.

## Limits

- The compiler is around 75MB, 22MB compressed, and the sysroot with the cache 3MB.
- A program is one file, with its data files under `data/`, kept in the
  browser that added them. It cannot use cgo,
  or anything whose C is not already in the cache: there is no clang.
  Only the ELF flavor of lld is linked in, for the same reason of size.
- The cache and the sysroot are made together, by the `playground` task,
  and a compiler with another hash ignores the cache.
- Memory is TinyGo's garbage collector's, for LLVM's allocations too, and
  they are scanned conservatively.
