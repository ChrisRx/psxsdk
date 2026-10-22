// Package volatile is the part of TinyGo's runtime/volatile that the
// toolchain's own code needs, with a plain version for every other build,
// so that a package reading hardware registers or words that mkdisc
// patches after the link compiles on a host too, where an editor and go
// vet see it.
package volatile
