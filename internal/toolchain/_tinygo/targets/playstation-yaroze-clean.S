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
# and the nops below make up the difference, when LIBPS_FIDELITY says there is
# anything to match (spec 30): the image, and the build compared with a reference.

#ifdef LIBPS_FIDELITY
#define PAD_INITHEAP 50
#define PAD_SYSCALL 23
#define PAD_MAIN 1
#else
#define PAD_INITHEAP 0
#define PAD_SYSCALL 0
#define PAD_MAIN 0
#endif

	.set noreorder
	.set noat

#ifndef CLEAN_LIBRARY_ONLY
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
#endif

# The routines of exp/cleanlib in assembly: its timing paths, where every
# instruction counts. Each instruction costs 2 cycles, and a GTE command none of
# its own but its length, spent at the next GTE instruction that comes before it
# is done, so that work between a command and its result reads free. The figures
# to meet are Sony's, per path, from probes (spec 24).

	.set CM_END, 2
	.set LSM_END, 5
	.set CMP_LEAD, 2
	.set QUAD_TURN, 116
	.set QUAD_SCALE, 79
	.set QUAD_END, 134
	.set FP_LEAD, 23
	.set FP_CHAR, 5
	.set FP_PCT, 0
	.set FP_WDIGIT, 0
	.set FP_NEG, 16
	.set FP_C, 19
	.set FP_X, 5
	.set FP_S, 5
	.set FP_SLEN, 0
	.set FP_FILL, 6
	.set FP_SIGN, 0
	.set FP_DIGIT, 6
	.set FP_XDIGIT, 12
	.set FP_SCHAR, 4
	.set FP_END, 0
	.set FP_CUT, 4
	.set FP_REFEMPTY, 2
	.set FP_REFUSED, 4
	.set LTW_DOWN, 45
	.set VW_PLAIN, 16
	.set VW_SUPER, 25
	.set VW_MID, 27
	.set VW_34, 12
	.set VW_END, 38
	.set LTW_PRE, 5
	.set LTW_QUICK, 9
	.set LTW_WALK, 18
	.set LTW_WEXIT, 27
	.set LS_MID, 18
	.set LS_POST, 24
	.set LM_LEAD, 25
	.set LM_END, 6
	.set PP_LEAD, 0
	.set MIME_END, 1
	.set FAST_PAD, 14
	.set SPRITE_ENTRY, 0
	.set SPRITE_PAD, 49
	.set BOX_PAD, 27
	.set LINK_ENTRY, 10
	.set LINK_PRIM, 2
	.set LINK_NONE, 6

# NOPS and the macros like it below exist only to match Sony's time: without
# LIBPS_FIDELITY they are empty, and a nop that a load or the GTE needs is written out.
	.macro NOPS n
#ifdef LIBPS_FIDELITY
	.rept \n
	nop
	.endr
#endif
	.endm

# An entry of the resident image is reached through a jump and its delay, which the
# routines' times below allow for. STUBTIME spends those two when linked in.
	.macro STUBTIME
#if defined(LIBPS_FIDELITY) && !defined(CLEAN_LIBRARY_ONLY)
	nop
	nop
#endif
	.endm

# GTE commands, and FLAG into t6, which the assembler will not take by name.
	.macro RTPT
	.word 0x4a280030
	.endm
	.macro RTPS
	.word 0x4a180001
	.endm
	.macro NCLIP
	.word 0x4b400006
	.endm
	.macro AVSZ3
	.word 0x4b58002d
	.endm
	.macro AVSZ4
	.word 0x4b68002e
	.endm
	.macro CFLAG
	.word 0x484ef800
	.endm

# LIGHTONE and LIGHTTHREE start the lighting of the normals in V0, or V0 to V2: by
# NCCS and NCCT, or for fog (light 2) NCDS and NCDT.
	.macro LIGHTONE light
	.if \light == 2
	.word 0x4ae80413
	.else
	.word 0x4b08041b
	.endif
	.endm
	.macro LIGHTTHREE light
	.if \light == 2
	.word 0x4af80416
	.else
	.word 0x4b18043f
	.endif
	.endm

# GSO_BASE is what a kind's start takes at the least, the first polygon's vertices found.
# GSO_RUN is the instructions from gso_run to that start, so that a run's end goes back
# GSO_RUN + GSO_BASE before its kind's own part. It all takes what Sony's does, on each way
# through, by the trace of what the GTE sees (spec 25). G starts a stretch of instructions
# between two things the GTE sees, and GAP makes it n long, in nops.
	.set GSO_BASE, 13
	.set GSO_RUN, 27
	.set GSO_SLED, 18
	.set GSO_EXIT, 3
	.set GSO_ENTRY, 48
	.set GSO_HIDDEN, 22
	.set GSO_CUT, 52

# The cut's working memory: see gso_cut. What's in the scratch argument is Sony's layout: the level,
# the screen, the two corners' palette and page, the first color word, an address, the cells of the
# polygon's corners, and then the frames, down a level at a time.
	.set CS_LIMX, 0
	.set CS_LIMY, 4
	.set CS_WH, 8
	.set CS_A0, 12
	.set CS_A1, 16
	.set CS_RA, 20
	.set CS_T0, 24
	.set CS_T1, 28
	.set CS_T2, 32
	.set CS_V1, 36
	.set CS_A3, 40
	.set CS_TAG, 44
	.set CS_LEV, 48
	.set CS_COL, 52
	.set CS_UV0, 56
	.set CS_UV1, 60
	.set CS_IR0, 64
	.set CS_SCR, 68
	.set CS_RGBC, 72
	.set CS_KEEP, 76
	.set CS_W5, 80
	.set CS_DEEP3, 84
	.set CS_DEEP4, 88
	.set CS_END, 92
	.set W_UV, 12
	.set W_COL, 16
	.set W_END, 20
	.set CS_ROOT, 24
	.set C_Z, 4
	.set C_UV, 8
	.set C_COL, 12
	.set C_SXY, 16
	.set C_SZ, 20
	.set C_SIZE, 24
	.set M0, 0
	.set M1, 24
	.set M2, 48
	.set QAB, 0
	.set QCA, 24
	.set QBE, 48
	.set QEC, 72
	.set QMD, 96

	.macro G
98:
	.endm
	.macro GAP n
#ifdef LIBPS_FIDELITY
	.space (\n) * 4 - (. - 98b)
98:
#endif
	.endm

# PREFETCH finds a polygon's vertices. The load of each is in the delay of the one before
# it, so that it takes 9 instructions.
	.macro PREFETCH v0, v1, v2
	lhu	$t3, \v0($a0)
	lhu	$t4, \v1($a0)
	lhu	$t5, \v2($a0)
	sll	$t3, $t3, 3
	sll	$t4, $t4, 3
	sll	$t5, $t5, 3
	addu	$t3, $t3, $t0
	addu	$t4, $t4, $t0
	addu	$t5, $t5, $t0
	.endm

# PSET sets the places in a kind's body and packet. light is 0 for an unlit body and
# otherwise lit; t, g and q whether it is textured, gouraud and a quad; flat that a gouraud
# body makes a flat packet.
	.macro PSET light, t, g, q, flat
	.set P_N, 3 + \q
	.if \light == 0
	.set P_COL, 4 + 4 * \t * P_N
	.set P_NC, 1 + \g * (P_N - 1)
	.set P_V0, P_COL + 4 * P_NC
	.set P_V1, P_V0 + 2
	.set P_V2, P_V0 + 4
	.set P_V3, P_V0 + 6
	.set P_SIZE, P_V0 + 8
	.else
	.set P_COL, 4
	.set P_B, 4 + 4 * (1 + \t * (P_N - 1))
	.set P_N0, P_B
	.if \g
	.set P_V0, P_B + 2
	.set P_N1, P_B + 4
	.set P_V1, P_B + 6
	.set P_N2, P_B + 8
	.set P_V2, P_B + 10
	.set P_N3, P_B + 12
	.set P_V3, P_B + 14
	.set P_SIZE, P_B + 4 * P_N
	.else
	.set P_V0, P_B + 2
	.set P_V1, P_B + 4
	.set P_V2, P_B + 6
	.set P_V3, P_B + 8
	.set P_SIZE, P_B + 8 + 4 * \q
	.endif
	.endif
	.set P_PG, \g * (1 - \flat)
	.set P_S, 1 + \t + P_PG
	.set P_PLEN, P_N * P_S + 1 - P_PG
	.set P_BACK, -P_SIZE
	.endm

# GSOSTART is a run's start: a kind's colors and the first polygon's vertices. Its way
# in takes col more than GSO_BASE. A run's a0 is its first polygon, a1 its length, t6 its
# header and a3 the attribute's translucency.
#
# Through a loop a0 is the next polygon, a1 how many are left, a2 where the packet goes, a3
# the first color word's top byte and mask (see POLYCODE), t0 the vertices, t1 the ordering
# table, t2 the shift, t3 to t5 the polygon's vertices and v1 the normals.
	.macro GSOSTART name, light, t, g, flat, col
	.type gso_\name, @function
gso_\name\():
	G
	POLYCODE \light, \t, \g, \flat
	PREFETCH P_V0, P_V1, P_V2
	GAP	GSO_BASE + \col
	.endm

# GSOBACK leaves a run: from the loop's end it takes ta instructions to the next run's
# start, GSO_RUN + GSO_BASE of them from gso_run, by a jump into the nops before it.
	.macro GSOBACK ta
#ifdef LIBPS_FIDELITY
	j	gso_run - 4 * (\ta - 2 - GSO_RUN - GSO_BASE)
#else
	j	gso_run
#endif
	nop
	.endm

# GSOEND is the end of a sorted polygon and of one not sorted, which is at label 7 with
# three instructions past the last thing the GTE saw. The first takes wf to the next, the
# second wo.
	.macro GSOEND wf, wo, ta, tao
	GAP	\wf - 2
	bnez	$a1, 1b
	nop
	GSOBACK \ta
7:	G
	GAP	\wo - 5
	bnez	$a1, 1b
	nop
	GSOBACK \tao
	.endm

# CF3 reads FLAG and takes the polygon out on a bad point, and CLIP3 does so for a polygon
# turned away, with what the GTE needs before NCLIP's result in between.
	.macro CF3
	CFLAG
	nop
	bltz	$t6, 7f
	nop
	.endm
	.macro MF3
	mfc2	$t6, $24
	nop
	blez	$t6, 7f
	nop
	.endm

# POLYLINK takes the depth in t6 to the entry in the ordering table, and makes the packet
# at a2 the first of it. t7, t8 and at go. It takes 14 instructions.
	.macro POLYLINK
	srav	$t6, $t6, $t2
	sll	$t6, $t6, 2
	addu	$t6, $t6, $t1
	lw	$t7, 0($t6)
	lui	$at, 0xff00
	and	$t8, $t7, $at
	xor	$t7, $t7, $t8
	lui	$at, P_PLEN << 8
	or	$t7, $t7, $at
	sw	$t7, 0($a2)
	sll	$at, $a2, 8
	srl	$at, $at, 8
	or	$t8, $t8, $at
	sw	$t8, 0($t6)
	.endm

# LITF is a lit or fogged flat polygon: its normal in t7, its color in the GTE before
# anything else, and its link while it is lit.
	.macro LITF name, light, t, q, g1, g2, g3, wf, wo, col, ta
	PSET	\light, \t, 0, \q, 0
	GSOSTART \name, \light, \t, 0, 0, \col
1:	lwc2	$0, 0($t3)
	lwc2	$1, 4($t3)
	G
	addiu	$a0, $a0, P_SIZE
	addiu	$a1, $a1, -1
	lhu	$t7, P_N0 + P_BACK($a0)
	GAP	3
	lwc2	$2, 0($t4)
	lwc2	$3, 4($t4)
	G
	sll	$t7, $t7, 3
	addu	$t7, $t7, $v1
	.if !\t
	lw	$v0, P_COL + P_BACK($a0)
	.endif
	GAP	3
	lwc2	$4, 0($t5)
	lwc2	$5, 4($t5)
	G
	.if !\t
	or	$v0, $v0, $a3
	.else
	nop
	.endif
	RTPT
	G
	lhu	$t3, P_V0($a0)
	lhu	$t4, P_V1($a0)
	lhu	$t5, P_V2($a0)
	sll	$t3, $t3, 3
	sll	$t4, $t4, 3
	sll	$t5, $t5, 3
	addu	$t3, $t3, $t0
	addu	$t4, $t4, $t0
	.if \q
	lhu	$t9, P_V3 + P_BACK($a0)
	.endif
	GAP	\g1
	.if \t
	mtc2	$a3, $6
	.else
	mtc2	$v0, $6
	.endif
	G
	addu	$t5, $t5, $t0
	.if \q
	sll	$t9, $t9, 3
	addu	$t9, $t9, $t0
	.endif
	POLYTEX	\t, $t6, $t8
	GAP	\g2
	CF3
	NCLIP
	.if \q
	lwc2	$0, 0($t9)
	lwc2	$1, 4($t9)
	.endif
	MF3
	.if \q
	swc2	$12, 8($a2)
	swc2	$13, 8 + 4 * P_S($a2)
	swc2	$14, 8 + 8 * P_S($a2)
	nop
	RTPS
	G
	GAP	\g3
	CF3
	AVSZ4
	.else
	AVSZ3
	.endif
	lwc2	$0, 0($t7)
	lwc2	$1, 4($t7)
	mfc2	$t6, $7
	nop
	LIGHTONE \light
	G
	POLYLINK
	GAP	16
	.if \q
	swc2	$14, 8 + 12 * P_S($a2)
	.else
	swc2	$12, 8($a2)
	swc2	$13, 8 + 4 * P_S($a2)
	swc2	$14, 8 + 8 * P_S($a2)
	.endif
	swc2	$22, 4($a2)
	G
	addiu	$a2, $a2, (P_PLEN + 1) * 4
	GSOEND	\wf, \wo, \ta, \ta
	.endm

# PREREST finds what's left of the next polygon's vertices for a lit gouraud kind, which
# finds t3 and the index of t4 in the time of its points' loads.
	.macro PREREST
	lhu	$t5, P_V2($a0)
	sll	$t4, $t4, 3
	addu	$t4, $t4, $t0
	sll	$t5, $t5, 3
	addu	$t5, $t5, $t0
	.endm

	.macro LITGHEAD
1:	lwc2	$0, 0($t3)
	lwc2	$1, 4($t3)
	G
	addiu	$a0, $a0, P_SIZE
	addiu	$a1, $a1, -1
	lhu	$t3, P_V0($a0)
	GAP	3
	lwc2	$2, 0($t4)
	lwc2	$3, 4($t4)
	G
	sll	$t3, $t3, 3
	addu	$t3, $t3, $t0
	lhu	$t4, P_V1($a0)
	GAP	3
	lwc2	$4, 0($t5)
	lwc2	$5, 4($t5)
	.endm

# LITG3 is a lit or fogged gouraud triangle: its normals in t7 to t9, its lighting after
# its depth, and its link while it is lit.
	.macro LITG3 name, light, t, g1, g2, gn, wf, wo, col, ta, tao
	PSET	\light, \t, 1, 0, 0
	GSOSTART \name, \light, \t, 1, 0, \col
	LITGHEAD
	.if \t
	nop
	.else
	lw	$v0, P_COL + P_BACK($a0)
	.endif
	RTPT
	G
	lhu	$t7, P_N0 + P_BACK($a0)
	lhu	$t8, P_N1 + P_BACK($a0)
	lhu	$t9, P_N2 + P_BACK($a0)
	sll	$t7, $t7, 3
	sll	$t8, $t8, 3
	sll	$t9, $t9, 3
	addu	$t7, $t7, $v1
	addu	$t8, $t8, $v1
	addu	$t9, $t9, $v1
	GAP	\g1
	.if \t
	mtc2	$a3, $6
	G
	POLYTEX	\t, $t6, $t5
	GAP	\g2
	.endif
	CF3
	NCLIP
	.if !\t
	G
	or	$v0, $v0, $a3
	GAP	\g2
	mtc2	$v0, $6
	.endif
	MF3
	AVSZ3
	lwc2	$0, 0($t7)
	lwc2	$1, 4($t7)
	lwc2	$2, 0($t8)
	lwc2	$3, 4($t8)
	lwc2	$4, 0($t9)
	lwc2	$5, 4($t9)
	mfc2	$t6, $7
	nop
	LIGHTTHREE \light
	G
	POLYLINK
	PREREST
	GAP	\gn
	swc2	$12, 8($a2)
	swc2	$13, 8 + 4 * P_S($a2)
	swc2	$14, 8 + 8 * P_S($a2)
	swc2	$20, 4($a2)
	swc2	$21, 4 + 4 * P_S($a2)
	swc2	$22, 4 + 8 * P_S($a2)
	G
	addiu	$a2, $a2, (P_PLEN + 1) * 4
	GAP	\wf - 2
	bnez	$a1, 1b
	nop
	GSOBACK \ta
7:	G
	PREREST
	GAP	\wo - 5
	bnez	$a1, 1b
	nop
	GSOBACK \tao
	.endm

# LITG4 is a lit or fogged gouraud quad: normals in t7, t8, v0 and at, and the fourth
# corner's lighting before the others'.
	.macro LITG4 name, light, t, g1, g2, g3, gq, gn, wf, wo, col, ta, tao
	PSET	\light, \t, 1, 1, 0
	GSOSTART \name, \light, \t, 1, 0, \col
	LITGHEAD
	nop
	RTPT
	G
	lhu	$t9, P_V3 + P_BACK($a0)
	lhu	$t7, P_N0 + P_BACK($a0)
	lhu	$t8, P_N1 + P_BACK($a0)
	sll	$t9, $t9, 3
	addu	$t9, $t9, $t0
	lhu	$v0, P_N2 + P_BACK($a0)
	sll	$t7, $t7, 3
	addu	$t7, $t7, $v1
	lhu	$at, P_N3 + P_BACK($a0)
	.if \t
	GAP	\g1
	mtc2	$a3, $6
	G
	.endif
	sll	$t8, $t8, 3
	addu	$t8, $t8, $v1
	sll	$v0, $v0, 3
	addu	$v0, $v0, $v1
	sll	$at, $at, 3
	addu	$at, $at, $v1
	POLYTEX	\t, $t6, $t5
	.if \t
	GAP	\g2
	.else
	GAP	\g1
	.endif
	CF3
	NCLIP
	lwc2	$0, 0($t9)
	lwc2	$1, 4($t9)
	MF3
	swc2	$12, 8($a2)
	swc2	$13, 8 + 4 * P_S($a2)
	swc2	$14, 8 + 8 * P_S($a2)
	nop
	RTPS
	G
	.if !\t
	lw	$t6, P_COL + P_BACK($a0)
	nop
	or	$t6, $t6, $a3
	GAP	\g3
	mtc2	$t6, $6
	.endif
	lwc2	$2, 0($t8)
	lwc2	$3, 4($t8)
	lwc2	$4, 0($v0)
	lwc2	$5, 4($v0)
	CF3
	AVSZ4
	lwc2	$0, 0($at)
	lwc2	$1, 4($at)
	mfc2	$t6, $7
	nop
	LIGHTONE \light
	G
	GAP	\gq
	swc2	$22, 4 + 12 * P_S($a2)
	lwc2	$0, 0($t7)
	lwc2	$1, 4($t7)
	LIGHTTHREE \light
	G
	POLYLINK
	PREREST
	GAP	\gn
	swc2	$14, 8 + 12 * P_S($a2)
	swc2	$20, 4($a2)
	swc2	$21, 4 + 4 * P_S($a2)
	swc2	$22, 4 + 8 * P_S($a2)
	G
	addiu	$a2, $a2, (P_PLEN + 1) * 4
	GAP	\wf - 2
	bnez	$a1, 1b
	nop
	GSOBACK \ta
7:	G
	PREREST
	GAP	\wo - 5
	bnez	$a1, 1b
	nop
	GSOBACK \tao
	.endm

# UHEAD is the start of a polygon with no lighting to do, whether unlit or lit with
# lighting off, up to its perspective.
	.macro UHEAD name, light, t, g, flat, q, col
	PSET	\light, \t, \g, \q, \flat
	GSOSTART \name, \light, \t, \g, \flat, \col
1:	lwc2	$0, 0($t3)
	lwc2	$1, 4($t3)
	lwc2	$2, 0($t4)
	lwc2	$3, 4($t4)
	lwc2	$4, 0($t5)
	lwc2	$5, 4($t5)
	addiu	$a0, $a0, P_SIZE
	RTPT
	.endm

# UA1 starts the stretch after the perspective, with what all ways want.
	.macro UA1
	G
	addiu	$a1, $a1, -1
	.endm

# PRESEVEN finds the rest of the next polygon's vertices when t3 and t4 are loading.
	.macro PRESEVEN
	lhu	$t5, P_V2($a0)
	sll	$t3, $t3, 3
	sll	$t4, $t4, 3
	sll	$t5, $t5, 3
	addu	$t3, $t3, $t0
	addu	$t4, $t4, $t0
	addu	$t5, $t5, $t0
	.endm

# FOURTH finds the fourth vertex in t9.
	.macro FOURTH
	lhu	$t9, P_V3 + P_BACK($a0)
	nop
	sll	$t9, $t9, 3
	addu	$t9, $t9, $t0
	.endm

# UCLIP takes a polygon on from FLAG to its turn, a quad's fourth point going into the GTE
# on the way. UPOINTS stores the first three points, UFOURTH starts the fourth's perspective
# and UFLAG is the second look at FLAG, then a quad's depth.
	.macro UCLIP q
	CF3
	NCLIP
	.if \q
	lwc2	$0, 0($t9)
	lwc2	$1, 4($t9)
	.endif
	MF3
	.endm
	.macro UPOINTS
	swc2	$12, 8($a2)
	swc2	$13, 8 + 4 * P_S($a2)
	swc2	$14, 8 + 8 * P_S($a2)
	.endm
	.macro UFOURTH
	nop
	RTPS
	G
	.endm
	.macro UFLAG
	CF3
	AVSZ4
	.endm

# ULATE links a polygon after its last store, in its wf. UEARLY links it between the depth
# and the stores, which follow it, and UEND finishes one so linked.
	.macro ULATE wf, wo, ta
	G
	POLYLINK
	addiu	$a2, $a2, (P_PLEN + 1) * 4
	GSOEND	\wf, \wo, \ta, \ta
	.endm
	.macro UEND wf, wo, ta
	G
	addiu	$a2, $a2, (P_PLEN + 1) * 4
	GSOEND	\wf, \wo, \ta, \ta
	.endm
	.macro UEARLY
	mfc2	$t6, $7
	G
	nop
	POLYLINK
	GAP	17
	.endm

# TEXFIRST masks an unlit textured kind's first color in v0 and sets its code.
	.macro TEXFIRST
	sll	$v0, $v0, 8
	srl	$v0, $v0, 8
	or	$v0, $v0, $a3
	.endm

# An unlit or lighting-off flat triangle.
	.macro UF3 name, light, col, ta
	UHEAD	\name, \light, 0, 0, 0, 0, \col
	UA1
	lw	$v0, P_COL + P_BACK($a0)
	lhu	$t3, P_V0($a0)
	or	$v0, $v0, $a3
	lhu	$t4, P_V1($a0)
	GAP	5
	mtc2	$v0, $22
	G
	PRESEVEN
	GAP	11
	UCLIP	0
	AVSZ3
	UEARLY
	swc2	$22, 4($a2)
	UPOINTS
	UEND	4, 6, \ta
	.endm

# An unlit flat textured triangle.
	.macro UFT3 name, col, ta
	UHEAD	\name, 0, 1, 0, 0, 0, \col
	UA1
	lw	$v0, P_COL + P_BACK($a0)
	lw	$t6, 4 + P_BACK($a0)
	TEXFIRST
	lw	$t7, 8 + P_BACK($a0)
	sw	$t6, 12($a2)
	sw	$t7, 12 + 4 * P_S($a2)
	lw	$t6, 12 + P_BACK($a0)
	GAP	11
	mtc2	$v0, $22
	G
	sw	$t6, 12 + 8 * P_S($a2)
	PREFETCH P_V0, P_V1, P_V2
	GAP	17
	UCLIP	0
	AVSZ3
	UEARLY
	swc2	$22, 4($a2)
	UPOINTS
	UEND	4, 6, \ta
	.endm

# A lighting-off flat textured triangle.
	.macro OFT3 name, col, ta
	UHEAD	\name, 3, 1, 0, 0, 0, \col
	UA1
	sw	$a3, 4($a2)
	POLYTEX	1, $t6, $t7
	PREFETCH P_V0, P_V1, P_V2
	GAP	27
	UCLIP	0
	AVSZ3
	UEARLY
	UPOINTS
	UEND	4, 6, \ta
	.endm

# An unlit or lighting-off flat quad.
	.macro UF4 name, light, col, ta
	UHEAD	\name, \light, 0, 0, 0, 1, \col
	UA1
	lw	$v0, P_COL + P_BACK($a0)
	lhu	$t3, P_V0($a0)
	or	$v0, $v0, $a3
	lhu	$t4, P_V1($a0)
	GAP	5
	mtc2	$v0, $22
	G
	PRESEVEN
	FOURTH
	GAP	14
	UCLIP	1
	swc2	$22, 4($a2)
	UPOINTS
	UFOURTH
	UFLAG
	mfc2	$t6, $7
	swc2	$14, 8 + 12 * P_S($a2)
	ULATE	20, 6, \ta
	.endm

# An unlit flat textured quad.
	.macro UFT4 name, col, ta
	UHEAD	\name, 0, 1, 0, 0, 1, \col
	UA1
	POLYTEX	1, $t6, $t7
	PREFETCH P_V0, P_V1, P_V2
	FOURTH
	GAP	26
	UCLIP	1
	UPOINTS
	UFOURTH
	lw	$v0, P_COL + P_BACK($a0)
	nop
	TEXFIRST
	sw	$v0, 4($a2)
	GAP	9
	UFLAG
	mfc2	$t6, $7
	swc2	$14, 8 + 12 * P_S($a2)
	ULATE	20, 6, \ta
	.endm

# A lighting-off flat textured quad, or a textured gouraud one, which makes the same.
	.macro OFT4 name, g, flat, col, g5, ta
	UHEAD	\name, 3, 1, \g, \flat, 1, \col
	UA1
	PREFETCH P_V0, P_V1, P_V2
	FOURTH
	GAP	14
	UCLIP	1
	UPOINTS
	UFOURTH
	POLYTEX	1, $t6, $t7
	sw	$a3, 4($a2)
	GAP	\g5
	UFLAG
	.if \g
	G
	GAP	5
	mfc2	$t6, $7
	nop
	.else
	mfc2	$t6, $7
	.endif
	swc2	$14, 8 + 12 * P_S($a2)
	ULATE	20, 6, \ta
	.endm

# An unlit gouraud triangle: its first color by hand, the others by the GTE, stored last.
	.macro UG3 name, col, ta
	UHEAD	\name, 0, 0, 1, 0, 0, \col
	UA1
	lw	$v0, P_COL + P_BACK($a0)
	lhu	$t3, P_V0($a0)
	or	$v0, $v0, $a3
	lhu	$t4, P_V1($a0)
	GAP	5
	mtc2	$v0, $20
	lwc2	$21, P_COL + 4 + P_BACK($a0)
	lwc2	$22, P_COL + 8 + P_BACK($a0)
	G
	PRESEVEN
	GAP	11
	UCLIP	0
	AVSZ3
	mfc2	$t6, $7
	swc2	$20, 4($a2)
	swc2	$21, 4 + 4 * P_S($a2)
	swc2	$22, 4 + 8 * P_S($a2)
	UPOINTS
	ULATE	20, 6, \ta
	.endm

# An unlit gouraud textured triangle.
	.macro UGT3 name, col, ta
	UHEAD	\name, 0, 1, 1, 0, 0, \col
	UA1
	lw	$v0, P_COL + P_BACK($a0)
	lw	$t6, 4 + P_BACK($a0)
	TEXFIRST
	lw	$t7, 8 + P_BACK($a0)
	sw	$t6, 12($a2)
	sw	$t7, 12 + 4 * P_S($a2)
	lw	$t6, 12 + P_BACK($a0)
	GAP	11
	mtc2	$v0, $20
	lwc2	$21, P_COL + 4 + P_BACK($a0)
	lwc2	$22, P_COL + 8 + P_BACK($a0)
	G
	sw	$t6, 12 + 8 * P_S($a2)
	PREFETCH P_V0, P_V1, P_V2
	GAP	17
	UCLIP	0
	AVSZ3
	mfc2	$t6, $7
	UPOINTS
	swc2	$20, 4($a2)
	swc2	$21, 4 + 4 * P_S($a2)
	swc2	$22, 4 + 8 * P_S($a2)
	ULATE	20, 6, \ta
	.endm

# An unlit gouraud quad, its colors found while its fourth point is.
	.macro UG4 name, col, ta
	UHEAD	\name, 0, 0, 1, 0, 1, \col
	UA1
	PREFETCH P_V0, P_V1, P_V2
	FOURTH
	GAP	15
	UCLIP	1
	UPOINTS
	UFOURTH
	lw	$v0, P_COL + P_BACK($a0)
	lw	$t6, P_COL + 4 + P_BACK($a0)
	or	$v0, $v0, $a3
	sw	$v0, 4($a2)
	sw	$t6, 4 + 4 * P_S($a2)
	lw	$v0, P_COL + 8 + P_BACK($a0)
	lw	$t6, P_COL + 12 + P_BACK($a0)
	sw	$v0, 4 + 8 * P_S($a2)
	sw	$t6, 4 + 12 * P_S($a2)
	GAP	10
	UFLAG
	G
	GAP	1
	mfc2	$t6, $7
	nop
	swc2	$14, 8 + 12 * P_S($a2)
	ULATE	20, 6, \ta
	.endm

