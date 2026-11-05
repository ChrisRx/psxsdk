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

	.set MIME_PAD, 4
	.set FAST_PAD, 14
	.set SPRITE_ENTRY, 0
	.set SPRITE_PAD, 49
	.set BOX_PAD, 27
	.set LINK_ENTRY, 9
	.set LINK_PRIM, 6

	.macro NOPS n
	.rept \n
	nop
	.endr
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

# PREFETCH finds the next polygon's vertices. The load of each is in the delay of the
# one before it, so that it takes 9 instructions, and a loop does this in the time the
# GTE takes over the polygon before.
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

# POLY is GsSortObject4's loop for a run of one kind of polygon, entered at gso_NAME
# with the first polygon in a0, the run's length in a1, the header in t6 and the
# attribute's translucency in a3, and it goes back to gso_run. Through the loop a0 is
# the next polygon, a1 how many are left, a2 where the packet goes, a3 the first color
# word's top byte and mask (see POLYCODE), t0 the vertices, t1 the ordering table, t2 the
# shift, t3 to t5 the next polygon's vertices and v1 the normals.
#
# light is 0 for an unlit body, 1 lit, 2 fogged and 3 lit with lighting off; t, g and q
# whether it is textured, gouraud and a quad; flat that a gouraud body makes a flat
# packet. A polygon has three ways out: FLAG says its points are bad, it turns away, or
# it is sorted. pe, pb and pa spend what those three take beyond this code on Sony's,
# and pr what a run does. pq is what stands in the fourth corner's wait for the GTE, and
# pz what a quad takes after its fourth point is found good. early puts the packet's
# filling in with what all three ways do. pt spends what only a polygon not sorted takes,
# for the kinds that jump past it.
	.macro POLY name, light, t, g, q, flat, pe, pb, pa, pr, pq, pz, early, pt
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
	# Lighting gouraud corners keeps the registers until late, so the next polygon's
	# vertices are found later on.
	.set P_LATE, \g * (\light == 1 || \light == 2)
gso_\name\():
	NOPS	\pr
	POLYCODE \light, \t, \g, \flat
	PREFETCH P_V0, P_V1, P_V2
