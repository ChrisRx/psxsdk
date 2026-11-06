# The routines a program for the resident library links into itself rather than calling
# there (exp/cleanlib/spec/26-members.md): puts, SetVideoMode and Krom2Tim2, which call
# into the library, and the float routines the compiler calls by name, which the static
# target needs too. Each has a section of its own so that --gc-sections can drop it.
#
# Each path takes the cycles Sony's did, from black-box probes. An instruction costs 2
# cycles, so that a path is a count of instructions and the nops make up the difference.
# The results are not all IEEE's: the comments say where they are not.

	.set noreorder
	.set noat

	.macro NOPS n
	.rept \n
	nop
	.endr
	.endm

# WAIT spends n instructions, an odd number from 3 up, with t9 gone.
	.macro WAIT n
	li    $t9, ((\n) - 3) / 2
97:	bnez  $t9, 97b
	addiu $t9, $t9, -1
	.endm

	.macro FUNC name
	.section .text.\name, "ax", @progbits
	.align 2
	.globl \name
	.type \name, @function
\name:
	.endm

# Comparisons go by the bit patterns, as sign and magnitude, with the two zeros equal: a NaN
# or an infinity is a number like any other. eq and ne are the same: 0 if equal, else 1.
	.macro EQUAL name
	FUNC \name
	beq   $a0, $a1, 3f
	sll   $t0, $a0, 1
	bnez  $t0, 1f
	li    $v0, 1
	sll   $t1, $a1, 1
	beqz  $t1, 2f
	nop
	NOPS  2
	jr    $ra
	nop
1:	NOPS  2
	jr    $ra
	nop
2:	NOPS  3
	jr    $ra
	li    $v0, 0
3:	NOPS  1
	jr    $ra
	li    $v0, 0
	.endm

	EQUAL __eqsf2
	EQUAL __nesf2

# ORDERED makes a comparison. result sets v0 from t6, which says that a is below b, in two
# instructions. The rest are its paths' padding: for a pair of different signs, an exponent
# below or above b's, a fraction below or above. A zero in a costs three instructions more.
	.macro ORDERED name, result, diff, elt, egt, flt, fgt
	FUNC \name
	beq   $a0, $a1, 9f
	sll   $t0, $a0, 1
	bnez  $t0, 1f
	sll   $t1, $a1, 1
	nop
	beqz  $t1, 8f
	nop
1:	xor   $t2, $a0, $a1
	bltz  $t2, 7f
	nop
	# The same sign. The magnitudes decide, whichever part differs first.
	sltu  $t3, $t0, $t1
	srl   $t6, $a0, 31
	xor   $t6, $t6, $t3
	sll   $t2, $t2, 1
	srl   $t2, $t2, 24
	beqz  $t2, 5f
	nop
	bnez  $t3, 2f
	nop
	NOPS  \egt
	\result
	jr    $ra
	nop
2:	NOPS  \elt
	\result
	jr    $ra
	nop
5:	bnez  $t3, 6f
	nop
	NOPS  \fgt
	\result
	jr    $ra
	nop
6:	NOPS  \flt
	\result
	jr    $ra
	nop
	# Different signs: a is below if it is the negative one.
7:	srl   $t6, $a0, 31
	NOPS  \diff
	\result
	jr    $ra
	nop
	# Both zeros.
8:	NOPS  5
	jr    $ra
	li    $v0, 0
	# The same bits.
9:	NOPS  3
	jr    $ra
	li    $v0, 0
	.endm

	.macro NEGLESS
	subu  $v0, $zero, $t6
	nop
	.endm
	.macro NOTLESS
	xori  $v0, $t6, 1
	nop
	.endm

	ORDERED __ltsf2, NEGLESS, 2, 2, 3, 14, 13
	ORDERED __lesf2, NOTLESS, 1, 2, 4, 14, 14
	ORDERED __gtsf2, NOTLESS, 1, 2, 1, 12, 13
	ORDERED __gesf2, NEGLESS, 2, 5, 3, 15, 15

