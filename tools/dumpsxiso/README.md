# dumpsxiso

The code in this directory is a translation into Go of dumpsxiso, part of
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

- It lists the image, or extracts its files to a directory, in place of
  writing the XML project file and licence file the original produces for
  mkpsxiso to rebuild the image from.
- It reads through this repository's `format/iso9660`, `format/psxdisc`
  and `format/cdrom` packages, which are independent implementations of
  those formats under the MIT license, rather than carrying its own sector
  and filesystem code.

The file here carries an SPDX header saying the same.

## Status

Nothing else in the repository uses this tool: `yaroze`, the libraries and
the playground do not import it. It runs as `go tool dumpsxiso`, and is
kept as it is.