# An unlit gouraud textured quad.
	.macro UGT4 name, col, ta
	UHEAD	\name, 0, 1, 1, 0, 1, \col
	UA1
	lw	$v0, P_COL + P_BACK($a0)
	lw	$t6, 4 + P_BACK($a0)
	TEXFIRST
	lw	$t7, 8 + P_BACK($a0)
	sw	$t6, 12($a2)
	sw	$t7, 12 + 4 * P_S($a2)
	lw	$t6, 12 + P_BACK($a0)
	GAP	11
	mtc2	$v0, $20
	lwc2	$21, P_COL + 4 + P_BACK($a0)
	lwc2	$22, P_COL + 8 + P_BACK($a0)
	lwc2	$6, P_COL + 12 + P_BACK($a0)
	G
	lw	$t7, 16 + P_BACK($a0)
	lhu	$t9, P_V3 + P_BACK($a0)
	sw	$t6, 12 + 8 * P_S($a2)
	sw	$t7, 12 + 12 * P_S($a2)
	sll	$t9, $t9, 3
	addu	$t9, $t9, $t0
	PREFETCH P_V0, P_V1, P_V2
	GAP	15
	UCLIP	1
	UPOINTS
	UFOURTH
	GAP	2
	UFLAG
	G
	GAP	5
	mfc2	$t6, $7
	nop
	swc2	$14, 8 + 12 * P_S($a2)
	swc2	$20, 4($a2)
	swc2	$21, 4 + 4 * P_S($a2)
	swc2	$22, 4 + 8 * P_S($a2)
	swc2	$6, 4 + 12 * P_S($a2)
	ULATE	20, 6, \ta
	.endm

# A lighting-off gouraud triangle: the one color to each corner by the GTE.
	.macro OG3 name, col, ta
	UHEAD	\name, 3, 0, 1, 0, 0, \col
	UA1
	lw	$v0, P_COL + P_BACK($a0)
	lhu	$t3, P_V0($a0)
	or	$v0, $v0, $a3
	lhu	$t4, P_V1($a0)
	GAP	5
	mtc2	$v0, $20
	G
	PRESEVEN
	GAP	10
	UCLIP	0
	AVSZ3
	UEARLY
	swc2	$20, 4($a2)
	swc2	$20, 4 + 4 * P_S($a2)
	swc2	$20, 4 + 8 * P_S($a2)
	UPOINTS
	UEND	4, 6, \ta
	.endm

# A lighting-off gouraud quad.
	.macro OG4 name, col, ta
	UHEAD	\name, 3, 0, 1, 0, 1, \col
	UA1
	lw	$v0, P_COL + P_BACK($a0)
	lhu	$t3, P_V0($a0)
	or	$v0, $v0, $a3
	lhu	$t4, P_V1($a0)
	GAP	5
	mtc2	$v0, $20
	G
	PRESEVEN
	FOURTH
	GAP	13
	UCLIP	1
	UPOINTS
	UFOURTH
	UFLAG
	UEARLY
	swc2	$20, 4($a2)
	swc2	$20, 4 + 4 * P_S($a2)
	swc2	$20, 4 + 8 * P_S($a2)
	swc2	$20, 4 + 12 * P_S($a2)
	swc2	$14, 8 + 12 * P_S($a2)
	UEND	4, 6, \ta
	.endm

# A lighting-off textured gouraud triangle makes a flat textured packet, its texture
# coordinates in the GTE's color registers while it works on the points.
	.macro OGT3 name, col, ta
	UHEAD	\name, 3, 1, 1, 1, 0, \col
	lwc2	$20, 4 + P_BACK($a0)
	lwc2	$21, 8 + P_BACK($a0)
	lwc2	$22, 12 + P_BACK($a0)
	UA1
	sw	$a3, 4($a2)
	PREFETCH P_V0, P_V1, P_V2
	GAP	21
	UCLIP	0
	AVSZ3
	mfc2	$t6, $7
	swc2	$20, 12($a2)
	swc2	$21, 12 + 4 * P_S($a2)
	swc2	$22, 12 + 8 * P_S($a2)
	UPOINTS
	ULATE	20, 6, \ta
	.endm

# GRAD is a lit 20h or 30h with a color for each corner (spec 22): a gouraud packet, never
# cut and with no translucency. g says it is a 30h, with a normal to each corner. FLAG
# and the turn go by the GTE's stores to memory, and each corner is lit on its own, the
# link in the time that takes. A 20h's first color word says 30h in the packet.
	.macro GRADSET g
	PSET	1, 0, 1, 0, 0
	.set P_B, 16
	.set P_N0, P_B
	.set P_V0, P_B + 2
	.if \g
	.set P_N1, P_B + 4
	.set P_V1, P_B + 6
	.set P_N2, P_B + 8
	.set P_V2, P_B + 10
	.set P_SIZE, P_B + 12
	.else
	.set P_N1, P_B
	.set P_N2, P_B
	.set P_V1, P_B + 4
	.set P_V2, P_B + 6
	.set P_SIZE, P_B + 8
	.endif
	.set P_BACK, -P_SIZE
	.set P_S, 2
	.set P_PLEN, 6
	.endm

	.macro GRADHEAD name, g, col, g1
	GRADSET	\g
	.type gso_\name, @function
gso_\name\():
	G
	PREFETCH P_V0, P_V1, P_V2
	GAP	GSO_BASE + \col
1:	lwc2	$0, 0($t3)
	lwc2	$1, 4($t3)
	lwc2	$2, 0($t4)
	lwc2	$3, 4($t4)
	lwc2	$4, 0($t5)
	lwc2	$5, 4($t5)
	addiu	$a0, $a0, P_SIZE
	addiu	$a1, $a1, -1
	RTPT
	G
	GAP	\g1
	CFLAG
	nop
	bltz	$t6, 7f
	nop
	G
	GAP	5
	NCLIP
	.endm

# GRADTURN is from the store of NCLIP's result to the points in the packet.
	.macro GRADTURN
	swc2	$24, 28($sp)
	lw	$t6, 28($sp)
	nop
	blez	$t6, 6f
	nop
	nop
	swc2	$12, 8($a2)
	swc2	$13, 16($a2)
	swc2	$14, 24($a2)
	.endm

# GRADCORNER lights a corner: the color at off, its normal's index at n.
	.macro GRADCORNER off, n, light, g6
	lhu	$t7, \n + P_BACK($a0)
	nop
	sll	$t7, $t7, 3
	addu	$t7, $t7, $v1
	.endm

# GRADEND leaves a polygon sorted, then one turned away at 6, one with a bad point at 7.
	.macro GRADEND wf, wb, ta, e
	G
	addiu	$a2, $a2, 28
	PREFETCH P_V0, P_V1, P_V2
	GAP	\wf - 2
	bnez	$a1, 1b
	nop
	GSOBACK \ta
7:	NOPS	\e
6:	G
	PREFETCH P_V0, P_V1, P_V2
	GAP	\wb - 4 - 2
	bnez	$a1, 1b
	nop
	GSOBACK \ta
	.endm

	.macro GRAD name, light, g, col, g1, g6, ta
	GRADHEAD \name, \g, \col, \g1
	nop
	GRADTURN
	nop
	nop
	AVSZ3
	nop
	swc2	$7, 32($sp)
	G
	GAP	\g6
	lwc2	$6, 4 + P_BACK($a0)
	lhu	$t7, P_N0 + P_BACK($a0)
	lw	$t6, 32($sp)
	sll	$t7, $t7, 3
	addu	$t7, $t7, $v1
	nop
	lwc2	$0, 0($t7)
	lwc2	$1, 4($t7)
	nop
	srav	$t6, $t6, $t2
	LIGHTONE \light
	G
	sll	$t6, $t6, 2
	addu	$t6, $t6, $t1
	lw	$t8, 0($t6)
	lui	$at, 0xff00
	and	$v0, $t8, $at
	xor	$t8, $t8, $v0
	lui	$at, P_PLEN << 8
	or	$t8, $t8, $at
	sw	$t8, 0($a2)
	sll	$at, $a2, 8
	GAP	10
	swc2	$22, 4($a2)
	srl	$at, $at, 8
	or	$v0, $v0, $at
	lwc2	$6, 8 + P_BACK($a0)
	sw	$v0, 0($t6)
	lhu	$t7, P_N1 + P_BACK($a0)
	nop
	sll	$t7, $t7, 3
	addu	$t7, $t7, $v1
	lwc2	$0, 0($t7)
	lwc2	$1, 4($t7)
	nop
	nop
	LIGHTONE \light
	G
	.if !\g
	li	$t8, 0x30
	sb	$t8, 7($a2)
	.endif
	GAP	9
	swc2	$22, 12($a2)
	nop
	nop
	lwc2	$6, 12 + P_BACK($a0)
	G
	lhu	$t7, P_N2 + P_BACK($a0)
	nop
	sll	$t7, $t7, 3
	addu	$t7, $t7, $v1
	GAP	5
	lwc2	$0, 0($t7)
	lwc2	$1, 4($t7)
	nop
	nop
	LIGHTONE \light
	G
	GAP	5
	swc2	$22, 20($a2)
	GRADEND	21, 24, \ta, 3
	.endm

# A gradated polygon with lighting off keeps its colors as they are.
	.macro OGRAD name, g, col, g1, ta
	GRADHEAD \name, \g, \col, \g1
	G
	GAP	4
	GRADTURN
	lw	$t7, 4 + P_BACK($a0)
	lw	$t8, 8 + P_BACK($a0)
	AVSZ3
	G
	.if !\g
	sll	$t7, $t7, 8
	srl	$t7, $t7, 8
	lui	$at, 0x3000
	or	$t7, $t7, $at
	.endif
	GAP	4
	swc2	$7, 32($sp)
	G
	sw	$t7, 4($a2)
	sw	$t8, 12($a2)
	lw	$t7, 12 + P_BACK($a0)
	lw	$t6, 32($sp)
	sw	$t7, 20($a2)
	srav	$t6, $t6, $t2
	sll	$t6, $t6, 2
	addu	$t6, $t6, $t1
	lw	$t7, 0($t6)
	lui	$at, 0xff00
	and	$t8, $t7, $at
	xor	$t7, $t7, $t8
	lui	$at, P_PLEN << 8
	or	$t7, $t7, $at
	sw	$t7, 0($a2)
	sll	$at, $a2, 8
	srl	$at, $at, 8
	or	$t8, $t8, $at
	sw	$t8, 0($t6)
	GRADEND	16, 21, \ta, 4
	.endm

# POLYCODE makes a3 what the kind's colors take from the header, from the header in t6
# and the translucency in a3: an untextured kind has its own color word, and only
# ORs in the translucency.
	.macro POLYCODE light, t, g, flat
	.if \t
	.if \light == 0
	srl	$t6, $t6, 25
	sll	$t6, $t6, 25
	or	$a3, $a3, $t6
	.else
	.if \flat
	srl	$t6, $t6, 24
	andi	$t6, $t6, 0xef
	ori	$t6, $t6, 1
	sll	$t6, $t6, 24
	.else
	srl	$t6, $t6, 24
	sll	$t6, $t6, 24
	.endif
	or	$a3, $a3, $t6
	lui	$t6, 0x80
	ori	$t6, $t6, 0x8080
	or	$a3, $a3, $t6
	.endif
	.endif
	.endm

# POLYTEX copies a textured kind's coordinates to the packet.
	.macro POLYTEX t, r0, r1
	.if \t
	lw	\r0, 4 + P_BACK($a0)
	lw	\r1, 8 + P_BACK($a0)
	sw	\r0, 12($a2)
	sw	\r1, 12 + 4 * P_S($a2)
	lw	\r0, 12 + P_BACK($a0)
	.if P_N == 4
	lw	\r1, 16 + P_BACK($a0)
	.else
	nop
	.endif
	sw	\r0, 12 + 8 * P_S($a2)
	.if P_N == 4
	sw	\r1, 12 + 12 * P_S($a2)
	.endif
	.endif
	.endm

	.section .text.GsSortObject4, "ax", @progbits
	.align 2

# GsSortObject4(obj, ot, shift, scratch): spec 21 and exp/cleanlib/tmd.go. A polygon's
# cut is left to Go. This walks the runs GsLinkObject4 marked, with what it needs
# between them on the stack: how many polygons are left, the translucency, the table of
# loops for the light mode, and two words the GTE stores.
#
# Nothing happens in what's left of a run's or a call's time on Sony's: it is spent in
# the nops before gso_run and gso_exit, by a jump into them.
	.globl GsSortObject4
	.type GsSortObject4, @function
GsSortObject4:
	STUBTIME
	lw	$t6, 0($a0)
	lw	$t7, 8($a0)
	bltz	$t6, gso_hidden
	srl	$t8, $t6, 9
	andi	$t8, $t8, 7
	bnez	$t8, gso_div
	addiu	$sp, $sp, -56
	# The light mode's table: each way takes what it does on Sony's.
	G
	andi	$t8, $t6, 0x40
	bnez	$t8, 1f
	andi	$t8, $t6, 0x20
	bnez	$t8, 2f
	srl	$t9, $t6, 3
	lui	$t9, %hi(GsLIGHT_MODE)
	lw	$t9, %lo(GsLIGHT_MODE)($t9)
	nop
	andi	$t9, $t9, 3
	addiu	$t9, $t9, -1
	beqz	$t9, 4f
	lui	$t9, %hi(gso_lit)
	addiu	$t9, $t9, %lo(gso_lit)
	GAP	GSO_ENTRY - 2
	b	5f
	nop
4:	G
	lui	$t9, %hi(gso_fog)
	addiu	$t9, $t9, %lo(gso_fog)
	GAP	GSO_ENTRY - 1 - 2 - 12
	b	5f
	nop
2:	G
	andi	$t9, $t9, 3
	addiu	$t9, $t9, -1
	beqz	$t9, 4f
	lui	$t9, %hi(gso_lit)
	addiu	$t9, $t9, %lo(gso_lit)
	GAP	GSO_ENTRY - 1 - 2 - 5
	b	5f
	nop
4:	G
	lui	$t9, %hi(gso_fog)
	addiu	$t9, $t9, %lo(gso_fog)
	GAP	GSO_ENTRY + 1 - 2 - 9
	b	5f
	nop
1:	G
	lui	$t9, %hi(gso_off)
	addiu	$t9, $t9, %lo(gso_off)
	GAP	GSO_ENTRY - 13 - 2 - 3
	b	5f
	nop
5:	sw	$t9, 24($sp)
	lui	$t9, %hi(gso_unlit)
	addiu	$t9, $t9, %lo(gso_unlit)
	sw	$t9, 36($sp)
gso_common:
	# Bit 30 is a bit of the command code.
	srl	$t8, $t6, 30
	andi	$t8, $t8, 1
	sll	$t8, $t8, 25
	sw	$t8, 20($sp)
	lw	$t0, 0($t7)
	lw	$v1, 8($t7)
	lw	$t8, 20($t7)
	lw	$t1, 4($a1)
	lw	$a0, 16($t7)
	sw	$t8, 16($sp)
	andi	$t2, $a2, 31
	lui	$a2, %hi(GsOUT_PACKET_P)
	lw	$a2, %lo(GsOUT_PACKET_P)($a2)
	b	gso_run
	nop
	NOPS	GSO_SLED
gso_run:
	lw	$t7, 16($sp)
	lw	$t6, 0($a0)
	beqz	$t7, gso_done
	andi	$a1, $t6, 0xffff
	beqz	$a1, gso_done
	sltu	$at, $t7, $a1
	beqz	$at, 1f
	srl	$t8, $t6, 24
	move	$a1, $t7
1:	subu	$t7, $t7, $a1
	sw	$t7, 16($sp)
	srl	$at, $t8, 5
	addiu	$at, $at, -1
	bnez	$at, gso_run
	andi	$t9, $t8, 1
	lw	$a3, 20($sp)
	bnez	$t9, 5f
	andi	$t7, $t8, 0x1c
	# With flag bit 2 a kind has the second half of the table, where only the triangles
	# without texture differ: they have a color for each corner.
	lw	$t9, 24($sp)
	srl	$at, $t6, 13
	andi	$at, $at, 0x20
	addu	$t7, $t7, $at
	b	6f
	addu	$t9, $t9, $t7
5:	lw	$t9, 36($sp)
	nop
	addu	$t9, $t9, $t7
	NOPS	3
6:	lw	$t9, 0($t9)
	nop
	jr	$t9
	nop
gso_done:
	lui	$t6, %hi(GsOUT_PACKET_P)
	sw	$a2, %lo(GsOUT_PACKET_P)($t6)
	addiu	$sp, $sp, 56
	NOPS	GSO_EXIT
	jr	$ra
	nop
gso_hidden:
	NOPS	GSO_HIDDEN
	jr	$ra
	nop

# With a cut asked for the same goes, with the tables of the loops that cut. Each light mode's
# time to gso_run is Sony's, and what the loops want of the level goes in gso_cut.
gso_div:
	G
	lui	$v0, %hi(gso_cut)
	addiu	$v0, $v0, %lo(gso_cut)
	sw	$ra, CS_RA($v0)
	sw	$t8, CS_LEV($v0)
	sw	$a3, CS_SCR($v0)
	sw	$t8, 0($a3)
	lh	$t9, CS_WH($v0)
	lh	$at, CS_WH + 2($v0)
	sw	$t9, 4($a3)
	sw	$at, 8($a3)
	addiu	$at, $a3, CS_ROOT + 0x4000
	sw	$at, CS_W5($v0)
	# The deepest frame, as far down as the level goes for three corners or four.
	addiu	$at, $t8, -1
	sll	$t9, $at, 2
	addu	$t9, $t9, $at
	sll	$t9, $t9, 1
	addu	$t9, $t9, $at
	sll	$t9, $t9, 3
	addu	$t9, $t9, $a3
	addiu	$t9, $t9, CS_ROOT + 3 * C_SIZE
	sw	$t9, CS_DEEP3($v0)
	sll	$t9, $at, 4
	addu	$t9, $t9, $at
	sll	$t9, $t9, 1
	addu	$t9, $t9, $at
	sll	$t9, $t9, 2
	addu	$t9, $t9, $a3
	addiu	$t9, $t9, CS_ROOT + 4 * C_SIZE
	sw	$t9, CS_DEEP4($v0)
	andi	$t8, $t6, 0x40
	bnez	$t8, 1f
	andi	$t8, $t6, 0x20
	bnez	$t8, 2f
	srl	$t9, $t6, 3
	lui	$t9, %hi(GsLIGHT_MODE)
	lw	$t9, %lo(GsLIGHT_MODE)($t9)
2:	nop
	andi	$t9, $t9, 3
	addiu	$t9, $t9, -1
	beqz	$t9, 4f
	lui	$t9, %hi(gso_clit)
	addiu	$t9, $t9, %lo(gso_clit)
	GAP	GSO_CUT
	b	5f
	nop
4:	G
	lui	$t9, %hi(gso_cfog)
	addiu	$t9, $t9, %lo(gso_cfog)
	GAP	GSO_CUT - 42
	b	5f
	nop
1:	G
	lui	$t9, %hi(gso_coff)
	addiu	$t9, $t9, %lo(gso_coff)
	GAP	GSO_CUT - 45
	b	5f
	nop
5:	sw	$t9, 24($sp)
	lui	$t9, %hi(gso_cunlit)
	addiu	$t9, $t9, %lo(gso_cunlit)
	b	gso_common
	sw	$t9, 36($sp)
	.size GsSortObject4, .-GsSortObject4

# The loops. For each kind: its name, what makes it, and the stretches of Sony's; col is
# what its start takes over GSO_BASE, and ta what the end of its run does.
	LITF	lf3, 1, 0, 0, 8, 6, 0, 4, 6, 6, 54
	LITF	lft3, 1, 1, 0, 12, 12, 0, 4, 6, 2, 56
	LITF	lf4, 1, 0, 1, 9, 8, 0, 4, 6, 1, 57
	LITF	lft4, 1, 1, 1, 9, 12, 9, 4, 6, 2, 54
	LITG3	lg3, 1, 0, 9, 5, 21, 7, 13, 6, 56, 51
	LITG3	lgt3, 1, 1, 9, 9, 22, 11, 13, 2, 56, 51
	LITG4	lg4, 1, 0, 16, 0, 5, 0, 21, 7, 13, 3, 55, 50
	LITG4	lgt4, 1, 1, 9, 16, 0, 8, 20, 7, 13, 2, 56, 51
	LITF	ff3, 2, 0, 0, 8, 6, 0, 4, 6, 6, 54
	LITF	fft3, 2, 1, 0, 12, 12, 0, 4, 6, 2, 56
	LITF	ff4, 2, 0, 1, 9, 8, 0, 4, 6, 1, 57
	LITF	fft4, 2, 1, 1, 9, 12, 9, 4, 6, 2, 54
	LITG3	fg3, 2, 0, 9, 5, 21, 7, 13, 6, 56, 51
	LITG3	fgt3, 2, 1, 9, 9, 22, 11, 13, 2, 56, 51
	LITG4	fg4, 2, 0, 16, 0, 5, 0, 21, 7, 13, 3, 55, 50
	LITG4	fgt4, 2, 1, 9, 16, 0, 8, 20, 7, 13, 2, 56, 51
	UF3	f3, 0, 1, 54
	UFT3	ft3, 0, 57
	UF4	f4, 0, 0, 54
	UFT4	ft4, 0, 54
	UG3	g3, 0, 56
	UGT3	gt3, 0, 57
	UG4	g4, 0, 57
	UGT4	gt4, 0, 58
	UF3	of3, 3, 11, 54
	OFT3	oft3, 7, 56
	UF4	of4, 3, 6, 57
	OFT4	oft4, 0, 0, 7, 19, 54
	OG3	og3, 9, 57
	OGT3	ogt3, 5, 57
	OG4	og4, 6, 56
	OFT4	ogt4, 1, 1, 5, 13, 57
	GRAD	lgr20, 1, 0, 22, 7, 1, 42
	GRAD	lgr30, 1, 1, 21, 2, 2, 43
	GRAD	fgr20, 2, 0, 22, 7, 1, 42
	GRAD	fgr30, 2, 1, 21, 2, 2, 43
	OGRAD	ogr20, 0, 22, 7, 42
	OGRAD	ogr30, 1, 21, 5, 43

# The tables, by the mode's textured, quad and gouraud bits, and then for the lit kinds the
# same with flag bit 2.
	.align 2
gso_unlit:
	.word gso_f3, gso_ft3, gso_f4, gso_ft4, gso_g3, gso_gt3, gso_g4, gso_gt4
gso_lit:
	.word gso_lf3, gso_lft3, gso_lf4, gso_lft4, gso_lg3, gso_lgt3, gso_lg4, gso_lgt4
	.word gso_lgr20, gso_lft3, gso_lf4, gso_lft4, gso_lgr30, gso_lgt3, gso_lg4, gso_lgt4
gso_fog:
	.word gso_ff3, gso_fft3, gso_ff4, gso_fft4, gso_fg3, gso_fgt3, gso_fg4, gso_fgt4
	.word gso_fgr20, gso_fft3, gso_ff4, gso_fft4, gso_fgr30, gso_fgt3, gso_fg4, gso_fgt4
gso_off:
	.word gso_of3, gso_oft3, gso_of4, gso_oft4, gso_og3, gso_ogt3, gso_og4, gso_ogt4
	.word gso_ogr20, gso_oft3, gso_of4, gso_oft4, gso_ogr30, gso_ogt3, gso_og4, gso_ogt4

# CTC, CFC: control registers from and to t0 to t5, which the assembler will not take by
# name. rt is the number of the general register, rd of the control register.
	.macro CTC rt, rd
	.word 0x48c00000 | (\rt << 16) | (\rd << 11)
	.endm
	.macro CFC rt, rd
	.word 0x48400000 | (\rt << 16) | (\rd << 11)
	.endm
	.macro GPF
	.word 0x4b98003d
	.endm
	.macro MVMVA12
	.word 0x4a486012
	.endm
# MVMVAIR0 and MVMVAIR12 multiply IR by the rotation, whole and shifted back by 12.
	.macro MVMVAIR0
	.word 0x4a41e012
	.endm
	.macro MVMVAIR12
	.word 0x4a49e012
	.endm

# Cut polygons: GsSortObject4 under GsDIV (spec 04, spec 31). The GTE sees what Sony's shows it
# from outside, at the same spacing, for each kind, level and way out.
#
# gso_cut holds what the cutting works from. A corner is a cell: its point in the model, its
# place and depth on the screen, its texture coordinates and its color. A level's frame has the
# five cells its cut makes, the corners of the piece being cut that come back into use, and where to
# go when the piece below it is done. The frames stand in for a stack, as a program's stack may be
# 1 KB of scratchpad.
	.section .bss.gso_cut, "aw", @nobits
	.align 2
	.globl libps_cut_limits
	.type libps_cut_limits, @object
libps_cut_limits:
gso_cut:
	.space CS_END
	.size libps_cut_limits, .-libps_cut_limits

# GAPS is GAP for a long pad, spent in a sled of nops all the loops share, as they are many. The way
# there and back takes at.
	.macro GAPS n
#ifdef LIBPS_FIDELITY
	.set GS_PAD, ((\n) * 4 - (. - 98b)) / 4
	.if GS_PAD > 8
	lui	$at, %hi(99f)
	addiu	$at, $at, %lo(99f)
	j	gso_sled_end - 4 * (GS_PAD - 6)
	nop
99:
	.else
	.space GS_PAD * 4
	.endif
98:
#endif
	.endm

	.section .text.gso_sled, "ax", @progbits
	.align 2
	.rept 140
	nop
	.endr
gso_sled_end:
	jr	$at
	nop

# NEAR3 and NEAR4 go on past a piece with a corner at half the projection or further. A corner nearer
# than that costs 4 cycles, and if all are the piece goes.
	.macro NEAR3 up
	CFC	14, 26
	lw	$t7, C_SZ($t3)
	sra	$t6, $t6, 1
	sltu	$at, $t7, $t6
	beqz	$at, 11f
	lw	$t7, C_SZ($t4)
	lw	$t8, C_SZ($t5)
	sltu	$at, $t7, $t6
	beqz	$at, 12f
	sltu	$at, $t8, $t6
	beqz	$at, 12f
	nop
	j	\up\()2
	lw	$t6, F_RET($a0)
11:	NOPS	2
12:
	.endm

	.macro NEAR4 up
	CFC	14, 26
	lw	$t7, C_SZ($t3)
	sra	$t6, $t6, 1
	sltu	$at, $t7, $t6
	beqz	$at, 11f
	lw	$t7, C_SZ($t4)
	lw	$t8, C_SZ($t5)
	sltu	$at, $t7, $t6
	beqz	$at, 19f
	lw	$v0, C_SZ($t9)
	sltu	$at, $t8, $t6
	beqz	$at, 12f
	sltu	$at, $v0, $t6
	beqz	$at, 12f
	nop
	NOPS	1
	j	\up\()2
	lw	$t6, F_RET($a0)
11:	NOPS	2
19:	NOPS	1
12:
	.endm

# SIDE3 and SIDE4 load a piece's places on one axis and go to lo and hi when the first is off the
# low or high side. RUNLOW and RUNHIGH are what is there: each corner on the same side costs 4
# cycles, and if all are, what follows them is where the piece goes.
	.macro SIDE3 cf, lim, off, lo, hi
	CFC	0, \cf
	lh	$t6, \lim($v1)
	lh	$t7, \lim + 2($v1)
	lh	$t8, \off($t3)
	lh	$v0, \off($t4)
	lh	$t1, \off($t5)
	slt	$at, $t8, $t6
	bnez	$at, \lo\()f
	slt	$at, $t7, $t8
	bnez	$at, \hi\()f
	slt	$at, $t7, $v0
	.endm

	.macro SIDE4 cf, lim, off, lo, hi
	CFC	0, \cf
	lh	$t6, \lim($v1)
	lh	$t7, \lim + 2($v1)
	lh	$t8, \off($t3)
	lh	$v0, \off($t4)
	lh	$t1, \off($t5)
	lh	$t2, \off($t9)
	slt	$at, $t8, $t6
	bnez	$at, \lo\()f
	slt	$at, $t7, $t8
	bnez	$at, \hi\()f
	slt	$at, $t7, $v0
	.endm

	.macro RUNLOW3 lab, way
\lab\():
	NOPS	1
	slt	$at, $v0, $t6
	beqz	$at, \way\()b
	slt	$at, $t1, $t6
	beqz	$at, \way\()b
	nop
	.endm
	.macro RUNHIGH3 lab, way, up
\lab\():
	beqz	$at, \way\()b
	slt	$at, $t7, $t1
	beqz	$at, \way\()b
	.ifb \up
	nop
	.else
	lw	$t6, F_RET($a0)
	.endif
	.endm
	.macro RUNLOW4 lab, way
\lab\():
	NOPS	1
	slt	$at, $v0, $t6
	beqz	$at, \way\()b
	slt	$at, $t1, $t6
	beqz	$at, \way\()b
	slt	$at, $t2, $t6
	beqz	$at, \way\()b
	nop
	.endm
	.macro RUNHIGH4 lab, way
\lab\():
	beqz	$at, \way\()b
	slt	$at, $t7, $t1
	beqz	$at, \way\()b
	slt	$at, $t7, $t2
	beqz	$at, \way\()b
	nop
	.endm

# TMID makes the three middles of the triangle t3, t4, t5 for the signed halfword at off, and TBYTE
# for the byte, which a color or texture coordinate is, one at a time as each keeps what's above it.
	.macro TMID off, l, sh
	\l	$t6, \off($t3)
	\l	$t7, \off($t4)
	\l	$t8, \off($t5)
	TMIDREST \off, \sh
	.endm
	.macro TMIDREST off, sh
	addu	$at, $t6, $t7
	sra	$at, $at, 1
	\sh	$at, M0 + \off($a0)
	addu	$at, $t7, $t8
	sra	$at, $at, 1
	\sh	$at, M1 + \off($a0)
	addu	$at, $t8, $t6
	sra	$at, $at, 1
	\sh	$at, M2 + \off($a0)
	.endm

