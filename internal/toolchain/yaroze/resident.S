# Start-up for a program on the resident library (internal/libps/spec/26-members.md): what
# runs from the entry point to C main, and from main's end back to the BIOS. Its time is
# part of every program's boot, so it is kept to the cycle of the start-up this replaces,
# by the probes in spec 26: InitHeap is reached 146 cycles after entry plus 10 per word
# cleared, with one word cleared at the least, the system call 44 after InitHeap returns
# and main four instructions after the system call returns. Each instruction here costs 2
# cycles, and the nops make up the difference.
#
# The data below is the program's to link other values into. The -1s mean what the BIOS
# or the start-up left: _sys_init_gp's link value is _gp, and -1 keeps the BIOS's gp;
# _sys_init_sp's -1 keeps the BIOS's sp; _sys_init_fp's -1 makes fp sp. _sys_init_heapsize
# is written here and not read, and _sys_ramsize is not read at all. _sys_init_bss_flag of
# 0 leaves .bss as it is.

	.set noreorder
	.set noat

	.text
	.align 2
	.globl _start
	.ent _start
_start:
	li    $t3, -1

	# gp, unless the word says to keep it.
	lui   $t0, %hi(_sys_init_gp)
	lw    $t0, %lo(_sys_init_gp)($t0)
	nop
	beq   $t0, $t3, 1f
	nop
	move  $gp, $t0
1:

	# The words from _fbss to _end: at least the one at _fbss, even if that is _end.
	lui   $t0, %hi(_sys_init_bss_flag)
	lw    $t0, %lo(_sys_init_bss_flag)($t0)
	nop
	beqz  $t0, 3f
	nop
	lui   $t0, %hi(_fbss)
	addiu $t0, $t0, %lo(_fbss)
	lui   $t1, %hi(_end)
	addiu $t1, $t1, %lo(_end)
	nop
2:	sw    $zero, 0($t0)
	addiu $t0, $t0, 4
	sltu  $at, $t0, $t1
	bnez  $at, 2b
	nop
3:

	# sp, and fp: sp unless the word says otherwise.
	lui   $t0, %hi(_sys_init_sp)
	lw    $t0, %lo(_sys_init_sp)($t0)
	nop
	beq   $t0, $t3, 4f
	nop
	move  $sp, $t0
4:	lui   $t0, %hi(_sys_init_fp)
	lw    $t0, %lo(_sys_init_fp)($t0)
	nop
	bne   $t0, $t3, 5f
	move  $fp, $t0
	move  $fp, $sp
5:

	# Keep the BIOS's ra for the return, and v0, v1 and a1 to a3 across InitHeap.
	lui   $t0, %hi(_sys_start_ra)
	sw    $ra, %lo(_sys_start_ra)($t0)

	# InitHeap(_sys_init_heapbase, sp - _sys_init_stacksize - _sys_init_heapbase).
	lui   $t0, %hi(_sys_init_heapbase)
	lw    $t1, %lo(_sys_init_heapbase)($t0)
	lui   $t0, %hi(_sys_init_stacksize)
	lw    $t0, %lo(_sys_init_stacksize)($t0)
	nop
	subu  $t0, $sp, $t0
	subu  $t0, $t0, $t1
	lui   $t2, %hi(_sys_init_heapsize)
	sw    $t0, %lo(_sys_init_heapsize)($t2)
	addiu $sp, $sp, -40
	sw    $v0, 20($sp)
	sw    $v1, 24($sp)
	sw    $a1, 28($sp)
	sw    $a2, 32($sp)
	sw    $a3, 36($sp)
	.rept 25
	nop
	.endr
	move  $a0, $t1
	jal   InitHeap
	move  $a1, $t0

	# Padding to the system call's cycle, and the registers back.
	.rept 16
	nop
	.endr
	lw    $v1, 24($sp)
	lw    $a1, 28($sp)
	lw    $a2, 32($sp)
	lw    $a3, 36($sp)

	# ExitCriticalSection, and on to main.
	li    $a0, 2
	syscall
	lw    $v0, 20($sp)
	nop
	jal   main
	addiu $sp, $sp, 40

	# main has returned: EnterCriticalSection, and then what exit does.
	nop
	nop
	li    $a0, 1
	syscall
	nop
	nop
	.end _start

# exit(status) goes back to the BIOS, with interrupts as they are and sp as it is,
# through __main's return. The status goes nowhere.
	.globl exit
	.type exit, @function
exit:
	lui   $t0, %hi(_sys_start_ra)
	lw    $ra, %lo(_sys_start_ra)($t0)
	nop
	nop
	.size exit, .-exit

# __main is what an old C compiler calls at the top of main. It has nothing to do.
	.globl __main
	.type __main, @function
__main:
	jr    $ra
	nop
	.size __main, .-__main

	.data
	.macro SYSWORD name, value
	.align 4
	.globl \name
\name:
	.word \value
	.endm
	SYSWORD _sys_ramsize, 0x00200000
	SYSWORD _sys_init_stacksize, 0x00010000
	SYSWORD _sys_init_sp, -1
	SYSWORD _sys_init_fp, -1
	SYSWORD _sys_init_gp, _gp
	SYSWORD _sys_init_heapbase, _end
	SYSWORD _sys_init_heapsize, 0
	SYSWORD _sys_init_bss_flag, 1
	.align 2
_sys_start_ra:
	.word 0
