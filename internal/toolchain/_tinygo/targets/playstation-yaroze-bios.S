# BIOS calls for psxsdk's exp/libps, the Net Yaroze library in Go.
#
# The kernel's functions are reached by jumping through a table's address,
# 0xA0, 0xB0 or 0xC0, with the function number in t1. These take the
# number first and the function's own arguments after it, shift them into
# place, and jump: the kernel returns to the caller directly. Assembly
# rather than inline asm, since a call inside inline asm would clobber
# registers the compiler is not told about.
#
# The fifth argument is at 16(sp) by the o32 convention, where the caller
# leaves room for the four in registers.

	.set noreorder
	.set noat

	.section .text.libps_biosA,"ax",@progbits
	.align 2
	.globl libps_biosA
libps_biosA:
	move $t1, $a0
	move $a0, $a1
	move $a1, $a2
	move $a2, $a3
	lw   $a3, 16($sp)
	li   $t2, 0xa0
	jr   $t2
	nop

	.section .text.libps_biosB,"ax",@progbits
	.align 2
	.globl libps_biosB
libps_biosB:
	move $t1, $a0
	move $a0, $a1
	move $a1, $a2
	move $a2, $a3
	lw   $a3, 16($sp)
	li   $t2, 0xb0
	jr   $t2
	nop

	.section .text.libps_biosC,"ax",@progbits
	.align 2
	.globl libps_biosC
libps_biosC:
	move $t1, $a0
	move $a0, $a1
	move $a1, $a2
	move $a2, $a3
	lw   $a3, 16($sp)
	li   $t2, 0xc0
	jr   $t2
	nop

# The interrupt entry: what the library's jmp_buf returns to when the
# kernel ends an exception, on the library's own stack. It runs the
# handler and returns from the exception, which never comes back.
	.section .text.libps_intr_entry,"ax",@progbits
	.align 2
	.globl libps_intr_entry
libps_intr_entry:
	addiu $sp, $sp, -16
	jal  libps_interrupt
	nop
	li   $t2, 0xb0
	jr   $t2
	li   $t1, 0x17

# libps_callC(fn): calls the C function at fn with no arguments, for a
# program's callback given by address. A tail call: the function returns
# to the caller.
	.section .text.libps_callC,"ax",@progbits
	.align 2
	.globl libps_callC
libps_callC:
	jr   $a0
	nop