# QMID1 and QMID2 are a quad's: QMID1 the three middles a cut makes first, QMID2 the fourth and the
# center, by the sum of two corners and the middle's own sum.
	.macro QLOAD off, l
	\l	$t6, \off($t3)
	\l	$t7, \off($t4)
	\l	$t8, \off($t5)
	\l	$v0, \off($t9)
	.endm
	.macro QMID1 off, sh
	addu	$at, $t6, $t7
	sra	$at, $at, 1
	\sh	$at, QAB + \off($a0)
	addu	$t2, $t7, $v0
	sra	$at, $t2, 1
	\sh	$at, QBE + \off($a0)
	addu	$at, $v0, $t8
	sra	$at, $at, 1
	\sh	$at, QEC + \off($a0)
	.endm
	.macro QMID2 off, sh
	addu	$t1, $t8, $t6
	sra	$at, $t1, 1
	\sh	$at, QCA + \off($a0)
	addu	$t1, $t1, $t2
	sra	$t1, $t1, 2
	\sh	$t1, QMD + \off($a0)
	.endm

# TPACK and QPACK write a packet from its cells, each a register and an offset. t1 is its length,
# t2 the flat color or the code's byte, t8 and v0 the first corners' palette and page.
	.macro PTAG t, g, size
	or	$a3, $a3, $t1
	sw	$a3, 0($a2)
	sll	$a3, $a2, 8
	srl	$a3, $a3, 8
	addiu	$a2, $a2, \size
	.endm

	.macro TPACK t, g, c0, o0, c1, o1, c2, o2
	.set TS, 4 * (1 + \t + \g)
	lw	$at, C_SXY + \o0(\c0)
	lw	$t6, C_SXY + \o1(\c1)
	lw	$t7, C_SXY + \o2(\c2)
	sw	$at, 8($a2)
	sw	$t6, 8 + TS($a2)
	sw	$t7, 8 + 2 * TS($a2)
	.if \t
	lhu	$at, C_UV + \o0(\c0)
	lhu	$t6, C_UV + \o1(\c1)
	lhu	$t7, C_UV + \o2(\c2)
	or	$at, $at, $t8
	or	$t6, $t6, $v0
	sw	$at, 12($a2)
	sw	$t6, 12 + TS($a2)
	sw	$t7, 12 + 2 * TS($a2)
	.endif
	.if \g
	sb	$t2, C_COL + 3 + \o0(\c0)
	lw	$at, C_COL + \o0(\c0)
	lw	$t6, C_COL + \o1(\c1)
	lw	$t7, C_COL + \o2(\c2)
	sw	$at, 4($a2)
	sw	$t6, 4 + TS($a2)
	sw	$t7, 4 + 2 * TS($a2)
	.else
	sw	$t2, 4($a2)
	.endif
	PTAG	\t, \g, 4 + 3 * TS + (1 - \g) * 4
	.endm

	.macro QPACK t, g, c0, o0, c1, o1, c2, o2, c3, o3
	.set TS, 4 * (1 + \t + \g)
	lw	$at, C_SXY + \o0(\c0)
	lw	$t6, C_SXY + \o1(\c1)
	lw	$t7, C_SXY + \o2(\c2)
	lw	$t0, C_SXY + \o3(\c3)
	sw	$at, 8($a2)
	sw	$t6, 8 + TS($a2)
	sw	$t7, 8 + 2 * TS($a2)
	sw	$t0, 8 + 3 * TS($a2)
	.if \t
	lhu	$at, C_UV + \o0(\c0)
	lhu	$t6, C_UV + \o1(\c1)
	lhu	$t7, C_UV + \o2(\c2)
	lhu	$t0, C_UV + \o3(\c3)
	or	$at, $at, $t8
	or	$t6, $t6, $v0
	sw	$at, 12($a2)
	sw	$t6, 12 + TS($a2)
	sw	$t7, 12 + 2 * TS($a2)
	sw	$t0, 12 + 3 * TS($a2)
	.endif
	.if \g
	sb	$t2, C_COL + 3 + \o0(\c0)
	lw	$at, C_COL + \o0(\c0)
	lw	$t6, C_COL + \o1(\c1)
	lw	$t7, C_COL + \o2(\c2)
	lw	$t0, C_COL + \o3(\c3)
	sw	$at, 4($a2)
	sw	$t6, 4 + TS($a2)
	sw	$t7, 4 + 2 * TS($a2)
	sw	$t0, 4 + 3 * TS($a2)
	.else
	sw	$t2, 4($a2)
	.endif
	PTAG	\t, \g, 4 + 4 * TS + (1 - \g) * 4
	.endm

# LEAFHEAD loads what every packet of a cut's last level has.
	.macro LEAFHEAD t, g, n
	lui	$t1, (4 * 0 + (\n) * (1 + \t + \g) + 1 - \g) << 8
	.if \g
	lbu	$t2, CS_A3 + 3($v1)
	.else
	lw	$t2, CS_COL($v1)
	.endif
	.if \t
	lw	$t8, CS_UV0($v1)
	lw	$v0, CS_UV1($v1)
	.endif
	.endm

# CUTTRI cuts a triangle: its corners in t3 to t5, a0 its level's frame, a1 that level, a2 the next
# packet, a3 the entry's first packet and v1 gso_cut. It returns to ra with a2 and a3 moved on and v1
# as it was. The last level links its four leaves.
#
# pre is the instructions from the last load to RTPT, n1 from RTPT to where the points are put, and
# n2 from there to the next piece's check. leaf is the instructions from the points to the way up.
# early has the frame written ahead of the points, where n2 is too short for it.
	.macro TNEXT name, a, b, c
	.ifb \a
	addiu	$t3, $a0, M0
	.else
	lw	$t3, \a($a0)
	.endif
	addiu	$t4, $a0, \b
	addiu	$t5, $a0, \c
	sw	$t3, F_SIZE + F_A($a0)
	sw	$t4, F_SIZE + F_B($a0)
	sw	$t5, F_SIZE + F_C($a0)
	addiu	$a0, $a0, F_SIZE
	bal	cut_\name
	sw	$ra, F_RET($a0)
	.endm

	.macro CUTTRI name, t, g, pre, n1, n2, leaf, early
	.section .text.GsSortCut.\name, "ax", @progbits
	.align 2
cut_\name\():
	NEAR3	cut_\name\()_up
	SIDE3	24, CS_LIMX, C_SXY, 13, 14
15:	NOPS	6
	SIDE3	25, CS_LIMY, C_SXY + 2, 16, 17
18:	TMID	0, lh, sh
	TMID	2, lh, sh
	TMID	C_Z, lh, sh
	NOPS	2
	lwc2	$0, M0($a0)
	lwc2	$1, M0 + C_Z($a0)
	lwc2	$2, M1($a0)
	lwc2	$3, M1 + C_Z($a0)
	lwc2	$4, M2($a0)
	lwc2	$5, M2 + C_Z($a0)
	G
	.if \t && \g
	TMID	C_UV, lbu, sb
	.else
	subu	$v0, $a0, $a1
	.if \g
	lbu	$t6, C_COL($t3)
	lbu	$t7, C_COL($t4)
	.endif
	.endif
	GAP	\pre
	RTPT
	G
	.if \t && \g
	subu	$v0, $a0, $a1
	TMID	C_UV + 1, lbu, sb
	.elseif \t
	TMID	C_UV, lbu, sb
	TMID	C_UV + 1, lbu, sb
	.endif
	.if \g
	.if \t
	TMID	C_COL, lbu, sb
	.else
	lbu	$t8, C_COL($t5)
	TMIDREST C_COL, sb
	.endif
	TMID	C_COL + 1, lbu, sb
	TMID	C_COL + 2, lbu, sb
	.endif
	.if \early == 0
	GAP	\n1 - 2
	.endif
	beqz	$v0, 21f
	nop
	.if \early
	addiu	$t6, $a0, M0
	addiu	$t7, $a0, M2
	sw	$t3, F_SIZE + F_A($a0)
	sw	$t6, F_SIZE + F_B($a0)
	sw	$t7, F_SIZE + F_C($a0)
	GAP	\n1
	.endif
	swc2	$17, M0 + C_SZ($a0)
	swc2	$18, M1 + C_SZ($a0)
	swc2	$19, M2 + C_SZ($a0)
	swc2	$12, M0 + C_SXY($a0)
	swc2	$13, M1 + C_SXY($a0)
	swc2	$14, M2 + C_SXY($a0)
	G
	addiu	$t4, $a0, M0
	addiu	$t5, $a0, M2
	.if \early == 0
	sw	$t3, F_SIZE + F_A($a0)
	sw	$t4, F_SIZE + F_B($a0)
	sw	$t5, F_SIZE + F_C($a0)
	.endif
	addiu	$a0, $a0, F_SIZE
	GAP	\n2 - 2
	bal	cut_\name
	sw	$ra, F_RET($a0)
	# The second piece, the third and the last.
	TNEXT	\name, F_B, M1, M0
	TNEXT	\name, F_C, M2, M1
	TNEXT	\name, , M1, M2
	lw	$t6, F_RET($a0)
	nop
	NOPS	2
	jr	$t6
	addiu	$a0, $a0, -F_SIZE
21:
	.if \early
	NOPS	\early
	.endif
	swc2	$12, M0 + C_SXY($a0)
	swc2	$13, M1 + C_SXY($a0)
	swc2	$14, M2 + C_SXY($a0)
	G
	LEAFHEAD \t, \g, 3
	TPACK	\t, \g, $t4, 0, $a0, M1, $a0, M0
	TPACK	\t, \g, $a0, M0, $a0, M1, $a0, M2
	TPACK	\t, \g, $t3, 0, $a0, M0, $a0, M2
	TPACK	\t, \g, $t5, 0, $a0, M2, $a0, M1
	GAPS	\leaf - 1
	j	cut_\name\()_up2
	lw	$t6, F_RET($a0)
	RUNLOW3	13, 15
	NOPS	6
	j	cut_\name\()_up2
	lw	$t6, F_RET($a0)
	RUNHIGH3 14, 15
	NOPS	2
	j	cut_\name\()_up2
	lw	$t6, F_RET($a0)
	RUNLOW3	16, 18
	NOPS	2
	j	cut_\name\()_up2
	lw	$t6, F_RET($a0)
	RUNHIGH3 17, 18, 1
cut_\name\()_up2:
	nop
	jr	$t6
	addiu	$a0, $a0, -F_SIZE
	.endm

	.set FT_RET, 84
	.set FT_SIZE, 88
	.set FQ_RET, 136
	.set FQ_SIZE, 140
	.set F_A, 72
	.set F_B, 76
	.set F_C, 80
	.set F_RET, 84
	.set F_SIZE, 88
	CUTTRI	f3, 0, 0, 1, 12, 5, 98, 6
	CUTTRI	ft3, 1, 0, 1, 27, 13, 150, 0
	CUTTRI	g3, 0, 1, 3, 37, 13, 126, 0
	CUTTRI	gt3, 1, 1, 12, 52, 13, 174, 0

# QCHECK is a quad's checks and the start of its z: the sum of a's and c's and ca's z, kept in t0 over
# the check on y.
	.macro QSTART name
	.section .text.GsSortCut.\name, "ax", @progbits
	.align 2
cut_\name\():
	NEAR4	cut_\name\()_up
	SIDE4	24, CS_LIMX, C_SXY, 13, 14
15:
	.endm

	.macro QCHECK name
	QSTART	\name
	lh	$at, C_Z($t3)
	lh	$t0, C_Z($t5)
	nop
	addu	$t0, $t0, $at
	sra	$at, $t0, 1
	sh	$at, QCA + C_Z($a0)
	SIDE4	25, CS_LIMY, C_SXY + 2, 16, 17
18:
	.endm

# QFRAME saves what's needed of the piece being cut for its other three pieces, and QRET where to go
# after the first. QDOWN goes on to the first.
	.macro QKID
	addiu	$t6, $a0, QAB
	sw	$t3, F_SIZE + F_A($a0)
	sw	$t6, F_SIZE + F_B($a0)
	addiu	$t6, $a0, QCA
	sw	$t6, F_SIZE + F_C($a0)
	addiu	$t6, $a0, QMD
	sw	$t6, F_SIZE + F_E($a0)
	.endm
	.macro QDOWNG name, n
	addiu	$t4, $a0, QAB
	addiu	$t5, $a0, QCA
	addiu	$t9, $a0, QMD
	addiu	$a0, $a0, F_SIZE
	GAP	\n - 2
	bal	cut_\name
	sw	$ra, F_RET($a0)
	QNEXT	\name, F_B, QBE, QAB
	QNEXT	\name, F_C, QCA, QEC
	QNEXT	\name, F_E, QEC, QBE
	lw	$t6, F_RET($a0)
	nop
	NOPS	2
	jr	$t6
	addiu	$a0, $a0, -F_SIZE
	.endm
	.macro QNEXT name, a, b, c
	lw	$t3, \a($a0)
	addiu	$t4, $a0, \b
	addiu	$t5, $a0, \c
	addiu	$t9, $a0, QMD
	sw	$t3, F_SIZE + F_A($a0)
	sw	$t4, F_SIZE + F_B($a0)
	sw	$t5, F_SIZE + F_C($a0)
	sw	$t9, F_SIZE + F_E($a0)
	addiu	$a0, $a0, F_SIZE
	bal	cut_\name
	sw	$ra, F_RET($a0)
	.endm

# QTAIL is what follows a quad's leaves.
	.macro QTAIL name
	QPACK	QT, QG, $t3, 0, $a0, QAB, $a0, QCA, $a0, QMD
	QPACK	QT, QG, $t4, 0, $a0, QBE, $a0, QAB, $a0, QMD
	QPACK	QT, QG, $t5, 0, $a0, QCA, $a0, QEC, $a0, QMD
	QPACK	QT, QG, $t9, 0, $a0, QEC, $a0, QBE, $a0, QMD
	.endm

	.macro QEND name, leaf
	GAPS	\leaf - 1
	j	cut_\name\()_up2
	lw	$t6, F_RET($a0)
	RUNLOW4	13, 15
	NOPS	7
	j	cut_\name\()_up2
	lw	$t6, F_RET($a0)
	RUNHIGH4 14, 15
	NOPS	3
	j	cut_\name\()_up2
	lw	$t6, F_RET($a0)
	RUNLOW4	16, 18
	NOPS	3
	j	cut_\name\()_up2
	lw	$t6, F_RET($a0)
	RUNHIGH4 17, 18
	lw	$t6, F_RET($a0)
cut_\name\()_up2:
	nop
	jr	$t6
	addiu	$a0, $a0, -F_SIZE
	.endm

# The three middles first, in x, y and z, and the rest of the five.
	.macro QXYZ
	QLOAD	0, lh
	QMID1	0, sh
	QMID2	0, sh
	QLOAD	2, lh
	QMID1	2, sh
	QMID2	2, sh
	QLOAD	C_Z, lh
	QMID1	C_Z, sh
	addu	$t1, $t0, $t2
	sra	$t1, $t1, 2
	sh	$t1, QMD + C_Z($a0)
	.endm

	.macro QLOAD012
	lwc2	$0, QAB($a0)
	lwc2	$1, QAB + C_Z($a0)
	lwc2	$2, QCA($a0)
	lwc2	$3, QCA + C_Z($a0)
	lwc2	$4, QMD($a0)
	lwc2	$5, QMD + C_Z($a0)
	.endm
	.macro QLOAD34
	lwc2	$0, QBE($a0)
	lwc2	$1, QBE + C_Z($a0)
	lwc2	$2, QEC($a0)
	lwc2	$3, QEC + C_Z($a0)
	.endm
	.macro QPUT012
	swc2	$12, QAB + C_SXY($a0)
	swc2	$13, QCA + C_SXY($a0)
	swc2	$14, QMD + C_SXY($a0)
	swc2	$17, QAB + C_SZ($a0)
	swc2	$18, QCA + C_SZ($a0)
	swc2	$19, QMD + C_SZ($a0)
	.endm

# A flat quad does its last three middles after the first RTPT, on the fourth and the center.
	.macro CUTF4 name, leaf
	.set QT, 0
	.set QG, 0
	QSTART	\name
	lh	$t0, C_Z($t3)
	lh	$ra, C_Z($t5)
	nop
	addu	$ra, $ra, $t0
	sra	$at, $ra, 1
	sh	$at, QCA + C_Z($a0)
	SIDE4	25, CS_LIMY, C_SXY + 2, 16, 17
18:	lh	$t6, 0($t3)
	lh	$t7, 0($t4)
	lh	$t8, 0($t5)
	addu	$at, $t6, $t7
	sra	$at, $at, 1
	sh	$at, QAB($a0)
	addu	$at, $t8, $t6
	sra	$at, $at, 1
	sh	$at, QCA($a0)
	lh	$t6, 2($t3)
	lh	$t7, 2($t4)
	lh	$t8, 2($t5)
	addu	$at, $t6, $t7
	sra	$at, $at, 1
	sh	$at, QAB + 2($a0)
	addu	$at, $t8, $t6
	sra	$at, $at, 1
	sh	$at, QCA + 2($a0)
	lh	$v0, C_Z($t9)
	lh	$t7, C_Z($t4)
	lh	$t8, C_Z($t5)
	addu	$at, $t0, $t7
	sra	$at, $at, 1
	sh	$at, QAB + C_Z($a0)
	addu	$at, $t7, $v0
	sra	$at, $at, 1
	sh	$at, QBE + C_Z($a0)
	NOPS	2
	lwc2	$0, QAB($a0)
	lwc2	$1, QAB + C_Z($a0)
	lwc2	$2, QCA($a0)
	lwc2	$3, QCA + C_Z($a0)
	nop
	RTPT
	G
	addu	$at, $v0, $t8
	sra	$at, $at, 1
	sh	$at, QEC + C_Z($a0)
	NOPS	1
	addu	$ra, $t0, $t7
	sra	$ra, $ra, 1
	addu	$ra, $ra, $at
	sra	$ra, $ra, 1
	sh	$ra, QMD + C_Z($a0)
	QLOAD	0, lh
	addu	$t1, $t8, $t6
	addu	$t2, $t7, $v0
	sra	$at, $t2, 1
	sh	$at, QBE($a0)
	addu	$at, $v0, $t8
	sra	$at, $at, 1
	sh	$at, QEC($a0)
	addu	$t1, $t1, $t2
	sra	$t1, $t1, 2
	sh	$t1, QMD($a0)
	QLOAD	2, lh
	addu	$t1, $t8, $t6
	addu	$t2, $t7, $v0
	sra	$at, $t2, 1
	sh	$at, QBE + 2($a0)
	addu	$at, $v0, $t8
	sra	$at, $at, 1
	sh	$at, QEC + 2($a0)
	addu	$t1, $t1, $t2
	sra	$t1, $t1, 2
	sh	$t1, QMD + 2($a0)
	GAP	39
	swc2	$12, QAB + C_SXY($a0)
	swc2	$13, QCA + C_SXY($a0)
	swc2	$17, QAB + C_SZ($a0)
	swc2	$18, QCA + C_SZ($a0)
	lwc2	$0, QBE($a0)
	lwc2	$1, QBE + C_Z($a0)
	lwc2	$2, QEC($a0)
	lwc2	$3, QEC + C_Z($a0)
	lwc2	$4, QMD($a0)
	lwc2	$5, QMD + C_Z($a0)
	nop
	RTPT
	G
	subu	$at, $a0, $a1
	beqz	$at, 21f
	nop
	QKID
	GAP	14
	swc2	$12, QBE + C_SXY($a0)
	swc2	$13, QEC + C_SXY($a0)
	swc2	$14, QMD + C_SXY($a0)
	swc2	$17, QBE + C_SZ($a0)
	swc2	$18, QEC + C_SZ($a0)
	swc2	$19, QMD + C_SZ($a0)
	G
	QDOWNG	\name, 6
21:	NOPS	6
	swc2	$12, QBE + C_SXY($a0)
	swc2	$13, QEC + C_SXY($a0)
	swc2	$14, QMD + C_SXY($a0)
	G
	LEAFHEAD 0, 0, 4
	QTAIL	\name
	QEND	\name, \leaf
	.endm

# The other quads do all five before the first RTPT, and what they make of texture coordinates and
# color goes on through both RTPTs. A last level does that after the points are stored.
	.macro BL off
	QLOAD	\off, lbu
	.endm
	.macro BAB off
	addu	$at, $t6, $t7
	srl	$at, $at, 1
	sb	$at, QAB + \off($a0)
	.endm
	.macro BBE off
	addu	$t2, $t7, $v0
	srl	$at, $t2, 1
	sb	$at, QBE + \off($a0)
	.endm
	.macro BEC off
	addu	$at, $v0, $t8
	srl	$at, $at, 1
	sb	$at, QEC + \off($a0)
	.endm
	.macro BCA off
	addu	$t1, $t8, $t6
	srl	$at, $t1, 1
	sb	$at, QCA + \off($a0)
	.endm
	.macro BMD off
	addu	$t1, $t1, $t2
	srl	$t1, $t1, 2
	sb	$t1, QMD + \off($a0)
	.endm
	.macro BALL off
	BL	\off
	BAB	\off
	BBE	\off
	BEC	\off
	BCA	\off
	BMD	\off
	.endm

# CUTQ's four stretches: before the second RTPT and after it, on the way down and at the last level.
	.macro CUTQ name, t, g, n1, n2, n3, leaf
	.set QT, \t
	.set QG, \g
	QCHECK	\name
	QXYZ
	NOPS	5
	QLOAD012
	nop
	RTPT
	G
	subu	$at, $a0, $a1
	beqz	$at, 21f
	nop
	QKID
	.if \g
	BALL	C_COL
	BL	C_COL + 1
	BAB	C_COL + 1
	.else
	BL	C_UV
	BAB	C_UV
	.endif
	GAP	\n1
	QPUT012
	QLOAD34
	nop
	RTPT
	G
	.if \g
	BBE	C_COL + 1
	BEC	C_COL + 1
	BCA	C_COL + 1
	BMD	C_COL + 1
	BALL	C_COL + 2
	.if \t
	BALL	C_UV
	BL	C_UV + 1
	addu	$at, $t6, $t7
	srl	$at, $at, 1
	.endif
	.else
	BBE	C_UV
	BEC	C_UV
	BCA	C_UV
	BMD	C_UV
	BALL	C_UV + 1
	.endif
	GAP	\n2
	swc2	$12, QBE + C_SXY($a0)
	swc2	$13, QEC + C_SXY($a0)
	swc2	$17, QBE + C_SZ($a0)
	swc2	$18, QEC + C_SZ($a0)
	G
	.if \t && \g
	sb	$at, QAB + C_UV + 1($a0)
	BBE	C_UV + 1
	BEC	C_UV + 1
	BCA	C_UV + 1
	BMD	C_UV + 1
	.endif
	QDOWNG	\name, \n3
21:
	G
	.if \g
	BALL	C_COL
	BL	C_COL + 1
	BAB	C_COL + 1
	BBE	C_COL + 1
	BEC	C_COL + 1
	.else
	BL	C_UV
	BAB	C_UV
	BBE	C_UV
	BEC	C_UV
	.endif
	GAP	\n1 - 3
	QPUT012
	QLOAD34
	nop
	RTPT
	G
	.if \g
	BCA	C_COL + 1
	BMD	C_COL + 1
	BALL	C_COL + 2
	.if \t
	BALL	C_UV
	BL	C_UV + 1
	BAB	C_UV + 1
	BBE	C_UV + 1
	.endif
	.else
	BCA	C_UV
	BMD	C_UV
	BALL	C_UV + 1
	.endif
	GAP	\n2 - 5 * (1 - \t * \g)
	swc2	$12, QBE + C_SXY($a0)
	swc2	$13, QEC + C_SXY($a0)
	.if \t && \g
	swc2	$17, QBE + C_SZ($a0)
	swc2	$18, QEC + C_SZ($a0)
	.endif
	G
	.if \t && \g
	BEC	C_UV + 1
	BCA	C_UV + 1
	BMD	C_UV + 1
	.endif
	LEAFHEAD \t, \g, 4
	QTAIL	\name
	QEND	\name, \leaf
	.endm

	.set F_A, 120
	.set F_B, 124
	.set F_C, 128
	.set F_E, 132
	.set F_RET, 136
	.set F_SIZE, 140
	CUTF4	f4, 110
	CUTQ	ft4, 1, 0, 18, 32, 6, 166
	CUTQ	g4, 0, 1, 37, 32, 6, 142
	CUTQ	gt4, 1, 1, 37, 56, 20, 207

# What a head does: a kind's way in, its polygon as far as the cut, and its way out. Its parts take
# a kind's places from PSET. light is 0 for an unlit body, 1 lit, 2 fog and 3 light off.
#
# CHSTART is a run's start and CHNEXT a polygon's. x is the instructions of CHNEXT, and s those from
# the jump to the run's start to the first polygon.
	.macro CHTOPF q
	lw	$t7, CS_SCR($t6)
	nop
	.if \q
	addiu	$t8, $t7, CS_ROOT
	sw	$t8, CS_ROOT + 4 * C_SIZE + 120($t7)
	addiu	$t8, $t7, CS_ROOT + C_SIZE
	sw	$t8, CS_ROOT + 4 * C_SIZE + 124($t7)
	addiu	$t8, $t7, CS_ROOT + 2 * C_SIZE
	sw	$t8, CS_ROOT + 4 * C_SIZE + 128($t7)
	addiu	$t8, $t7, CS_ROOT + 3 * C_SIZE
	sw	$t8, CS_ROOT + 4 * C_SIZE + 132($t7)
	.else
	addiu	$t8, $t7, CS_ROOT
	sw	$t8, CS_ROOT + 3 * C_SIZE + 72($t7)
	addiu	$t8, $t7, CS_ROOT + C_SIZE
	sw	$t8, CS_ROOT + 3 * C_SIZE + 76($t7)
	addiu	$t8, $t7, CS_ROOT + 2 * C_SIZE
	sw	$t8, CS_ROOT + 3 * C_SIZE + 80($t7)
	.endif
	.endm

	.macro CHSTART name, light, t, s, x, dbl, q
	.section .text.GsSortCut.c\name, "ax", @progbits
	.align 2
	.type gso_c\name, @function
gso_c\name\():
	G
	.ifnb \dbl
	srl	$at, $t6, 17
	andi	$at, $at, 1
	beqz	$at, 3f
	lui	$at, %hi(gso_c\dbl)
	addiu	$at, $at, %lo(gso_c\dbl)
	jr	$at
	nop
3:	G
	.endif
	# The code: a textured polygon has no use for the unlit bit, and its color is fixed.
	srl	$t6, $t6, 24
	.if \t && (\light == 0)
	andi	$t6, $t6, 0xfe
	.endif
	sll	$t6, $t6, 24
	or	$a3, $a3, $t6
	lui	$t6, %hi(gso_cut)
	addiu	$t6, $t6, %lo(gso_cut)
	sw	$a3, CS_A3($t6)
	sw	$t0, CS_T0($t6)
	sw	$t1, CS_T1($t6)
	sw	$t2, CS_T2($t6)
	.if \light == 0 || \light == 3
	move	$v1, $t6
	.endif
	sw	$v1, CS_V1($t6)
	.if \t
	lui	$at, 0xfd00
	and	$at, $at, $a3
	lui	$t7, 0x80
	ori	$t7, $t7, 0x8080
	or	$at, $at, $t7
	sw	$at, CS_RGBC($t6)
	.endif
	CHTOPF	0\q
	.endm

# CHNEXT finds a polygon's vertices and for a lit triangle its normals, in t7 to v0, and copies its
# points. Lit quads find their normals with the last polygon's end.
	.macro CHNORM g
	lhu	$t7, P_N0($a0)
	.if \g
	lhu	$t8, P_N1($a0)
	lhu	$v0, P_N2($a0)
	.else
	nop
	.endif
	sll	$t7, $t7, 3
	.if \g
	sll	$t8, $t8, 3
	sll	$v0, $v0, 3
	.endif
	addu	$t7, $t7, $v1
	.if \g
	addu	$t8, $t8, $v1
	addu	$v0, $v0, $v1
	.endif
	.endm

	.macro COPYV r, i
	lw	$at, 0(\r)
	lw	$ra, 4(\r)
	sw	$at, CS_ROOT + \i * C_SIZE($t0)
	sw	$ra, CS_ROOT + \i * C_SIZE + 4($t0)
	.endm

	.macro CHNEXT light, t, g, q, x
7:	G
	.if \q
	lhu	$t9, P_V3($a0)
	.endif
	PREFETCH P_V0, P_V1, P_V2
	.if \q
	sll	$t9, $t9, 3
	addu	$t9, $t9, $t0
	.endif
	.if \q == 0 && (\light == 1 || \light == 2)
	CHNORM	\g
	.endif
	.if \light == 0 || \light == 3
	lw	$t0, CS_SCR($v1)
	.else
	lui	$t6, %hi(gso_cut)
	addiu	$t6, $t6, %lo(gso_cut)
	lw	$t0, CS_SCR($t6)
	.endif
	COPYV	$t3, 0
	COPYV	$t4, 1
	COPYV	$t5, 2
	.if \q
	COPYV	$t9, 3
	.endif
	.if \t == 0 && (\light == 1 || \light == 2)
	lw	$at, P_COL($a0)
	nop
	sll	$at, $at, 8
	srl	$at, $at, 8
	or	$at, $at, $a3
	sw	$at, CS_RGBC($t6)
	.endif
	GAPS	\x
	.endm

# CHTOP is a polygon's first points, FLAG, NCLIP and the way out at label 6 for one turned away.
	.macro CHTOP
