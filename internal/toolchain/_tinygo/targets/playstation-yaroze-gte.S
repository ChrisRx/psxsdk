# The GTE's control registers, for the yaroze package: the functions it
# cannot write as inline assembly, since the assembler takes no ctc2 or
# cfc2 and a .word for one needs a fixed register. The rest of the GTE is
# inline in yaroze/gte_yaroze.go. Each loads its arguments and moves them
# in, or moves the flag out, in the pattern of the SDK's inline macros.

	.set noreorder
	.set noat

# SetRotMatrix(m *MATRIX): the rotation matrix.
	.section .text.SetRotMatrix,"ax",@progbits
	.align 2
	.globl SetRotMatrix
SetRotMatrix:
	lw $t0, 0($a0)
	lw $t1, 4($a0)
	lw $t2, 8($a0)
	lw $t3, 12($a0)
	lh $t4, 16($a0)
	.word 0x48C80000                      # ctc2 $t0, $0
	.word 0x48C90800                      # ctc2 $t1, $1
	.word 0x48CA1000                      # ctc2 $t2, $2
	.word 0x48CB1800                      # ctc2 $t3, $3
	.word 0x48CC2000                      # ctc2 $t4, $4
	jr $ra
	nop

# SetTransMatrix(m *MATRIX): the translation vector.
	.section .text.SetTransMatrix,"ax",@progbits
	.align 2
	.globl SetTransMatrix
SetTransMatrix:
	lw $t0, 20($a0)
	lw $t1, 24($a0)
	lw $t2, 28($a0)
	.word 0x48C82800                      # ctc2 $t0, $5
	.word 0x48C93000                      # ctc2 $t1, $6
	.word 0x48CA3800                      # ctc2 $t2, $7
	jr $ra
	nop

# ReadFlag() int32: the flag register.
	.section .text.ReadFlag,"ax",@progbits
	.align 2
	.globl ReadFlag
ReadFlag:
	.word 0x4842F800                      # cfc2 $v0, $31
	jr $ra
	nop