# __negsf2 turns -0 into +0, and leaves +0 as it is.
	FUNC __negsf2
	beqz  $a0, 1f
	move  $v0, $a0
	lui   $t0, 0x8000
	xor   $v0, $a0, $t0
	jr    $ra
	nop
1:	nop
	jr    $ra
	nop

# __subsf3 is a if b is a zero and -b if a is a zero, with no sum and none of its oddities.
# The rest is the library's sum.
	FUNC __subsf3
	addiu $sp, $sp, -24
	sw    $ra, 16($sp)
	sll   $t0, $a1, 1
	beqz  $t0, 1f
	sll   $t1, $a0, 1
	beqz  $t1, 2f
	lui   $t2, 0x8000
	xor   $a1, $a1, $t2
	NOPS  4
	jal   __addsf3
	nop
	lw    $ra, 16($sp)
	NOPS  3
	jr    $ra
	addiu $sp, $sp, 24
1:	move  $v0, $a0
	lw    $ra, 16($sp)
	NOPS  5
	jr    $ra
	addiu $sp, $sp, 24
2:	xor   $v0, $a1, $t2
	lw    $ra, 16($sp)
	NOPS  12
	jr    $ra
	addiu $sp, $sp, 24

# __fixsfsi truncates. -0 comes back as it went in, 80000000h. Anything from 2^31 up reports
# a range error and gives 7FFFFFFFh, and anything from -2^31 down gives 80000000h without
# one. Infinities and NaNs are just large.
	FUNC __fixsfsi
	sll   $t0, $a0, 1
	beqz  $t0, 3f
	srl   $t1, $t0, 24
	sltiu $t2, $t1, 126
	bnez  $t2, 4f
	sltiu $t2, $t1, 158
	beqz  $t2, 5f
	sll   $t2, $a0, 8
	lui   $t3, 0x8000
	or    $t2, $t2, $t3
	srl   $t2, $t2, 1
	li    $t3, 157
	subu  $t3, $t3, $t1
	srlv  $v0, $t2, $t3
	bgez  $a0, 1f
	nop
	subu  $v0, $zero, $v0
1:	NOPS  12
	jr    $ra
	nop
	# A zero.
3:	NOPS  8
	jr    $ra
	move  $v0, $a0
	# Below one half.
4:	NOPS  18
	jr    $ra
	li    $v0, 0
	# Too large, positive or negative.
5:	bgez  $a0, 6f
	nop
	NOPS  7
	jr    $ra
	lui   $v0, 0x8000
6:	addiu $sp, $sp, -24
	sw    $ra, 16($sp)
	nop
	jal   _err_math
	li    $a0, 34
	lw    $ra, 16($sp)
	lui   $v0, 0x7fff
	ori   $v0, $v0, 0xffff
	NOPS  2
	jr    $ra
	addiu $sp, $sp, 24

# __fixunssfsi reports a domain error for anything negative, -0 included, and then goes on
# with the magnitude: its result is the negative of the positive one's, and -0 comes back as
# it went in. From 2^32 up it reports a range error too, and gives FFFFFFFFh either way.
	FUNC __fixunssfsi
	beqz  $a0, 6f
	sll   $t0, $a0, 1
	addiu $sp, $sp, -32
	sw    $ra, 16($sp)
	sw    $s0, 20($sp)
	bgez  $a0, 3f
	move  $s0, $a0
	NOPS  2
	jal   _err_math
	li    $a0, 33
	sll   $t0, $s0, 1
	beqz  $t0, 2f
	srl   $t1, $t0, 24
	sltiu $t2, $t1, 127
	bnez  $t2, 1f
	sltiu $t2, $t1, 159
	beqz  $t2, 5f
	nop
	sll   $t2, $s0, 8
	lui   $t3, 0x8000
	or    $t2, $t2, $t3
	li    $t3, 158
	subu  $t3, $t3, $t1
	srlv  $v0, $t2, $t3
	subu  $v0, $zero, $v0
	NOPS  8
	b     9f
	nop
	# Below one.