1:	lwc2	$0, 0($t3)
	lwc2	$1, 4($t3)
	lwc2	$2, 0($t4)
	lwc2	$3, 4($t4)
	lwc2	$4, 0($t5)
	lwc2	$5, 4($t5)
	nop
	RTPT
	nop
	CFLAG
	nop
	nop
	NCLIP
	lui	$v1, %hi(gso_cut)
	addiu	$v1, $v1, %lo(gso_cut)
	nop
	mfc2	$t6, $24
	nop
	blez	$t6, 6f
	nop
	.endm

# CHZ takes the depth in t6 to the entry of the ordering table, and a3 to what it held.
	.macro CHZ
	nop
	srav	$t6, $t6, $t2
	sll	$t6, $t6, 2
	addu	$t6, $t6, $t1
	sw	$t6, CS_TAG($v1)
	lw	$a3, 0($t6)
	.endm

# CHPOLY takes the polygon's texture coordinates, and color if flat, to where the cut wants them.
	.macro CHPOLY light, t, g, q
	sll	$a3, $a3, 8
	srl	$a3, $a3, 8
	.if \t
	lw	$t6, 4($a0)
	lw	$at, 8($a0)
	sw	$t6, CS_ROOT + C_UV($t0)
	sw	$at, CS_ROOT + C_SIZE + C_UV($t0)
	srl	$t6, $t6, 16
	srl	$at, $at, 16
	sll	$t7, $at, 16
	or	$t6, $t6, $t7
	sw	$t6, W_UV($t0)
	sll	$t6, $t6, 16
	sw	$t6, CS_UV0($v1)
	sw	$t7, CS_UV1($v1)
	lw	$t6, 12($a0)
	.if \q
	lw	$at, 16($a0)
	.else
	nop
	.endif
	sw	$t6, CS_ROOT + 2 * C_SIZE + C_UV($t0)
	.if \q
	sw	$at, CS_ROOT + 3 * C_SIZE + C_UV($t0)
	.endif
	.endif
	lbu	$t7, CS_A3 + 3($v1)
	lw	$at, CS_W5($v1)
	nop
	sw	$at, W_END($t0)
	.if \g
	# Every corner's color word has the code on top.
	.if \light == 0
	lw	$t6, P_COL($a0)
	lw	$at, P_COL + 4($a0)
	sw	$t6, CS_ROOT + C_COL($t0)
	sw	$at, CS_ROOT + C_SIZE + C_COL($t0)
	lw	$t6, P_COL + 8($a0)
	.if \q
	lw	$at, P_COL + 12($a0)
	.else
	nop
	.endif
	sw	$t6, CS_ROOT + 2 * C_SIZE + C_COL($t0)
	.if \q
	sw	$at, CS_ROOT + 3 * C_SIZE + C_COL($t0)
	.endif
	.elseif \light == 3
	.if \t
	lw	$t6, CS_RGBC($v1)
	nop
	.else
	lw	$t6, P_COL($a0)
	nop
	.endif
	sw	$t6, CS_ROOT + C_COL($t0)
	sw	$t6, CS_ROOT + C_SIZE + C_COL($t0)
	sw	$t6, CS_ROOT + 2 * C_SIZE + C_COL($t0)
	.if \q
	sw	$t6, CS_ROOT + 3 * C_SIZE + C_COL($t0)
	.endif
	.endif
	sb	$t7, W_COL + 3($t0)
	.else
	# A flat polygon has its color in one place.
	.if \light == 0
	lw	$t6, P_COL($a0)
	nop
	sw	$t6, W_COL($t0)
	sb	$t7, W_COL + 3($t0)
	lw	$t6, W_COL($t0)
	.elseif \light == 3
	.if \t
	lw	$t6, CS_RGBC($v1)
	nop
	.else
	lw	$t6, P_COL($a0)
	nop
	.endif
	sw	$t6, W_COL($t0)
	sb	$t7, W_COL + 3($t0)
	lw	$t6, W_COL($t0)
	.else
	sb	$t7, W_COL + 3($t0)
	lw	$t6, W_COL($t0)
	.endif
	nop
	sw	$t6, CS_COL($v1)
	.endif
	sw	$a0, CS_A0($v1)
	sw	$a1, CS_A1($v1)
	.endm

# CHCALL goes to the cut with the polygon's corners as the piece to cut.
	.macro CHCALL name, q, p
	addiu	$t3, $t0, CS_ROOT
	addiu	$t4, $t0, CS_ROOT + C_SIZE
	addiu	$t5, $t0, CS_ROOT + 2 * C_SIZE
	.if \q
	addiu	$t9, $t0, CS_ROOT + 3 * C_SIZE
	addiu	$a0, $t0, CS_ROOT + 4 * C_SIZE
	lw	$at, FQ_RET($a0)
	.else
	addiu	$a0, $t0, CS_ROOT + 3 * C_SIZE
	lw	$at, FT_RET($a0)
	.endif
	.if \q
	lw	$a1, CS_DEEP4($v1)
	.else
	lw	$a1, CS_DEEP3($v1)
	.endif
	sw	$at, CS_KEEP($v1)
	GAPS	\p - 2
	.if \q
	jal	cut_\name
	sw	$ra, FQ_RET($a0)
	.else
	jal	cut_\name
	sw	$ra, FT_RET($a0)
	.endif
	.endm

# CHTAIL is what follows the cut: the entry gets its packets, and the head its registers back.
# tl is the instructions from the cut's return to the end of GsSortObject4, and a polygon turned
# away takes back more, its first back being the back's instructions after mf24. l0 has it load V0.
	.macro CHTAIL light, t, g, q, tl, x, back, l0
8:	G
	lw	$at, CS_KEEP($v1)
	.if \q
	.set Q_F, FQ_SIZE + FQ_RET
	.else
	.set Q_F, FT_SIZE + FT_RET
	.endif
	lw	$t6, CS_TAG($v1)
	sw	$at, Q_F($a0)
	lui	$at, 0xff00
	lw	$t7, 0($t6)
	lw	$a0, CS_A0($v1)
	and	$t7, $t7, $at
	or	$t7, $t7, $a3
	sw	$t7, 0($t6)
	lw	$a1, CS_A1($v1)
	lw	$ra, CS_RA($v1)
	lw	$t0, CS_T0($v1)
	lw	$t1, CS_T1($v1)
	lw	$t2, CS_T2($v1)
	.if \t == 0
	lw	$a3, CS_A3($v1)
	.endif
	lw	$v1, CS_V1($v1)
5:	addiu	$a1, $a1, -1
	addiu	$a0, $a0, P_SIZE
	.if \q && (\light == 1 || \light == 2)
	CHNORM	\g
	.endif
	GAPS	\tl - 16
	bnez	$a1, 7b
	nop
	j	gso_run
	nop
6:	lw	$ra, CS_RA($v1)
	.if \l0
	lwc2	$0, 0($t3)
	.endif
	lw	$t0, CS_T0($v1)
	lw	$v1, CS_V1($v1)
	.if ((\back) / 2 - (\tl) + 6 - (\t) - (\l0)) > 8
	G
	GAPS	(\back) / 2 - (\tl) + 6 - (\t) - (\l0)
	.else
	NOPS	(\back) / 2 - (\tl) + 6 - (\t) - (\l0)
	.endif
	b	5b
	nop
	.endm

	.macro CHLIT3 name, kind, light, t, g, s, x, k, p, tl, back, dbl
	PSET	\light, \t, \g, 0, 0
	CHSTART	\name, \light, \t, \s, \x, \dbl
	GAPS	\s - \x + 1
	CHNEXT	\light, \t, \g, 0, \x
	CHTOP
	lwc2	$0, 0($t7)
	lwc2	$1, 4($t7)
	.if \g
	lwc2	$2, 0($t8)
	lwc2	$3, 4($t8)
	lwc2	$4, 0($v0)
	lwc2	$5, 4($v0)
	.else
	lwc2	$2, 0($t7)
	lwc2	$3, 4($t7)
	lwc2	$4, 0($t7)
	lwc2	$5, 4($t7)
	.endif
	lwc2	$6, CS_RGBC($v1)
	nop
	LIGHTTHREE \light
	G
	GAP	6
	swc2	$12, CS_ROOT + C_SXY($t0)
	swc2	$13, CS_ROOT + C_SIZE + C_SXY($t0)
	swc2	$14, CS_ROOT + 2 * C_SIZE + C_SXY($t0)
	.if \g
	swc2	$20, CS_ROOT + C_COL($t0)
	.else
	swc2	$20, W_COL($t0)
	.endif
	swc2	$21, CS_ROOT + C_SIZE + C_COL($t0)
	swc2	$22, CS_ROOT + 2 * C_SIZE + C_COL($t0)
	nop
	AVSZ3
	nop
	mfc2	$t6, $7
	G
	CHZ
	GAP	\k - 1
	swc2	$17, CS_ROOT + C_SZ($t0)
	swc2	$18, CS_ROOT + C_SIZE + C_SZ($t0)
	swc2	$19, CS_ROOT + 2 * C_SIZE + C_SZ($t0)
	G
	CHPOLY	\light, \t, \g, 0
	CHCALL	\kind, 0, \p
	CHTAIL	\light, \t, \g, 0, \tl, \x, \back, 1
	.endm

	.macro CHPLAIN3 name, kind, light, t, g, s, x, k, p, tl, back, dbl
	PSET	\light, \t, \g, 0, 0
	CHSTART	\name, \light, \t, \s, \x, \dbl
	GAPS	\s - \x + 1
	CHNEXT	\light, \t, \g, 0, \x
	CHTOP
	swc2	$12, CS_ROOT + C_SXY($t0)
	swc2	$13, CS_ROOT + C_SIZE + C_SXY($t0)
	swc2	$14, CS_ROOT + 2 * C_SIZE + C_SXY($t0)
	swc2	$8, CS_IR0($v1)
	nop
	AVSZ3
	nop
	mfc2	$t6, $7
	G
	CHZ
	GAP	\k - 1
	swc2	$17, CS_ROOT + C_SZ($t0)
	swc2	$18, CS_ROOT + C_SIZE + C_SZ($t0)
	swc2	$19, CS_ROOT + 2 * C_SIZE + C_SZ($t0)
	G
	CHPOLY	\light, \t, \g, 0
	CHCALL	\kind, 0, \p
	CHTAIL	\light, \t, \g, 0, \tl, \x, \back, 0
	.endm

# CHQUAD is a quad's way as far as its depth, with its fourth point.
	.macro CHQUAD k
	swc2	$12, CS_ROOT + C_SXY($t0)
	swc2	$13, CS_ROOT + C_SIZE + C_SXY($t0)
	swc2	$14, CS_ROOT + 2 * C_SIZE + C_SXY($t0)
	lwc2	$0, 0($t9)
	lwc2	$1, 4($t9)
	nop
	RTPS
	G
	GAP	3
	swc2	$14, CS_ROOT + 3 * C_SIZE + C_SXY($t0)
	CFLAG
	swc2	$8, CS_IR0($v1)
	nop
	nop
	AVSZ4
	nop
	mfc2	$t6, $7
	G
	CHZ
	GAP	\k - 2
	swc2	$16, CS_ROOT + C_SZ($t0)
	swc2	$17, CS_ROOT + C_SIZE + C_SZ($t0)
	swc2	$18, CS_ROOT + 2 * C_SIZE + C_SZ($t0)
	swc2	$19, CS_ROOT + 3 * C_SIZE + C_SZ($t0)
	.endm

	.macro CHPLAIN4 name, kind, light, t, g, s, x, k, p, tl, back
	PSET	\light, \t, \g, 1, 0
	CHSTART	\name, \light, \t, \s, \x, , 1
	GAPS	\s - \x + 1
	CHNEXT	\light, \t, \g, 1, \x
	CHTOP
	CHQUAD	\k
	G
	CHPOLY	\light, \t, \g, 1
	CHCALL	\kind, 1, \p
	CHTAIL	\light, \t, \g, 1, \tl, \x, \back, 0
	.endm

# A lit flat quad is lit after its depth.
	.macro CHLITF4 name, kind, light, t, s, x, k, p1, p, tl, back
	PSET	\light, \t, 0, 1, 0
	CHSTART	\name, \light, \t, \s, \x, , 1
	CHNORM	0
	GAPS	\s - \x + 1
	CHNEXT	\light, \t, 0, 1, \x
	CHTOP
	CHQUAD	\k
	G
	.if \light == 2
	lw	$at, CS_IR0($v1)
	.endif
	GAP	\p1
	lwc2	$0, 0($t7)
	lwc2	$1, 4($t7)
	lwc2	$6, CS_RGBC($v1)
	.if \light == 2
	mtc2	$at, $8
	.endif
	nop
	LIGHTONE \light
	swc2	$22, W_COL($t0)
	G
	CHPOLY	\light, \t, 0, 1
	CHCALL	\kind, 1, \p
	CHTAIL	\light, \t, 0, 1, \tl, \x, \back, 0
	.endm

# A lit gouraud quad has three corners by NCCT and the fourth by NCCS.
	.macro CHLITG4 name, kind, light, t, s, x, k, p1, n4, p, tl, back
	PSET	\light, \t, 1, 1, 0
	CHSTART	\name, \light, \t, \s, \x, , 1
	CHNORM	1
	GAPS	\s - \x + 1
	CHNEXT	\light, \t, 1, 1, \x
	CHTOP
	CHQUAD	\k
	G
	GAP	\p1
	lwc2	$0, 0($t7)
	lwc2	$1, 4($t7)
	lwc2	$2, 0($t8)
	lwc2	$3, 4($t8)
	lwc2	$4, 0($v0)
	lwc2	$5, 4($v0)
	lwc2	$6, CS_RGBC($v1)
	nop
	LIGHTTHREE \light
	G
	lhu	$t7, P_N3($a0)
	GAP	3
	swc2	$20, CS_ROOT + C_COL($t0)
	swc2	$21, CS_ROOT + C_SIZE + C_COL($t0)
	swc2	$22, CS_ROOT + 2 * C_SIZE + C_COL($t0)
	G
	sll	$t7, $t7, 3
	lw	$t8, CS_V1($v1)
	nop
	addu	$t7, $t7, $t8
	GAP	8
	lwc2	$0, 0($t7)
	lwc2	$1, 4($t7)
	lwc2	$6, CS_RGBC($v1)
	nop
	LIGHTONE \light
	swc2	$22, CS_ROOT + 3 * C_SIZE + C_COL($t0)
	G
	CHPOLY	\light, \t, 1, 1
	CHCALL	\kind, 1, \p
	CHTAIL	\light, \t, 1, 1, \tl, \x, \back, 0
	.endm

# In fog a gouraud polygon has each point and each corner's color by itself, each with
# its own IR0, as RTPT and NCDT have only one.
	.macro CHFOGTOP
1:	lwc2	$0, 0($t3)
	lwc2	$1, 4($t3)
	nop
	RTPS
	mfc2	$at, $8
	lwc2	$0, 0($t4)
	lwc2	$1, 4($t4)
	nop
	RTPS
	mfc2	$ra, $8
	lwc2	$0, 0($t5)
	lwc2	$1, 4($t5)
	nop
	RTPS
	nop
	mfc2	$t9, $8
	CFLAG
	nop
	nop
	NCLIP
	lui	$v1, %hi(gso_cut)
	addiu	$v1, $v1, %lo(gso_cut)
	nop
	mfc2	$t6, $24
	nop
	blez	$t6, 6f
	nop
	lwc2	$0, 0($t7)
	lwc2	$1, 4($t7)
	lwc2	$6, CS_RGBC($v1)
	mtc2	$at, $8
	nop
	LIGHTONE 2
	nop
	nop
	swc2	$12, CS_ROOT + C_SXY($t0)
	swc2	$22, CS_ROOT + C_COL($t0)
	lwc2	$0, 0($t8)
	lwc2	$1, 4($t8)
	mtc2	$ra, $8
	nop
	LIGHTONE 2
	nop
	nop
	swc2	$13, CS_ROOT + C_SIZE + C_SXY($t0)
	swc2	$22, CS_ROOT + C_SIZE + C_COL($t0)
	lwc2	$0, 0($v0)
	lwc2	$1, 4($v0)
	mtc2	$t9, $8
	nop
	LIGHTONE 2
	nop
	nop
	swc2	$14, CS_ROOT + 2 * C_SIZE + C_SXY($t0)
	swc2	$22, CS_ROOT + 2 * C_SIZE + C_COL($t0)
	AVSZ3
	nop
	mfc2	$t6, $7
	.endm

	.macro CHFOG3 name, kind, t, s, x, k, p, tl, back
	PSET	2, \t, 1, 0, 0
	CHSTART	\name, 2, \t, \s, \x
	GAPS	\s - \x + 1
	CHNEXT	2, \t, 1, 0, \x
	CHFOGTOP
	G
	CHZ
	GAP	\k - 1
	swc2	$17, CS_ROOT + C_SZ($t0)
	swc2	$18, CS_ROOT + C_SIZE + C_SZ($t0)
	swc2	$19, CS_ROOT + 2 * C_SIZE + C_SZ($t0)
	G
	CHPOLY	2, \t, 1, 0
	CHCALL	\kind, 0, \p
	CHTAIL	2, \t, 1, 0, \tl, \x, \back, 0
	.endm

	.macro CHFOG4 name, kind, t, s, x, k, k2, p1, p, tl, back
	PSET	2, \t, 1, 1, 0
	CHSTART	\name, 2, \t, \s, \x, , 1
	CHNORM	1
	GAPS	\s - \x + 1
	CHNEXT	2, \t, 1, 1, \x
	CHFOGTOP
	G
	lhu	$t9, P_V3($a0)
	move	$v0, $t6
	lhu	$t7, P_N3($a0)
	lw	$t8, CS_V1($v1)
	lw	$at, CS_T0($v1)
	sll	$t9, $t9, 3
	addu	$t9, $t9, $at
	sll	$t7, $t7, 3
	addu	$t7, $t7, $t8
	GAP	\k - 1
	lwc2	$0, 0($t9)
	lwc2	$1, 4($t9)
	nop
	RTPS
	swc2	$14, CS_ROOT + 3 * C_SIZE + C_SXY($t0)
	swc2	$8, CS_IR0($v1)
	CFLAG
	mfc2	$t8, $19
	G
	move	$t6, $v0
	srav	$t6, $t6, $t2
	sll	$t6, $t6, 2
	addu	$t6, $t6, $t1
	sw	$t6, CS_TAG($v1)
	GAP	\k2
	swc2	$16, CS_ROOT + C_SZ($t0)
	swc2	$17, CS_ROOT + C_SIZE + C_SZ($t0)
	swc2	$18, CS_ROOT + 2 * C_SIZE + C_SZ($t0)
	swc2	$19, CS_ROOT + 3 * C_SIZE + C_SZ($t0)
	G
	lw	$a3, 0($t6)
	lw	$t6, CS_IR0($v1)
	GAP	\p1
	lwc2	$0, 0($t7)
	lwc2	$1, 4($t7)
	lwc2	$6, CS_RGBC($v1)
	mtc2	$t6, $8
	nop
	LIGHTONE 2
	swc2	$22, CS_ROOT + 3 * C_SIZE + C_COL($t0)
	G
	CHPOLY	2, \t, 1, 1
	CHCALL	\kind, 1, \p
	CHTAIL	2, \t, 1, 1, \tl, \x, \back, 0
	.endm

# A textured flat triangle with flag bit 1 is drawn whichever way it faces: no NCLIP.
	.macro CHDOUBLE name, kind, light, s, x, k, k2, p, tl
	PSET	\light, 1, 0, 0, 0
	.section .text.GsSortCut.c\name, "ax", @progbits
	.align 2
gso_c\name\():
	G
	srl	$t6, $t6, 24
	.if \light == 0
	andi	$t6, $t6, 0xfe
	.endif
	sll	$t6, $t6, 24
	or	$a3, $a3, $t6
	lui	$t6, %hi(gso_cut)
	addiu	$t6, $t6, %lo(gso_cut)
	sw	$a3, CS_A3($t6)
	sw	$t0, CS_T0($t6)
	sw	$t1, CS_T1($t6)
	sw	$t2, CS_T2($t6)
	.if \light == 0 || \light == 3
	move	$v1, $t6
	.endif
	sw	$v1, CS_V1($t6)
	lui	$at, 0xfd00
	and	$at, $at, $a3
	lui	$t7, 0x80
	ori	$t7, $t7, 0x8080
	or	$at, $at, $t7
	sw	$at, CS_RGBC($t6)
	CHTOPF	0
	GAPS	\s - \x + 1
	CHNEXT	\light, 1, 0, 0, \x
	lwc2	$0, 0($t3)
	lwc2	$1, 4($t3)
	lwc2	$2, 0($t4)
	lwc2	$3, 4($t4)
	lwc2	$4, 0($t5)
	lwc2	$5, 4($t5)
	nop
	RTPT
	lui	$v1, %hi(gso_cut)
	addiu	$v1, $v1, %lo(gso_cut)
	nop
	nop
	swc2	$12, CS_ROOT + C_SXY($t0)
	swc2	$13, CS_ROOT + C_SIZE + C_SXY($t0)
	swc2	$14, CS_ROOT + 2 * C_SIZE + C_SXY($t0)
	CFLAG
	swc2	$8, CS_IR0($v1)
	nop
	AVSZ3
	mfc2	$t6, $7
	G
	CHZ
	.if \light == 1 || \light == 2
	GAP	\k
	lwc2	$0, 0($t7)
	lwc2	$1, 4($t7)
	lwc2	$2, 0($t7)
	lwc2	$3, 4($t7)
	lwc2	$4, 0($t7)
	lwc2	$5, 4($t7)
	lwc2	$6, CS_RGBC($v1)
	.if \light == 2
	lwc2	$8, CS_IR0($v1)
	.endif
	nop
	LIGHTTHREE \light
	G
	GAP	3
	swc2	$20, W_COL($t0)
	swc2	$21, CS_ROOT + C_SIZE + C_COL($t0)
	swc2	$22, CS_ROOT + 2 * C_SIZE + C_COL($t0)
	G
	GAP	\k2
	.else
	GAP	\k
	.endif
	swc2	$17, CS_ROOT + C_SZ($t0)
	swc2	$18, CS_ROOT + C_SIZE + C_SZ($t0)
	swc2	$19, CS_ROOT + 2 * C_SIZE + C_SZ($t0)
	G
	CHPOLY	\light, 1, 0, 0
	CHCALL	\kind, 0, \p
	CHTAIL	\light, 1, 0, 0, \tl, \x, 2 * (\tl - 5), 0
	.endm

	CHPLAIN3	f3, f3, 0, 0, 0, 72, 25, 11, 29, 39, 102
	CHLIT3	lf3, f3, 1, 0, 0, 101, 42, 11, 27, 41, 106
	CHLIT3	ff3, f3, 2, 0, 0, 101, 42, 11, 27, 41, 106
	CHPLAIN3	of3, f3, 3, 0, 0, 82, 25, 11, 27, 41, 106
	CHDOUBLE	dft3, ft3, 0, 80, 28, 8, 0, 39, 44
	CHDOUBLE	dlft3, ft3, 1, 80, 28, 16, 6, 40, 43
	CHDOUBLE	dfft3, ft3, 2, 80, 28, 18, 6, 40, 43
	CHDOUBLE	doft3, ft3, 3, 79, 28, 8, 0, 39, 43
	CHPLAIN3	ft3, ft3, 0, 1, 0, 75, 29, 12, 41, 44, 112, dft3
	CHLIT3	lft3, ft3, 1, 1, 0, 100, 45, 12, 40, 43, 110, dlft3
	CHLIT3	fft3, ft3, 2, 1, 0, 100, 45, 12, 40, 43, 110, dfft3
	CHPLAIN3	oft3, ft3, 3, 1, 0, 84, 30, 12, 39, 43, 110, doft3
	CHPLAIN3	g3, g3, 0, 0, 1, 71, 23, 11, 34, 41, 106
	CHLIT3	lg3, g3, 1, 0, 1, 103, 42, 11, 27, 44, 112
	CHFOG3	fg3, g3, 0, 103, 42, 11, 27, 44, 112
	CHPLAIN3	og3, g3, 3, 0, 1, 81, 22, 11, 31, 44, 112
	CHPLAIN3	gt3, gt3, 0, 1, 1, 71, 22, 11, 50, 42, 108
	CHLIT3	lgt3, gt3, 1, 1, 1, 100, 41, 11, 40, 44, 112
	CHFOG3	fgt3, gt3, 1, 100, 41, 11, 40, 44, 112
	CHPLAIN3	ogt3, gt3, 3, 1, 1, 79, 22, 11, 43, 44, 112
	CHPLAIN4	f4, f4, 0, 0, 0, 88, 40, 13, 30, 40, 106
	CHLITF4	lf4, f4, 1, 0, 96, 37, 13, 10, 28, 47, 120
	CHLITF4	ff4, f4, 2, 0, 96, 37, 13, 11, 28, 47, 120
	CHPLAIN4	of4, f4, 3, 0, 0, 94, 37, 13, 30, 43, 112
	CHPLAIN4	ft4, ft4, 0, 1, 0, 88, 40, 13, 48, 40, 106
	CHLITF4	lft4, ft4, 1, 1, 99, 40, 13, 9, 44, 40, 106
	CHLITF4	fft4, ft4, 2, 1, 99, 40, 13, 10, 44, 40, 106
	CHPLAIN4	oft4, ft4, 3, 1, 0, 98, 40, 13, 45, 40, 106
	CHPLAIN4	g4, g4, 0, 0, 1, 88, 37, 13, 38, 43, 112
	CHLITG4	lg4, g4, 1, 0, 96, 38, 12, 21, 16, 27, 47, 120
	CHFOG4	fg4, g4, 0, 115, 55, 11, 8, 9, 27, 47, 120
	CHPLAIN4	og4, g4, 3, 0, 1, 95, 38, 13, 38, 42, 110
	CHPLAIN4	gt4, gt4, 0, 1, 1, 88, 36, 13, 57, 44, 114
	CHLITG4	lgt4, gt4, 1, 1, 100, 37, 13, 19, 16, 45, 43, 112
	CHFOG4	fgt4, gt4, 1, 118, 53, 12, 8, 9, 44, 43, 112
	CHPLAIN4	ogt4, gt4, 3, 1, 1, 96, 37, 13, 48, 43, 112

# The tables, as GsSortObject4 has for what is not cut.
	.section .text.GsSortCut.tables, "ax", @progbits
	.align 2
gso_cunlit:
	.word gso_cf3, gso_cft3, gso_cf4, gso_cft4, gso_cg3, gso_cgt3, gso_cg4, gso_cgt4
gso_clit:
	.word gso_clf3, gso_clft3, gso_clf4, gso_clft4, gso_clg3, gso_clgt3, gso_clg4, gso_clgt4
	.word gso_lgr20, gso_clft3, gso_clf4, gso_clft4, gso_lgr30, gso_clgt3, gso_clg4, gso_clgt4
gso_cfog:
	.word gso_cff3, gso_cfft3, gso_cff4, gso_cfft4, gso_cfg3, gso_cfgt3, gso_cfg4, gso_cfgt4
	.word gso_fgr20, gso_cfft3, gso_cff4, gso_cfft4, gso_fgr30, gso_cfgt3, gso_cfg4, gso_cfgt4
gso_coff:
	.word gso_cof3, gso_coft3, gso_cof4, gso_coft4, gso_cog3, gso_cogt3, gso_cog4, gso_cogt4
	.word gso_ogr20, gso_coft3, gso_cof4, gso_coft4, gso_ogr30, gso_cogt3, gso_cog4, gso_cogt4

# libps_va_sprintf(args) and libps_va_printf(args): formatters A and B of cformat.go's comment,
# without doubles, each path at Sony's cycles (spec 31). sprintf calls nothing, keeping everything in
# registers, as a program's stack may be small.
	.set SF_ENTRY, 5
	.set SF_END, 2
	.set SF_DIR, 0
	.set SF_FLAG, 0
	.set SF_WID, 0
	.set SF_PCT, 0
	.set SF_C, 21
	.set SF_END10, 0
	.set SF_DIG16, 1
	.set SF_FD, 0
	.set SF_FH, 0
	.set SF_SLOW, 8

	.section .text.libps_sprintf, "ax", @progbits
	.align 2
libps_hexl:
	.ascii "0123456789ABCDEF0123456789abcdef"
libps_pow10:
	.word 1, 10, 100, 1000, 10000, 100000, 1000000, 10000000, 100000000, 1000000000

	.globl libps_va_sprintf
	.type libps_va_sprintf, @function
libps_va_sprintf:
	lw	$a3, 0($a0)
	lw	$a1, 4($a0)
	addiu	$a2, $a0, 8
	move	$a0, $a3
	NOPS	SF_ENTRY
	lbu	$v0, 0($a1)
	nop
	beqz	$v0, 1f
	nop
	NOPS	2
	b	1f
	nop
sf_more:
	NOPS	4
1:	lbu	$v0, 0($a1)
	addiu	$a1, $a1, 1
	addiu	$at, $v0, -37
	beqz	$at, 2f
	sb	$v0, 0($a3)
	beqz	$v0, 90f
	nop
	b	sf_more
	addiu	$a3, $a3, 1
	# The directive: its flags,
2:	NOPS	SF_DIR
	move	$t0, $zero
3:	lbu	$v0, 0($a1)
	nop
	addiu	$at, $v0, -45
	beqz	$at, 4f
	li	$t9, 1
	addiu	$at, $v0, -43
	beqz	$at, 4f
	li	$t9, 2
	addiu	$at, $v0, -32
	beqz	$at, 4f
	li	$t9, 4
	addiu	$at, $v0, -35
	beqz	$at, 4f
	li	$t9, 8
	addiu	$at, $v0, -48
	bnez	$at, 5f
	li	$t9, 16
4:	or	$t0, $t0, $t9
	NOPS	SF_FLAG
	b	3b
	addiu	$a1, $a1, 1
	# its width,
5:	addiu	$at, $v0, -42
	bnez	$at, 6f
	move	$t1, $zero
	lw	$t1, 0($a2)
	addiu	$a1, $a1, 1
	addiu	$a2, $a2, 4
	bgez	$t1, 8f
	nop
	ori	$t0, $t0, 1
	b	8f
	subu	$t1, $zero, $t1
