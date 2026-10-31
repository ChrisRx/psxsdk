# The GTE's control registers, for psxsdk's exp/libps: the Go library
# cannot write ctc2 inline, since the assembler takes no ctc2 and a .word
# for one needs a fixed register. Each takes its arguments in a0 to a2 and
# moves them in, as libgte did.

	.set noreorder
	.set noat

# libps_initGeom: the GTE enabled in the status register, and its
# constants set as libgte's InitGeom did: ZSF3 341, ZSF4 256, H 1000, DQA
# -4194, DQB 0x1400000, and the screen offset 0, 0.
	.section .text.libps_initGeom,"ax",@progbits
	.align 2
	.globl libps_initGeom
libps_initGeom:
	.word 0x40026000                      # mfc0 $v0, $12
	nop
	lui  $v1, 0x4000
	or   $v0, $v0, $v1
	.word 0x40826000                      # mtc0 $v0, $12
	nop
	li   $t0, 341
	.word 0x48C8E800                      # ctc2 $t0, $29
	nop
	li   $t0, 256
	.word 0x48C8F000                      # ctc2 $t0, $30
	nop
	li   $t0, 1000
	.word 0x48C8D000                      # ctc2 $t0, $26
	nop
	li   $t0, -4194
	.word 0x48C8D800                      # ctc2 $t0, $27
	nop
	lui  $t0, 0x140
	.word 0x48C8E000                      # ctc2 $t0, $28
	nop
	.word 0x48C0C000                      # ctc2 $zero, $24
	.word 0x48C0C800                      # ctc2 $zero, $25
	nop
	jr   $ra
	nop

# libps_setGeomOffset(x, y): the screen offset, in 16.16.
	.section .text.libps_setGeomOffset,"ax",@progbits
	.align 2
	.globl libps_setGeomOffset
libps_setGeomOffset:
	sll  $a0, $a0, 16
	sll  $a1, $a1, 16
	.word 0x48C4C000                      # ctc2 $a0, $24
	.word 0x48C5C800                      # ctc2 $a1, $25
	jr   $ra
	nop

# libps_setBackColor(r, g, b): the background color, in 20.4.
	.section .text.libps_setBackColor,"ax",@progbits
	.align 2
	.globl libps_setBackColor
libps_setBackColor:
	sll  $a0, $a0, 4
	sll  $a1, $a1, 4
	sll  $a2, $a2, 4
	.word 0x48C46800                      # ctc2 $a0, $13
	.word 0x48C57000                      # ctc2 $a1, $14
	.word 0x48C67800                      # ctc2 $a2, $15
	jr   $ra
	nop

# libps_getGeomScreen(): the projection distance H.
	.section .text.libps_getGeomScreen,"ax",@progbits
	.align 2
	.globl libps_getGeomScreen
libps_getGeomScreen:
	.word 0x4842D000                      # cfc2 $v0, $26
	jr   $ra
	nop

# libps_rotTransPers4(v0, v1, v2, v3, sxy0, sxy1, sxy2, sxy3, p, flag):
# four vertices through the rotation, translation and perspective, the
# first three by RTPT and the fourth by RTPS, their screen points and the
# last one's depth stored where the pointers say, the flags of both
# commands ORed into flag, and the last one's interpolation factor
# returned over 4. As libgte's RotTransPers4.
	.section .text.libps_rotTransPers4,"ax",@progbits
	.align 2
	.globl libps_rotTransPers4
libps_rotTransPers4:
	.word 0xC8800000                      # lwc2 $0, 0($a0)
	.word 0xC8810004                      # lwc2 $1, 4($a0)
	.word 0xC8A20000                      # lwc2 $2, 0($a1)
	.word 0xC8A30004                      # lwc2 $3, 4($a1)
	.word 0xC8C40000                      # lwc2 $4, 0($a2)
	.word 0xC8C50004                      # lwc2 $5, 4($a2)
	nop
	.word 0x4A280030                      # RTPT
	lw   $t0, 16($sp)
	lw   $t1, 20($sp)
	lw   $t2, 24($sp)
	.word 0xE90C0000                      # swc2 $12, 0($t0)
	.word 0xE92D0000                      # swc2 $13, 0($t1)
	.word 0xE94E0000                      # swc2 $14, 0($t2)
	.word 0x4843F800                      # cfc2 $v1, $31
	.word 0xC8E00000                      # lwc2 $0, 0($a3)
	.word 0xC8E10004                      # lwc2 $1, 4($a3)
	nop
	.word 0x4A180001                      # RTPS
	lw   $t0, 28($sp)
	lw   $t1, 32($sp)
	lw   $t2, 36($sp)
	.word 0xE90E0000                      # swc2 $14, 0($t0)
	.word 0xE9280000                      # swc2 $8, 0($t1)
	.word 0x4848F800                      # cfc2 $t0, $31
	.word 0x48029800                      # mfc2 $v0, $19
	or   $t0, $t0, $v1
	sw   $t0, 0($t2)
	jr   $ra
	sra  $v0, $v0, 2

