# The compiler in the browser

The recipe for TinyGo compiled to WebAssembly, the compiler the
playground runs and `nycw` embeds. `build.sh` is the whole of it; the
other files are what it feeds to LLVM.

How it is built, why clang is left out, how the playground runs a build,
the workflows and the limits are on the documentation's page,
[The compiler in the browser](../../docs/architecture/compiler.md)
([on the site](https://chrisrx.github.io/psxsdk/architecture/compiler/)).