1:	NOPS  12
	b     9f
	li    $v0, 0
	# -0.
2:	NOPS  2
	b     9f
	move  $v0, $s0
	# A positive number.
3:	srl   $t1, $t0, 24
	sltiu $t2, $t1, 127
	bnez  $t2, 4f
	sltiu $t2, $t1, 159
	beqz  $t2, 7f
	nop
	sll   $t2, $a0, 8
	lui   $t3, 0x8000
	or    $t2, $t2, $t3
	li    $t3, 158
	subu  $t3, $t3, $t1
	srlv  $v0, $t2, $t3
	NOPS  11
	b     9f
	nop
	# Below one.
4:	NOPS  15
	b     9f
	li    $v0, 0
	# Too large, after a negative one's domain error.
5:	NOPS  2
	jal   _err_math
	li    $a0, 34
	b     9f
	li    $v0, -1
	# A zero.
6:	NOPS  16
	jr    $ra
	li    $v0, 0
	# Too large.
7:	NOPS  5
	jal   _err_math
	li    $a0, 34
	b     9f
	li    $v0, -1
9:	lw    $s0, 20($sp)
	lw    $ra, 16($sp)
	nop
	jr    $ra
	addiu $sp, $sp, 32

# __fixunsdfsi is __fixunssfsi on a double, the same in all but its time. Below 2^32 that goes
# up by two instructions for each bit set in the top eleven of the low word, as DFBITS does,
# and by one if the high word has nothing but a sign. It truncates.
	.macro DFBITS
	sll   $t3, $s1, 1
	bnez  $t3, 2f
	nop
	nop
2:	srl   $t0, $s0, 21
	li    $t2, 11
1:	andi  $t4, $t0, 1
	beqz  $t4, 3f
	srl   $t0, $t0, 1
	nop
	nop
3:	addiu $t2, $t2, -1
	bnez  $t2, 1b
	nop
	sltiu $t2, $t1, 1023
	.endm

# DFVALUE is the number below 2^32 in t0, whole, with the exponent in t1. t2 goes.
	.macro DFVALUE
	sll   $t0, $s1, 11
	lui   $t2, 0x8000
	or    $t0, $t0, $t2
	srl   $t2, $s0, 21
	or    $t0, $t0, $t2
	li    $t2, 1054
	subu  $t2, $t2, $t1
	.endm

	FUNC __fixunsdfsi
	or    $t0, $a0, $a1
	beqz  $t0, 9f
	nop
	addiu $sp, $sp, -56
	sw    $ra, 16($sp)
	sw    $s0, 20($sp)
	sw    $s1, 24($sp)
	move  $s0, $a0
	bgez  $a1, 5f
	move  $s1, $a1
	# A negative number: a domain error first.
	NOPS  2
	jal   _err_math
	li    $a0, 33
	sll   $t0, $s1, 1
	or    $t0, $t0, $s0
	bnez  $t0, 4f
	sll   $t0, $s1, 1
	# -0.
	NOPS  4
	b     8f
	move  $v0, $s1
4:	srl   $t1, $t0, 21
	sltiu $t2, $t1, 1055
	bnez  $t2, 41f
	nop
	# Too large.
	NOPS  2
	jal   _err_math
	li    $a0, 34
	nop
	b     8f
	li    $v0, -1
41:	DFBITS
	bnez  $t2, 42f
	nop
	WAIT  113
	DFVALUE
	srlv  $t0, $t0, $t2
	subu  $v0, $zero, $t0
	b     8f
	nop
42:	WAIT  117
	b     8f
	li    $v0, 0
	# A positive number.
5:	sll   $t0, $s1, 1
	srl   $t1, $t0, 21
	sltiu $t2, $t1, 1055
	bnez  $t2, 51f
	nop
	# Too large.
	NOPS  6
	jal   _err_math
	li    $a0, 34
	nop
	b     8f
	li    $v0, -1
