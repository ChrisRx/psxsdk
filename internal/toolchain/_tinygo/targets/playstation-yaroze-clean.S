# Start-up for the playstation-yaroze-clean target: what runs from the
# entry point to C main in a program built against psxsdk's exp/cleanlib.
# Written from exp/cleanlib/spec/07-startup.md.
#
# At entry the BIOS has set sp and fp to the top of the stack and left
# interrupts off. This sets gp, clears .sbss and .bss, hands the clean
# library's InitHeap everything from the end of the image to 64KB below
# the stack, enables interrupts and calls main. v0, v1, a1 to a3 and s0 to
# s7 reach main as the BIOS left them.
#
# The time to main is part of every program's boot timing, so it is kept
# to the cycle of the start-up this replaces: InitHeap is reached 146
# cycles after entry plus 10 per word cleared, the system call 58 cycles
# after InitHeap is entered (with the clean InitHeap), and main 486 after
# the system call. Each instruction here costs 2 cycles, fetched from RAM,
# and the nops below make up the difference.

#define PAD_INITHEAP 50
#define PAD_SYSCALL 23
#define PAD_MAIN 1

	.set noreorder
	.set noat

	.text
	.align 2
	.globl _start
	.ent _start
_start:
	lui   $gp, %hi(_gp)
	addiu $gp, $gp, %lo(_gp)

	# Clear the words from _fbss up to, not including, _end.
	lui   $t0, %hi(_fbss)
	addiu $t0, $t0, %lo(_fbss)
	lui   $t1, %hi(_end)
	addiu $t1, $t1, %lo(_end)
	sltu  $at, $t0, $t1
	beqz  $at, 2f
	nop
1:	sw    $zero, 0($t0)
	addiu $t0, $t0, 4
	sltu  $at, $t0, $t1
	bnez  $at, 1b
	nop
2:
	move  $fp, $sp

	# Keep the BIOS's ra for a return from main, and v0, v1 and a1 to a3
	# across InitHeap.
	lui   $t2, %hi(libps_start_ra)
	sw    $ra, %lo(libps_start_ra)($t2)
	addiu $sp, $sp, -40
	sw    $v0, 20($sp)
	sw    $v1, 24($sp)
	sw    $a1, 28($sp)
	sw    $a2, 32($sp)
	sw    $a3, 36($sp)

	# The clearing loop takes its 10 cycles a word. The rest of the 146 is
	# made up here.
	.rept PAD_INITHEAP
	nop
	.endr

	# InitHeap(_end, sp - 64KB - _end).
	move  $a0, $t1
	lui   $t2, 1
	subu  $a1, $fp, $t2
	jal   InitHeap
	subu  $a1, $a1, $a0

	# Padding to the system call's cycle, and after it to main's.
	.rept PAD_SYSCALL
	nop
	.endr

	# ExitCriticalSection.
	li    $a0, 2
	syscall
	.rept PAD_MAIN
	nop
	.endr

	lw    $v0, 20($sp)
	lw    $v1, 24($sp)
	lw    $a1, 28($sp)
	lw    $a2, 32($sp)
	lw    $a3, 36($sp)
	jal   main
	addiu $sp, $sp, 40

	# main does not return in a Go program. Should it, EnterCriticalSection
	# and back to the BIOS.
	li    $a0, 1
	syscall
	lui   $t2, %hi(libps_start_ra)
	lw    $ra, %lo(libps_start_ra)($t2)
	nop
	jr    $ra
	nop
	.end _start

	.data
	.align 2
libps_start_ra:
	.word 0
