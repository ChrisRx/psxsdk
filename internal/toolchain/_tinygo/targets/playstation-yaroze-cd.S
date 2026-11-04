# Entry points the resident library has that symusr.o does not list.
# Sony exported CdRead but not the CdControl that positions the drive for
# it, so a program could read a file only by name, through CdReadFile,
# which searches the directory for it each time. The addresses come from
# the library itself: CdReadFile calls CdControl with CdlSetloc and the
# file's position before it calls CdRead, and afterwards moves that
# position on through CdPosToInt and CdIntToPos. All read as libcd's do:
# the control routines with their retries, CdControl waiting for the
# drive's acknowledge and CdControlB for the command's completion (the
# routine at 0x80039618 is CdControlF, which waits for neither), and the
# conversions as the arithmetic between a sector number and binary coded
# minutes, seconds and sectors. psxsdk's yaroze package declares them.
#
# They are weak, as symusr.S's are, and under LIBPS_GO, where nothing is
# resident, the file defines nothing, so a call the program's own library
# lacks fails at link time by name.
#ifndef LIBPS_GO
	.weak CdControl
	.set CdControl, 0x800394E0
	.weak CdControlB
	.set CdControlB, 0x80039744
	.weak CdIntToPos
	.set CdIntToPos, 0x80039914
	.weak CdPosToInt
	.set CdPosToInt, 0x80039A18
#endif
