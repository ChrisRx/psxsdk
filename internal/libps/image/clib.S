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

# GsInitCoordinate2(super, base): gs3d_static.go says what it does. In assembly so that its
# first instruction is not a branch (spec 33), and each path takes Sony's time: 46 cycles
# with a null super and 58 with one.
	.section .text.GsInitCoordinate2, "ax", @progbits
	.align 2
	.globl GsInitCoordinate2
	.type GsInitCoordinate2, @function
GsInitCoordinate2:
	lui	$at, %hi(GsIDMATRIX)
	addiu	$v0, $at, %lo(GsIDMATRIX)
	lw	$at, %lo(GsIDMATRIX)($at)
	lw	$v1, 16($v0)
	lw	$a2, 20($v0)
	lw	$a3, 24($v0)
	lw	$t0, 28($v0)
	lw	$t1, 12($v0)
	lw	$t2, 8($v0)
	lw	$v0, 4($v0)
	nop
	sw	$v0, 8($a1)
	sw	$t2, 12($a1)
	sw	$t1, 16($a1)
	sw	$t0, 32($a1)
	sw	$a3, 28($a1)
	sw	$a2, 24($a1)
	sw	$v1, 20($a1)
	sw	$a0, 72($a1)
	sw	$at, 4($a1)
	beqz	$a0, 1f
	sw	$zero, 0($a1)
	sw	$a1, 76($a0)
	NOPS	5
1:	jr	$ra
	nop
	.size GsInitCoordinate2, .-GsInitCoordinate2

# __addsf3(a, b): softfloat.go's sAdd, the words in a0 and a1, at Sony's time for what it is given
# (spec 34). What the time follows, each a place below: a zero first takes 34 cycles and a zero
# second 40, a smaller operand out of reach (more than 25 places) 54, or 60 when it comes first,
# and an exact cancellation 108. Others take 150, and 2 for each negative operand, 2 when the
# first has the smaller exponent, 6 for a carry and 4 and 8 a place for moving down, 2 on either
# for exponents the same, 4 when rounding carries out of the top, less 2 when the bit that ends
# the result is odd before rounding, and what the range error takes: _err_math's work, written
# out because its Go takes too long, and 2 for a negative result.
	.section .text.__addsf3, "ax", @progbits
	.align 2
	.globl __addsf3
	.type __addsf3, @function
__addsf3:
	sll	$t0, $a0, 1
	beqz	$t0, .Laddsf_azero
	srl	$t1, $t0, 24
	sll	$t2, $a1, 1
	beqz	$t2, .Laddsf_bzero
	srl	$t3, $t2, 24
	subu	$t4, $t1, $t3
	bgez	$t4, .Laddsf_afirst
	move	$t5, $a0
	move	$a0, $a1
	move	$a1, $t5
	subu	$t4, $t3, $t1
	slti	$t5, $t4, 26
	beqz	$t5, .Laddsf_far
	nop
	b	.Laddsf_near
	nop
.Laddsf_afirst:
	slti	$t5, $t4, 26
	beqz	$t5, .Laddsf_far
	nop
	NOPS	4
# From here a0 has the exponent that isn't smaller, t4 how far apart they are.
.Laddsf_near:
	sll	$t1, $a0, 1
	srl	$t1, $t1, 24
	sll	$t2, $a0, 9
	srl	$t2, $t2, 3
	lui	$t7, 0x2000
	or	$t2, $t2, $t7
	sll	$t3, $a1, 9
	srl	$t3, $t3, 3
	or	$t3, $t3, $t7
	bgez	$a0, 1f
	nop
	nop
1:	bgez	$a1, 1f
	nop
	nop
1:	beqz	$t4, .Laddsf_even
	subu	$t5, $zero, $t4
# Line up the smaller, its lost bits counting if it is negative.
	sllv	$t5, $t3, $t5
	srlv	$t3, $t3, $t4
	sltu	$t5, $zero, $t5
	srl	$t6, $a1, 31
	and	$t5, $t5, $t6
	addu	$t3, $t3, $t5
	NOPS	5
.Laddsf_sum:
	xor	$t6, $a0, $a1
	bltz	$t6, .Laddsf_sub
	addu	$v0, $t2, $t3
	srl	$t6, $v0, 30
	NOPS	2
	beqz	$t6, .Laddsf_round
	nop
	srl	$v0, $v0, 1
	addiu	$t1, $t1, 1
	nop