1:	lwc2	$0, 0($t3)
2:	lwc2	$1, 4($t3)
	lwc2	$2, 0($t4)
	lwc2	$3, 4($t4)
	lwc2	$4, 0($t5)
	lwc2	$5, 4($t5)
	addiu	$a0, $a0, P_SIZE
	addiu	$a1, $a1, -1
	RTPT
	# The RTPT's 23 cycles: what's here is in all three ways.
	.if P_LATE
	lhu	$t7, P_N0 + P_BACK($a0)
	lhu	$t8, P_N1 + P_BACK($a0)
	lhu	$t9, P_N2 + P_BACK($a0)
	sll	$t7, $t7, 3
	sll	$t8, $t8, 3
	sll	$t9, $t9, 3
	addu	$t7, $t7, $v1
	addu	$t8, $t8, $v1
	addu	$t9, $t9, $v1
	.if \t
	NOPS	3
	.else
	lw	$v0, P_COL + P_BACK($a0)
	nop
	or	$v0, $v0, $a3
	.endif
	.else
	PREFETCH P_V0, P_V1, P_V2
	.if \q
	lhu	$t9, P_V3 + P_BACK($a0)
	nop
	sll	$t9, $t9, 3
	.else
	NOPS	3
	.endif
	.if (\light == 1 || \light == 2) && !\q
	# The normal and color of a lit flat triangle.
	lhu	$t7, P_N0 + P_BACK($a0)
	.if \t
	nop
	.else
	lw	$t8, P_COL + P_BACK($a0)
	.endif
	sll	$t7, $t7, 3
	addu	$t7, $t7, $v1
	.if !\t
	or	$t8, $t8, $a3
	.endif
	.endif
	.endif
	.if \early == 1
	POLYCOPY \light, \t, \g, $t6, $v0
	.endif
	.if \early == 2
	POLYTEX \t, $t6, $v0
	.endif
	NOPS	\pe
	CFLAG
	NCLIP
	bltz	$t6, 7f
	.if \q && !P_LATE
	addu	$t9, $t9, $t0
	.else
	nop
	.endif
	swc2	$12, 8($a2)
	swc2	$13, 8 + 4 * P_S($a2)
	swc2	$14, 8 + 8 * P_S($a2)
	.if \q && !P_LATE
	lwc2	$0, 0($t9)
	lwc2	$1, 4($t9)
	.endif
	NOPS	\pb
	mfc2	$t6, $24
	nop
	blez	$t6, 7f
	nop
	NOPS	\pa
	.if P_LATE && \q
	POLYLATEQUAD \light, \t, \pq, \pz
	.else
	.if \q
	POLYFOURTH \light, \t, \g, \pq, \pz, \early
	.else
	AVSZ3
	.if !\early
	POLYCOPY \light, \t, \g, $t6, $v0
	.endif
	.endif
	.if \light == 1 || \light == 2
	# The polygon's entry, then its lighting, with the link in the time that takes.
	.if \g && !\t
	mtc2	$v0, $6
	.endif
	lui	$at, 0xff00
	.if \q || !\t || \early
	NOPS	2
	.endif
	mfc2	$t6, $7
	lui	$v0, P_PLEN << 8
	srav	$t6, $t6, $t2
	sll	$t6, $t6, 2
	addu	$t6, $t6, $t1
	.if \g && \t
	mtc2	$a3, $6
	.endif
	.if !\g
	.if \t
	mtc2	$a3, $6
	.else
	mtc2	$t8, $6
	.endif
	.endif
	.if \g
	lwc2	$0, 0($t7)
	lwc2	$1, 4($t7)
	lwc2	$2, 0($t8)
	lwc2	$3, 4($t8)
	lwc2	$4, 0($t9)
	lwc2	$5, 4($t9)
	.else
	lwc2	$0, 0($t7)
	lwc2	$1, 4($t7)
	.endif
	lw	$t7, 0($t6)
	nop
	.if \g
	LIGHTTHREE \light
	.else
	LIGHTONE \light
	.endif
	POLYLINK
	.if \g
	PREFETCH P_V0, P_V1, P_V2
	NOPS	3 - 2 * (\light == 2)
	mfc2	$t6, $20
	mfc2	$t7, $21
	mfc2	$t8, $22
	sw	$t6, 4($a2)
	sw	$t7, 4 + 4 * P_S($a2)
	sw	$t8, 4 + 8 * P_S($a2)
	# Sorted, the last of a run takes a little longer to leave.
	addiu	$a2, $a2, (P_PLEN + 1) * 4
	bnez	$a1, 2b
	lwc2	$0, 0($t3)
	NOPS	5
	j	gso_run
	nop
	.else
	NOPS	1 - (\light == 2)
	mfc2	$t6, $22
	addiu	$a2, $a2, (P_PLEN + 1) * 4
	sw	$t6, 4 - (P_PLEN + 1) * 4($a2)
	.endif
	.else
	# The entry, then the link, its slots all used.
	lui	$at, 0xff00
	sll	$t8, $a2, 8
	srl	$t8, $t8, 8
	mfc2	$t6, $7
	lui	$v0, P_PLEN << 8
	srav	$t6, $t6, $t2
	sll	$t6, $t6, 2
	addu	$t6, $t6, $t1
	lw	$t7, 0($t6)
	addiu	$a2, $a2, (P_PLEN + 1) * 4
	and	$at, $t7, $at
	xor	$t7, $t7, $at
	or	$t7, $t7, $v0
	sw	$t7, -(P_PLEN + 1) * 4($a2)
	or	$at, $at, $t8
	sw	$at, 0($t6)
	.endif
	.endif