6:	addiu	$at, $v0, -48
	sltiu	$t9, $at, 10
	beqz	$t9, 8f
	sll	$t9, $t1, 2
	addu	$t1, $t9, $t1
	sll	$t1, $t1, 1
	addu	$t1, $t1, $at
	addiu	$a1, $a1, 1
	lbu	$v0, 0($a1)
	NOPS	SF_WID
	b	6b
	nop
	# its precision,
8:	lbu	$v0, 0($a1)
	li	$t2, -1
	addiu	$at, $v0, -46
	bnez	$at, 10f
	nop
	lbu	$v0, 1($a1)
	addiu	$a1, $a1, 1
	addiu	$at, $v0, -42
	bnez	$at, 9f
	move	$t2, $zero
	lw	$t2, 0($a2)
	addiu	$a1, $a1, 1
	addiu	$a2, $a2, 4
	bgez	$t2, 10f
	nop
	b	10f
	li	$t2, -1
9:	addiu	$at, $v0, -48
	sltiu	$t9, $at, 10
	beqz	$t9, 10f
	sll	$t9, $t2, 2
	addu	$t2, $t9, $t2
	sll	$t2, $t2, 1
	addu	$t2, $t2, $at
	addiu	$a1, $a1, 1
	lbu	$v0, 0($a1)
	nop
	b	9b
	nop
	# its h, l and L, and what it converts.
10:	lbu	$t9, 0($a1)
	nop
	addiu	$at, $t9, -104
	bnez	$at, 11f
	addiu	$at, $t9, -108
	ori	$t0, $t0, 32
	NOPS	10
	b	10b
	addiu	$a1, $a1, 1
11:	beqz	$at, 12f
	addiu	$at, $t9, -76
	bnez	$at, 13f
	nop
12:	NOPS	10
	b	10b
	addiu	$a1, $a1, 1
13:	addiu	$a1, $a1, 1
	move	$t3, $zero
	move	$t4, $zero
	move	$t5, $zero
	move	$t6, $zero
	move	$t7, $zero
	move	$t8, $zero
	addiu	$at, $t9, -37
	bnez	$at, 14f
	addiu	$at, $t9, -110
	sb	$at, 0($a3)
	li	$v0, 37
	sb	$v0, 0($a3)
	NOPS	SF_PCT
	b	1b
	addiu	$a3, $a3, 1
14:	bnez	$at, 15f
	addiu	$at, $t9, -99
	lw	$t3, 0($a2)
	subu	$v0, $a3, $a0
	andi	$t0, $t0, 32
	bnez	$t0, 145f
	addiu	$a2, $a2, 4
	b	1b
	sw	$v0, 0($t3)
145:	b	1b
	sh	$v0, 0($t3)
15:	bnez	$at, 16f
	addiu	$at, $t9, -115
	lw	$t3, 0($a2)
	li	$t7, 1
	li	$t2, -1
	addiu	$a2, $a2, 4
	NOPS	SF_C
	b	40f
	andi	$t0, $t0, 0xffef
16:	bnez	$at, 20f
	addiu	$at, $t9, -100
	lw	$t3, 0($a2)
	andi	$at, $t0, 8
	addiu	$a2, $a2, 4
	beqz	$at, 17f
	andi	$t0, $t0, 0xffef
	# A counted string, the byte at p being its length.
	lbu	$t7, 0($t3)
	addiu	$t3, $t3, 1
	bltz	$t2, 40f
	slt	$at, $t2, $t7
	beqz	$at, 40f
	nop
	b	40f
	move	$t7, $t2
17:	bnez	$t3, 18f
	nop
	bltz	$t2, 40f
	nop
	b	40f
	move	$t7, $t2
18:	NOPS	0
	beq	$t7, $t2, 40f
	addu	$at, $t3, $t7
	lbu	$at, 0($at)
	nop
	beqz	$at, 40f
	nop
	b	18b
	addiu	$t7, $t7, 1
	# A number.
20:	beqz	$at, 22f
	addiu	$at, $t9, -105
	beqz	$at, 22f
	addiu	$at, $t9, -117
	beqz	$at, 23f
	addiu	$at, $t9, -111
	beqz	$at, 24f
	addiu	$at, $t9, -120
	beqz	$at, 25f
	addiu	$at, $t9, -88
	beqz	$at, 26f
	addiu	$at, $t9, -112
	beqz	$at, 26f
	nop
	b	90f
	nop
22:	lw	$t3, 0($a2)
	andi	$at, $t0, 32
	beqz	$at, 221f
	addiu	$a2, $a2, 4
	sll	$t3, $t3, 16
	sra	$t3, $t3, 16
221:	bgez	$t3, 222f
	andi	$at, $t0, 2
	li	$t5, 45
	b	28f
	subu	$t3, $zero, $t3
222:	beqz	$at, 223f
	andi	$at, $t0, 4
	b	28f
	li	$t5, 43
223:	beqz	$at, 28f
	nop
	b	28f
	li	$t5, 32
23:	b	27f
	lw	$t3, 0($a2)
24:	lw	$t3, 0($a2)
	b	27f
	li	$t4, 3
25:	lw	$t3, 0($a2)
	b	27f
	li	$t4, 4
26:	lw	$t3, 0($a2)
	li	$t4, 4
	ori	$t0, $t0, 128
27:	andi	$at, $t0, 32
	beqz	$at, 271f
	addiu	$a2, $a2, 4
	andi	$t3, $t3, 0xffff
271:
28:
29:	addiu	$at, $t9, -112
	bnez	$at, 30f
	nop
	li	$t2, 8
	andi	$t0, $t0, 0xffef
30:	bnez	$t3, 31f
	nop
	beqz	$t2, 34f
	nop
31:	li	$t7, 1
	bnez	$t4, 33f
	move	$v0, $t3
	# Digits by the powers of ten, or by the bits a digit has.
	lui	$v1, %hi(libps_pow10 + 4)
	addiu	$v1, $v1, %lo(libps_pow10 + 4)
32:	slti	$at, $t7, 10
	beqz	$at, 34f
	nop
	lw	$at, 0($v1)
	addiu	$v1, $v1, 4
	sltu	$at, $v0, $at
	bnez	$at, 34f
	nop
	b	32b
	addiu	$t7, $t7, 1
33:	srlv	$v0, $v0, $t4
	beqz	$v0, 34f
	nop
	b	33b
	addiu	$t7, $t7, 1
	# With nothing round the digits they come straight.
34:	andi	$at, $t0, 0x1f
	or	$at, $at, $t1
	or	$at, $at, $t5
	bnez	$at, 341f
	nop
	bgez	$t2, 341f
	nop
	move	$v1, $zero
	bnez	$t4, 340f
	nop
	bnez	$t3, 3401f
	nop
	NOPS	4
	b	60f
	nop
3401:	NOPS	10
	b	60f
	nop
340:	NOPS	SF_FH
	b	60f
	nop
341:	NOPS	SF_SLOW
	andi	$at, $t0, 8
	beqz	$at, 342f
	addiu	$at, $t4, -4
	bnez	$at, 342f
	nop
	move	$t6, $t9
342:	slt	$at, $t7, $t2
	beqz	$at, 35f
	nop
	subu	$t8, $t2, $t7
35:	andi	$at, $t0, 8
	beqz	$at, 40f
	addiu	$at, $t4, -3
	bnez	$at, 40f
	nop
	bnez	$t8, 40f
	nop
	beqz	$t3, 40f
	nop
	li	$t8, 1
	# What goes round what a directive converts.
40:	andi	$at, $t0, 0x1f
	or	$at, $at, $t1
	or	$at, $at, $t5
	or	$at, $at, $t8
	bnez	$at, 401f
	nop
	b	48f
	move	$v1, $zero
401:	addu	$at, $t7, $t8
	beqz	$t5, 41f
	nop
	addiu	$at, $at, 1
41:	beqz	$t6, 42f
	nop
	addiu	$at, $at, 2
42:	subu	$v1, $t1, $at
	blez	$v1, 45f
	andi	$at, $t0, 1
	bnez	$at, 45f
	andi	$at, $t0, 16
	beqz	$at, 43f
	nop
	bgez	$t2, 43f
	nop
	addu	$t8, $t8, $v1
	b	45f
	move	$v1, $zero
43:	li	$v0, 32
44:	sb	$v0, 0($a3)
	NOPS	3
	addiu	$v1, $v1, -1
	bgtz	$v1, 44b
	addiu	$a3, $a3, 1
45:	beqz	$t5, 46f
	nop
	sb	$t5, 0($a3)
	addiu	$a3, $a3, 1
46:	beqz	$t6, 47f
	li	$v0, 48
	sb	$v0, 0($a3)
	sb	$t6, 1($a3)
	addiu	$a3, $a3, 2
47:	blez	$t8, 48f
	li	$v0, 48
	NOPS	2
	sb	$v0, 0($a3)
	addiu	$t8, $t8, -1
	b	47b
	addiu	$a3, $a3, 1
48:	addiu	$at, $t9, -99
	bnez	$at, 49f
	addiu	$at, $t9, -115
	sb	$t3, 0($a3)
	b	70f
	addiu	$a3, $a3, 1
49:	bnez	$at, 60f
	nop
	blez	$t7, 70f
	lbu	$v0, 0($t3)
	addiu	$t3, $t3, 1
	addiu	$t7, $t7, -1
	sb	$v0, 0($a3)
	b	49b + 4
	addiu	$a3, $a3, 1
	# The digits. A power of ten's by division, from the top; a shift's by the bits.
60:	blez	$t7, 70f
	addiu	$t7, $t7, -1
	bnez	$t4, 62f
	sll	$at, $t7, 2
	lui	$t8, %hi(libps_pow10)
	addiu	$t8, $t8, %lo(libps_pow10)
	addu	$t8, $t8, $at
61:	lw	$at, 0($t8)
	addiu	$t8, $t8, -4
	addiu	$t7, $t7, -1
	divu	$zero, $t3, $at
	mflo	$v0
	mfhi	$t3
	addiu	$v0, $v0, 48
	sb	$v0, 0($a3)
	bgez	$t7, 61b
	addiu	$a3, $a3, 1
	NOPS	SF_END10
	b	70f
	nop
62:	sll	$t8, $t7, 2
	addiu	$at, $t4, -4
	beqz	$at, 621f
	sll	$t6, $t4, 3
	subu	$t8, $t8, $t7
621:	addiu	$t6, $t6, -17
	andi	$at, $t0, 128
	lui	$t5, %hi(libps_hexl)
	bnez	$at, 63f
	addiu	$t5, $t5, %lo(libps_hexl)
	addiu	$t5, $t5, 16
63:	srlv	$v0, $t3, $t8
	and	$v0, $v0, $t6
	addu	$v0, $v0, $t5
	lbu	$v0, 0($v0)
	subu	$t8, $t8, $t4
	addiu	$t7, $t7, -1
	sb	$v0, 0($a3)
	NOPS	SF_DIG16
	bgez	$t7, 63b
	addiu	$a3, $a3, 1
	# The rest of the width.
70:	blez	$v1, 1b
	li	$v0, 32
	sb	$v0, 0($a3)
	addiu	$v1, $v1, -1
	b	70b
	addiu	$a3, $a3, 1
90:	sb	$zero, 0($a3)
	subu	$v0, $a3, $a0
	NOPS	SF_END
	jr	$ra
	nop
	.size libps_va_sprintf, .-libps_va_sprintf

	.set PF_ENTRY, 14
	.set PF_NUM, 0
	.set PF_RAW, 10
	.set PF_NL, 17
	.set PF_TAB, 0
	.set PC_NL, 1
	.set PC_HIGH, 1
	.set PC_PLAIN, 8
	.set PC_NLIN, 14
	.set PF_HIGH, 3
	.set PF_LIT, 14
	.set PF_FAST, 0
	.set PF_SLOW, 0
	.set PF_DIG, 3
	.set PF_C, 0
	.set PF_S, 7
	.set PF_PCT, 0
	.set PF_OTHER, 0
	.set PF_SIGN, 0
	.set PF_FILL, 0
	.set PF_END, 0

	.section .text.libps_printf, "ax", @progbits
	.align 2
	.set PL_FRAME, 192
	.set PL_SHIFT, PL_FRAME - 144
	.set PL_SIGN, PL_FRAME - 140
	.set PL_PREFIX, PL_FRAME - 136
	.set PL_CONV, PL_FRAME - 132
	.set PL_BODY, PL_FRAME - 128
	.set PL_FILL, PL_FRAME - 124
	.set PL_T0, PL_FRAME - 120
	.set PL_T1, PL_FRAME - 116
	.set PL_RA, PL_FRAME - 8
	.set PL_RA2, PL_FRAME - 112
	.set PL_S, PL_FRAME - 64

# PPUT sends the register's byte to the TTY.
	.macro PPUT r
	move	$a0, \r
	jal	libps_putint
	nop
	.endm

	.globl libps_va_printf
	.type libps_va_printf, @function
libps_va_printf:
	addiu	$sp, $sp, -PL_FRAME
	sw	$ra, PL_RA($sp)
	sw	$s0, PL_S + 0($sp)
	sw	$s1, PL_S + 4($sp)
	sw	$s2, PL_S + 8($sp)
	sw	$s3, PL_S + 12($sp)
	sw	$s4, PL_S + 16($sp)
	sw	$s5, PL_S + 20($sp)
	sw	$s6, PL_S + 24($sp)
	sw	$s7, PL_S + 28($sp)
	lw	$s1, 0($a0)
	addiu	$s0, $a0, 4
	move	$s2, $zero
	NOPS	PF_ENTRY
	beqz	$s1, 90f
	nop
	# All of a literal's time comes before its write, by what the byte is: from one write
	# to the next, 582 cycles to a printing byte, 570 to any other.
1:	lbu	$a0, 0($s1)
	addiu	$s1, $s1, 1
	beqz	$a0, 90f
	addiu	$at, $a0, -37
	beqz	$at, 2f
	sltiu	$at, $a0, 0x20
	bnez	$at, 11f
	sltiu	$at, $a0, 0x7f
	beqz	$at, 12f
	nop
	NOPS	PF_LIT
	jal	libps_putplain
	nop
	b	1b
	nop
12:	NOPS	PF_HIGH
	jal	libps_puthigh
	nop
	b	1b
	nop
	# A newline and a tab go to putchar. Any other byte below a space goes straight to the write.
11:	addiu	$at, $a0, -10
	beqz	$at, 13f
	addiu	$at, $a0, -9
	beqz	$at, 14f
	nop
	NOPS	PF_RAW
	jal	libps_putraw
	nop
	b	1b
	nop
13:	NOPS	PF_NL
	jal	libps_putnl
	nop
	b	1b
	nop
14:	NOPS	PF_TAB
	jal	libps_putint
	nop
	b	1b
	nop
	# A directive is a loop until a letter that converts.
2:	move	$s3, $zero
	move	$s4, $zero
	li	$s5, -1
3:	lbu	$t0, 0($s1)
	addiu	$s1, $s1, 1
	beqz	$t0, 90f
	addiu	$at, $t0, -32
	beqz	$at, 31f
	li	$t1, 4
	addiu	$at, $t0, -35
	beqz	$at, 31f
	li	$t1, 8
	addiu	$at, $t0, -45
	beqz	$at, 31f
	li	$t1, 1
	addiu	$at, $t0, -43
	beqz	$at, 31f
	li	$t1, 2
	addiu	$at, $t0, -48
	beqz	$at, 31f
	li	$t1, 16
	addiu	$at, $t0, -104
	beqz	$at, 31f
	li	$t1, 32
	addiu	$at, $t0, -108
	beqz	$at, 31f
	li	$t1, 64
	addiu	$at, $t0, -76
	beqz	$at, 3b
	addiu	$at, $t0, -42
	bnez	$at, 32f
	addiu	$at, $t0, -46
	lw	$s4, 0($s0)
	addiu	$s0, $s0, 4
	bgez	$s4, 3b
	nop
	ori	$s3, $s3, 1
	b	3b
	subu	$s4, $zero, $s4
31:	b	3b
	or	$s3, $s3, $t1
32:	bnez	$at, 35f
	addiu	$at, $t0, -49
	lbu	$t1, 0($s1)
	nop
	addiu	$at, $t1, -42
	bnez	$at, 33f
	move	$s5, $zero
	addiu	$s1, $s1, 1
	lw	$s5, 0($s0)
	addiu	$s0, $s0, 4
	bgez	$s5, 3b
	nop
	b	3b
	li	$s5, -1
33:	addiu	$at, $t1, -48
	sltiu	$t2, $at, 10
	beqz	$t2, 3b
	sll	$t2, $s5, 2
	addu	$s5, $t2, $s5
	sll	$s5, $s5, 1
	addu	$s5, $s5, $at
	addiu	$s1, $s1, 1
	lbu	$t1, 0($s1)
	b	33b
	nop
35:	sltiu	$at, $at, 9
	beqz	$at, 37f
	nop
	addiu	$s4, $t0, -48
36:	lbu	$t1, 0($s1)
	nop
	addiu	$at, $t1, -48
	sltiu	$t2, $at, 10
	beqz	$t2, 3b
	sll	$t2, $s4, 2
	addu	$s4, $t2, $s4
	sll	$s4, $s4, 1
	addu	$s4, $s4, $at
	b	36b
	addiu	$s1, $s1, 1
	# What it converts.
37:	sw	$zero, PL_SIGN($sp)
	sw	$zero, PL_PREFIX($sp)
	sw	$zero, PL_SHIFT($sp)
	sw	$t0, PL_CONV($sp)
	move	$s7, $zero
	addiu	$at, $t0, -37
	beqz	$at, 38f
	addiu	$at, $t0, -100
	beqz	$at, 50f
	addiu	$at, $t0, -105
	beqz	$at, 50f
	addiu	$at, $t0, -117
	beqz	$at, 53f
	addiu	$at, $t0, -120
	beqz	$at, 54f
	addiu	$at, $t0, -88
	beqz	$at, 55f
	addiu	$at, $t0, -111
	beqz	$at, 56f
	addiu	$at, $t0, -112
	beqz	$at, 54f
	addiu	$at, $t0, -68
	beqz	$at, 57f
	addiu	$at, $t0, -85
	beqz	$at, 58f
	addiu	$at, $t0, -79
	beqz	$at, 59f
	addiu	$at, $t0, -115
	beqz	$at, 40f
	addiu	$at, $t0, -99
	beqz	$at, 42f
	addiu	$at, $t0, -110
	beqz	$at, 44f
	addiu	$at, $t0, -37
	# Any other letter, or a percent sign, is itself.
38:	NOPS	PF_OTHER
	move	$a0, $t0
	jal	libps_putint
	addiu	$s2, $s2, 1
	b	1b
	nop
	# A string, with what's in it counted.
40:	lw	$s6, 0($s0)
	addiu	$s0, $s0, 4
	bnez	$s6, 41f
	nop
	lui	$s6, %hi(libps_null)
	addiu	$s6, $s6, %lo(libps_null)
41:	beq	$s7, $s5, 70f
	addu	$at, $s6, $s7
	lbu	$at, 0($at)
	nop
	beqz	$at, 70f
	nop
	b	41b
	addiu	$s7, $s7, 1
42:	lw	$s6, 0($s0)
	addiu	$s0, $s0, 4
	NOPS	PF_C
	b	70f
	li	$s7, 1
44:	lw	$t1, 0($s0)
	addiu	$s0, $s0, 4
	andi	$at, $s3, 96
	addiu	$at, $at, -32
	beqz	$at, 45f
	nop
	b	1b
	sw	$s2, 0($t1)
45:	b	1b
	sh	$s2, 0($t1)
	# A number. A short one's low half is signed here, without doubles.
57:	li	$t0, 100
	b	50f
	ori	$s3, $s3, 64
58:	li	$t0, 117
	ori	$s3, $s3, 64
	b	53f
	sw	$t0, PL_CONV($sp)
59:	li	$t0, 111
	ori	$s3, $s3, 64
	b	56f
	sw	$t0, PL_CONV($sp)
50:	lw	$s6, 0($s0)
	addiu	$s0, $s0, 4
	andi	$at, $s3, 96
	addiu	$at, $at, -32
	bnez	$at, 500f
	nop
	sll	$s6, $s6, 16
	sra	$s6, $s6, 16
500:	bgez	$s6, 501f
	andi	$at, $s3, 2
	li	$at, 45
	sw	$at, PL_SIGN($sp)
	b	60f
	subu	$s6, $zero, $s6
501:	beqz	$at, 502f
	andi	$at, $s3, 4
	li	$at, 43
	b	60f
	sw	$at, PL_SIGN($sp)
502:	beqz	$at, 60f
	li	$at, 32
	b	60f
	sw	$at, PL_SIGN($sp)
53:	jal	libps_printf_short
	nop
	b	60f
	nop
54:	li	$at, 112
	beq	$t0, $at, 541f
	nop
	jal	libps_printf_short
	nop
	b	542f
	nop
541:	lw	$s6, 0($s0)
	addiu	$s0, $s0, 4
542:	li	$at, 4
	b	551f
	sw	$at, PL_SHIFT($sp)
55:	jal	libps_printf_short
	ori	$s3, $s3, 128
	li	$at, 4
	sw	$at, PL_SHIFT($sp)
551:	andi	$at, $s3, 8
	beqz	$at, 60f
	nop
	beqz	$s6, 60f
	li	$at, 112
	lw	$t0, PL_CONV($sp)
	nop
	beq	$t0, $at, 60f
	nop
	b	60f
	sw	$t0, PL_PREFIX($sp)
56:	jal	libps_printf_short
	nop
	li	$at, 3
	sw	$at, PL_SHIFT($sp)
	# How many digits.
60:	li	$at, 100
	lw	$t0, PL_CONV($sp)
	lw	$t1, PL_SHIFT($sp)
	bnez	$s6, 61f
	nop
	beqz	$s5, 65f
	nop
61:	li	$s7, 1
	bnez	$t1, 63f
	move	$t2, $s6
	lui	$t3, %hi(libps_pow10 + 4)
	addiu	$t3, $t3, %lo(libps_pow10 + 4)
62:	slti	$at, $s7, 10
	beqz	$at, 65f
	nop
	lw	$at, 0($t3)
	addiu	$t3, $t3, 4
	sltu	$at, $t2, $at
	bnez	$at, 65f
	nop
	b	62b
	addiu	$s7, $s7, 1
63:	srlv	$t2, $t2, $t1
	beqz	$t2, 65f
	nop
	b	63b
	addiu	$s7, $s7, 1
65:	lw	$t0, PL_CONV($sp)
	li	$at, 111
	bne	$t0, $at, 66f
	andi	$at, $s3, 8
	beqz	$at, 66f
	nop
	beqz	$s6, 66f
	nop
	ori	$s3, $s3, 256
	addiu	$s7, $s7, 1
66:	bltz	$s5, 67f
	li	$at, 0xffef
	and	$s3, $s3, $at
67:	li	$at, 100
	sw	$at, PL_CONV($sp)
	lw	$t1, PL_SIGN($sp)
	andi	$at, $s3, 0x1f
	or	$at, $at, $s4
	or	$at, $at, $t1
	bnez	$at, 70f
	nop
	bgez	$s5, 70f
	nop
	NOPS	PF_NUM
	b	84f
	addu	$s2, $s2, $s7
	# The field: sign and prefix and bytes together, or a number's precision.
70:	lw	$t1, PL_SIGN($sp)
	andi	$at, $s3, 0x1f
	or	$at, $at, $s4
	or	$at, $at, $t1
	bnez	$at, 701f
	nop
	bgez	$s5, 701f
	nop
	addu	$s2, $s2, $s7
	sw	$zero, PL_FILL($sp)
	NOPS	PF_FAST
	b	79f
	sw	$zero, PL_BODY($sp)
701:	NOPS	PF_SLOW
	lw	$t2, PL_PREFIX($sp)
	beqz	$t1, 71f
	move	$t3, $s7
	addiu	$t3, $t3, 1
71:	beqz	$t2, 72f
	lw	$t0, PL_CONV($sp)
	addiu	$t3, $t3, 2
72:	move	$t4, $t3
	addiu	$at, $t0, -100
	bnez	$at, 73f
	slt	$at, $t4, $s5
	beqz	$at, 73f
	nop
	move	$t4, $s5
73:	slt	$at, $s4, $t4
	bnez	$at, 74f
	move	$t5, $t4
	move	$t5, $s4
74:	addu	$s2, $s2, $t5
	sw	$t3, PL_BODY($sp)
	subu	$t4, $s4, $t4
	sw	$t4, PL_FILL($sp)
	# Spaces before, unless left or zero.
	andi	$at, $s3, 17
	bnez	$at, 76f
	lw	$t4, PL_FILL($sp)
	jal	libps_printf_fill
	li	$a0, 32
76:	lw	$a0, PL_SIGN($sp)
	nop
	beqz	$a0, 77f
	nop
	NOPS	PF_SIGN
	jal	libps_putint
	nop
77:	lw	$a0, PL_PREFIX($sp)
	nop
	beqz	$a0, 78f
	nop
	li	$a0, 48
	jal	libps_putint
	nop
	lw	$a0, PL_PREFIX($sp)
	jal	libps_putint
	nop
78:	andi	$at, $s3, 17
	addiu	$at, $at, -16
	bnez	$at, 79f
	lw	$t4, PL_FILL($sp)
	jal	libps_printf_fill
	li	$a0, 48
79:	lw	$t0, PL_CONV($sp)
	nop
	addiu	$at, $t0, -99
	bnez	$at, 80f
	addiu	$at, $t0, -115
	jal	libps_putint
	andi	$a0, $s6, 0xff
	b	89f
	nop
80:	bnez	$at, 82f
	nop
81:	blez	$s7, 89f
	nop
	lbu	$a0, 0($s6)
	addiu	$s6, $s6, 1
	NOPS	PF_S
	jal	libps_putint
	addiu	$s7, $s7, -1
	b	81b
	nop
	# The digits, first to last.
82:	lw	$t3, PL_BODY($sp)
	nop
	subu	$t0, $s5, $t3
	blez	$t0, 83f
	nop
821:	li	$a0, 48
	sw	$t0, PL_T0($sp)
	jal	libps_putint
	nop
	lw	$t0, PL_T0($sp)
	nop
	addiu	$t0, $t0, -1
	bgtz	$t0, 821b
	nop
83:	andi	$at, $s3, 256
	beqz	$at, 84f
	nop
	li	$a0, 48
	jal	libps_putint
	addiu	$s7, $s7, -1
84:	blez	$s7, 89f
	addiu	$s7, $s7, -1
	lw	$t1, PL_SHIFT($sp)
	nop
	bnez	$t1, 85f
	sll	$at, $s7, 2
	lui	$t0, %hi(libps_pow10)
	addu	$t0, $t0, $at
	lw	$at, %lo(libps_pow10)($t0)
	nop
	divu	$zero, $s6, $at
	mflo	$a0
	mfhi	$s6
	b	86f
	addiu	$a0, $a0, 48
85:	sll	$at, $s7, 2
	addiu	$t0, $t1, -4
	beqz	$t0, 851f
	nop
	subu	$at, $at, $s7
851:	srlv	$t0, $s6, $at
	sll	$at, $t1, 3
	addiu	$at, $at, -17
	and	$t0, $t0, $at
	lui	$t1, %hi(libps_hexl)
	andi	$at, $s3, 128
	bnez	$at, 852f
	addiu	$t1, $t1, %lo(libps_hexl)
	addiu	$t1, $t1, 16
852:	addu	$t1, $t1, $t0
	lbu	$a0, 0($t1)
86:	NOPS	PF_DIG
	jal	libps_putint
	nop
	b	84b
	nop
89:	lw	$t4, PL_FILL($sp)
	andi	$at, $s3, 1
	beqz	$at, 1b
	nop
	jal	libps_printf_fill
	li	$a0, 32
	b	1b
	nop
90:	move	$v0, $s2
	NOPS	PF_END
	lw	$ra, PL_RA($sp)
	lw	$s0, PL_S + 0($sp)
	lw	$s1, PL_S + 4($sp)
	lw	$s2, PL_S + 8($sp)
	lw	$s3, PL_S + 12($sp)
	lw	$s4, PL_S + 16($sp)
	lw	$s5, PL_S + 20($sp)
	lw	$s6, PL_S + 24($sp)
	lw	$s7, PL_S + 28($sp)
	jr	$ra
	addiu	$sp, $sp, PL_FRAME
	.size libps_va_printf, .-libps_va_printf

# libps_printf_short takes a number's word into s6, its low half signed if short and not long.
libps_printf_short:
	lw	$s6, 0($s0)
	addiu	$s0, $s0, 4
	andi	$at, $s3, 96
	addiu	$at, $at, -32
	bnez	$at, 1f
	nop
	sll	$s6, $s6, 16
	sra	$s6, $s6, 16
1:	jr	$ra
	nop

# libps_printf_fill writes t4 copies of a0, keeping its count on the stack past its return.
libps_printf_fill:
	blez	$t4, 3f
	nop
	sw	$ra, PL_RA2($sp)
	sw	$a0, PL_T0($sp)
	sw	$t4, PL_T1($sp)
1:	NOPS	PF_FILL
	jal	libps_putint
	nop
	lw	$a0, PL_T0($sp)
	lw	$t4, PL_T1($sp)
	nop
	addiu	$t4, $t4, -1
	bgtz	$t4, 2f
	sw	$t4, PL_T1($sp)
	lw	$ra, PL_RA2($sp)
	nop
3:	jr	$ra
	nop
2:	b	1b
	nop

# VSync's mode 1, in sync.go. Any other mode goes to the Go.
	.section .text.VSync, "ax", @progbits
	.align 2
	.globl VSync
	.type VSync, @function