.Laddsf_round:
	srl	$t6, $v0, 6
	andi	$t6, $t6, 1
	addiu	$v0, $v0, 31
	addu	$v0, $v0, $t6
	srl	$v0, $v0, 6
	bnez	$t6, 1f
	nop
	nop
1:	srl	$t6, $v0, 24
	beqz	$t6, 1f
	nop
	srl	$v0, $v0, 1
	addiu	$t1, $t1, 1
1:	sltiu	$t6, $t1, 255
	beqz	$t6, .Laddsf_range
	sll	$t1, $t1, 23
	sll	$v0, $v0, 9
	srl	$v0, $v0, 9
	or	$v0, $v0, $t1
	srl	$a0, $a0, 31
	sll	$a0, $a0, 31
	NOPS	2
	jr	$ra
	or	$v0, $v0, $a0
.Laddsf_sub:
	subu	$v0, $t2, $t3
	beqz	$v0, .Laddsf_zero
	srl	$t6, $v0, 29
	bnez	$t6, .Laddsf_round
	nop
1:	sll	$v0, $v0, 1
	srl	$t6, $v0, 29
	beqz	$t6, 1b
	addiu	$t1, $t1, -1
	b	.Laddsf_round
	nop
.Laddsf_zero:
	jr	$ra
	move	$v0, $zero
# Exponents the same: the larger significand goes first.
.Laddsf_even:
	sltu	$t6, $t2, $t3
	subu	$t6, $zero, $t6
	xor	$t0, $t2, $t3
	and	$t0, $t0, $t6
	xor	$t2, $t2, $t0
	xor	$t3, $t3, $t0
	xor	$t0, $a0, $a1
	and	$t0, $t0, $t6
	xor	$a0, $a0, $t0
	xor	$a1, $a1, $t0
	b	.Laddsf_sum
	nop
.Laddsf_range:
	addiu	$sp, $sp, -24
	sw	$ra, 20($sp)
	sw	$a0, 16($sp)
	bgez	$a0, 1f
	nop
	nop
1:	NOPS	13
	li	$t0, 34
	lui	$t1, %hi(math_errno)
	sw	$t0, %lo(math_errno)($t1)
	lui	$a0, 0xf400
	ori	$a0, $a0, 2
	li	$a1, 0x0302
	li	$t2, 0xb0
	jalr	$t2
	li	$t1, 7
	lw	$a0, 16($sp)
	lw	$ra, 20($sp)
	addiu	$sp, $sp, 24
	srl	$a0, $a0, 31
	sll	$a0, $a0, 31
	lui	$v0, 0x7f80
	jr	$ra
	or	$v0, $v0, $a0
.Laddsf_azero:
	b	.Laddsf_short
	move	$a0, $a1
.Laddsf_far:
	nop
.Laddsf_bzero:
	NOPS	2
.Laddsf_short:
	NOPS	8
	jr	$ra
	move	$v0, $a0
	.size __addsf3, .-__addsf3

# PAD c spends c cycles, c even and at least 8, with t8. A turn is 6.
	.macro PAD c
	li	$t8, ((\c) - 2) / 6
98:	addiu	$t8, $t8, -1
	bnez	$t8, 98b
	nop
	NOPS	((\c) - 2 - 6 * (((\c) - 2) / 6)) / 2
	.endm

# A program's __divdf3(a, b): softfloat.go's dDivQuotient, in the turns Sony's time follows (spec 34).
# The significands are in a1:a0 and a3:a2, the quotient grows in v1:v0, t0 counts its 54 bits, t6 is
# the exponent and t9 the sign. Over a turn with no bit of the quotient, a bit takes 262 cycles, or
# 274 where the high words match and 282 where the whole significands do, 286 for a match on the
# first turn. 8 where the high words match and the low ones say no bit. 4 less for a borrow at bit 16,
# and at bit 32, and 4 more for a carry between the words when doubling. Before the turns, 76 when
# the dividend starts below the divisor, and 8 more if only its low word says so. After them, 10 if
# anything is left and 10 for a fraction of zero, 4 for bit 32 of the quotient. Below the normal
# range, 24 a place of the shortfall, plus 4 for each high word's low bit shifted out.
	.set DIV_PRE, 74
	.set DIV_EQ, 18
	.set DIV_BIT, 230
	.set DIV_REST, 146
	.set DIV_END, 660
	.set DIV_RANGE, 680
	.set DIV_SMALL, 628
	.set DIV_BZERO, 60
	.set DIV_AZERO, 64
	.section .text.libps_c___divdf3, "ax", @progbits
	.align 2
	.globl libps_c___divdf3
	.type libps_c___divdf3, @function