7:
	.if P_LATE
	PREFETCH P_V0, P_V1, P_V2
	NOPS	\pt
	.endif
8:	bnez	$a1, 2b
	lwc2	$0, 0($t3)
	j	gso_run
	nop
	.endm

# POLYLINK makes the packet at a2 the first of the entry at t6, whose word is in t7,
# with the length's top byte in v0 and 0xff000000 in at.
	.macro POLYLINK
	and	$at, $t7, $at
	xor	$t7, $t7, $at
	or	$t7, $t7, $v0
	sw	$t7, 0($a2)
	sll	$t8, $a2, 8
	srl	$t8, $t8, 8
	or	$at, $at, $t8
	sw	$at, 0($t6)
	.endm

# POLYFOURTH takes a quad's fourth point, whose address is in t9, to the packet,
# filling in the packet in the time the GTE takes, and leaves the depth going. A lit
# flat quad finds its normal and color here too, in t7 and t8. A bad point ends the
# polygon.
	.macro POLYFOURTH light, t, g, pq, pz, early
	RTPS
	.if (\light == 1 || \light == 2) && !\g
	lhu	$t7, P_N0 + P_BACK($a0)
	.if \t
	nop
	.else
	lw	$t8, P_COL + P_BACK($a0)
	.endif
	sll	$t7, $t7, 3
	addu	$t7, $t7, $v1
	.if !\t
	or	$t8, $t8, $a3
	.endif
	.endif
	.if \early == 0
	POLYCOPY \light, \t, \g, $t6, $v0
	.endif
	.if \early == 2
	POLYCOL \light, \t, \g, $t6, $v0
	.endif
	NOPS	\pq
	CFLAG
	swc2	$14, 8 + 12 * P_S($a2)
	bltz	$t6, 7f
	AVSZ4
	NOPS	\pz
	.endm

# POLYLATEQUAD is the rest of a lit gouraud quad, sorted: lighting comes first, with
# the time it takes filled by what the rest will want, then the fourth point and the
# depth, then the fourth corner's lighting, with the link in its time.
	.macro POLYLATEQUAD light, t, pq, pz
	.if \t
	mtc2	$a3, $6
	.else
	mtc2	$v0, $6
	.endif
	lwc2	$0, 0($t7)
	lwc2	$1, 4($t7)
	lwc2	$2, 0($t8)
	lwc2	$3, 4($t8)
	lwc2	$4, 0($t9)
	lwc2	$5, 4($t9)
	lhu	$t7, P_V3 + P_BACK($a0)
	lhu	$v0, P_N3 + P_BACK($a0)
	LIGHTTHREE \light
	sll	$t7, $t7, 3
	addu	$t7, $t7, $t0
	sll	$v0, $v0, 3
	addu	$v0, $v0, $v1
	PREFETCH P_V0, P_V1, P_V2
	.if \t
	POLYCOPY \light, \t, 1, $t8, $t9
	NOPS	1 - 2 * (\light == 2)
	.else
	NOPS	7 - 2 * (\light == 2)
	.endif
	mfc2	$t6, $20
	mfc2	$t8, $21
	mfc2	$t9, $22
	lwc2	$0, 0($t7)
	lwc2	$1, 4($t7)
	sw	$t6, 4($a2)
	sw	$t8, 4 + 4 * P_S($a2)
	RTPS
	sw	$t9, 4 + 8 * P_S($a2)
	lui	$t8, P_PLEN << 8
	lui	$t9, 0xff00
	NOPS	5
	NOPS	\pq
	CFLAG
	swc2	$14, 8 + 12 * P_S($a2)
	bltz	$t6, 8f
	AVSZ4
	NOPS	\pz
	nop
	nop
	nop
	mfc2	$t6, $7
	nop
	srav	$t6, $t6, $t2
	sll	$t6, $t6, 2
	addu	$t6, $t6, $t1
	lwc2	$0, 0($v0)
	lwc2	$1, 4($v0)
	lw	$t7, 0($t6)
	nop
	LIGHTONE \light
	and	$at, $t7, $t9
	xor	$t7, $t7, $at
	or	$t7, $t7, $t8
	sw	$t7, 0($a2)
	sll	$t8, $a2, 8
	srl	$t8, $t8, 8
	or	$at, $at, $t8
	sw	$at, 0($t6)
	NOPS	1 - (\light == 2)
	mfc2	$t6, $22
	addiu	$a2, $a2, (P_PLEN + 1) * 4
	b	8f
	sw	$t6, 4 + 12 * P_S - (P_PLEN + 1) * 4($a2)
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