VSync:
	STUBTIME
	li	$at, 1
	beq	$a0, $at, 1f
	lui	$v0, 0x1f80
	j	libps_vsync_wait
	nop
1:	NOPS	13
	lw	$v0, 0x1110($v0)
	lui	$at, %hi(libps_vsync_lines)
	lw	$at, %lo(libps_vsync_lines)($at)
	nop
	subu	$v0, $v0, $at
	jr	$ra
	andi	$v0, $v0, 0xffff
	.size VSync, .-VSync

# GsGetActiveBuff and GsSetWorkBase, in gs.go.
	.section .text.GsGetActiveBuff, "ax", @progbits
	.align 2
	.globl GsGetActiveBuff
	.type GsGetActiveBuff, @function
GsGetActiveBuff:
	STUBTIME
	lui	$v0, %hi(PSDIDX)
	lw	$v0, %lo(PSDIDX)($v0)
	jr	$ra
	andi	$v0, $v0, 1
	.size GsGetActiveBuff, .-GsGetActiveBuff

	.section .text.GsSetWorkBase, "ax", @progbits
	.align 2
	.globl GsSetWorkBase
	.type GsSetWorkBase, @function
GsSetWorkBase:
	STUBTIME
	lui	$at, %hi(GsOUT_PACKET_P)
	jr	$ra
	sw	$a0, %lo(GsOUT_PACKET_P)($at)
	.size GsSetWorkBase, .-GsSetWorkBase

# libps_putint is putchar for printf's own bytes, with no time of its own beyond its tests.
	.section .text.putchar, "ax", @progbits
	.align 2
	.globl libps_putint
libps_putint:
	andi	$a0, $a0, 0xff
	sltiu	$at, $a0, 0x20
	bnez	$at, libps_putlow
	sltiu	$at, $a0, 0x7f
	bnez	$at, libps_putplain
	nop
	b	libps_puthigh
	nop

# putchar(c): kernel.go says what it does. The plain text goes to the write from here.
	.section .text.putchar, "ax", @progbits
	.align 2
	.globl putchar
	.type putchar, @function
putchar:
	andi	$a0, $a0, 0xff
	sltiu	$at, $a0, 0x20
	bnez	$at, 2f
	sltiu	$at, $a0, 0x7f
	bnez	$at, 6f
	# A byte from 7Fh up prints as highClass in pad.go says.
	.globl libps_puthigh
libps_puthigh:
	addiu	$at, $a0, -0x93
	sltiu	$t0, $at, 86
	beqz	$t0, 5f
	lui	$t0, %hi(libps_class_low)
	addu	$t0, $t0, $at
	lbu	$t0, %lo(libps_class_low)($t0)
	nop
	andi	$t0, $t0, 0x97
	beqz	$t0, 3f
	nop
	NOPS	PC_HIGH
	.globl libps_putplain
libps_putplain:
1:	lui	$t0, %hi(libps_tty_column)
	lw	$at, %lo(libps_tty_column)($t0)
	nop
	addiu	$at, $at, 1
	sw	$at, %lo(libps_tty_column)($t0)
	.globl libps_putraw
libps_putraw:
3:	lui	$a1, %hi(libps_tty_char)
	addiu	$a1, $a1, %lo(libps_tty_char)
	sb	$a0, 0($a1)
	li	$a0, 1
	li	$a2, 1
	li	$t1, 0x35
	li	$t0, 0xb0
	jr	$t0
	nop
5:	NOPS	4
	b	3b
	nop
6:	NOPS	PC_PLAIN
	b	1b
	nop
	# A newline goes with a carriage return before it, a tab to Go, the rest straight to the write.
libps_putlow:
2:	addiu	$at, $a0, -10
	beqz	$at, 4f
	addiu	$at, $a0, -9
	beqz	$at, 7f
	nop
	NOPS	4
	b	3b
	nop
7:	j	libps_putchar_other
	nop
4:	NOPS	PC_NLIN
	.globl libps_putnl
libps_putnl:
	addiu	$sp, $sp, -24
	sw	$ra, 16($sp)
	lui	$t0, %hi(libps_tty_column)
	sw	$zero, %lo(libps_tty_column)($t0)
	li	$a0, 13
	lui	$a1, %hi(libps_tty_char)
	addiu	$a1, $a1, %lo(libps_tty_char)
	sb	$a0, 0($a1)
	li	$a0, 1
	li	$a2, 1
	li	$t1, 0x35
	li	$t0, 0xb0
	jalr	$t0
	nop
	lw	$ra, 16($sp)
	li	$a0, 10
	NOPS	PC_NL
	b	3b
	addiu	$sp, $sp, 24
	.size putchar, .-putchar

	.section .bss.libps_tty, "aw", @nobits
	.align 2
	.globl libps_class_low
libps_class_low:
	.space 16
	.globl libps_pad_buf
libps_pad_buf:
	.space 72
	.globl libps_vsync_lines
libps_vsync_lines:
	.space 4
	.globl libps_tty_column
libps_tty_column:
	.space 4
	.globl libps_tty_char
libps_tty_char:
	.space 4

	.section .rodata.libps_null, "a", @progbits
libps_null:
	.asciz "(null)"

# libps_mul64(a, b) and libps_mulhi(a, b) are the CPU's product, for the soft float: the compiler's
# 64-bit product is a good deal longer than Sony's time for a product, whole.
	.section .text.libps_mul, "ax", @progbits
	.align 2
	.globl libps_mul64
libps_mul64:
	multu	$a0, $a1
	mflo	$v0
	mfhi	$v1
	jr	$ra
	nop
	.globl libps_mulhi
libps_mulhi:
	multu	$a0, $a1
	mfhi	$v0
	jr	$ra
	nop
# SETROT loads the rotation at the given register into the GTE, its words in t0 to t4 first.
	.macro LOADROT m
	lw	$t0, 0(\m)
	lw	$t1, 4(\m)
	lw	$t2, 8(\m)
	lw	$t3, 12(\m)
	lh	$t4, 16(\m)
	.endm
	.macro PUTROT
	CTC	8, 0
	CTC	9, 1
	CTC	10, 2
	CTC	11, 3
	CTC	12, 4
	.endm

# LVHEAD multiplies the 32-bit vector at the register by the GTE's rotation, in two parts,
# as turnLong in matrix.go has it: the magnitude of each element above its low 15 bits, and
# those bits, each with the element's sign. The first part's sums go up 3 and meet the
# second's in LVSUM. A negative element and a negative sum of the first part take a little
# longer. At the end t0 to t2 hold the first part's sums, and the second is going.
	.macro LVHEAD v, off
	lw	$t5, \off(\v)
	lw	$t6, \off + 4(\v)
	bltz	$t5, 31f
	srl	$t0, $t5, 15
	andi	$t5, $t5, 0x7fff
41:	lw	$t7, \off + 8(\v)
	bltz	$t6, 32f
	srl	$t1, $t6, 15
	andi	$t6, $t6, 0x7fff
42:	bltz	$t7, 33f
	srl	$t2, $t7, 15
	andi	$t7, $t7, 0x7fff
43:	mtc2	$t0, $9
	mtc2	$t1, $10
	mtc2	$t2, $11
	nop
	MVMVAIR0
	mfc2	$t0, $25
	mfc2	$t1, $26
	mfc2	$t2, $27
	mtc2	$t5, $9
	mtc2	$t6, $10
	mtc2	$t7, $11
	nop
	MVMVAIR12
	bltz	$t0, 34f
	sll	$t0, $t0, 3
44:	bltz	$t1, 35f
	sll	$t1, $t1, 3
45:	bltz	$t2, 36f
	sll	$t2, $t2, 3
46:
	.endm

# LVNEG is what LVHEAD's branches go to, kept out of the way.
	.macro LVNEG
31:	subu	$t5, $zero, $t5
	srl	$t0, $t5, 15
	subu	$t0, $zero, $t0
	andi	$t5, $t5, 0x7fff
	b	41b
	subu	$t5, $zero, $t5
32:	subu	$t6, $zero, $t6
	srl	$t1, $t6, 15
	subu	$t1, $zero, $t1
	andi	$t6, $t6, 0x7fff
	b	42b
	subu	$t6, $zero, $t6
33:	subu	$t7, $zero, $t7
	srl	$t2, $t7, 15
	subu	$t2, $zero, $t2
	andi	$t7, $t7, 0x7fff
	b	43b
	subu	$t7, $zero, $t7
34:	nop
	b	44b
	nop
35:	nop
	b	45b
	nop
36:	nop
	b	46b
	nop
	.endm

# LVSUM adds the second part's result to t0 to t2.
	.macro LVSUM
	mfc2	$t3, $25
	mfc2	$t4, $26
	mfc2	$t5, $27
	addu	$t0, $t0, $t3
	addu	$t1, $t1, $t4
	addu	$t2, $t2, $t5
	.endm

# MULMAT multiplies the matrix at a1 by the GTE's rotation column by column into a2, as far
# as the last column's first part, whose multiply is going, with the column before it stored:
# the words MULMATEND or CompMatrix store. Each column is read before anything writes to it, so
# a2 may be a1. t0 to t6 go.
	.macro MULMAT
	G
	lh	$t6, 16($a1)
	lhu	$t0, 0($a1)
	lh	$t1, 6($a1)
	lh	$t2, 12($a1)
	sll	$t1, $t1, 16
	or	$t0, $t0, $t1
	GAP	6
	mtc2	$t0, $0
	mtc2	$t2, $1
	nop
	MVMVA12
	G
	lhu	$t0, 2($a1)
	lh	$t1, 8($a1)
	lh	$t2, 14($a1)
	sll	$t1, $t1, 16
	or	$t0, $t0, $t1
	GAP	5
	mfc2	$t3, $9
	mfc2	$t4, $10
	mfc2	$t5, $11
	mtc2	$t0, $0
	mtc2	$t2, $1
	sh	$t3, 0($a2)
	MVMVA12
	G
	sh	$t4, 6($a2)
	lh	$t1, 10($a1)
	lhu	$t0, 4($a1)
	sh	$t5, 12($a2)
	sll	$t1, $t1, 16
	or	$t0, $t0, $t1
	GAP	6
	mfc2	$t3, $9
	mfc2	$t4, $10
	mfc2	$t5, $11
	mtc2	$t0, $0
	mtc2	$t6, $1
	sh	$t3, 2($a2)
	MVMVA12
	G
	sh	$t4, 8($a2)
	sh	$t5, 14($a2)
	GAP	8
	mfc2	$t3, $9
	mfc2	$t4, $10
	.endm

# MULMATEND finishes a product with the last column.
	.macro MULMATEND
	G
	sh	$t3, 4($a2)
	sh	$t4, 10($a2)
	GAP	8
	swc2	$11, 16($a2)
	sh	$zero, 18($a2)
	.endm

	.section .text.Matrix, "ax", @progbits
	.align 2
# ApplyMatrix(m, v0, v1), ApplyMatrixSV(m, v0, v1): matrix.go has what they do.
	.globl ApplyMatrix
	.type ApplyMatrix, @function
ApplyMatrix:
	STUBTIME
	LOADROT	$a0
	PUTROT
	lwc2	$0, 0($a1)
	lwc2	$1, 4($a1)
	move	$v0, $a2
	MVMVA12
	swc2	$25, 0($a2)
	swc2	$26, 4($a2)
	jr	$ra
	swc2	$27, 8($a2)
	.size ApplyMatrix, .-ApplyMatrix

	.globl ApplyMatrixSV
	.type ApplyMatrixSV, @function
ApplyMatrixSV:
	STUBTIME
	LOADROT	$a0
	PUTROT
	lwc2	$0, 0($a1)
	lwc2	$1, 4($a1)
	move	$v0, $a2
	MVMVA12
	mfc2	$t0, $9
	mfc2	$t1, $10
	mfc2	$t2, $11
	sh	$t0, 0($a2)
	sh	$t1, 2($a2)
	jr	$ra
	sh	$t2, 4($a2)
	.size ApplyMatrixSV, .-ApplyMatrixSV

# ApplyMatrixLV(m, v0, v1): LVHEAD says how.
	.globl ApplyMatrixLV
	.type ApplyMatrixLV, @function
ApplyMatrixLV:
	STUBTIME
	LOADROT	$a0
	PUTROT
	LVHEAD	$a1, 0
	NOPS	1
	LVSUM
	sw	$t0, 0($a2)
	sw	$t1, 4($a2)
	sw	$t2, 8($a2)
	jr	$ra
	move	$v0, $a2
	LVNEG
	.size ApplyMatrixLV, .-ApplyMatrixLV

# MulMatrix0(m0, m1, m2) makes m2's rotation m0's times m1's, on the GTE. MulMatrix(m0, m1)
# is the same into m0.
	.globl MulMatrix
	.type MulMatrix, @function
MulMatrix:
	STUBTIME
	move	$a2, $a0
	.size MulMatrix, .-MulMatrix
	.globl MulMatrix0
	.type MulMatrix0, @function
MulMatrix0:
	STUBTIME
	LOADROT	$a0
	PUTROT
	MULMAT
	G
	sh	$t3, 4($a2)
	sh	$t4, 10($a2)
	GAP	6
	swc2	$11, 16($a2)
	nop
	jr	$ra
	move	$v0, $a2
	.size MulMatrix0, .-MulMatrix0

# CompMatrix(m0, m1, m2): matrix.go says what it makes.
	.globl CompMatrix
	.type CompMatrix, @function
CompMatrix:
	STUBTIME
	LOADROT	$a0
	PUTROT
	MULMAT
	swc2	$11, 16($a2)
	G
	lhu	$t0, 20($a1)
	lh	$t1, 24($a1)
	lh	$t2, 28($a1)
	sll	$t1, $t1, 16
	or	$t0, $t0, $t1
	GAP	5
	mtc2	$t0, $0
	mtc2	$t2, $1
	sh	$t3, 4($a2)
	MVMVA12
	G
	sh	$t4, 10($a2)
	sh	$zero, 18($a2)
	lw	$t3, 20($a0)
	lw	$t4, 24($a0)
	lw	$t5, 28($a0)
	GAP	8
	mfc2	$t0, $25
	mfc2	$t1, $26
	mfc2	$t2, $27
	addu	$t0, $t0, $t3
	addu	$t1, $t1, $t4
	addu	$t2, $t2, $t5
	sw	$t0, 20($a2)
	sw	$t1, 24($a2)
	sw	$t2, 28($a2)
	NOPS	CM_END
	jr	$ra
	move	$v0, $a2
	.size CompMatrix, .-CompMatrix

# libps_compose(a, b, out) makes out the transformation b and then a: compose in matrix.go.
# It leaves a3, v1, t8, t9, and v0 alone, which GsGetLs and the like keep things in.
	.globl libps_compose
	.type libps_compose, @function
libps_compose:
	LOADROT	$a0
	NOPS	2
	PUTROT
	LVHEAD	$a1, 20
	lw	$t3, 20($a0)
	lw	$t4, 24($a0)
	lw	$t5, 28($a0)
	mfc2	$t6, $25
	mfc2	$t7, $26
	mfc2	$at, $27
	addu	$t0, $t0, $t6
	addu	$t1, $t1, $t7
	addu	$t2, $t2, $at
	addu	$t0, $t0, $t3
	addu	$t1, $t1, $t4
	addu	$t2, $t2, $t5
	sw	$t0, 20($a2)
	sw	$t1, 24($a2)
	sw	$t2, 28($a2)
	LOADROT	$a0
	NOPS	2
	PUTROT
	MULMAT
	MULMATEND
	jr	$ra
	nop
	LVNEG
	.size libps_compose, .-libps_compose

# libps_ltw(c) sees to c's Workm and returns its address (localToWorld, spec 19). It goes up
# the chain to the nearest system that is good: flagged with the frame's count, or
# the root. The highest system on the way with a flag of 0 is the first to work out, with all
# below it, each from the one above: a good one's child if there's no flag of 0. Workm holds
# the way back down while that is done. Each way takes what it does on Sony's.
	.set COORD, 4
	.set WORKM, 36
	.set SUPER, 72
	.globl libps_ltw
	.type libps_ltw, @function
libps_ltw:
	move	$v1, $ra
	lui	$t0, %hi(PSDCNT)
	lw	$a3, %lo(PSDCNT)($t0)
	lw	$t1, SUPER($a0)
	lw	$t2, 0($a0)
	beqz	$t1, ltw_root
	move	$t3, $a0
	bne	$t2, $a3, ltw_walk
	move	$t4, $zero
	# Good already.
	NOPS	LTW_QUICK
	jr	$v1
	addiu	$v0, $a0, WORKM
ltw_root:
	# A root on its own, its flag in t2.
	b	ltw_top
	move	$t4, $zero
ltw_walk:
	lw	$t2, 0($t3)
	lw	$t5, 0($t1)
	bnez	$t2, 1f
	lw	$t6, SUPER($t1)
	move	$t4, $t3
1:	beq	$t5, $a3, ltw_good
	nop
	beqz	$t6, ltw_atroot
	nop
	move	$t3, $t1
	b	ltw_walk
	move	$t1, $t6
ltw_good:
	move	$t4, $t3
	beqz	$t6, ltw_rootgood
	move	$t3, $t1
	nop
	nop
	b	ltw_links
	nop
ltw_rootgood:
	nop
	b	ltw_top
	move	$t2, $t5
ltw_atroot:
	bnez	$t5, 2f
	move	$t2, $t5
	move	$t4, $t3
2:	move	$t3, $t1
ltw_top:
	# A root with a flag of 0 or the count takes its Coord. Any other flag leaves it, and
	# has its flag back.
	nop
	beq	$t2, $a3, 5f
	sw	$a3, 0($t3)
	bnez	$t2, ltw_other
	nop
5:	lw	$t0, COORD($t3)
	lw	$t1, COORD + 4($t3)
	lw	$t2, COORD + 8($t3)
	lw	$t5, COORD + 12($t3)
	sw	$t0, WORKM($t3)
	sw	$t1, WORKM + 4($t3)
	sw	$t2, WORKM + 8($t3)
	sw	$t5, WORKM + 12($t3)
	lw	$t0, COORD + 16($t3)
	lw	$t1, COORD + 20($t3)
	lw	$t2, COORD + 24($t3)
	lw	$t5, COORD + 28($t3)
	sw	$t0, WORKM + 16($t3)
	sw	$t1, WORKM + 20($t3)
	sw	$t2, WORKM + 24($t3)
	sw	$t5, WORKM + 28($t3)
ltw_links:
	# The way back down, from c to the first to work out, then the same time on up to
	# the top.
	move	$t8, $a0
	beqz	$t4, 6f
	move	$t9, $a0
5:	lw	$t1, SUPER($t8)
	beq	$t8, $t4, 6f
	nop
	sw	$t8, WORKM($t1)
	b	5b
	move	$t8, $t1
6:	lw	$t1, SUPER($t8)
	beq	$t8, $t3, 7f
	nop
	nop
	b	6b
	move	$t8, $t1
7:	beqz	$t4, 10f
	move	$t8, $t4
	NOPS	LTW_WALK
8:	lw	$a0, SUPER($t8)
	lw	$v0, WORKM($t8)
	sw	$a3, 0($t8)
	addiu	$a0, $a0, WORKM
	addiu	$a1, $t8, COORD
	jal	libps_compose
	addiu	$a2, $t8, WORKM
	beq	$t8, $t9, 9f
	move	$t8, $v0
	NOPS	LTW_DOWN
	b	8b
	nop
	# A way that worked something out, with what GsGetLws wants back.
9:	lw	$a1, 12($sp)
	NOPS	LTW_WEXIT
10:	jr	$v1
	addiu	$v0, $t9, WORKM
ltw_other:
	sw	$t2, 0($t3)
	beqz	$t4, ltw_links
	nop
	b	ltw_links
	nop
	.size libps_ltw, .-libps_ltw

# GsGetLw(coord, m), GsGetLs(coord, m), GsGetLws(coord, lw, ls): gs3d.go says what they do.
# On a stack of two words they call what they call, as a program's stack may be small.
	.macro COPYMATRIX to, from
	lw	$t0, 0(\from)
	lw	$t1, 4(\from)
	lw	$t2, 8(\from)
	lw	$t3, 12(\from)
	sw	$t0, 0(\to)
	sw	$t1, 4(\to)
	sw	$t2, 8(\to)
	sw	$t3, 12(\to)
	lw	$t0, 16(\from)
	lw	$t1, 20(\from)
	lw	$t2, 24(\from)
	lw	$t3, 28(\from)
	sw	$t0, 16(\to)
	sw	$t1, 20(\to)
	sw	$t2, 24(\to)
	sw	$t3, 28(\to)
	.endm

	.globl GsGetLw
	.type GsGetLw, @function
GsGetLw:
	STUBTIME
	addiu	$sp, $sp, -24
	sw	$ra, 16($sp)
	sw	$a1, 12($sp)
	NOPS	LTW_PRE
	jal	libps_ltw
	nop
	COPYMATRIX $a1, $v0
	lw	$ra, 16($sp)
	addiu	$sp, $sp, 24
	jr	$ra
	nop
	.size GsGetLw, .-GsGetLw

	.globl GsGetLs
	.type GsGetLs, @function
GsGetLs:
	STUBTIME
	addiu	$sp, $sp, -24
	sw	$ra, 16($sp)
	sw	$a1, 20($sp)
	NOPS	LTW_PRE
	jal	libps_ltw
	nop
	lui	$a0, %hi(GsWSMATRIX)
	addiu	$a0, $a0, %lo(GsWSMATRIX)
	move	$a1, $v0
	lw	$a2, 20($sp)
	NOPS	LS_MID
	jal	libps_compose
	nop
	lw	$ra, 16($sp)
	addiu	$sp, $sp, 24
	NOPS	LS_POST
	jr	$ra
	nop
	.size GsGetLs, .-GsGetLs

	.globl GsGetLws
	.type GsGetLws, @function
GsGetLws:
	STUBTIME
	addiu	$sp, $sp, -24
	sw	$ra, 16($sp)
	sw	$a2, 20($sp)
	sw	$a1, 12($sp)
	NOPS	LTW_PRE + 1
	jal	libps_ltw
	nop
	COPYMATRIX $a1, $v0
	lui	$a0, %hi(GsWSMATRIX)
	addiu	$a0, $a0, %lo(GsWSMATRIX)
	move	$a1, $v0
	lw	$a2, 20($sp)
	NOPS	LS_MID
	jal	libps_compose
	nop
	lw	$ra, 16($sp)
	addiu	$sp, $sp, 24
	NOPS	LS_POST + 1
	jr	$ra
	nop
	.size GsGetLws, .-GsGetLws

# libps_view(view, super) makes view, in super's coordinates, the world-to-screen matrix:
# viewUnder in gs3d.go. GsSetView2(pv) is that, or with no super a copy of the view.
	.globl GsSetView2
	.type GsSetView2, @function
GsSetView2:
	STUBTIME
	lw	$a1, 32($a0)
	nop
	bnez	$a1, 1f
	lui	$t4, %hi(GsWSMATRIX)
	addiu	$t4, $t4, %lo(GsWSMATRIX)
	lui	$t5, %hi(libps_ws_unscaled)
	addiu	$t5, $t5, %lo(libps_ws_unscaled)
	lw	$t0, 0($a0)
	lw	$t1, 4($a0)
	lw	$t2, 8($a0)
	lw	$t3, 12($a0)
	sw	$t0, 0($t4)
	sw	$t1, 4($t4)
	sw	$t2, 8($t4)
	sw	$t3, 12($t4)
	sw	$t0, 0($t5)
	sw	$t1, 4($t5)
	sw	$t2, 8($t5)
	sw	$t3, 12($t5)
	lw	$t0, 16($a0)
	lw	$t1, 20($a0)
	lw	$t2, 24($a0)
	lw	$t3, 28($a0)
	sw	$t0, 16($t4)
	sw	$t1, 20($t4)
	sw	$t2, 24($t4)
	sw	$t3, 28($t4)
	sw	$t0, 16($t5)
	sw	$t1, 20($t5)
	sw	$t2, 24($t5)
	sw	$t3, 28($t5)
	NOPS	VW_PLAIN
	jr	$ra
	move	$v0, $zero
1:	NOPS	VW_SUPER
	.size GsSetView2, .-GsSetView2
	.globl libps_view
	.type libps_view, @function
libps_view:
	NOPS	4
	addiu	$sp, $sp, -24
	sw	$ra, 16($sp)
	sw	$a0, 20($sp)
	jal	libps_ltw
	move	$a0, $a1
	lui	$a0, %hi(view_inverse)
	addiu	$a0, $a0, %lo(view_inverse)
	# The inverse's rotation: the transpose of the world's.
	lh	$t0, 0($v0)
	lh	$t1, 6($v0)
	lh	$t2, 12($v0)
	sh	$t0, 0($a0)
	sh	$t1, 2($a0)
	sh	$t2, 4($a0)
	lh	$t0, 2($v0)
	lh	$t1, 8($v0)
	lh	$t2, 14($v0)
	sh	$t0, 6($a0)
	sh	$t1, 8($a0)
	sh	$t2, 10($a0)
	lh	$t0, 4($v0)
	lh	$t1, 10($v0)
	lh	$t2, 16($v0)
	sh	$t0, 12($a0)
	sh	$t1, 14($a0)
	sh	$t2, 16($a0)
	addiu	$a1, $v0, 20
	NOPS	VW_MID
	LOADROT	$a0
	PUTROT
	LVHEAD	$a1, 0
	NOPS	3
	LVSUM
	move	$a1, $a0
	subu	$t0, $zero, $t0
	subu	$t1, $zero, $t1
	subu	$t2, $zero, $t2
	sw	$t0, 20($a0)
	sw	$t1, 24($a0)
	sw	$t2, 28($a0)
	lw	$a0, 20($sp)
	lui	$a2, %hi(GsWSMATRIX)
	addiu	$a2, $a2, %lo(GsWSMATRIX)
	NOPS	VW_34
	jal	libps_compose
	nop
	lui	$t4, %hi(GsWSMATRIX)
	addiu	$t4, $t4, %lo(GsWSMATRIX)
	lui	$t5, %hi(libps_ws_unscaled)
	addiu	$t5, $t5, %lo(libps_ws_unscaled)
	COPYMATRIX $t5, $t4
	lw	$ra, 16($sp)
	addiu	$sp, $sp, 24
	NOPS	VW_END
	jr	$ra
	move	$v0, $zero
	LVNEG
	.size libps_view, .-libps_view

	.section .bss.view_inverse, "aw", @nobits
	.align 2
view_inverse:
	.space 32
	.globl libps_ws_unscaled
libps_ws_unscaled:
	.space 32
	.section .text.Matrix, "ax", @progbits

#ifdef LIBPS_FIDELITY
# libps_delay(n) spends n nops and a call's few instructions, by a jump into the nops.
	.globl libps_delay
	.type libps_delay, @function
libps_delay:
	lui	$t0, %hi(1f)
	addiu	$t0, $t0, %lo(1f)
	sll	$a0, $a0, 2
	subu	$t0, $t0, $a0
	jr	$t0
	nop
	NOPS	130
1:	jr	$ra
	nop
	.size libps_delay, .-libps_delay

# What Krom2Tim leaves below the stack (spec 30). libps_krom_clear(n) clears the n bytes
# below sp, at Sony's 8 cycles a byte. libps_krom_glyph(at, x, y, addr) copies the 30 bytes at
# addr to cell x, y of the table at at from sp, or spends the time of it for an addr of 0.
# libps_krom_image(at, pix, stride, lines) copies lines of stride bytes from pix to at
# from sp, in lines 128 bytes apart: 12 cycles a word and 18 a line, so that all of
# this is whole turns of the spin it comes off.
	.section .text.KromFrame, "ax", @progbits
	.align 2
	.globl libps_krom_clear
	.type libps_krom_clear, @function
libps_krom_clear:
	subu	$t0, $sp, $a0
1:	addiu	$t0, $t0, 1
	nop
	bne	$t0, $sp, 1b
	sb	$zero, -1($t0)
	jr	$ra
	nop
	.size libps_krom_clear, .-libps_krom_clear

	.globl libps_krom_glyph
	.type libps_krom_glyph, @function
libps_krom_glyph:
	sll	$a1, $a1, 5
	sll	$a2, $a2, 9
	addu	$a0, $a0, $sp
	addu	$a0, $a0, $a1
	addu	$a0, $a0, $a2
	bnez	$a3, 1f
	addiu	$t0, $a0, 30
	move	$a3, $a0
1:	lbu	$t1, 0($a3)
	addiu	$a3, $a3, 1
	addiu	$a0, $a0, 1
	sb	$t1, -1($a0)
	bne	$a0, $t0, 1b
	nop
	nop
	nop
	nop
	jr	$ra
	nop
	.size libps_krom_glyph, .-libps_krom_glyph

	.globl libps_krom_image
	.type libps_krom_image, @function
libps_krom_image:
	addu	$a0, $a0, $sp
1:	addu	$t0, $a0, $a2
	move	$t1, $a0
	beq	$t1, $t0, 3f
	nop
2:	lw	$t2, 0($a1)
	addiu	$a1, $a1, 4
	sw	$t2, 0($t1)
	addiu	$t1, $t1, 4
	bne	$t1, $t0, 2b
	nop
3:	addiu	$a3, $a3, -1
	nop
	nop
	bnez	$a3, 1b
	addiu	$a0, $a0, 128
	jr	$ra
	nop
	.size libps_krom_image, .-libps_krom_image
	.section .text.Matrix, "ax", @progbits
#endif

# libps_kanji_frame(st, size) calls libps_kanji_flush(st, frame) on a frame of size bytes
# taken off the stack, the callee's argument home first. Go keeps nothing this big on the stack.
	.section .text.KanjiFrame, "ax", @progbits
	.align 2
	.globl libps_kanji_frame
	.type libps_kanji_frame, @function