libps_c___divdf3:
	xor	$t9, $a1, $a3
	srl	$t9, $t9, 31
	sll	$t9, $t9, 31
	sll	$t0, $a3, 1
	or	$t7, $t0, $a2
	beqz	$t7, .Ldiv_bzero
	sll	$t1, $a1, 1
	or	$t7, $t1, $a0
	beqz	$t7, .Ldiv_azero
	nop
	bnez	$t0, 1f
	nop
	NOPS	2
1:	bnez	$t1, 1f
	nop
	nop
1:	srl	$t1, $t1, 21
	srl	$t0, $t0, 21
	subu	$t6, $t1, $t0
	addiu	$t6, $t6, 1023
	lui	$t7, 0x10
	sll	$a1, $a1, 12
	srl	$a1, $a1, 12
	or	$a1, $a1, $t7
	sll	$a3, $a3, 12
	srl	$a3, $a3, 12
	or	$a3, $a3, $t7
	sltu	$t7, $a1, $a3
	bnez	$t7, .Ldiv_pre
	nop
	bne	$a1, $a3, .Ldiv_nopre0
	sltu	$t7, $a0, $a2
	bnez	$t7, .Ldiv_pretie
	nop
	b	.Ldiv_start
	nop
.Ldiv_nopre0:
	NOPS	2
	b	.Ldiv_start
	nop
.Ldiv_pretie:
.Ldiv_pre:
	bgez	$a0, 1f
	nop
	NOPS	2
1:	srl	$t7, $a0, 31
	sll	$a1, $a1, 1
	or	$a1, $a1, $t7
	sll	$a0, $a0, 1
	addiu	$t6, $t6, -1
	PAD	DIV_PRE
.Ldiv_start:
	li	$t0, 54
	move	$v0, $zero
	move	$v1, $zero
.Ldiv_turn:
	srl	$t7, $v0, 31
	sll	$v1, $v1, 1
	or	$v1, $v1, $t7
	sll	$v0, $v0, 1
	bne	$a1, $a3, .Ldiv_unlike
	nop
# The high words match.
	bne	$a0, $a2, 1f
	sltu	$t7, $a0, $a2
	PAD	DIV_EQ
	b	.Ldiv_bit
	nop
1:	bnez	$t7, .Ldiv_lowless
	addiu	$t8, $t0, -54
	bnez	$t8, 1f
	nop
	NOPS	7
	b	.Ldiv_bit
	nop
1:	nop
	b	.Ldiv_bit
	nop
.Ldiv_lowless:
	nop
	b	.Ldiv_nobit
	nop
.Ldiv_unlike:
	sltu	$t7, $a1, $a3
	bnez	$t7, .Ldiv_nobit
	nop
.Ldiv_bit:
	ori	$v0, $v0, 1
	andi	$t7, $a0, 0xffff
	andi	$t8, $a2, 0xffff
	sltu	$t7, $t7, $t8
	bnez	$t7, 1f
	nop
	NOPS	2
1:	sltu	$t7, $a0, $a2
	bnez	$t7, 1f
	nop
	NOPS	2
1:	subu	$a0, $a0, $a2
	subu	$a1, $a1, $a3
	subu	$a1, $a1, $t7
	PAD	DIV_BIT
.Ldiv_nobit:
	bgez	$a0, 1f
	nop
	NOPS	2
1:	srl	$t7, $a0, 31
	sll	$a1, $a1, 1
	or	$a1, $a1, $t7
	sll	$a0, $a0, 1
	PAD	DIV_REST
	addiu	$t0, $t0, -1
	bnez	$t0, .Ldiv_turn
	nop
# Past the turns: a1:a0 holds what's left, v1:v0 the quotient with one bit below the last place.
	or	$a0, $a0, $a1
	sltu	$a0, $zero, $a0
	beqz	$a0, 1f
	nop
	NOPS	5
1:	andi	$t7, $v1, 1
	beqz	$t7, 1f
	nop
	NOPS	2
1:	blez	$t6, .Ldiv_small
	nop