51:	DFBITS
	bnez  $t2, 52f
	nop
	WAIT  117
	nop
	DFVALUE
	b     8f
	srlv  $v0, $t0, $t2
52:	WAIT  121
	b     8f
	li    $v0, 0
8:	lw    $s1, 24($sp)
	lw    $s0, 20($sp)
	lw    $ra, 16($sp)
	nop
	jr    $ra
	addiu $sp, $sp, 56
	# A zero.
9:	NOPS  22
	jr    $ra
	li    $v0, 0

# __floatsisf rounds half up on the magnitude, not to even. -2^31 is the one input Sony's
# does not return from, as it loops for good: this gives CF000000h, and takes no care over
# the time.
	FUNC __floatsisf
	beqz  $a0, 9f
	move  $t0, $a0
	bgez  $a0, 1f
	move  $t3, $zero
	subu  $t0, $zero, $a0
	lui   $t3, 0x8000
1:	srl   $t1, $t0, 24
	bnez  $t1, 5f
	li    $t4, 150
	# Up to 24 bits: into place, four bits at a time and then one.
	lui   $t1, 0x10
	sltu  $t2, $t0, $t1
	beqz  $t2, 2f
	nop
11:	sll   $t0, $t0, 4
	sltu  $t2, $t0, $t1
	bnez  $t2, 11b
	addiu $t4, $t4, -4
2:	lui   $t1, 0x80
	sltu  $t2, $t0, $t1
	beqz  $t2, 3f
	nop
21:	sll   $t0, $t0, 1
	sltu  $t2, $t0, $t1
	bnez  $t2, 21b
	addiu $t4, $t4, -1
3:	WAIT  21
	nop
	b     8f
	nop
	# More: into place at bit 30, then rounded off to 24 bits.
5:	bltz  $t0, 7f
	li    $t4, 157
	lui   $t1, 0x4000
	sltu  $t2, $t0, $t1
	beqz  $t2, 62f
	nop
	NOPS  2
61:	sll   $t0, $t0, 1
	sltu  $t2, $t0, $t1
	bnez  $t2, 61b
	addiu $t4, $t4, -1
62:	addiu $t0, $t0, 0x40
	srl   $t0, $t0, 7
	srl   $t1, $t0, 24
	beqz  $t1, 8f
	nop
	srl   $t0, $t0, 1
	addiu $t4, $t4, 1
8:	sll   $t0, $t0, 9
	srl   $t0, $t0, 9
	sll   $t4, $t4, 23
	or    $v0, $t0, $t4
	or    $v0, $v0, $t3
	jr    $ra
	nop
	# -2^31.
7:	jr    $ra
	lui   $v0, 0xcf00
9:	NOPS  4
	jr    $ra
	li    $v0, 0

#ifndef LIBPS_GO

# puts writes the string as it is, with no newline after it, a character at a time through
# the library's putchar, and returns what the last putchar did. For an empty string it
# leaves v0 and v1 as they were. A null string reads as "<NULL>".
	FUNC puts
	addiu $sp, $sp, -24
	sw    $ra, 16($sp)
	sw    $s0, 20($sp)
	bnez  $a0, 1f
	move  $s0, $a0
	lui   $s0, %hi(puts_null)
	b     1f
	addiu $s0, $s0, %lo(puts_null)
2:	nop
	jal   putchar
	nop
1:	lb    $a0, 0($s0)
	addiu $s0, $s0, 1
	bnez  $a0, 2b
	nop
	lw    $ra, 16($sp)
	lw    $s0, 20($sp)
	nop
	jr    $ra
	addiu $sp, $sp, 24

	.section .rodata.puts, "a", @progbits
puts_null:
	.asciz "<NULL>"

# SetVideoMode is the library's half, then SsSetTickMode(5), which sets the rate of sequences
# back to the new mode's. It returns the mode before. A program may bring its own of this and
# of Krom2Tim2, which are weak, though of none of the others.
	.section .text.SetVideoMode, "ax", @progbits
	.align 2
	.weak SetVideoMode
	.type SetVideoMode, @function
