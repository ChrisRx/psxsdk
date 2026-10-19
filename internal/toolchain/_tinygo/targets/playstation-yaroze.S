# The Net Yaroze crt0 and the startup parameters it reads, from libps.a.
# Converted by ecoff2asm. Must be linked first so _start is at __load.

	.set noreorder
	.set noat

# libps.a(startup.o)

# Undefined references: EnterCriticalSection, ExitCriticalSection, InitHeap, _end, _fbss, _gp, _sys_init_bss_flag, _sys_init_fp, _sys_init_gp, _sys_init_heapbase, _sys_init_heapsize, _sys_init_sp, _sys_init_stacksize, _sys_ramsize, main

	.text
	.align 2
.Lstartup_sect_text:
	.globl __main
	.globl _start
	.globl exit
_start:
	.word 0x2408FFFF
	lui $t1, %hi(_sys_init_gp)               # 0x3C090000 REFHI
	lw $t1, %lo(_sys_init_gp)($t1)           # 0x8D290000 REFLO
	.word 0x00000000
	.word 0x11090002
	.word 0x00000000
	.word 0x0120E021
.Lstartup_skip_gp:
	lui $at, %hi(.Lstartup_st_reg)           # 0x3C010000 REFHI
	addiu $at, $at, %lo(.Lstartup_st_reg)    # 0x242101C0 REFLO
	.word 0xAC3F007C
	lui $t0, %hi(_sys_init_bss_flag)         # 0x3C080000 REFHI
	lw $t0, %lo(_sys_init_bss_flag)($t0)     # 0x8D080000 REFLO
	.word 0x00000000
	.word 0x1100000B
	.word 0x00000000
	lui $t0, %hi(_fbss)                      # 0x3C080000 REFHI
	addiu $t0, $t0, %lo(_fbss)               # 0x25080000 REFLO
	lui $t1, %hi(_end)                       # 0x3C090000 REFHI
	addiu $t1, $t1, %lo(_end)                # 0x25290000 REFLO
.Lstartup_clrit:
	.word 0xAD000000
	.word 0x25080004
	.word 0x0109082B
	.word 0x1420FFFC
	.word 0x00000000
	.word 0x00000000
.Lstartup_skip_ibss:
	.word 0x2408FFFF
	lui $t1, %hi(_sys_init_sp)               # 0x3C090000 REFHI
	lw $t1, %lo(_sys_init_sp)($t1)           # 0x8D290000 REFLO
	.word 0x00000000
	.word 0x11090002
	.word 0x00000000
	.word 0x0120E821
.Lstartup_skip_isp:
	.word 0x2408FFFF
	lui $t1, %hi(_sys_init_fp)               # 0x3C090000 REFHI
	lw $t1, %lo(_sys_init_fp)($t1)           # 0x8D290000 REFLO
	.word 0x00000000
	.word 0x15090002
	.word 0x0120F021
	.word 0x03A0F021
.Lstartup_skip_fp:
	.word 0x2408FFFF
	lui $t1, %hi(_sys_init_heapbase)         # 0x3C090000 REFHI
	lw $t1, %lo(_sys_init_heapbase)($t1)     # 0x8D290000 REFLO
	.word 0x00000000
	.word 0x11280035
	.word 0x00000000
	.word 0x03A04021
	lui $t1, %hi(_sys_init_stacksize)        # 0x3C090000 REFHI
	lw $t1, %lo(_sys_init_stacksize)($t1)    # 0x8D290000 REFLO
	.word 0x00000000
	.word 0x01094023
	lui $t1, %hi(_sys_init_heapbase)         # 0x3C090000 REFHI
	lw $t1, %lo(_sys_init_heapbase)($t1)     # 0x8D290000 REFLO
	.word 0x00000000
	.word 0x01094023
	lui $at, %hi(_sys_init_heapsize)         # 0x3C010000 REFHI
	sw $t0, %lo(_sys_init_heapsize)($at)     # 0xAC280000 REFLO
	lui $at, %hi(.Lstartup_st_reg)           # 0x3C010000 REFHI
	addiu $at, $at, %lo(.Lstartup_st_reg)    # 0x242101C0 REFLO
	.word 0xAC220008
	.word 0xAC23000C
	.word 0xAC240010
	.word 0xAC250014
	.word 0xAC260018
	.word 0xAC27001C
	.word 0xAC300040
	.word 0xAC310044
	.word 0xAC320048
	.word 0xAC33004C
	.word 0xAC340050
	.word 0xAC350054
	.word 0xAC360058
	.word 0xAC37005C
	.word 0xAC3A0068
	.word 0xAC3B006C
	.word 0x01202021
	lui $a1, %hi(_sys_init_heapsize)         # 0x3C050000 REFHI
	lw $a1, %lo(_sys_init_heapsize)($a1)     # 0x8CA50000 REFLO
	jal InitHeap                             # 0x0C000000 JMPADDR
	.word 0x00000000
	lui $at, %hi(.Lstartup_st_reg)           # 0x3C010000 REFHI
	addiu $at, $at, %lo(.Lstartup_st_reg)    # 0x242101C0 REFLO
	.word 0x8C220008
	.word 0x8C23000C
	.word 0x8C240010
	.word 0x8C250014
	.word 0x8C260018
	.word 0x8C27001C
	.word 0x8C300040
	.word 0x8C310044
	.word 0x8C320048
	.word 0x8C33004C
	.word 0x8C340050
	.word 0x8C350054
	.word 0x8C360058
	.word 0x8C37005C
	.word 0x8C3A0068
	.word 0x8C3B006C