# libps_setLightMatrix(m): the light direction matrix, from a MATRIX.
	.section .text.libps_setLightMatrix,"ax",@progbits
	.align 2
	.globl libps_setLightMatrix
libps_setLightMatrix:
	lw   $t0, 0($a0)
	lw   $t1, 4($a0)
	lw   $t2, 8($a0)
	lw   $t3, 12($a0)
	lw   $t4, 16($a0)
	.word 0x48C84000                      # ctc2 $t0, $8
	.word 0x48C94800                      # ctc2 $t1, $9
	.word 0x48CA5000                      # ctc2 $t2, $10
	.word 0x48CB5800                      # ctc2 $t3, $11
	.word 0x48CC6000                      # ctc2 $t4, $12
	jr   $ra
	nop

# libps_setColorMatrix(m): the light color matrix, from a MATRIX.
	.section .text.libps_setColorMatrix,"ax",@progbits
	.align 2
	.globl libps_setColorMatrix
libps_setColorMatrix:
	lw   $t0, 0($a0)
	lw   $t1, 4($a0)
	lw   $t2, 8($a0)
	lw   $t3, 12($a0)
	lw   $t4, 16($a0)
	.word 0x48C88000                      # ctc2 $t0, $16
	.word 0x48C98800                      # ctc2 $t1, $17
	.word 0x48CA9000                      # ctc2 $t2, $18
	.word 0x48CB9800                      # ctc2 $t3, $19
	.word 0x48CCA000                      # ctc2 $t4, $20
	jr   $ra
	nop

# libps_setFarColor(r, g, b): the fog's far color, in 20.4.
	.section .text.libps_setFarColor,"ax",@progbits
	.align 2
	.globl libps_setFarColor
libps_setFarColor:
	sll  $a0, $a0, 4
	sll  $a1, $a1, 4
	sll  $a2, $a2, 4
	.word 0x48C4A800                      # ctc2 $a0, $21
	.word 0x48C5B000                      # ctc2 $a1, $22
	.word 0x48C6B800                      # ctc2 $a2, $23
	jr   $ra
	nop

# libps_setGeomScreen(h): the projection distance H.
	.section .text.libps_setGeomScreen,"ax",@progbits
	.align 2
	.globl libps_setGeomScreen
libps_setGeomScreen:
	.word 0x48C4D000                      # ctc2 $a0, $26
	jr   $ra
	nop

# libps_setDQA(v) and libps_setDQB(v): the depth cue's factor and offset.
	.section .text.libps_setDQA,"ax",@progbits
	.align 2
	.globl libps_setDQA
libps_setDQA:
	.word 0x48C4D800                      # ctc2 $a0, $27
	jr   $ra
	nop

	.section .text.libps_setDQB,"ax",@progbits
	.align 2
	.globl libps_setDQB
libps_setDQB:
	.word 0x48C4E000                      # ctc2 $a0, $28
	jr   $ra
	nop

# libps_readRotTrans(m): the rotation matrix and the translation vector
# as they are, into a MATRIX, for PushMatrix.
	.section .text.libps_readRotTrans,"ax",@progbits
	.align 2
	.globl libps_readRotTrans
libps_readRotTrans:
	.word 0x48480000                      # cfc2 $t0, $0
	.word 0x48490800                      # cfc2 $t1, $1
	.word 0x484A1000                      # cfc2 $t2, $2
	.word 0x484B1800                      # cfc2 $t3, $3
	.word 0x484C2000                      # cfc2 $t4, $4
	sw   $t0, 0($a0)
	sw   $t1, 4($a0)
	sw   $t2, 8($a0)
	sw   $t3, 12($a0)
	sw   $t4, 16($a0)
	.word 0x48482800                      # cfc2 $t0, $5
	.word 0x48493000                      # cfc2 $t1, $6
	.word 0x484A3800                      # cfc2 $t2, $7
	sw   $t0, 20($a0)
	sw   $t1, 24($a0)
	sw   $t2, 28($a0)
	jr   $ra
	nop

# libps_getGeomOffset(out): the screen offset, in whole pixels, into
# two words.
	.section .text.libps_getGeomOffset,"ax",@progbits
	.align 2
	.globl libps_getGeomOffset
libps_getGeomOffset:
	.word 0x4848C000                      # cfc2 $t0, $24
	.word 0x4849C800                      # cfc2 $t1, $25
	sra  $t0, $t0, 16
	sra  $t1, $t1, 16
	sw   $t0, 0($a0)
	sw   $t1, 4($a0)
	jr   $ra
	nop
