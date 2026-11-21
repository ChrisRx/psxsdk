# BIOS calls for internal/libps, the Net Yaroze library in Go.
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

# libps_syscall(code): the syscall instruction with the code in a0, as the
# kernel takes it (1 enters a critical section, 2 leaves one), returning
# what the kernel leaves in v0. The kernel resumes after the instruction.
	.section .text.libps_syscall,"ax",@progbits
	.align 2
	.globl libps_syscall
libps_syscall:
	syscall
	jr   $ra
	nop

# libps_draw_irq: the drawing queue's entry in the kernel's interrupt
# chain, enqueued with SysEnqIntRP as a verifier. The kernel's own DMA
# handler takes the DMA interrupt, so the library's custom exit never sees
# it; this runs before that handler. A tail call to libps_draw_check,
# which returns 0 so that the kernel's handlers still run.
	.section .text.libps_draw_irq,"ax",@progbits
	.align 2
	.globl libps_draw_irq
libps_draw_irq:
	j    libps_draw_check
	nop

# libps_sound_irq: the sound system's entry in the kernel's interrupt
# chain, enqueued by the first sound call. A tail call to
# libps_sound_check, which runs the sound tick at a vertical blank and
# returns 0.
	.section .text.libps_sound_irq,"ax",@progbits
	.align 2
	.globl libps_sound_irq
libps_sound_irq:
	j    libps_sound_check
	nop