# POLYCOPY fills in what the packet holds from the model, leaving the colors of a lit
# polygon to its lighting.
	.macro POLYCOPY light, t, g, r0, r1
	POLYTEX \t, \r0, \r1
	POLYCOL \light, \t, \g, \r0, \r1
	.endm

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

	.macro POLYCOL light, t, g, r0, r1
	.if \light == 0
	lw	\r0, P_COL + P_BACK($a0)
	nop
	.if \t
	sll	\r0, \r0, 8
	srl	\r0, \r0, 8
	.endif
	or	\r0, \r0, $a3
	sw	\r0, 4($a2)
	.if \g
	lw	\r0, P_COL + 4 + P_BACK($a0)
	lw	\r1, P_COL + 8 + P_BACK($a0)
	sw	\r0, 4 + 4 * P_S($a2)
	sw	\r1, 4 + 8 * P_S($a2)
	.if P_N == 4
	lw	\r0, P_COL + 12 + P_BACK($a0)
	nop
	sw	\r0, 4 + 12 * P_S($a2)
	.endif
	.endif
	.endif
	.if \light == 3
	.if \t
	move	\r0, $a3
	.else
	lw	\r0, P_COL + P_BACK($a0)
	nop
	or	\r0, \r0, $a3
	.endif
	sw	\r0, 4($a2)
	.if P_PG
	sw	\r0, 4 + 4 * P_S($a2)
	sw	\r0, 4 + 8 * P_S($a2)
	.if P_N == 4
	sw	\r0, 4 + 12 * P_S($a2)
	.endif
	.endif
	.endif
	.endm

	.section .text.GsSortObject4, "ax", @progbits
	.align 2

# GsSortObject4(obj, ot, shift, scratch): spec 21 and exp/cleanlib/tmd.go. A polygon's
# cut is left to Go. This walks the runs GsLinkObject4 marked, with what it needs
# between them on the stack: how many polygons are left, the translucency, the table of
# loops for the light mode, ra, and for Go the state gsoGrad reads.
	.globl GsSortObject4
	.type GsSortObject4, @function
GsSortObject4:
	NOPS	22
	lw	$t6, 0($a0)
	lw	$t7, 8($a0)
	bltz	$t6, 9f
	srl	$t8, $t6, 9
	andi	$t8, $t8, 7
	bnez	$t8, gso_div
	addiu	$sp, $sp, -56
	NOPS	17
	# The light mode's table.
	andi	$t8, $t6, 0x40
	bnez	$t8, 1f
	lui	$t9, %hi(gso_off)
	andi	$t8, $t6, 0x20
	bnez	$t8, 2f
	srl	$t8, $t6, 3
	lui	$t8, %hi(GsLIGHT_MODE)
	lw	$t8, %lo(GsLIGHT_MODE)($t8)
	nop
2:	andi	$t8, $t8, 3
	addiu	$t8, $t8, -1
	lui	$t9, %hi(gso_fog)
	beqz	$t8, 3f
	addiu	$t9, $t9, %lo(gso_fog)
	lui	$t9, %hi(gso_lit)
	b	3f
	addiu	$t9, $t9, %lo(gso_lit)