.Lstartup_skip_ih:
.Lstartup_call_main:
	jal ExitCriticalSection                  # 0x0C000000 JMPADDR
	.word 0x00000000
	jal main                                 # 0x0C000000 JMPADDR
	.word 0x00000000
	jal EnterCriticalSection                 # 0x0C000000 JMPADDR
	.word 0x00000000
exit:
	lui $at, %hi(.Lstartup_st_reg)           # 0x3C010000 REFHI
	addiu $at, $at, %lo(.Lstartup_st_reg)    # 0x242101C0 REFLO
	.word 0x8C3F007C
	.word 0x00000000
__main:
	.word 0x03E00008
	.word 0x00000000
	.word 0x00000000
	.word 0x00000000
	.word 0x00000000

	.data
	.align 2
.Lstartup_sect_data:
.Lstartup_st_reg:
	.space 336

# libps.a(stdef1.o)

	.section .sdata,"aw",@progbits
	.align 2
.Lstdef1_sect_sdata:
	.globl _sys_ramsize
_sys_ramsize:
	.word 0x00200000
	.word 0x00000000
	.word 0x00000000
	.word 0x00000000

# libps.a(stdef2.o)

	.section .sdata,"aw",@progbits
	.align 2
.Lstdef2_sect_sdata:
	.globl _sys_init_stacksize
_sys_init_stacksize:
	.word 0x00010000
	.word 0x00000000
	.word 0x00000000
	.word 0x00000000

# libps.a(stdef3.o)

	.section .sdata,"aw",@progbits
	.align 2
.Lstdef3_sect_sdata:
	.globl _sys_init_sp
_sys_init_sp:
	.word 0xFFFFFFFF
	.word 0x00000000
	.word 0x00000000
	.word 0x00000000

# libps.a(stdef4.o)

	.section .sdata,"aw",@progbits
	.align 2
.Lstdef4_sect_sdata:
	.globl _sys_init_fp
_sys_init_fp:
	.word 0xFFFFFFFF
	.word 0x00000000
	.word 0x00000000
	.word 0x00000000

# libps.a(stdef5.o)

# Undefined references: _gp

	.section .sdata,"aw",@progbits
	.align 2
.Lstdef5_sect_sdata:
	.globl _sys_init_gp
_sys_init_gp:
	.word _gp
	.word 0x00000000
	.word 0x00000000
	.word 0x00000000

# libps.a(stdef6.o)

# Undefined references: _end

	.section .sdata,"aw",@progbits
	.align 2
.Lstdef6_sect_sdata:
	.globl _sys_init_heapbase
_sys_init_heapbase:
	.word _end
	.word 0x00000000
	.word 0x00000000
	.word 0x00000000

# libps.a(stdef7.o)

	.section .sdata,"aw",@progbits
	.align 2
.Lstdef7_sect_sdata:
	.globl _sys_init_heapsize
_sys_init_heapsize:
	.space 16

# libps.a(stdef8.o)

	.section .sdata,"aw",@progbits
	.align 2
.Lstdef8_sect_sdata:
	.globl _sys_init_bss_flag
_sys_init_bss_flag:
	.word 0x00000001
	.word 0x00000000
	.word 0x00000000
	.word 0x00000000

