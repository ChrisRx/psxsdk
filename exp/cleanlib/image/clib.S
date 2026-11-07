# Entries the image holds in assembly, where Go cannot do what the entry needs.
# Each .globl names a resident entry point (see spec 15).
	.set noreorder
	.set noat

# The routines the linked-in target has too.
#define CLEAN_LIBRARY_ONLY
#include "playstation-yaroze-clean.S"

	.text

# setjmp(buf) saves ra, sp, fp, s0 to s7 and gp, twelve words, and returns 0. It
# changes no other register (spec 16).
	.globl setjmp
	.type setjmp, @function
setjmp:
	sw	$ra, 0($a0)
	sw	$sp, 4($a0)
	sw	$fp, 8($a0)
	sw	$s0, 12($a0)
	sw	$s1, 16($a0)
	sw	$s2, 20($a0)
	sw	$s3, 24($a0)
	sw	$s4, 28($a0)
	sw	$s5, 32($a0)
	sw	$s6, 36($a0)
	sw	$s7, 40($a0)
	sw	$gp, 44($a0)
	jr	$ra
	move	$v0, $zero
	.size setjmp, .-setjmp

# longjmp(buf, v) makes setjmp return again with v as it is, 0 included.
	.globl longjmp
	.type longjmp, @function
longjmp:
	lw	$ra, 0($a0)
	lw	$sp, 4($a0)
	lw	$fp, 8($a0)
	lw	$s0, 12($a0)
	lw	$s1, 16($a0)
	lw	$s2, 20($a0)
	lw	$s3, 24($a0)
	lw	$s4, 28($a0)
	lw	$s5, 32($a0)
	lw	$s6, 36($a0)
	lw	$s7, 40($a0)
	lw	$gp, 44($a0)
	jr	$ra
	move	$v0, $a1
	.size longjmp, .-longjmp

# Krom2RawAdd(code) is the kernel's function B0h 51h, reached as if the caller
# had called it: a0, ra and the stack as they are, its result the caller's
# (spec 18).
	.globl Krom2RawAdd
	.type Krom2RawAdd, @function
Krom2RawAdd:
	li	$t2, 0xb0
	jr	$t2
	li	$t1, 0x51
	.size Krom2RawAdd, .-Krom2RawAdd

# _dbl_shift(dst, right, lo, hi, n) stores the 64 bits lo, hi shifted by n at dst, the fifth
# word on the stack, and returns dst and the high word in v1: left with a word of 0 at a1,
# what a C caller of the long long shift leaves there, and right and arithmetically with any
# other (spec 32). An n of 0 or below shifts nothing. One shift costs what Sony's turn does
# (28 cycles left, 24 right), up to 64, by when the result is settled and Sony's alone goes on.
	.globl _dbl_shift
	.type _dbl_shift, @function
_dbl_shift:
	lw	$t0, 16($sp)
	move	$v1, $a1
	blez	$t0, 2f
	slti	$a1, $t0, 64
	bnez	$a1, 1f
	nop
	li	$t0, 64
1:	beqz	$v1, 3f
	nop
4:	sll	$a1, $a3, 31
	srl	$a2, $a2, 1
	or	$a2, $a2, $a1
	sra	$a3, $a3, 1
	addiu	$t0, $t0, -1
	nop
	nop
	nop
	nop
	nop
	bnez	$t0, 4b
	nop
	b	2f
	nop
3:	srl	$a1, $a2, 31
	sll	$a2, $a2, 1
	sll	$a3, $a3, 1
	or	$a3, $a3, $a1
	addiu	$t0, $t0, -1
	nop
	nop
	nop
	nop
	nop
	nop
	nop
	bnez	$t0, 3b
	nop
2:	sw	$a2, 0($a0)
	sw	$a3, 4($a0)
	move	$v1, $a3
	jr	$ra
	move	$v0, $a0
	.size _dbl_shift, .-_dbl_shift