1:	addiu	$t9, $t9, %lo(gso_off)
3:	sw	$t9, 24($sp)
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
gso_run:
	lw	$t7, 16($sp)
	lw	$t6, 0($a0)
	beqz	$t7, 8f
	andi	$a1, $t6, 0xffff
	beqz	$a1, 8f
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
	# A lit 20h or 30h with a color for each corner goes by Go.
	srl	$t9, $t6, 18
	andi	$t9, $t9, 1
	beqz	$t9, 4f
	andi	$t9, $t7, 0xc
	beqz	$t9, gso_grad
	nop
4:	lw	$t9, 24($sp)
	b	6f
	addu	$t9, $t9, $t7
5:	lui	$t9, %hi(gso_unlit)
	addu	$t9, $t9, $t7
	addiu	$t9, $t9, %lo(gso_unlit)
6:	lw	$t9, 0($t9)
	nop
	jr	$t9
	nop
8:	lui	$t6, %hi(GsOUT_PACKET_P)
	sw	$a2, %lo(GsOUT_PACKET_P)($t6)
	addiu	$sp, $sp, 56
9:	jr	$ra
	nop

gso_div:
	j	libps_gso_div
	addiu	$sp, $sp, 56

# A run of gradated triangles goes by Go, with what it needs on the stack: gsoState.
gso_grad:
	sw	$ra, 28($sp)
	sw	$t0, 32($sp)
	sw	$v1, 36($sp)
	sw	$t1, 40($sp)
	sw	$t2, 44($sp)
	sw	$a2, 48($sp)
	lui	$t9, %hi(gso_lit)
	addiu	$t9, $t9, %lo(gso_lit)
	lw	$t8, 24($sp)
	nop
	subu	$t8, $t8, $t9
	srl	$t8, $t8, 5
	sw	$t8, 52($sp)
	move	$a2, $a1
	move	$a1, $t6
	jal	libps_gso_grad
	addiu	$a3, $sp, 32
	move	$a0, $v0
	lw	$ra, 28($sp)
	lw	$t0, 32($sp)
	lw	$v1, 36($sp)
	lw	$t1, 40($sp)
	lw	$t2, 44($sp)
	lw	$a2, 48($sp)
	j	gso_run
	nop
	.size GsSortObject4, .-GsSortObject4

