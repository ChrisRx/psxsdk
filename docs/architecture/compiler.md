---
title: The compiler in the browser
weight: 2
description: TinyGo compiled to WebAssembly, with LLVM and lld linked in: how it is built, why clang is left out, how the playground runs a build, and its limits.
---

The [playground](/play/) builds a program and runs it without anything installed, and so does [`nycw`](../tools/nyc.md). The compiler behind both is TinyGo itself, compiled to WebAssembly, with LLVM and lld linked into it the way a release build of TinyGo links them. In the browser it runs against a file system held in memory, and on the desktop `nyc` runs it in-process under [wazero](https://wazero.io/). [`web/compiler`](../../web/compiler) holds the recipe, and `build.sh` is the whole of it.

## TinyGo compiles TinyGo

TinyGo is a Go program bound to LLVM through cgo, and Go's own WebAssembly ports have no cgo. TinyGo's does: its wasip1 target compiles the C parts with clang and links them with wasm-ld. So TinyGo compiles TinyGo, against LLVM built for wasm32-wasi:

- **LLVM** (`llvm/configure.sh`): LLVM, clang and lld as static libraries for wasm32-wasi, with only the Mips backend, no threads and no tools, using nixpkgs' wasi cross toolchain. `llvm-wasi.patch` and `wasi-compat/` are what LLVM's Support library needs to compile without signals, processes, sockets or users: a few declarations and stubs, and the code paths that catch crashes or run subprocesses cut out. `libcxx.sh` builds libc++ with its threading API on, because LLVM's headers use `std::mutex` whatever `LLVM_ENABLE_THREADS` says. Nothing ever starts a thread.
- **Glue** (`glue/`): the C and C++ that is not LLVM's. That is go-llvm's bindings, an lld driver with only the ELF linker, TinyGo's clang driver for the full build, and `wasi-stubs.c`, the POSIX functions LLVM links against and WASI does not have, each failing the way a system without the feature would.
- **The target** (`target/gen.sh`): wasip1, with every library above on the link line and 32MB stacks. TinyGo runs every goroutine, the main one included, on a heap-allocated stack of the target's default size, and LLVM's recursion needs far more than the usual 64K. With less, it overflows and corrupts memory, and fails differently on every build.
- **TinyGo** (`tinygo-wasm.patch`, applied by `prepare.sh` to a copy of the fork): the fork carries only the PlayStation targets, as a pull request to TinyGo would, and the patch holds what running the compiler as a WebAssembly program needs. That is `cmd/tinygo-wasm`, the build command alone, and under the `wasm` build tag a loader that lists packages with go/build instead of running `go list`, takes GOROOT and GOVERSION from the environment instead of `go env`, uses a GOROOT merged in advance, and runs lld in-process instead of as a subprocess. `cmd/tinygo-sysroot`, also in the patch, gathers that merged GOROOT, the psx packages and the target files into the tree the compiler reads. `prepare.sh` says how to regenerate the patch.

## Leaving clang out

A release TinyGo compiles a target's C and assembly with an in-process clang, and clang is more than half of the module. So there are two builds. The shipped `tinygo.wasm` leaves clang out and is 75MB, 22MB gzipped. `tinygo-full.wasm` keeps it, at 169MB, and never leaves the build machine: `mkcache.sh` runs it under wasmtime over every example, against the same sysroot paths the browser uses, and the cache it fills ships in `sysroot.tar.gz`, keyed by the shipped compiler's hash. A first build then finds the target's C and the standard library already compiled. The playground's sysroot holds the cache at TinyGo's default `-opt=z` and is 5MB; `nycw`'s holds `-opt=2` as well and is 8.8MB.

## A build in the playground

`compile-worker.js` fetches the compiler, the tools and the sysroot once, and `pipeline.js` runs each build in a worker: the program's `//go:generate` lines, then the compiler, then `mips1fix`, `eco2exe` and `mkpsxiso`. The tools are the repository's own, compiled to wasip1 by the Go toolchain and run through `wasi.js`, a WASI host over an in-memory file system. The result is a disc the player boots like an example.

What is built is a project (`project.js`): the program's Go files, a tab each over the editor (`files.js`), and its data files, in the Files sidebar (`sidebar.js`, with `inspect.js` to view them), uploaded or copied from an example. The data goes under `assets/`, for `mkpsxiso` to put on the disc where the program registers it with package `disc`. Sources under `assets/` are converted by the program's own `//go:generate` lines (`convert.js`), which the worker runs with the converters built to wasip1. A scene is placed in the Scene tab (`scene-tab.js`, drawing with `preview.js`) and cooked by `mkscene` the same way, for package `scene` to read on the console. Projects are kept in the browser's IndexedDB, and go out and come in as zips laid out like the examples, so a download runs with `go run`.

While a project runs, each cooked scene is also written into the emulator's RAM over the file the program loaded, and package `scene`'s `live` variable tells the program to read it again, so the console follows the editor without a rebuild. That goes through the worker's `symbol`, `peek` and `poke` messages to `psx.symbol`, `psx.read` and `psx.write` in `web/main.go`.

## Building it

`task compiler` builds the two modules, which takes most of an hour, and `task playground` adds them, the tools, gofmt and the sysroot to the site. The fork is checked out at the revision `flake.nix` pins, so the compiler in the browser is the TinyGo the examples were built with, and `task nyc` copies the same compiler and a sysroot into `cmd/nycw/wasm` for `nycw` to embed.

In CI, the Compiler workflow builds the modules when `web/compiler`, the flake or the fork's revision changes, or on request. It caches them under a key made of those, less the Markdown (`key.sh`), publishes them as a release named by the key (`release.sh`), which outlives the cache's week, and then runs the Pages workflow. Pages restores the cache, or downloads the release on a miss, through the same composite action, and never builds the compiler itself: with neither, a push to main skips the deploy until the Compiler workflow has published one, and a pull request builds the site without the playground. A release published from a local build (`release.sh .build/compiler`) before the change is pushed saves the workflow the build.

## Limits

- There is no clang, so a program cannot use cgo or anything whose C is not already in the cache. Only the ELF flavor of lld is linked in, for size.
- The cache and the sysroot are made together, by `task playground`, and a compiler with another hash ignores the cache.
- The compiler builds only at the optimization levels its cache holds.
- Memory is TinyGo's garbage collector's, LLVM's allocations included, and is scanned conservatively.
- A project is kept in the browser that made it, until it is downloaded.