libps_kanji_frame:
	addiu	$sp, $sp, -24
	sw	$ra, 20($sp)
	sw	$s0, 16($sp)
	move	$s0, $sp
	subu	$sp, $sp, $a1
	jal	libps_kanji_flush
	addiu	$a1, $sp, 16
	move	$sp, $s0
	lw	$ra, 20($sp)
	lw	$s0, 16($sp)
	jr	$ra
	addiu	$sp, $sp, 24
	.size libps_kanji_frame, .-libps_kanji_frame

# The C library's byte copies, sets and compares: clib.go has what each does at its edges.
# The loops take Sony's time a byte. The first byte's load is in a branch's delay, and the
# last load reads one past the end.
	.section .text.CBytes, "ax", @progbits
	.align 2
	.globl bcopy
	.type bcopy, @function
bcopy:
	STUBTIME
	beqz	$a0, 2f
	nop
	blez	$a2, 2f
	lbu	$t0, 0($a0)
1:	addiu	$a2, $a2, -1
	addiu	$a0, $a0, 1
	sb	$t0, 0($a1)
	addiu	$a1, $a1, 1
	bgtz	$a2, 1b
	lbu	$t0, 0($a0)
2:	jr	$ra
	nop
	.size bcopy, .-bcopy

	.globl libps_c_memcpy
	.type libps_c_memcpy, @function
libps_c_memcpy:
	STUBTIME
	beqz	$a0, 2f
	move	$v0, $a0
	blez	$a2, 2f
	lbu	$t0, 0($a1)
1:	addiu	$a2, $a2, -1
	addiu	$a1, $a1, 1
	sb	$t0, 0($a0)
	addiu	$a0, $a0, 1
	bgtz	$a2, 1b
	lbu	$t0, 0($a1)
2:	jr	$ra
	nop
	.size libps_c_memcpy, .-libps_c_memcpy

	.globl libps_c_memset
	.type libps_c_memset, @function
libps_c_memset:
	STUBTIME
	blez	$a2, 2f
	move	$v0, $a0
	beqz	$a0, 2f
	nop
1:	sb	$a1, 0($a0)
	addiu	$a2, $a2, -1
	bgtz	$a2, 1b
	addiu	$a0, $a0, 1
	jr	$ra
	nop
2:	jr	$ra
	move	$v0, $zero
	.size libps_c_memset, .-libps_c_memset

	.globl bzero
	.type bzero, @function
bzero:
	STUBTIME
	blez	$a1, 2f
	nop
	beqz	$a0, 2f
	nop
1:	sb	$zero, 0($a0)
	addiu	$a1, $a1, -1
	bgtz	$a1, 1b
	addiu	$a0, $a0, 1
2:	jr	$ra
	nop
	.size bzero, .-bzero

	.globl bcmp
	.type bcmp, @function
bcmp:
	STUBTIME
	.size bcmp, .-bcmp
	.globl libps_c_memcmp
	.type libps_c_memcmp, @function
libps_c_memcmp:
	STUBTIME
	NOPS	CMP_LEAD
1:	lbu	$t0, 0($a0)
	lbu	$t1, 0($a1)
	addiu	$a0, $a0, 1
	addiu	$a1, $a1, 1
	bne	$t0, $t1, 2f
	addiu	$a2, $a2, -1
	bgtz	$a2, 1b
	nop
	jr	$ra
	move	$v0, $zero
2:	jr	$ra
	subu	$v0, $t0, $t1
	.size libps_c_memcmp, .-libps_c_memcmp
	.section .text.Matrix, "ax", @progbits

	.section .text.FntPrintC, "ax", @progbits
	.align 2
# libps_va_FntPrint(args): FntPrint as C calls it, with what libFntPrint in cformat.go said
# of it: formatter C. The first word is a stream's id if it is below the count of streams
# open, and otherwise the format, for stream 0. A formatted byte is a literal byte to the
# stream's limit: written and counted before the room is checked, so that the one that does
# not fit lands at text[room] and the print ends with -1 (spec 01). An unknown letter takes
# no word and makes the width's spaces or one, and the format's own NUL after a percent sign
# is read as such a letter: the format goes on past it (spec 17).
#
# a1 is the text's place, a2 its last room, t2 the format, t3 the next word, t4 the stream.
	.set FP_BASE, 36
	.set FP_STREAM, 44
	.set FP_OPEN, 388
	.set FP_TEXT, 16
	.set FP_ROOM, 20
	.set FP_N, 28

# FPPUT writes the byte in the register to the text, and goes to the cut past its limit.
	.macro FPPUT r
	sb	\r, 0($a1)
	addiu	$a1, $a1, 1
	sltu	$at, $a2, $a1
	bnez	$at, fp_cut
	nop
	.endm

	.globl libps_va_FntPrint
	.type libps_va_FntPrint, @function
libps_va_FntPrint:
	lui	$t0, %hi(libps_font_base)
	lw	$t0, %lo(libps_font_base)($t0)
	li	$v0, -1
	beqz	$t0, fp_ret
	nop
	lw	$t1, FP_OPEN($t0)
	lw	$t2, 0($a0)
	beqz	$t1, fp_ret
	addiu	$t4, $t0, FP_BASE
	sltu	$at, $t2, $t1
	beqz	$at, 1f
	addiu	$t3, $a0, 4
	sll	$at, $t2, 2
	sll	$t0, $t2, 3
	addu	$at, $at, $t0
	sll	$t0, $t2, 5
	addu	$at, $at, $t0
	addu	$t4, $t4, $at
	lw	$t2, 4($a0)
	addiu	$t3, $a0, 8
1:	lw	$t5, FP_N($t4)
	lw	$t6, FP_ROOM($t4)
	lw	$t7, FP_TEXT($t4)
	slt	$at, $t5, $t6
	beqz	$at, fp_refused
	addu	$a1, $t7, $t5
	addu	$a2, $t7, $t6
	addiu	$a2, $a2, -1
	NOPS	FP_LEAD
fp_next:
	lbu	$t0, 0($t2)
	addiu	$t2, $t2, 1
	lbu	$t8, 0($t2)
	li	$at, 0x25
	beq	$t0, $at, fp_percent
	nop
fp_lit:
	beqz	$t0, fp_end
	nop
	NOPS	FP_CHAR
	sb	$t0, 0($a1)
	addiu	$a1, $a1, 1
	sltu	$at, $a2, $a1
	beqz	$at, fp_next
	nop
	b	fp_cut
	nop
fp_percent:
	# A second percent sign is the literal's way, taking a little more.
	bne	$t8, $at, fp_dir
	li	$a3, 0x20
	addiu	$t2, $t2, 1
	NOPS	FP_PCT
	b	fp_lit
	nop
fp_dir:
	# t9 the fill, v0 the width, a3 the letter below.
	li	$t9, 0x20
	li	$at, 0x30
	bne	$t8, $at, 4f
	move	$v0, $zero
	li	$t9, 0x30
4:	# The width's digits.
	addiu	$at, $t8, -0x30
	sltiu	$at, $at, 10
	beqz	$at, 5f
	nop
	sll	$at, $v0, 2
	addu	$v0, $v0, $at
	sll	$v0, $v0, 1
	addiu	$at, $t8, -0x30
	addu	$v0, $v0, $at
	addiu	$t2, $t2, 1
	lbu	$t8, 0($t2)
	NOPS	FP_WDIGIT
	b	4b
	nop
5:	addiu	$t2, $t2, 1
	li	$at, 0x64
	beq	$t8, $at, fp_d
	li	$at, 0x78
	beq	$t8, $at, fp_x
	li	$at, 0x58
	beq	$t8, $at, fp_X
	li	$at, 0x63
	beq	$t8, $at, fp_c
	li	$at, 0x73
	beq	$t8, $at, fp_s
	nop
	# Unknown: spaces for the width, or one.
fp_unknown:
	li	$t9, 0x20
	bnez	$v0, 6f
	move	$t5, $zero
	li	$v0, 1
6:	li	$t8, 0
	b	fp_fill
	nop
fp_d:
	lw	$t6, 0($t3)
	addiu	$t3, $t3, 4
	li	$t9, 0x20
	li	$t5, 1
	bgez	$t6, 1f
	li	$t8, 1
	subu	$t6, $zero, $t6
	li	$t5, 2
	li	$t8, 2
	NOPS	FP_NEG
1:	# t5 the field's size so far; the digits: t0 the count, by the powers of ten.
	li	$t0, 1
	lui	$t1, %hi(fp_pow10 + 4)
	addiu	$t1, $t1, %lo(fp_pow10 + 4)
2:	sltiu	$at, $t0, 10
	beqz	$at, 3f
	nop
	lw	$at, 0($t1)
	nop
	sltu	$at, $t6, $at
	bnez	$at, 3f
	addiu	$t1, $t1, 4
	b	2b
	addiu	$t0, $t0, 1
3:	addu	$t5, $t5, $t0
	addiu	$t5, $t5, -1
	move	$v1, $t0
	b	fp_fill
	li	$a3, 10
fp_x:
	lw	$t6, 0($t3)
	lui	$a0, %hi(fp_hexl)
	addiu	$a0, $a0, %lo(fp_hexl)
	b	fp_hex
	nop
fp_X:
	lw	$t6, 0($t3)
	lui	$a0, %hi(fp_hexu)
	addiu	$a0, $a0, %lo(fp_hexu)
fp_hex:
	NOPS	FP_X
	addiu	$t3, $t3, 4
	li	$t5, 1
	srl	$t0, $t6, 4
1:	beqz	$t0, 2f
	srl	$t0, $t0, 4
	b	1b
	addiu	$t5, $t5, 1
2:	move	$v1, $t5
	li	$t8, 3
	b	fp_fill
	li	$a3, 16
fp_c:
	lw	$t6, 0($t3)
	addiu	$t3, $t3, 4
	li	$t9, 0x20
	li	$t5, 1
	li	$t8, 4
	NOPS	FP_C
	b	fp_fill
	nop
fp_s:
	NOPS	FP_S
	lw	$t6, 0($t3)
	addiu	$t3, $t3, 4
	li	$t9, 0x20
	move	$t5, $zero
	beqz	$t6, fp_unknown
	nop
1:	addu	$t0, $t6, $t5
	lbu	$t0, 0($t0)
	nop
	beqz	$t0, 2f
	nop
	NOPS	FP_SLEN
	b	1b
	addiu	$t5, $t5, 1
2:	bnez	$t5, 3f
	li	$t8, 5
	NOPS	14
3:
fp_fill:
	# The width's fill: t8 says what comes after it.
	subu	$v0, $v0, $t5
1:	blez	$v0, 2f
	nop
	sb	$t9, 0($a1)
	addiu	$a1, $a1, 1
	sltu	$at, $a2, $a1
	bnez	$at, fp_cut
	addiu	$v0, $v0, -1
	NOPS	FP_FILL
	b	1b
	nop
2:	beqz	$t8, fp_next
	li	$at, 4
	beq	$t8, $at, fp_putc
	li	$at, 5
	beq	$t8, $at, fp_puts
	li	$at, 3
	beq	$t8, $at, fp_putx
	li	$at, 2
	bne	$t8, $at, fp_putd
	li	$at, 0x2d
	FPPUT	$at
	NOPS	FP_SIGN
fp_putd:
	# The digits from the top: v1 of them. a3 is the base: t1 walks back through the powers.
	sll	$at, $v1, 2
	lui	$t1, %hi(fp_pow10)
	addiu	$t1, $t1, %lo(fp_pow10)
	addu	$t1, $t1, $at
1:	addiu	$t1, $t1, -4
	lw	$at, 0($t1)
	nop
	divu	$zero, $t6, $at
	mflo	$t0
	multu	$t0, $at
	mflo	$at
	subu	$t6, $t6, $at
	addiu	$t0, $t0, 0x30
	NOPS	FP_DIGIT
	sb	$t0, 0($a1)
	addiu	$a1, $a1, 1
	sltu	$at, $a2, $a1
	bnez	$at, fp_cut
	addiu	$v1, $v1, -1
	bnez	$v1, 1b
	nop
	b	fp_next
	nop
fp_putx:
	sll	$t1, $v1, 2
1:	addiu	$t1, $t1, -4
	srlv	$t0, $t6, $t1
	andi	$t0, $t0, 15
	addu	$t0, $t0, $a0
	lbu	$t0, 0($t0)
	nop
	NOPS	FP_XDIGIT - 1
	sb	$t0, 0($a1)
	addiu	$a1, $a1, 1
	sltu	$at, $a2, $a1
	bnez	$at, fp_cut
	addiu	$v1, $v1, -1
	bnez	$v1, 1b
	nop
	b	fp_next
	nop
fp_putc:
	FPPUT	$t6
	b	fp_next
	nop
fp_puts:
1:	beqz	$t5, fp_next
	nop
	lbu	$t0, 0($t6)
	addiu	$t6, $t6, 1
	NOPS	FP_SCHAR
	FPPUT	$t0
	b	1b
	addiu	$t5, $t5, -1
fp_end:
	lw	$t7, FP_TEXT($t4)
	sb	$zero, 0($a1)
	subu	$v0, $a1, $t7
	sw	$v0, FP_N($t4)
	NOPS	FP_END
fp_ret:
	jr	$ra
	nop
fp_cut:
	lw	$t7, FP_TEXT($t4)
	nop
	subu	$v0, $a1, $t7
	sw	$v0, FP_N($t4)
	NOPS	FP_CUT
	jr	$ra
	li	$v0, -1
fp_refused:
	# As fntRefused, by the format's first byte.
	lbu	$t0, 0($t2)
	nop
	bnez	$t0, 1f
	nop
	NOPS	FP_REFEMPTY
1:	NOPS	FP_REFUSED
	jr	$ra
	li	$v0, -1
	.size libps_va_FntPrint, .-libps_va_FntPrint

	.section .rodata.fp_tables, "a", @progbits
	.align 2
fp_pow10:
	.word 1, 10, 100, 1000, 10000, 100000, 1000000, 10000000, 100000000, 1000000000
fp_hexl:
	.ascii "0123456789abcdef"
fp_hexu:
	.ascii "0123456789ABCDEF"
	.section .bss.libps_font_base, "aw", @nobits
	.align 2
	.globl libps_font_base
libps_font_base:
	.space 4
	.section .text.Matrix, "ax", @progbits

# GsSetLsMatrix(m): the GTE's rotation and translation.
	.globl GsSetLsMatrix
	.type GsSetLsMatrix, @function
GsSetLsMatrix:
	STUBTIME
	NOPS	3
	LOADROT	$a0
	PUTROT
	G
	lw	$t0, 20($a0)
	lw	$t1, 24($a0)
	lw	$t2, 28($a0)
	GAP	7
	CTC	8, 5
	CTC	9, 6
	CTC	10, 7
	NOPS	LSM_END
	jr	$ra
	nop
	.size GsSetLsMatrix, .-GsSetLsMatrix

# HOLD8 stores the GTE's rotation and translation at the register.
	.macro HOLD8 r
	CFC	8, 0
	CFC	9, 1
	sw	$t0, 0(\r)
	sw	$t1, 4(\r)
	CFC	8, 2
	CFC	9, 3
	sw	$t0, 8(\r)
	sw	$t1, 12(\r)
	CFC	8, 4
	nop
	sw	$t0, 16(\r)
	CFC	8, 5
	CFC	9, 6
	CFC	10, 7
	sw	$t0, 20(\r)
	sw	$t1, 24(\r)
	sw	$t2, 28(\r)
	.endm

# PUT8 loads them back from it.
	.macro PUT8 r
	lw	$t0, 0(\r)
	lw	$t1, 4(\r)
	CTC	8, 0
	CTC	9, 1
	lw	$t0, 8(\r)
	lw	$t1, 12(\r)
	CTC	8, 2
	CTC	9, 3
	lw	$t0, 16(\r)
	nop
	CTC	8, 4
	lw	$t0, 20(\r)
	lw	$t1, 24(\r)
	lw	$t2, 28(\r)
	nop
	CTC	8, 5
	CTC	9, 6
	CTC	10, 7
	.endm

# GsSetLightMatrix(m) loads the light matrix with the light directions times m's rotation,
# leaving the GTE's rotation and translation as they were.
	.globl GsSetLightMatrix
	.type GsSetLightMatrix, @function
GsSetLightMatrix:
	STUBTIME
	move	$a1, $a0
	lui	$a0, %hi(GsLIGHTWSMATRIX)
	addiu	$a0, $a0, %lo(GsLIGHTWSMATRIX)
	lui	$a2, %hi(light_scratch)
	addiu	$a2, $a2, %lo(light_scratch)
	lui	$a3, %hi(light_held)
	addiu	$a3, $a3, %lo(light_held)
	NOPS	LM_LEAD
	HOLD8	$a3
	G
	LOADROT	$a0
	GAP	14
	PUTROT
	MULMAT
	MULMATEND
	G
	GAP	15
	PUT8	$a3
	G
	lw	$t0, 0($a2)
	lw	$t1, 4($a2)
	lw	$t2, 8($a2)
	lw	$t3, 12($a2)
	lh	$t4, 16($a2)
	GAP	9
	CTC	8, 8
	CTC	9, 9
	CTC	10, 10
	CTC	11, 11
	CTC	12, 12
	NOPS	LM_END
	jr	$ra
	nop
	.size GsSetLightMatrix, .-GsSetLightMatrix

	.section .bss.light_scratch, "aw", @nobits
	.align 2
light_scratch:
	.space 32
light_held:
	.space 32
	.text

	.section .text.TransMatrix, "ax", @progbits
	.align 2
# TransposeMatrix(m0, m1): matrix.go has why it goes in this order.
	.globl TransposeMatrix
	.type TransposeMatrix, @function
TransposeMatrix:
	STUBTIME
	lh	$t0, 0($a0)
	lh	$t1, 6($a0)
	sh	$t0, 0($a1)
	sh	$t1, 2($a1)
	lh	$t0, 12($a0)
	lh	$t1, 2($a0)
	sh	$t0, 4($a1)
	sh	$t1, 6($a1)
	lh	$t0, 8($a0)
	lh	$t1, 14($a0)
	sh	$t0, 8($a1)
	sh	$t1, 10($a1)
	lh	$t0, 4($a0)
	lh	$t1, 10($a0)
	sh	$t0, 12($a1)
	sh	$t1, 14($a1)
	lh	$t0, 16($a0)
	move	$v0, $a1
	jr	$ra
	sh	$t0, 16($a1)
	.size TransposeMatrix, .-TransposeMatrix

# TransMatrix(m, v) sets m's translation and returns m.
	.globl TransMatrix
	.type TransMatrix, @function
TransMatrix:
	STUBTIME
	lw	$t0, 0($a1)
	lw	$t1, 4($a1)
	lw	$t2, 8($a1)
	sw	$t0, 20($a0)
	sw	$t1, 24($a0)
	sw	$t2, 28($a0)
	jr	$ra
	move	$v0, $a0
	.size TransMatrix, .-TransMatrix

	.section .text.PushMatrix, "ax", @progbits
	.align 2
# PushMatrix and PopMatrix save and restore the GTE's rotation and translation, on a
# stack of twenty, and do nothing past its ends.
	.globl PushMatrix
	.type PushMatrix, @function
PushMatrix:
	STUBTIME
	lui	$t6, %hi(gte_stack_n)
	lw	$t7, %lo(gte_stack_n)($t6)
	lui	$t5, %hi(gte_stack)
	sltiu	$at, $t7, 20
	beqz	$at, 1f
	addiu	$t5, $t5, %lo(gte_stack)
	sll	$at, $t7, 5
	addu	$t5, $t5, $at
	addiu	$t7, $t7, 1
	HOLD8	$t5
	sw	$t7, %lo(gte_stack_n)($t6)
1:	jr	$ra
	nop
	.size PushMatrix, .-PushMatrix

	.globl PopMatrix
	.type PopMatrix, @function
PopMatrix:
	STUBTIME
	lui	$t6, %hi(gte_stack_n)
	lw	$t7, %lo(gte_stack_n)($t6)
	lui	$t5, %hi(gte_stack)
	beqz	$t7, 1f
	addiu	$t5, $t5, %lo(gte_stack)
	addiu	$t7, $t7, -1
	sll	$at, $t7, 5
	addu	$t5, $t5, $at
	sw	$t7, %lo(gte_stack_n)($t6)
	NOPS	PP_LEAD
	PUT8	$t5
1:	jr	$ra
	nop
	.size PopMatrix, .-PopMatrix

	.section .text.rand, "ax", @progbits
	.align 2
# rand and srand: the C library's linear congruence, 0 until srand.
	.globl rand
	.type rand, @function
rand:
	STUBTIME
	lui	$t0, %hi(rand_state)
	lw	$t1, %lo(rand_state)($t0)
	lui	$t2, 0x41c6
	ori	$t2, $t2, 0x4e6d
	mult	$t1, $t2
	mflo	$t1
	addiu	$t1, $t1, 12345
	sw	$t1, %lo(rand_state)($t0)
	srl	$v0, $t1, 16
	jr	$ra
	andi	$v0, $v0, 0x7fff
	.size rand, .-rand

	.globl srand
	.type srand, @function
srand:
	STUBTIME
	lui	$t0, %hi(rand_state)
	jr	$ra
	sw	$a0, %lo(rand_state)($t0)
	.size srand, .-srand

# GetRCnt, StartRCnt and ResetRCnt, by the root counter's number in the spec's low half.
# Past the last they do nothing and return 0.
	.macro RCNT name
	.globl \name
	.type \name, @function
\name:
	andi	$a0, $a0, 0xffff
	sltiu	$at, $a0, 3
	beqz	$at, 1f
	move	$v0, $zero
	lui	$t0, 0x1f80
	.endm

	.section .text.RCnt, "ax", @progbits
	.align 2
	RCNT GetRCnt
	sll	$a0, $a0, 4
	addu	$t0, $t0, $a0
	lhu	$v0, 0x1100($t0)
	nop
1:	jr	$ra
	nop
	.size GetRCnt, .-GetRCnt

	RCNT StartRCnt
	lw	$t1, 0x1074($t0)
	li	$t2, 16
	sllv	$t2, $t2, $a0
	or	$t1, $t1, $t2
	sw	$t1, 0x1074($t0)
	li	$v0, 1
1:	jr	$ra
	nop
	.size StartRCnt, .-StartRCnt

	RCNT ResetRCnt
	sll	$a0, $a0, 4
	addu	$t0, $t0, $a0
	sw	$zero, 0x1100($t0)
	li	$v0, 1
1:	jr	$ra
	nop
	.size ResetRCnt, .-ResetRCnt

# SINCOS leaves the sine and cosine of the angle a in s and c, with the table's address
# in v1, and no branch on the quarter, so that every angle takes one time, and a
# negative one, its own way through, 10 cycles more. t7, t8, t9, at and v0 go. As sinCos
# in matrix.go: the table is read one step early in the second and fourth quarters.
	.macro SINBODY a, s, c
	sra	$t7, $t7, 31
	xor	$v0, $t7, \a
	andi	$v0, $v0, 0x3ff
	sll	$v0, $v0, 1
	addu	\s, $v1, $v0
	lh	\s, 0(\s)
	addiu	\c, $v1, 0x7fe
	subu	\c, \c, $v0
	lh	\c, 0(\c)
	sll	$at, \a, 20
	sra	$at, $at, 31
	xor	\s, \s, $at
	subu	\s, \s, $at
	xor	$v0, $at, $t7
	xor	\c, \c, $v0
	subu	\c, \c, $v0
	.endm

	.macro SINCOS a, s, c, neg, back
	bltz	\a, \neg
	sll	$t7, \a, 21
	SINBODY	\a, \s, \c
\back:
	.endm

# SINNEG is SINCOS's way for a negative angle, out of the way of the other.
	.macro SINNEG a, s, c, neg, back, pad=1
\neg:
	subu	$t9, $zero, \a
	sll	$t7, $t9, 21
	SINBODY	$t9, \s, \c
	NOPS	\pad
	b	\back
	subu	\s, $zero, \s
	.endm

# MUL12 is dst = a * b >> 12.
	.macro MUL12 dst, a, b
	mult	\a, \b
	mflo	\dst
	sra	\dst, \dst, 12
	.endm

	.section .text.RotMatrix, "ax", @progbits
	.align 2
# RotMatrix(r, m): matrix.go says what it makes. It goes in the order Y with Z first,
# then X.
	.globl RotMatrix
	.type RotMatrix, @function
RotMatrix:
	STUBTIME
	lh	$a2, 0($a0)
	lh	$a3, 2($a0)
	lh	$a0, 4($a0)
	lui	$v1, %hi(libps_sine_table)
	addiu	$v1, $v1, %lo(libps_sine_table)
	SINCOS	$a2, $t0, $t1, rm_nx, rm_bx
	SINCOS	$a3, $t2, $t3, rm_ny, rm_by
	SINCOS	$a0, $t4, $t5, rm_nz, rm_bz
	subu	$t6, $zero, $t2
	MUL12	$a2, $t6, $t5
	MUL12	$a3, $t6, $t4
	MUL12	$v0, $t3, $t5
	sh	$v0, 0($a1)
	subu	$t6, $zero, $t3
	MUL12	$v0, $t6, $t4
	sh	$v0, 2($a1)
	sh	$t2, 4($a1)
	MUL12	$v0, $t1, $t4
	MUL12	$v1, $t0, $a2
	subu	$v0, $v0, $v1
	sh	$v0, 6($a1)
	MUL12	$v0, $t1, $t5
	MUL12	$v1, $t0, $a3
	addu	$v0, $v0, $v1
	sh	$v0, 8($a1)
	subu	$t6, $zero, $t0
	MUL12	$v0, $t6, $t3
	sh	$v0, 10($a1)
	MUL12	$v0, $t0, $t4
	MUL12	$v1, $t1, $a2
	addu	$v0, $v0, $v1
	sh	$v0, 12($a1)
	MUL12	$v0, $t0, $t5
	MUL12	$v1, $t1, $a3
	subu	$v0, $v0, $v1
	sh	$v0, 14($a1)
	MUL12	$v0, $t1, $t3
	sh	$v0, 16($a1)
	jr	$ra
	move	$v0, $a1
	SINNEG	$a2, $t0, $t1, rm_nx, rm_bx
	SINNEG	$a3, $t2, $t3, rm_ny, rm_by, 0
	SINNEG	$a0, $t4, $t5, rm_nz, rm_bz
	.size RotMatrix, .-RotMatrix

# ROTCOL is a column of RotMatrixX and Z: rows ra and rb of the matrix at a1 become
# (c*a oa s*b) >> 12 and (s*a ob c*b) >> 12, oa and ob being subu or addu. ROTCOLY is
# RotMatrixY's, whose signs differ.
	.macro ROTCOL ra, rb, j, oa, ob
	lh	$a2, 2 * (3 * \ra + \j)($a1)
	lh	$a3, 2 * (3 * \rb + \j)($a1)
	mult	$t1, $a2
	mflo	$t2
	mult	$t0, $a3
	mflo	$t3
	mult	$t0, $a2
	mflo	$t4
	mult	$t1, $a3
	mflo	$t5
	\oa	$t2, $t2, $t3
	\ob	$t4, $t4, $t5
	sra	$t2, $t2, 12
	sra	$t4, $t4, 12
	sh	$t2, 2 * (3 * \ra + \j)($a1)
	sh	$t4, 2 * (3 * \rb + \j)($a1)
	.endm

	.macro ROTCOLY j
	lh	$a2, 2 * (0 + \j)($a1)
	lh	$a3, 2 * (6 + \j)($a1)
	mult	$t1, $a2
	mflo	$t2
	mult	$t0, $a3
	mflo	$t3
	mult	$t1, $a3
	mflo	$t4
	mult	$t0, $a2
	mflo	$t5
	addu	$t2, $t2, $t3
	subu	$t4, $t4, $t5
	sra	$t2, $t2, 12
	sra	$t4, $t4, 12
	sh	$t2, 2 * (0 + \j)($a1)
	sh	$t4, 2 * (6 + \j)($a1)
	.endm

	.macro ROTABOUT name, ra, rb, oa, ob, pad
	.globl \name
	.type \name, @function
\name:
	lui	$v1, %hi(libps_sine_table)
	addiu	$v1, $v1, %lo(libps_sine_table)
	SINCOS	$a0, $t0, $t1, \name\()_n, \name\()_b
	ROTCOL	\ra, \rb, 0, \oa, \ob
	ROTCOL	\ra, \rb, 1, \oa, \ob
	ROTCOL	\ra, \rb, 2, \oa, \ob
	NOPS	\pad
	jr	$ra
	move	$v0, $a1
	SINNEG	$a0, $t0, $t1, \name\()_n, \name\()_b
	.size \name, .-\name
	.endm

	.section .text.RotAbout, "ax", @progbits
	.align 2
	ROTABOUT RotMatrixX, 1, 2, subu, addu, 17
	ROTABOUT RotMatrixZ, 0, 1, subu, addu, 17

# Y's second row takes -s*a, which is c*b - s*a.
	.globl RotMatrixY
	.type RotMatrixY, @function
RotMatrixY:
	STUBTIME
	lui	$v1, %hi(libps_sine_table)
	addiu	$v1, $v1, %lo(libps_sine_table)
	SINCOS	$a0, $t0, $t1, rmy_n, rmy_b
	ROTCOLY	0
	ROTCOLY	1
	ROTCOLY	2
	NOPS	18
	jr	$ra
	move	$v0, $a1
	SINNEG	$a0, $t0, $t1, rmy_n, rmy_b
	.size RotMatrixY, .-RotMatrixY

	.section .bss.gte_stack, "aw", @nobits
	.align 2
gte_stack:
	.space 20 * 32
gte_stack_n:
	.space 4
	.section .bss.rand_state, "aw", @nobits
	.align 2
rand_state:
	.space 4

	.section .rodata.libps_sine_table, "a", @progbits
	.align 1
	.globl libps_sine_table