SetVideoMode:
	addiu $sp, $sp, -24
	sw    $ra, 20($sp)
	NOPS  2
	jal   SetVideoMode_orig
	nop
	sw    $v0, 16($sp)
	jal   SsSetTickMode
	li    $a0, 5
	lw    $v0, 16($sp)
	lw    $ra, 20($sp)
	nop
	jr    $ra
	addiu $sp, $sp, 24

# Krom2Tim2 renders the Shift JIS string a0 as a 4-bit TIM at a1, its pixels for a2, a3 in
# video memory and its palette for the stack's words, spec 13 for the layout. A lead byte
# from 81h to 98h and whatever follows it are a code for the library's Krom2RawAdd2, a byte
# from 20h to 7Eh its full-width code, a newline starts the next row, and any other byte
# asks for code 0, which is a blank cell, and makes the result -1. Sixteen rows are the
# most: at the end of the sixteenth the string is over, whatever it goes on to hold.
#
# It makes the time of Sony's, whose stack frame of 74KB it does not: the pixels are written
# last, from the glyphs' addresses, which wait in the cells' first words. The time from
# entry to the first Krom2RawAdd2, from one to the next and from the last to the return are
# Sony's, on a budget in s7 that each path takes its own cycles from, and KROMSPIN
# spends the rest of. Interrupts come on top on both.

# KROMSPIN spends t9's cycles, an even number from 12 up. t0 goes.
	.macro KROMSPIN
	andi  $t0, $t9, 2
	subu  $t9, $t9, $t0
	beqz  $t0, 98f
	addiu $t9, $t9, -12
	nop
98:	bgtz  $t9, 98b
	addiu $t9, $t9, -4
	.endm

	.set K_START, 589510 + 18 - 180	# to the first character, less the fixed work
	.set K_FIRST, 22		# if there is a character at all
	.set KP_TWO, 36			# the first pass, by kind of character
	.set KP_NL, 34
	.set KP_ONE, 40
	.set KP_BAD, 44
	.set K_G2, 428 - 18 - 26 - 10	# the wait before a two-byte code's call
	.set K_GBAD, 506 - 18 - 30 - 10	# before a byte that is not text
	.set K_GASC, 18 + 36 + 14	# what comes off Sony's time for a byte of text
	.set K_END, 60			# what comes off the end's time in any case
	.set K_E00, K_END + 16		# by where the cursor ended: on the first row or not,
	.set K_E0X, K_END + 22		# in the row's first cell or not
	.set K_E10, K_END + 32
	.set K_E11, K_END + 22
	.set K_E16, K_END + 26
	.set K_E1X, K_END + 32
	.set K_ROW, 26			# a cell with nothing in it
	.set K_CELL, 964		# a cell with a glyph

	.section .text.Krom2Tim2, "ax", @progbits
	.align 2
	.weak Krom2Tim2
	.type Krom2Tim2, @function
Krom2Tim2:
	addiu $sp, $sp, -56
	sw    $ra, 16($sp)
	sw    $s0, 20($sp)
	sw    $s1, 24($sp)
	sw    $s2, 28($sp)
	sw    $s3, 32($sp)
	sw    $s4, 36($sp)
	sw    $s5, 40($sp)
	sw    $s6, 44($sp)
	sw    $s7, 48($sp)
	move  $s0, $a0
	addiu $s1, $a1, 64
	move  $s4, $zero
	move  $s5, $zero
	move  $s6, $zero
	lui   $s7, %hi(K_START)
	addiu $s7, $s7, %lo(K_START)
	lbu   $t0, 0($a0)
	move  $t6, $a0
	beqz  $t0, 1f
	move  $t7, $zero
	addiu $s7, $s7, K_FIRST - 2
1:	move  $t8, $zero

	# The first pass: how many cells, how many rows, and the result. It stops at the end of
	# the sixteenth row, and whatever comes after is not looked at.
