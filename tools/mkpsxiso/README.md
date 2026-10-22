# mkpsxiso

The code in this directory is a translation into Go of
[mkpsxiso](https://github.com/Lameguy64/mkpsxiso), by Lameguy64 and its
contributors, which is licensed under the GNU General Public License,
version 2. As a translation it is a derivative work, so it is licensed under
the same terms: GPL-2.0-only, whose text is in
[`licenses/GPL-2.0-only.txt`](../../licenses/GPL-2.0-only.txt) at the root
of the repository. It is not covered by the MIT license the rest of the
repository is under, and a program that links it is bound by the GPL.

## Changes from the original

The translation was made in 2026 by Chris Marshall, and differs from the
original in more than language:

- It takes a directory tree, host files placed at paths with `--file`, and
  the executable named by `--boot`, in place of the XML project file the
  original is driven by.
- It finds the records that package `disc` leaves in the executable, places
  the files they name, and writes each file's sector and length back into
  the executable, which the original has no counterpart to.
- It writes through this repository's `format/iso9660`, `format/psxdisc`
  and `format/cdrom` packages, which are independent implementations of
  those formats under the MIT license, rather than carrying its own sector
  and filesystem code.

Every file here carries an SPDX header saying the same.

## Status

`mkdisc`, in `internal/mkdisc` and `tools/mkdisc`, replaces this tool. It
was written from the ISO 9660 and CD-ROM XA standards and the library
packages above, takes the same flags, and is what `yaroze disc`, `yaroze iso` and
the playground build with. This tool is kept as it was, for comparison, and
still runs as `go tool mkpsxiso`.
