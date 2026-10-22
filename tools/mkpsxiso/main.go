// SPDX-License-Identifier: GPL-2.0-only
// SPDX-FileCopyrightText: Lameguy64 and the mkpsxiso contributors
// SPDX-FileCopyrightText: 2026 Chris Marshall
//
// This file is part of a translation into Go of mkpsxiso,
// https://github.com/Lameguy64/mkpsxiso, made in 2026 by Chris Marshall and
// changed in the translation to take a directory and the records a program
// leaves in its executable in place of an XML project file. See README.md
// in this directory. It is free software under the GNU General Public
// License version 2, the text of which is licenses/GPL-2.0-only.txt at the
// root of the repository, and it is not covered by the MIT license the rest
// of the repository is under.

// Command mkpsxiso builds a PlayStation disc image from a directory. It is
// the translation of mkpsxiso this repository began with; mkdisc, which yaroze
// uses, replaces it.
package main

import (
	"log"
)

func main() {
	if err := Command("mkpsxiso").Execute(); err != nil {
		log.Fatal(err)
	}
}