2:	lbu   $t0, 0($t6)
	addiu $t6, $t6, 1
	beqz  $t0, 6f
	addiu $t1, $s5, -16
	beqz  $t1, 61f
	addiu $t1, $t0, -0x81
	sltiu $t1, $t1, 0x18
	beqz  $t1, 3f
	addiu $t2, $t0, -0x20
	addiu $t6, $t6, 1
	b     5f
	addiu $s7, $s7, -KP_TWO
3:	addiu $t1, $t0, -0x0a
	bnez  $t1, 4f
	sltiu $t2, $t2, 0x5f
	li    $t8, 1
	addiu $s5, $s5, 1
	move  $s4, $zero
	b     2b
	addiu $s7, $s7, -KP_NL
4:	bnez  $t2, 5f
	addiu $s7, $s7, -KP_ONE
	li    $s6, -1
	addiu $s7, $s7, KP_ONE - KP_BAD
5:	addiu $t7, $t7, 1
	addiu $s4, $s4, 1
	srl   $t1, $s4, 4
	addu  $s5, $s5, $t1
	b     2b
	andi  $s4, $s4, 15
61:	addiu $s7, $s7, -6

	# cols in s2: the count up to 16, or 16 with a newline. rows in s3, one more than
	# the cursor's row if anything is in that row or it is the first. cells in t3.
6:	sltiu $t1, $t7, 17
	xori  $t1, $t1, 1
	or    $t1, $t1, $t8
	subu  $t1, $zero, $t1
	li    $s2, 16
	and   $s2, $s2, $t1
	nor   $t1, $t1, $zero
	and   $t1, $t7, $t1
	or    $s2, $s2, $t1
	sltu  $t1, $zero, $s4
	sltiu $t2, $s5, 1
	or    $t1, $t1, $t2
	addu  $s3, $s5, $t1
	multu $s3, $s2
	mflo  $t3

	# The header.
	li    $t0, 0x10
	sw    $t0, 0($a1)
	li    $t0, 8
	sw    $t0, 4($a1)
	li    $t0, 44
	sw    $t0, 8($a1)
	lw    $t0, 56 + 16($sp)
	lw    $t1, 56 + 20($sp)
	nop
	sll   $t1, $t1, 16
	or    $t0, $t0, $t1
	sw    $t0, 12($a1)
	lui   $t0, 1
	ori   $t0, $t0, 0x10
	sw    $t0, 16($a1)
	lw    $t0, 56 + 24($sp)
	lw    $t1, 56 + 28($sp)
	sll   $t0, $t0, 16
	or    $t0, $t0, $t1
	sw    $t0, 20($a1)
	sw    $zero, 24($a1)
	sw    $zero, 28($a1)
	sw    $zero, 32($a1)
	sw    $zero, 36($a1)
	sw    $zero, 40($a1)
	sw    $zero, 44($a1)
	sw    $zero, 48($a1)
	sll   $t0, $t3, 7
	addiu $t0, $t0, 12
	sw    $t0, 52($a1)
	sll   $t1, $a3, 16
	or    $t1, $t1, $a2
	sw    $t1, 56($a1)
	sll   $t0, $s3, 20
	sll   $t1, $s2, 2
	or    $t0, $t0, $t1
	sw    $t0, 60($a1)

	# The pixels, cleared, at 6 cycles a word, 32 words a cell.
	sll   $t0, $t3, 7
	addu  $t0, $t0, $s1
	beq   $t0, $s1, 8f
	move  $t1, $s1
7:	addiu $t1, $t1, 4
	bne   $t1, $t0, 7b
	sw    $zero, -4($t1)
8:	sll   $t0, $t3, 6
	sll   $t1, $t3, 7
	addu  $t0, $t0, $t1
	subu  $s7, $s7, $t0
	nop
	sll   $s2, $s2, 3
	move  $s4, $zero
	move  $s5, $zero
	move  $t9, $s7
	KROMSPIN

	# Each character: its code, the time it takes on Sony's, the call, and its address in
	# its cell's first word.