# The loops, each kind: name, light, t, g, q, flat, then pe, pb, pa, pr and pq.
	POLY f3, 0, 0, 0, 0, 0, 0, 0, 7, 20, 0, 0, 1, 0
	POLY ft3, 0, 1, 0, 0, 0, 3, 0, 7, 19, 0, 0, 1, 0
	POLY f4, 0, 0, 0, 1, 0, 3, 0, 0, 19, 8, 1, 1, 0
	POLY ft4, 0, 1, 0, 1, 0, 0, 0, 4, 16, 8, 4, 1, 0
	POLY g3, 0, 0, 1, 0, 0, 6, 0, 0, 21, 0, 0, 0, 0
	POLY gt3, 0, 1, 1, 0, 0, 1, 0, 8, 19, 0, 0, 1, 0
	POLY g4, 0, 0, 1, 1, 0, 2, 0, 3, 22, 0, 6, 0, 0
	POLY gt4, 0, 1, 1, 1, 0, 9, 0, 0, 20, 0, 7, 2, 0
	POLY lf3, 1, 0, 0, 0, 0, 3, 0, 0, 21, 0, 0, 0, 0
	POLY lft3, 1, 1, 0, 0, 0, 7, 0, 0, 13, 0, 0, 1, 0
	POLY lf4, 1, 0, 0, 1, 0, 11, 0, 0, 19, 3, 0, 0, 0
	POLY lft4, 1, 1, 0, 1, 0, 7, 0, 5, 11, 4, 0, 1, 0
	POLY lg3, 1, 0, 1, 0, 0, 0, 0, 3, 18, 0, 0, 0, 6
	POLY lgt3, 1, 1, 1, 0, 0, 0, 0, 5, 8, 0, 0, 1, 3
	POLY lg4, 1, 0, 1, 1, 0, 0, 0, 0, 19, 0, 0, 0, 9
	POLY lgt4, 1, 1, 1, 1, 0, 0, 0, 0, 13, 0, 0, 1, 11
	POLY ff3, 2, 0, 0, 0, 0, 3, 0, 0, 27, 0, 0, 0, 0
	POLY fft3, 2, 1, 0, 0, 0, 7, 0, 0, 19, 0, 0, 1, 0
	POLY ff4, 2, 0, 0, 1, 0, 11, 0, 0, 25, 3, 0, 0, 0
	POLY fft4, 2, 1, 0, 1, 0, 7, 0, 5, 17, 4, 0, 1, 0
	POLY fg3, 2, 0, 1, 0, 0, 0, 0, 1, 24, 0, 0, 0, 6
	POLY fgt3, 2, 1, 1, 0, 0, 0, 0, 3, 14, 0, 0, 1, 3
	POLY fg4, 2, 0, 1, 1, 0, 0, 0, 0, 25, 0, 0, 0, 9
	POLY fgt4, 2, 1, 1, 1, 0, 0, 0, 0, 19, 0, 0, 1, 11
	POLY of3, 3, 0, 0, 0, 0, 0, 0, 7, 25, 0, 0, 1, 0
	POLY oft3, 3, 1, 0, 0, 0, 5, 0, 6, 17, 0, 0, 1, 0
	POLY of4, 3, 0, 0, 1, 0, 3, 0, 0, 23, 8, 1, 1, 0
	POLY oft4, 3, 1, 0, 1, 0, 1, 0, 13, 15, 0, 4, 0, 0
	POLY og3, 3, 0, 1, 0, 0, 3, 0, 3, 26, 0, 0, 0, 0
	POLY ogt3, 3, 1, 1, 0, 1, 2, 0, 8, 14, 0, 0, 1, 0
	POLY og4, 3, 0, 1, 1, 0, 0, 0, 0, 22, 8, 4, 1, 0
	POLY ogt4, 3, 1, 1, 1, 1, 1, 0, 7, 14, 0, 10, 0, 0

# The tables, by the mode's textured, quad and gouraud bits. gso_lit, gso_fog and
# gso_off follow each other: gso_grad takes the light mode from the distance.
	.align 2
gso_unlit:
	.word gso_f3, gso_ft3, gso_f4, gso_ft4, gso_g3, gso_gt3, gso_g4, gso_gt4
gso_lit:
	.word gso_lf3, gso_lft3, gso_lf4, gso_lft4, gso_lg3, gso_lgt3, gso_lg4, gso_lgt4
gso_fog:
	.word gso_ff3, gso_fft3, gso_ff4, gso_fft4, gso_fg3, gso_fgt3, gso_fg4, gso_fgt4
gso_off:
	.word gso_of3, gso_oft3, gso_of4, gso_oft4, gso_og3, gso_ogt3, gso_og4, gso_ogt4

# CTC, CFC: control registers from and to t0 to t5, which the assembler will not take by
# name. rt is the number of the general register, rd of the control register.
	.macro CTC rt, rd
	.word 0x48c00000 | (\rt << 16) | (\rd << 11)
	.endm
	.macro CFC rt, rd
	.word 0x48400000 | (\rt << 16) | (\rd << 11)
	.endm
	.macro MVMVA12
	.word 0x4a486012
	.endm
	.macro MVMVA0
	.word 0x4a406012
	.endm

# SETROT loads the rotation at a0 into the GTE, the last of its load's delay used by the
# instruction before and the one given. a0 stays, t0 to t4 go.
	.macro SETROT
	lw	$t0, 0($a0)
	lw	$t1, 4($a0)
	CTC	8, 0
	lw	$t2, 8($a0)
	CTC	9, 1
	lw	$t3, 12($a0)
	CTC	10, 2
	lh	$t4, 16($a0)
	CTC	11, 3
	.endm

	.section .text.ApplyMatrix, "ax", @progbits
	.align 2