libps_sine_table:
	.hword 0, 6, 13, 19, 25, 31, 38, 44, 50, 57, 63, 69, 75, 82, 88, 94
	.hword 101, 107, 113, 119, 126, 132, 138, 144, 151, 157, 163, 170, 176, 182, 188, 195
	.hword 201, 207, 214, 220, 226, 232, 239, 245, 251, 257, 264, 270, 276, 283, 289, 295
	.hword 301, 308, 314, 320, 326, 333, 339, 345, 351, 358, 364, 370, 376, 383, 389, 395
	.hword 401, 408, 414, 420, 426, 433, 439, 445, 451, 458, 464, 470, 476, 483, 489, 495
	.hword 501, 508, 514, 520, 526, 533, 539, 545, 551, 557, 564, 570, 576, 582, 589, 595
	.hword 601, 607, 613, 620, 626, 632, 638, 644, 651, 657, 663, 669, 675, 682, 688, 694
	.hword 700, 706, 713, 719, 725, 731, 737, 744, 750, 756, 762, 768, 774, 781, 787, 793
	.hword 799, 805, 811, 818, 824, 830, 836, 842, 848, 854, 861, 867, 873, 879, 885, 891
	.hword 897, 904, 910, 916, 922, 928, 934, 940, 946, 953, 959, 965, 971, 977, 983, 989
	.hword 995, 1001, 1007, 1014, 1020, 1026, 1032, 1038, 1044, 1050, 1056, 1062, 1068, 1074, 1080, 1086
	.hword 1092, 1099, 1105, 1111, 1117, 1123, 1129, 1135, 1141, 1147, 1153, 1159, 1165, 1171, 1177, 1183
	.hword 1189, 1195, 1201, 1207, 1213, 1219, 1225, 1231, 1237, 1243, 1249, 1255, 1261, 1267, 1273, 1279
	.hword 1285, 1291, 1297, 1303, 1309, 1315, 1321, 1327, 1332, 1338, 1344, 1350, 1356, 1362, 1368, 1374
	.hword 1380, 1386, 1392, 1398, 1404, 1409, 1415, 1421, 1427, 1433, 1439, 1445, 1451, 1457, 1462, 1468
	.hword 1474, 1480, 1486, 1492, 1498, 1503, 1509, 1515, 1521, 1527, 1533, 1538, 1544, 1550, 1556, 1562
	.hword 1567, 1573, 1579, 1585, 1591, 1596, 1602, 1608, 1614, 1620, 1625, 1631, 1637, 1643, 1648, 1654
	.hword 1660, 1666, 1671, 1677, 1683, 1689, 1694, 1700, 1706, 1711, 1717, 1723, 1729, 1734, 1740, 1746
	.hword 1751, 1757, 1763, 1768, 1774, 1780, 1785, 1791, 1797, 1802, 1808, 1813, 1819, 1825, 1830, 1836
	.hword 1842, 1847, 1853, 1858, 1864, 1870, 1875, 1881, 1886, 1892, 1898, 1903, 1909, 1914, 1920, 1925
	.hword 1931, 1936, 1942, 1947, 1953, 1958, 1964, 1970, 1975, 1981, 1986, 1992, 1997, 2002, 2008, 2013
	.hword 2019, 2024, 2030, 2035, 2041, 2046, 2052, 2057, 2062, 2068, 2073, 2079, 2084, 2090, 2095, 2100
	.hword 2106, 2111, 2117, 2122, 2127, 2133, 2138, 2143, 2149, 2154, 2159, 2165, 2170, 2175, 2181, 2186
	.hword 2191, 2197, 2202, 2207, 2213, 2218, 2223, 2228, 2234, 2239, 2244, 2249, 2255, 2260, 2265, 2270
	.hword 2276, 2281, 2286, 2291, 2296, 2302, 2307, 2312, 2317, 2322, 2328, 2333, 2338, 2343, 2348, 2353
	.hword 2359, 2364, 2369, 2374, 2379, 2384, 2389, 2394, 2399, 2405, 2410, 2415, 2420, 2425, 2430, 2435
	.hword 2440, 2445, 2450, 2455, 2460, 2465, 2470, 2475, 2480, 2485, 2490, 2495, 2500, 2505, 2510, 2515
	.hword 2520, 2525, 2530, 2535, 2540, 2545, 2550, 2555, 2559, 2564, 2569, 2574, 2579, 2584, 2589, 2594
	.hword 2598, 2603, 2608, 2613, 2618, 2623, 2628, 2632, 2637, 2642, 2647, 2652, 2656, 2661, 2666, 2671
	.hword 2675, 2680, 2685, 2690, 2694, 2699, 2704, 2709, 2713, 2718, 2723, 2727, 2732, 2737, 2741, 2746
	.hword 2751, 2755, 2760, 2765, 2769, 2774, 2779, 2783, 2788, 2792, 2797, 2802, 2806, 2811, 2815, 2820
	.hword 2824, 2829, 2833, 2838, 2843, 2847, 2852, 2856, 2861, 2865, 2870, 2874, 2878, 2883, 2887, 2892
	.hword 2896, 2901, 2905, 2910, 2914, 2918, 2923, 2927, 2932, 2936, 2940, 2945, 2949, 2953, 2958, 2962
	.hword 2967, 2971, 2975, 2979, 2984, 2988, 2992, 2997, 3001, 3005, 3009, 3014, 3018, 3022, 3026, 3031
	.hword 3035, 3039, 3043, 3048, 3052, 3056, 3060, 3064, 3068, 3073, 3077, 3081, 3085, 3089, 3093, 3097
	.hword 3102, 3106, 3110, 3114, 3118, 3122, 3126, 3130, 3134, 3138, 3142, 3146, 3150, 3154, 3158, 3162
	.hword 3166, 3170, 3174, 3178, 3182, 3186, 3190, 3194, 3198, 3202, 3206, 3210, 3214, 3217, 3221, 3225
	.hword 3229, 3233, 3237, 3241, 3244, 3248, 3252, 3256, 3260, 3264, 3267, 3271, 3275, 3279, 3282, 3286
	.hword 3290, 3294, 3297, 3301, 3305, 3309, 3312, 3316, 3320, 3323, 3327, 3331, 3334, 3338, 3342, 3345
	.hword 3349, 3352, 3356, 3360, 3363, 3367, 3370, 3374, 3378, 3381, 3385, 3388, 3392, 3395, 3399, 3402
	.hword 3406, 3409, 3413, 3416, 3420, 3423, 3426, 3430, 3433, 3437, 3440, 3444, 3447, 3450, 3454, 3457
	.hword 3461, 3464, 3467, 3471, 3474, 3477, 3481, 3484, 3487, 3490, 3494, 3497, 3500, 3504, 3507, 3510
	.hword 3513, 3516, 3520, 3523, 3526, 3529, 3532, 3536, 3539, 3542, 3545, 3548, 3551, 3555, 3558, 3561
	.hword 3564, 3567, 3570, 3573, 3576, 3579, 3582, 3585, 3588, 3591, 3594, 3597, 3600, 3603, 3606, 3609
	.hword 3612, 3615, 3618, 3621, 3624, 3627, 3630, 3633, 3636, 3639, 3642, 3644, 3647, 3650, 3653, 3656
	.hword 3659, 3661, 3664, 3667, 3670, 3673, 3675, 3678, 3681, 3684, 3686, 3689, 3692, 3695, 3697, 3700
	.hword 3703, 3705, 3708, 3711, 3713, 3716, 3719, 3721, 3724, 3727, 3729, 3732, 3734, 3737, 3739, 3742
	.hword 3745, 3747, 3750, 3752, 3755, 3757, 3760, 3762, 3765, 3767, 3770, 3772, 3775, 3777, 3779, 3782
	.hword 3784, 3787, 3789, 3791, 3794, 3796, 3798, 3801, 3803, 3805, 3808, 3810, 3812, 3815, 3817, 3819
	.hword 3822, 3824, 3826, 3828, 3831, 3833, 3835, 3837, 3839, 3842, 3844, 3846, 3848, 3850, 3852, 3854
	.hword 3857, 3859, 3861, 3863, 3865, 3867, 3869, 3871, 3873, 3875, 3877, 3879, 3881, 3883, 3885, 3887
	.hword 3889, 3891, 3893, 3895, 3897, 3899, 3901, 3903, 3905, 3907, 3909, 3910, 3912, 3914, 3916, 3918
	.hword 3920, 3921, 3923, 3925, 3927, 3929, 3930, 3932, 3934, 3936, 3937, 3939, 3941, 3943, 3944, 3946
	.hword 3948, 3949, 3951, 3953, 3954, 3956, 3958, 3959, 3961, 3962, 3964, 3965, 3967, 3969, 3970, 3972
	.hword 3973, 3975, 3976, 3978, 3979, 3981, 3982, 3984, 3985, 3987, 3988, 3989, 3991, 3992, 3994, 3995
	.hword 3996, 3998, 3999, 4001, 4002, 4003, 4005, 4006, 4007, 4008, 4010, 4011, 4012, 4014, 4015, 4016
	.hword 4017, 4019, 4020, 4021, 4022, 4023, 4024, 4026, 4027, 4028, 4029, 4030, 4031, 4032, 4034, 4035
	.hword 4036, 4037, 4038, 4039, 4040, 4041, 4042, 4043, 4044, 4045, 4046, 4047, 4048, 4049, 4050, 4051
	.hword 4052, 4053, 4053, 4054, 4055, 4056, 4057, 4058, 4059, 4060, 4060, 4061, 4062, 4063, 4064, 4064
	.hword 4065, 4066, 4067, 4067, 4068, 4069, 4070, 4070, 4071, 4072, 4072, 4073, 4074, 4074, 4075, 4076
	.hword 4076, 4077, 4077, 4078, 4079, 4079, 4080, 4080, 4081, 4081, 4082, 4082, 4083, 4083, 4084, 4084
	.hword 4085, 4085, 4086, 4086, 4087, 4087, 4088, 4088, 4088, 4089, 4089, 4089, 4090, 4090, 4090, 4091
	.hword 4091, 4091, 4092, 4092, 4092, 4092, 4093, 4093, 4093, 4093, 4094, 4094, 4094, 4094, 4094, 4095
	.hword 4095, 4095, 4095, 4095, 4095, 4095, 4096, 4096, 4096, 4096, 4096, 4096, 4096, 4096, 4096, 4096
	.hword 4096
	.text

	.section .text.GsLinkObject4, "ax", @progbits
	.align 2
# GsLinkObject4(table, obj, n): tmd.go says what it does. A primitive takes 56 cycles, or
# 62 for the two modes that look at the flag for their size, and a new run a little more.
	.globl GsLinkObject4
	.type GsLinkObject4, @function
GsLinkObject4:
	STUBTIME
	NOPS	LINK_ENTRY
	sll	$t0, $a2, 5
	sll	$t1, $a2, 2
	subu	$t0, $t0, $t1
	addu	$t0, $t0, $a0
	sw	$t0, 8($a1)
	lui	$at, 0x8000
	or	$t0, $t0, $at
	lw	$t1, 16($t0)
	lw	$t2, 20($t0)
	move	$t3, $t1
	move	$t4, $zero
	lui	$t6, %hi(link_sizes)
	beqz	$t2, 5f
	addiu	$t6, $t6, %lo(link_sizes)
	move	$t5, $zero
	# A new run by the mode takes less than one by the flag alone. The length goes in the
	# run's first header at each primitive: the last time is the one that stays.
1:	lw	$t7, 0($t1)
	nop
	srl	$t7, $t7, 16
	xor	$at, $t7, $t5
	srl	$v0, $at, 8
	bnez	$v0, 7f
	sh	$t4, 0($t3)
	bnez	$at, 8f
	nop
2:	move	$t5, $t7
	addiu	$t4, $t4, 1
	srl	$t8, $t7, 8
	andi	$t8, $t8, 0xfd
	addiu	$t8, $t8, -0x20
	sltiu	$at, $t8, 30
	beqz	$at, 6f
	addu	$t9, $t6, $t8
	lbu	$t9, 0($t9)
	andi	$t8, $t8, 0xef
	beqz	$t9, 6f
	nop
	bnez	$t8, 3f
	addiu	$t2, $t2, -1
	andi	$at, $t7, 4
	sll	$at, $at, 1
	addu	$t9, $t9, $at
3:	addu	$t1, $t1, $t9
	NOPS	LINK_PRIM
	bnez	$t2, 1b
	nop
	sh	$t4, 0($t3)
	jr	$ra
	move	$v0, $zero
5:	NOPS	LINK_NONE
	sh	$zero, 0($t1)
	jr	$ra
	move	$v0, $zero
# A new run: by the mode, and by the flag alone.
7:	move	$t3, $t1
	b	2b
	move	$t4, $zero
8:	move	$t3, $t1
	nop
	b	2b
	move	$t4, $zero
# A mode with no size: the walk stays here, and what is left is the run's.
6:	addu	$t4, $t4, $t2
	addiu	$t4, $t4, -1
	sh	$t4, 0($t3)
	srl	$a0, $t7, 8
	j	libps_link_unassigned
	move	$a1, $t2
	.size GsLinkObject4, .-GsLinkObject4

	.section .rodata.link_sizes, "a", @progbits
link_sizes:
	.byte 16, 16, 0, 0, 24, 28, 0, 0, 20, 16, 0, 0, 32, 32, 0, 0
	.byte 20, 24, 0, 0, 28, 36, 0, 0, 24, 28, 0, 0, 36, 44
	.text

# The 2D sorts. libps_place is what they add to a point: x and y of the GTE's screen
# offset in the GsOFSGTE mode, 0 in the GsOFSGPU mode (sort.go keeps it).
#
# SPRITEBODY sorts the sprite at a0 into the table at a1, entry a2, with its attribute
# in t0, its size in t1 and its corner in t3.
	.macro SPRITEBODY
	lui	$t9, %hi(GsOUT_PACKET_P)
	lw	$v0, %lo(GsOUT_PACKET_P)($t9)
	lw	$t4, 12($a0)
	lw	$t5, 16($a0)
	srl	$t6, $t0, 28
	andi	$t6, $t6, 3
	sll	$t6, $t6, 5
	srl	$t7, $t0, 24
	andi	$t7, $t7, 3
	sll	$t7, $t7, 7
	or	$t6, $t6, $t7
	andi	$t7, $t4, 0x1f
	or	$t6, $t6, $t7
	ori	$t6, $t6, 0x200
	lui	$t7, 0xe100
	or	$t6, $t6, $t7
	sw	$t6, 4($v0)
	srl	$t6, $t0, 29
	andi	$t6, $t6, 2
	srl	$t7, $t0, 6
	andi	$t7, $t7, 1
	or	$t6, $t6, $t7
	ori	$t6, $t6, 0x64
	sll	$t6, $t6, 24
	lw	$t7, 20($a0)
	nop
	sll	$t7, $t7, 8
	srl	$t7, $t7, 8
	or	$t6, $t6, $t7
	sw	$t6, 8($v0)
	lui	$t6, %hi(libps_place)
	lw	$t6, %lo(libps_place)($t6)
	nop
	addu	$t7, $t3, $t6
	andi	$t7, $t7, 0xffff
	srl	$t8, $t3, 16
	srl	$at, $t6, 16
	addu	$t8, $t8, $at
	sll	$t8, $t8, 16
	or	$t7, $t7, $t8
	sw	$t7, 12($v0)
	srl	$t6, $t5, 16
	sll	$t6, $t6, 6
	srl	$t7, $t5, 4
	andi	$t7, $t7, 0x3f
	or	$t6, $t6, $t7
	sll	$t6, $t6, 16
	srl	$t7, $t4, 16
	or	$t6, $t6, $t7
	sw	$t6, 16($v0)
	sw	$t1, 20($v0)
	lw	$t6, 4($a1)
	andi	$a2, $a2, 0xffff
	sll	$a2, $a2, 2
	addu	$t6, $t6, $a2
	lw	$t7, 0($t6)
	lui	$at, 0xff00
	and	$t8, $t7, $at
	xor	$t7, $t7, $t8
	lui	$at, 0x0500
	or	$t7, $t7, $at
	sw	$t7, 0($v0)
	sll	$at, $v0, 8
	srl	$at, $at, 8
	or	$t8, $t8, $at
	sw	$t8, 0($t6)
	addiu	$v0, $v0, 24
	sw	$v0, %lo(GsOUT_PACKET_P)($t9)
	.endm

	.section .text.Sprites, "ax", @progbits
	.align 2
	.globl GsSortFastSprite
	.type GsSortFastSprite, @function
GsSortFastSprite:
	STUBTIME
	lw	$t0, 0($a0)
	lw	$t1, 8($a0)
	bltz	$t0, 1f
	andi	$t2, $t1, 0xffff
	beqz	$t2, 1f
	srl	$t2, $t1, 16
	beqz	$t2, 1f
	nop
	lw	$t3, 4($a0)
	NOPS	FAST_PAD
	SPRITEBODY
1:	jr	$ra
	nop
	.size GsSortFastSprite, .-GsSortFastSprite

# GsSortSprite makes a sprite with GsROTOFF, or with no turn, a scale of 1 and no flip.
# A quad is left to Go.
	.globl GsSortSprite
	.type GsSortSprite, @function
GsSortSprite:
	STUBTIME
	NOPS	SPRITE_ENTRY
	lw	$t0, 0($a0)
	lw	$t1, 8($a0)
	lw	$t2, 32($a0)
	lw	$t3, 28($a0)
	lui	$t4, 0x1000
	ori	$t4, $t4, 0x1000
	sll	$t5, $t0, 4
	bltz	$t5, 2f
	lui	$t5, 0x00c0
	and	$t5, $t0, $t5
	or	$t5, $t5, $t2
	xor	$t6, $t3, $t4
	or	$t5, $t5, $t6
	bnez	$t5, 3f
	nop
	b	4f
	nop
2:	NOPS	8
4:	bltz	$t0, 1f
	andi	$t2, $t1, 0xffff
	beqz	$t2, 1f
	srl	$t2, $t1, 16
	beqz	$t2, 1f
	lw	$t3, 4($a0)
	lw	$t4, 24($a0)
	nop
	subu	$t5, $t3, $t4
	andi	$t5, $t5, 0xffff
	srl	$t3, $t3, 16
	srl	$t4, $t4, 16
	subu	$t3, $t3, $t4
	sll	$t3, $t3, 16
	or	$t3, $t3, $t5
	NOPS	SPRITE_PAD
	SPRITEBODY
1:	jr	$ra
	nop
3:	bltz	$t0, 1b
	andi	$t4, $t1, 0xffff
	beqz	$t4, 1b
	srl	$t4, $t1, 16
	beqz	$t4, 1b
	nop
# The quad of GsSortSprite: spec 20 says what it makes. The GTE's rotation is the turn by
# Rotate/360 taken in 16 bits, scaled, a corner's z is 0 and its depth the projection
# distance, and each way takes what it does on Sony's, however the angle's sign falls.
sprite_quad:
	NOPS	33
	G
	move	$t5, $zero
	li	$t6, 4096
	beqz	$t2, 2f
	nop
	li	$at, 360
	div	$zero, $t2, $at
	mflo	$t7
	sll	$t7, $t7, 16
	sra	$t7, $t7, 16
	sra	$a3, $t7, 31
	xor	$t9, $t7, $a3
	subu	$t9, $t9, $a3
	lui	$v1, %hi(libps_sine_table)
	addiu	$v1, $v1, %lo(libps_sine_table)
	sll	$t7, $t9, 21
	SINBODY	$t9, $t5, $t6
	xor	$t5, $t5, $a3
	subu	$t5, $t5, $a3
	GAP	QUAD_TURN
2:	G
	# t7 and t8: the top row's elements. t9: the second's first. v1: its second. a3: R33.
	lui	$at, 0x1000
	ori	$at, $at, 0x1000
	beq	$t3, $at, 3f
	li	$a3, 4096
	lh	$t4, 28($a0)
	lh	$at, 30($a0)
	subu	$v0, $zero, $t5
	mult	$t6, $t4
	mflo	$t7
	sra	$t7, $t7, 12
	mult	$v0, $at
	mflo	$t8
	sra	$t8, $t8, 12
	mult	$t5, $t4
	mflo	$t9
	sra	$t9, $t9, 12
	mult	$t6, $at
	mflo	$v1
	sra	$v1, $v1, 12
	move	$a3, $zero
	GAP	QUAD_SCALE
	b	4f
	nop
3:	move	$t7, $t6
	subu	$t8, $zero, $t5
	move	$t9, $t5
	move	$v1, $t6
4:	G
	andi	$t7, $t7, 0xffff
	sll	$t8, $t8, 16
	or	$t7, $t7, $t8
	sll	$t8, $t9, 16
	andi	$t9, $v1, 0xffff
	CFC	12, 26
	GAP	28
	CTC	15, 0
	CTC	24, 1
	CTC	25, 2
	CTC	0, 3
	CTC	7, 4
	G
	lh	$t5, 4($a0)
	lh	$t6, 6($a0)
	GAP	7
	CTC	13, 5
	CTC	14, 6
	CTC	12, 7
	G
	lh	$t7, 24($a0)
	lh	$t8, 26($a0)
	lhu	$t9, 8($a0)
	lhu	$v1, 10($a0)
	subu	$t9, $t9, $t7
	subu	$v1, $v1, $t8
	subu	$t7, $zero, $t7
	subu	$t8, $zero, $t8
	andi	$t7, $t7, 0xffff
	andi	$t9, $t9, 0xffff
	sll	$t8, $t8, 16
	sll	$v1, $v1, 16
	lui	$t4, %hi(quad_corners)
	addiu	$t4, $t4, %lo(quad_corners)
	or	$t5, $t7, $t8
	or	$t6, $t9, $t8
	or	$t7, $t7, $v1
	or	$t9, $t9, $v1
	sw	$t5, 0($t4)
	sw	$t6, 8($t4)
	sw	$t7, 16($t4)
	sw	$t9, 24($t4)
	lui	$t9, %hi(GsOUT_PACKET_P)
	lw	$a3, %lo(GsOUT_PACKET_P)($t9)
	GAP	43
	lwc2	$0, 0($t4)
	lwc2	$1, 4($t4)
	lwc2	$2, 8($t4)
	lwc2	$3, 12($t4)
	lwc2	$4, 16($t4)
	lwc2	$5, 20($t4)
	nop
	RTPT
	NOPS	3
	swc2	$12, 8($a3)
	swc2	$13, 16($a3)
	swc2	$14, 24($a3)
	CFLAG
	lwc2	$0, 24($t4)
	lwc2	$1, 28($t4)
	nop
	RTPS
	NOPS	3
	swc2	$14, 32($a3)
	swc2	$8, 32($t4)
	CFLAG
	mfc2	$t7, $19
	G
	# The packet's other words: code and color, then the texture's.
	lw	$t5, 20($a0)
	srl	$t6, $t0, 29
	andi	$t6, $t6, 2
	srl	$t7, $t0, 6
	andi	$t7, $t7, 1
	or	$t6, $t6, $t7
	ori	$t6, $t6, 0x2c
	sll	$t6, $t6, 24
	sll	$t5, $t5, 8
	srl	$t5, $t5, 8
	or	$t5, $t5, $t6
	sw	$t5, 4($a3)
	lbu	$t5, 14($a0)
	lhu	$t6, 8($a0)
	lbu	$t7, 15($a0)
	andi	$t6, $t6, 255
	addu	$t6, $t6, $t5
	addiu	$t6, $t6, -1
	andi	$t6, $t6, 0xff
	# t1 u of the first and third, t4 of the second and fourth.
	move	$t1, $t5
	sll	$at, $t0, 8
	bgez	$at, 5f
	move	$t4, $t6
	move	$t1, $t6
	move	$t4, $t5
5:	lhu	$t6, 10($a0)
	nop
	andi	$t6, $t6, 255
	addu	$t6, $t6, $t7
	addiu	$t6, $t6, -1
	andi	$t6, $t6, 0xff
	# t5 v of the first two, t7 of the others.
	move	$t5, $t7
	sll	$at, $t0, 9
	bgez	$at, 6f
	move	$t7, $t6
	move	$t5, $t6
	lbu	$t7, 15($a0)
6:	sll	$t5, $t5, 8
	sll	$t7, $t7, 8
	lh	$t6, 18($a0)
	lh	$at, 16($a0)
	sll	$t6, $t6, 6
	sra	$at, $at, 4
	andi	$at, $at, 0x3f
	or	$t6, $t6, $at
	sll	$t6, $t6, 16
	or	$t6, $t6, $t5
	or	$t6, $t6, $t1
	sw	$t6, 12($a3)
	lhu	$t6, 12($a0)
	srl	$at, $t0, 28
	andi	$t6, $t6, 0x1f
	andi	$at, $at, 3
	sll	$at, $at, 5
	or	$t6, $t6, $at
	srl	$at, $t0, 24
	andi	$at, $at, 3
	sll	$at, $at, 7
	or	$t6, $t6, $at
	sll	$t6, $t6, 16
	or	$t6, $t6, $t5
	or	$t6, $t6, $t4
	sw	$t6, 20($a3)
	or	$t6, $t7, $t1
	sw	$t6, 28($a3)
	or	$t6, $t7, $t4
	sw	$t6, 36($a3)
	lw	$t6, 4($a1)
	andi	$a2, $a2, 0xffff
	sll	$a2, $a2, 2
	addu	$t6, $t6, $a2
	lw	$t7, 0($t6)
	lui	$at, 0xff00
	and	$t8, $t7, $at
	xor	$t7, $t7, $t8
	lui	$at, 0x0900
	or	$t7, $t7, $at
	sw	$t7, 0($a3)
	sll	$at, $a3, 8
	srl	$at, $at, 8
	or	$t8, $t8, $at
	sw	$t8, 0($t6)
	addiu	$a3, $a3, 40
	sw	$a3, %lo(GsOUT_PACKET_P)($t9)
	GAP	QUAD_END
	jr	$ra
	nop
	.section .bss.quad_corners, "aw", @nobits
	.align 2
quad_corners:
	.space 36
	.section .text.Sprites, "ax", @progbits
	.size GsSortSprite, .-GsSortSprite

	.section .text.GsSortBoxFill, "ax", @progbits
	.align 2
# GsSortBoxFill: the mode word and a filled rectangle.
	.globl GsSortBoxFill
	.type GsSortBoxFill, @function
GsSortBoxFill:
	STUBTIME
	lw	$t0, 0($a0)
	nop
	NOPS	3
	bltz	$t0, 1f
	lui	$t9, %hi(GsOUT_PACKET_P)
	lw	$v0, %lo(GsOUT_PACKET_P)($t9)
	NOPS	BOX_PAD
	srl	$t6, $t0, 28
	andi	$t6, $t6, 3
	sll	$t6, $t6, 5
	ori	$t6, $t6, 0x200
	lui	$t7, 0xe100
	or	$t6, $t6, $t7
	sw	$t6, 4($v0)
	srl	$t6, $t0, 29
	andi	$t6, $t6, 2
	ori	$t6, $t6, 0x60
	sll	$t6, $t6, 24
	lw	$t7, 12($a0)
	lw	$t3, 4($a0)
	sll	$t7, $t7, 8
	srl	$t7, $t7, 8
	or	$t6, $t6, $t7
	sw	$t6, 8($v0)
	lui	$t6, %hi(libps_place)
	lw	$t6, %lo(libps_place)($t6)
	nop
	addu	$t7, $t3, $t6
	andi	$t7, $t7, 0xffff
	srl	$t8, $t3, 16
	srl	$at, $t6, 16
	addu	$t8, $t8, $at
	sll	$t8, $t8, 16
	or	$t7, $t7, $t8
	sw	$t7, 12($v0)
	lw	$t1, 8($a0)
	nop
	sw	$t1, 16($v0)
	lw	$t6, 4($a1)
	andi	$a2, $a2, 0xffff
	sll	$a2, $a2, 2
	addu	$t6, $t6, $a2
	lw	$t7, 0($t6)
	lui	$at, 0xff00
	and	$t8, $t7, $at
	xor	$t7, $t7, $t8
	lui	$at, 0x0400
	or	$t7, $t7, $at
	sw	$t7, 0($v0)
	sll	$at, $v0, 8
	srl	$at, $at, 8
	or	$t8, $t8, $at
	sw	$t8, 0($t6)
	addiu	$v0, $v0, 20
	sw	$v0, %lo(GsOUT_PACKET_P)($t9)
1:	jr	$ra
	nop
	.size GsSortBoxFill, .-GsSortBoxFill

	.section .bss.libps_place, "aw", @nobits
	.align 2
	.globl libps_place
libps_place:
	.space 4
	.text

	.section .text.gteMIMefunc, "ax", @progbits
	.align 2
# gteMIMefunc(otp, dfp, n, p): each element of the vectors at otp becomes itself plus
# its element at dfp times p, by GPL.
	.globl gteMIMefunc
	.type gteMIMefunc, @function
gteMIMefunc:
	STUBTIME
	mtc2	$a3, $8
	blez	$a2, 3f
	lh	$t0, 0($a1)
	lh	$t1, 2($a1)
	lh	$t2, 4($a1)
1:	mtc2	$t0, $9
	mtc2	$t1, $10
	mtc2	$t2, $11
	addiu	$a2, $a2, -1
	addiu	$a1, $a1, 8
	GPF
	G
	lh	$t3, 0($a0)
	lh	$t4, 2($a0)
	lh	$t5, 4($a0)
	GAP	5
	mfc2	$t0, $9
	mfc2	$t1, $10
	mfc2	$t2, $11
	addu	$t3, $t3, $t0
	addu	$t4, $t4, $t1
	addu	$t5, $t5, $t2
	sh	$t3, 0($a0)
	sh	$t4, 2($a0)
	sh	$t5, 4($a0)
	blez	$a2, 2f
	addiu	$a0, $a0, 8
	lh	$t0, 0($a1)
	lh	$t1, 2($a1)
	lh	$t2, 4($a1)
	b	1b
	nop
2:	NOPS	MIME_END
3:	jr	$ra
	nop
	.size gteMIMefunc, .-gteMIMefunc