10:	lbu   $t0, 0($s0)
	addiu $s0, $s0, 1
	beqz  $t0, 20f
	addiu $t1, $s5, -16
	beqz  $t1, 20f
	addiu $t1, $t0, -0x81
	sltiu $t1, $t1, 0x18
	beqz  $t1, 11f
	addiu $t2, $t0, -0x20
	lbu   $t1, 0($s0)
	addiu $s0, $s0, 1
	sll   $t0, $t0, 8
	or    $a0, $t0, $t1
	b     13f
	li    $t9, K_G2
11:	addiu $t1, $t0, -0x0a
	beqz  $t1, 14f
	sltiu $t1, $t2, 0x5f
	bnez  $t1, 12f
	nop
	li    $t9, K_GBAD
	b     13f
	move  $a0, $zero
12:	sll   $t2, $t2, 2
	lui   $t1, %hi(krom_text)
	addiu $t1, $t1, %lo(krom_text)
	addu  $t1, $t1, $t2
	lw    $a0, 0($t1)
	nop
	srl   $t9, $a0, 16
	andi  $a0, $a0, 0xffff
13:	KROMSPIN
	jal   Krom2RawAdd2
	nop
	sll   $t0, $s5, 11
	sll   $t1, $s4, 3
	addu  $t0, $t0, $t1
	addu  $t0, $t0, $s1
	sw    $v0, 0($t0)
	addiu $s4, $s4, 1
	srl   $t1, $s4, 4
	addu  $s5, $s5, $t1
	b     10b
	andi  $s4, $s4, 15
	# A newline, 32 cycles in all.
14:	addiu $s5, $s5, 1
	move  $s4, $zero
	b     10b
	nop

	# The string has ended: the time left, by where the cursor is.
20:	li    $t0, 6438 - K_E00
	bnez  $s5, 21f
	nop
	beqz  $s4, 22f
	nop
	li    $t0, 4320
	multu $s4, $t0
	mflo  $s7
	addiu $s7, $s7, 6486 - K_E0X
	b     24f
	nop
21:	bnez  $s4, 23f
	addiu $t2, $s5, -1
	beqz  $t2, 212f
	addiu $t2, $s5, -16
	beqz  $t2, 213f
	nop
	li    $t0, 68334
	multu $s5, $t0
	mflo  $s7
	addiu $s7, $s7, 6422 - K_E10
	b     24f
	nop
	# The end of the first row, and of the sixteenth.
212:	li    $s7, 74764 - K_E11
	b     24f
	nop
213:	li    $s7, 1094416 - K_E16
	b     24f
	nop
22:	move  $s7, $t0
	b     24f
	nop
23:	li    $t0, 68334
	multu $s5, $t0
	mflo  $s7
	li    $t0, 1952
	addiu $t1, $s4, -1
	multu $t1, $t0
	mflo  $t0
	addu  $s7, $s7, $t0
	li    $t0, 40118 - K_E1X
	addu  $s7, $s7, $t0

	# Then the glyphs: each cell's word and its 15 rows of two bytes. t3 counts the
	# cells, t8 is the cell, s2 the bytes a row of the image takes.
24:	move  $t8, $zero
25:	beq   $t8, $t3, 30f
	srl   $t0, $t8, 4
	sll   $t0, $t0, 11
	andi  $t1, $t8, 15
	sll   $t1, $t1, 3
	addu  $t0, $t0, $t1
	addu  $t4, $t0, $s1
	lw    $t5, 0($t4)
	addiu $t8, $t8, 1
	bnez  $t5, 26f
	addiu $s7, $s7, -K_ROW
	b     25b
	nop
26:	lui   $t6, %hi(krom_nibbles)
	addiu $t6, $t6, %lo(krom_nibbles)
	li    $t7, 15
	addiu $s7, $s7, K_ROW - K_CELL