# ApplyMatrix(m, v0, v1), ApplyMatrixSV(m, v0, v1): matrix.go has what they do.
	.globl ApplyMatrix
	.type ApplyMatrix, @function
ApplyMatrix:
	SETROT
	lwc2	$0, 0($a1)
	lwc2	$1, 4($a1)
	CTC	12, 4
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
	SETROT
	lwc2	$0, 0($a1)
	lwc2	$1, 4($a1)
	CTC	12, 4
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

	.section .text.ApplyMatrixLV, "ax", @progbits
	.align 2
# ApplyMatrixLV(m, v0, v1), in two parts: spec 19's turnLong in matrix.go.
	.globl ApplyMatrixLV
	.type ApplyMatrixLV, @function
ApplyMatrixLV:
	SETROT
	lw	$t5, 0($a1)
	CTC	12, 4
	lw	$t6, 4($a1)
	lw	$t7, 8($a1)
	andi	$t0, $t5, 0x7fff
	sll	$t1, $t6, 17
	srl	$t1, $t1, 1
	or	$t0, $t0, $t1
	andi	$t1, $t7, 0x7fff
	mtc2	$t0, $0
	mtc2	$t1, $1
	sra	$t5, $t5, 15
	sra	$t6, $t6, 15
	MVMVA12
	sra	$t7, $t7, 15
	andi	$t5, $t5, 0xffff
	sll	$t6, $t6, 16
	or	$t5, $t5, $t6
	mfc2	$t0, $25
	mfc2	$t1, $26
	mfc2	$t2, $27
	mtc2	$t5, $0
	mtc2	$t7, $1
	move	$v0, $a2
	nop
	MVMVA0
	NOPS	4
	mfc2	$t3, $25
	mfc2	$t4, $26
	mfc2	$t5, $27
	sll	$t3, $t3, 3
	sll	$t4, $t4, 3
	sll	$t5, $t5, 3
	addu	$t0, $t0, $t3
	addu	$t1, $t1, $t4
	addu	$t2, $t2, $t5
	NOPS	9
	sw	$t0, 0($a2)
	sw	$t1, 4($a2)
	jr	$ra
	sw	$t2, 8($a2)
	.size ApplyMatrixLV, .-ApplyMatrixLV

	.section .text.TransMatrix, "ax", @progbits
	.align 2
# TransMatrix(m, v) sets m's translation and returns m.
	.globl TransMatrix
	.type TransMatrix, @function
TransMatrix:
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
	lui	$t6, %hi(gte_stack_n)
	lw	$t7, %lo(gte_stack_n)($t6)
	lui	$t5, %hi(gte_stack)
	sltiu	$at, $t7, 20
	beqz	$at, 1f
	addiu	$t5, $t5, %lo(gte_stack)
	sll	$at, $t7, 5
	addu	$t5, $t5, $at
	addiu	$t7, $t7, 1
	sw	$t7, %lo(gte_stack_n)($t6)
	CFC	8, 0
	CFC	9, 1
	sw	$t0, 0($t5)
	CFC	8, 2
	sw	$t1, 4($t5)
	CFC	9, 3
	sw	$t0, 8($t5)
	CFC	8, 4
	sw	$t1, 12($t5)
	CFC	9, 5
	sw	$t0, 16($t5)
	CFC	8, 6
	sw	$t1, 20($t5)
	CFC	9, 7
	sw	$t0, 24($t5)
	sw	$t1, 28($t5)
	nop
1:	jr	$ra
	nop
	.size PushMatrix, .-PushMatrix

	.globl PopMatrix
	.type PopMatrix, @function
