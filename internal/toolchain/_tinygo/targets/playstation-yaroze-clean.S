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

	.macro NOPS n
	.rept \n
	nop
	.endr
	.endm

# An entry of the resident image is reached through a jump and its delay, which the
# routines' times below allow for. STUBTIME spends those two when linked in.
	.macro STUBTIME
#ifndef CLEAN_LIBRARY_ONLY
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
	.set GSO_ENTRY, 51
	.set GSO_HIDDEN, 22

	.macro G
98:
	.endm
	.macro GAP n
	.space (\n) * 4 - (. - 98b)
98:
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
	j	gso_run - 4 * (\ta - 2 - GSO_RUN - GSO_BASE)
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
	.endif
	GAP	1
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
5:	lui	$t9, %hi(gso_unlit)
	addu	$t9, $t9, $t7
	addiu	$t9, $t9, %lo(gso_unlit)
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

gso_div:
	j	libps_gso_div
	addiu	$sp, $sp, 56
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
	sh	$zero, 18($a2)
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
	NOPS	FP_XDIGIT
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
	NOPS	4
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