27:	lbu   $t0, 0($t5)
	lbu   $t2, 1($t5)
	srl   $t1, $t0, 4
	sll   $t1, $t1, 1
	addu  $t1, $t1, $t6
	lhu   $t1, 0($t1)
	andi  $t0, $t0, 15
	sll   $t0, $t0, 1
	addu  $t0, $t0, $t6
	lhu   $t0, 0($t0)
	nop
	sll   $t0, $t0, 16
	or    $t0, $t0, $t1
	sw    $t0, 0($t4)
	srl   $t1, $t2, 4
	sll   $t1, $t1, 1
	addu  $t1, $t1, $t6
	lhu   $t1, 0($t1)
	andi  $t0, $t2, 15
	sll   $t0, $t0, 1
	addu  $t0, $t0, $t6
	lhu   $t0, 0($t0)
	nop
	sll   $t0, $t0, 16
	or    $t0, $t0, $t1
	sw    $t0, 4($t4)
	addiu $t5, $t5, 2
	addu  $t4, $t4, $s2
	addiu $t7, $t7, -1
	bnez  $t7, 27b
	nop
	b     25b
	nop

30:	move  $t9, $s7
	KROMSPIN
	move  $v0, $s6
	lw    $ra, 16($sp)
	lw    $s0, 20($sp)
	lw    $s1, 24($sp)
	lw    $s2, 28($sp)
	lw    $s3, 32($sp)
	lw    $s4, 36($sp)
	lw    $s5, 40($sp)
	lw    $s6, 44($sp)
	lw    $s7, 48($sp)
	jr    $ra
	addiu $sp, $sp, 56
	.size Krom2Tim2, .-Krom2Tim2

	.section .rodata.Krom2Tim2, "a", @progbits
	.align 2
# A nibble's four pixels, the leftmost the top bit and the lowest nibble of the result.
krom_nibbles:
	.short 0x0000, 0x1000, 0x0100, 0x1100, 0x0010, 0x1010, 0x0110, 0x1110
	.short 0x0001, 0x1001, 0x0101, 0x1101, 0x0011, 0x1011, 0x0111, 0x1111
# For each byte from 20h to 7Eh, its full-width code and in the top half the wait before its
# call, which goes by what kind of byte it is on Sony's.
	.macro TEXT g, codes:vararg
	.irp c, \codes
	.word (((\g) - K_GASC) << 16) | \c
	.endr
	.endm
krom_text:
	TEXT 476, 0x8140, 0x8149, 0x8168, 0x8194, 0x8190, 0x8193, 0x8195, 0x8166
	TEXT 476, 0x8169, 0x816a, 0x8196, 0x817b, 0x8143, 0x817c, 0x8144, 0x815e
	TEXT 492, 0x824f, 0x8250, 0x8251, 0x8252, 0x8253, 0x8254, 0x8255, 0x8256, 0x8257, 0x8258
	TEXT 494, 0x8146, 0x8147, 0x8171, 0x8181, 0x8172, 0x8148, 0x8197
	TEXT 508, 0x8260, 0x8261, 0x8262, 0x8263, 0x8264, 0x8265, 0x8266, 0x8267, 0x8268, 0x8269
	TEXT 508, 0x826a, 0x826b, 0x826c, 0x826d, 0x826e, 0x826f, 0x8270, 0x8271, 0x8272, 0x8273
	TEXT 508, 0x8274, 0x8275, 0x8276, 0x8277, 0x8278, 0x8279
	TEXT 510, 0x816d, 0x818f, 0x816e, 0x814f, 0x8151, 0x8165
	TEXT 524, 0x8281, 0x8282, 0x8283, 0x8284, 0x8285, 0x8286, 0x8287, 0x8288, 0x8289, 0x828a
	TEXT 524, 0x828b, 0x828c, 0x828d, 0x828e, 0x828f, 0x8290, 0x8291, 0x8292, 0x8293, 0x8294
	TEXT 524, 0x8295, 0x8296, 0x8297, 0x8298, 0x8299, 0x829a
	TEXT 522, 0x816f, 0x8162, 0x8170, 0x8150
#endif