# Rounded to nearest, ties to even.
	sll	$t7, $v1, 31
	srl	$t8, $v0, 1
	or	$t7, $t7, $t8
	srl	$a3, $v1, 1
	andi	$t8, $t7, 1
	or	$t8, $t8, $a0
	and	$t8, $t8, $v0
	andi	$t8, $t8, 1
	addu	$v0, $t7, $t8
	sltu	$t8, $v0, $t8
	addu	$v1, $a3, $t8
	srl	$t7, $v1, 21
	beqz	$t7, 1f
	nop
	srl	$v1, $v1, 1
	move	$v0, $zero
	addiu	$t6, $t6, 1
	b	2f
	nop
1:	NOPS	5
2:	sltiu	$t7, $t6, 2047
	beqz	$t7, .Ldiv_range
	lui	$t7, 0x10
	nor	$t7, $t7, $zero
	and	$v1, $v1, $t7
	or	$t7, $v1, $v0
	bnez	$t7, 1f
	nop
	NOPS	5
1:	sll	$t6, $t6, 20
	or	$v1, $v1, $t6
	or	$v1, $v1, $t9
	PAD	DIV_END
	jr	$ra
	nop
# Above the top: an infinity and a range error.
.Ldiv_range:
	lui	$t7, 0x10
	nor	$t7, $t7, $zero
	and	$v1, $v1, $t7
	or	$t7, $v1, $v0
	bnez	$t7, 1f
	nop
	NOPS	5
1:	bgez	$t9, 1f
	nop
	nop
1:	addiu	$sp, $sp, -24
	sw	$ra, 20($sp)
	sw	$t9, 16($sp)
	PAD	DIV_RANGE
	li	$t0, 34
	lui	$t1, %hi(math_errno)
	sw	$t0, %lo(math_errno)($t1)
	lui	$a0, 0xf400
	ori	$a0, $a0, 2
	li	$a1, 0x0302
	li	$t2, 0xb0
	jalr	$t2
	li	$t1, 7
	lw	$t9, 16($sp)
	lw	$ra, 20($sp)
	addiu	$sp, $sp, 24
	lui	$v1, 0x7ff0
	or	$v1, $v1, $t9
	jr	$ra
	move	$v0, $zero
# Below the bottom: a denormal, shifted down a place at a time as the time goes, a word of what's
# left dropped. The half is a 32-bit shift.
.Ldiv_small:
	li	$t7, 1
	subu	$t6, $t7, $t6
	lui	$t7, 0x10
	sll	$t8, $v1, 11
	srl	$t8, $t8, 11
	srl	$t7, $v0, 1
	or	$t8, $t8, $t7
	bnez	$t8, 1f
	nop
	NOPS	5
1:	bnez	$a0, 1f
	nop
	NOPS	2
1:	move	$t0, $t6
	move	$a2, $v0
	move	$a3, $v1
.Ldiv_shift:
	andi	$t7, $a3, 1
	beqz	$t7, 1f
	nop
	NOPS	2
1:	srl	$a3, $a3, 1
	addiu	$t0, $t0, -1
	NOPS	5
	bgez	$t0, .Ldiv_shift
	nop
	PAD	DIV_SMALL
	slti	$t7, $t6, 63
	bnez	$t7, 1f
	nop
	NOPS	16
	move	$v0, $zero
	jr	$ra
	move	$v1, $t9
1:	li	$t7, 1
	sllv	$t7, $t7, $t6
	addu	$a0, $v0, $t7
	sltu	$t7, $a0, $t7
	addu	$a1, $v1, $t7
	addiu	$t0, $t6, 1
	slti	$t7, $t0, 32
	bnez	$t7, 1f
	nop
	addiu	$t0, $t0, -32
	srlv	$v0, $a1, $t0
	nop
	b	2f
	move	$v1, $zero
1:	srlv	$v0, $a0, $t0
	subu	$t7, $zero, $t0
	sllv	$t7, $a1, $t7
	or	$v0, $v0, $t7
	srlv	$v1, $a1, $t0
2:	lui	$t7, 0x10
	nor	$t7, $t7, $zero
	and	$v1, $v1, $t7
	jr	$ra
	or	$v1, $v1, $t9
.Ldiv_bzero:
	bltz	$t9, 1f
	lui	$v1, 0x7fff
	NOPS	2
1:	ori	$v1, $v1, 0xffff
	or	$v1, $v1, $t9
	PAD	DIV_BZERO
	jr	$ra
	li	$v0, -1
.Ldiv_azero:
	bnez	$t0, 1f
	nop
	NOPS	2
1:	move	$v1, $t9
	PAD	DIV_AZERO
	jr	$ra
	move	$v0, $zero
	.size libps_c___divdf3, .-libps_c___divdf3