PopMatrix:
	lui	$t6, %hi(gte_stack_n)
	lw	$t7, %lo(gte_stack_n)($t6)
	lui	$t5, %hi(gte_stack)
	beqz	$t7, 1f
	addiu	$t5, $t5, %lo(gte_stack)
	addiu	$t7, $t7, -1
	sll	$at, $t7, 5
	addu	$t5, $t5, $at
	sw	$t7, %lo(gte_stack_n)($t6)
	lw	$t0, 0($t5)
	lw	$t1, 4($t5)
	CTC	8, 0
	lw	$t0, 8($t5)
	CTC	9, 1
	lw	$t1, 12($t5)
	CTC	8, 2
	lw	$t0, 16($t5)
	CTC	9, 3
	lw	$t1, 20($t5)
	CTC	8, 4
	lw	$t0, 24($t5)
	CTC	9, 5
	lw	$t1, 28($t5)
	CTC	8, 6
	nop
	CTC	9, 7
	nop
1:	jr	$ra
	nop
	.size PopMatrix, .-PopMatrix

	.section .text.rand, "ax", @progbits
	.align 2
# rand and srand: the C library's linear congruence, 0 until srand.
	.globl rand
	.type rand, @function
rand:
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
	andi	$v0, $t7, 0x3ff
	xor	$v0, $v0, \a
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
	.macro SINNEG a, s, c, neg, back
\neg:
	subu	$t9, $zero, \a
	sll	$t7, $t9, 21
	SINBODY	$t9, \s, \c
	subu	\s, $zero, \s
	nop
	b	\back
	nop
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
	SINNEG	$a3, $t2, $t3, rm_ny, rm_by
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
	ROTABOUT RotMatrixX, 1, 2, subu, addu, 16
	ROTABOUT RotMatrixZ, 0, 1, subu, addu, 16

# Y's second row takes -s*a, which is c*b - s*a.
	.globl RotMatrixY
	.type RotMatrixY, @function
RotMatrixY:
	lui	$v1, %hi(libps_sine_table)
	addiu	$v1, $v1, %lo(libps_sine_table)
	SINCOS	$a0, $t0, $t1, rmy_n, rmy_b
	ROTCOLY	0
	ROTCOLY	1
	ROTCOLY	2
	NOPS	17
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
1:	lw	$t7, 0($t1)
	nop
	srl	$t7, $t7, 16
	beq	$t7, $t5, 2f
	nop
	beqz	$t4, 2f
	sh	$t4, 0($t3)
	move	$t3, $t1
	move	$t4, $zero
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
5:	jr	$ra
	nop
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
3:	j	libps_sprite_quad
	nop
	.size GsSortSprite, .-GsSortSprite

	.section .text.GsSortBoxFill, "ax", @progbits
	.align 2
# GsSortBoxFill: the mode word and a filled rectangle.
	.globl GsSortBoxFill
	.type GsSortBoxFill, @function
GsSortBoxFill:
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
	mtc2	$a3, $8
	blez	$a2, 2f
	nop
1:	lh	$t0, 0($a0)
	lh	$t1, 2($a0)
	lh	$t2, 4($a0)
	mtc2	$t0, $25
	mtc2	$t1, $26
	mtc2	$t2, $27
	lh	$t0, 0($a1)
	lh	$t1, 2($a1)
	lh	$t2, 4($a1)
	mtc2	$t0, $9
	mtc2	$t1, $10
	mtc2	$t2, $11
	addiu	$a2, $a2, -1
	addiu	$a1, $a1, 8
	.word 0x4a08003e
	NOPS	MIME_PAD
	mfc2	$t0, $9
	mfc2	$t1, $10
	mfc2	$t2, $11
	sh	$t0, 0($a0)
	sh	$t1, 2($a0)
	sh	$t2, 4($a0)
	bgtz	$a2, 1b
	addiu	$a0, $a0, 8
2:	jr	$ra
	nop
	.size gteMIMefunc, .-gteMIMefunc
