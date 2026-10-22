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

package main

import (
	"testing"

	"go.chrisrx.dev/x/assert"
)

func TestSystemCNF(t *testing.T) {
	assert.Equal(t, "BOOT=cdrom:\\MAIN.EXE;1\nTCB=4\nEVENT=10\nSTACK=801FF000\n",
		string(systemCNF("main.exe;1", defaultStack)))
	assert.Equal(t, "BOOT=cdrom:\\MAIN.EXE;1\nTCB=4\nEVENT=10\nSTACK=801FFFF0\n",
		string(systemCNF("MAIN.EXE;1", 0x801ffff0)))
}

func TestCheckStack(t *testing.T) {
	for _, sp := range []uint32{defaultStack, 0x801ffff0, 0x80200000, 0x80100000} {
		assert.NoError(t, checkStack(sp), "stack %#08x", sp)
	}

	assert.Error(t, "outside main RAM", checkStack(0x80000000))
	assert.Error(t, "outside main RAM", checkStack(0x80200008))
	assert.Error(t, "outside main RAM", checkStack(0x001ff000))
	assert.Error(t, "outside main RAM", checkStack(0xa01ff000))
	assert.Error(t, "not 8-byte aligned", checkStack(0x801ffff4))
}
