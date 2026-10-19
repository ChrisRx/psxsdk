# Members of libps.a that are linked into the program rather than resident.
# Converted by ecoff2asm, one section per member so --gc-sections can drop it.

	.set noreorder
	.set noat

# libps.a(puts.o)

# Undefined references: putchar

	.section .text.puts,"ax",@progbits
	.align 2
.Lputs_sect_text:
	.globl puts
puts:
	.word 0x27BDFFE8
	.word 0xAFB00010
	.word 0x00808021
	.word 0x16000006
	.word 0xAFBF0014
	lui $s0, %hi(.Lputs_sect_rdata)          # 0x3C100000 REFHI
	j .Lputs_sect_text+0x28                  # 0x0800000A JMPADDR
	addiu $s0, $s0, %lo(.Lputs_sect_rdata)   # 0x26100050 REFLO
	jal putchar                              # 0x0C000000 JMPADDR
	.word 0x00042603
	.word 0x92040000
	.word 0x00000000
	.word 0x00042600
	.word 0x1480FFFA
	.word 0x26100001
	.word 0x8FBF0014
	.word 0x8FB00010
	.word 0x03E00008
	.word 0x27BD0018
	.word 0x00000000

	.section .rodata.puts,"a",@progbits
	.align 2
.Lputs_sect_rdata:
	.word 0x4C554E3C
	.word 0x00003E4C
	.word 0x00000000
	.word 0x00000000

# libps.a(video.o)

# Undefined references: SsSetTickMode

	.section .text.video,"ax",@progbits
	.align 2
.Lvideo_sect_text:
	.globl SetVideoMode
SetVideoMode:
	.word 0x27BDFFE8
	.word 0xAFBF0014
	.word 0x3C018003
	.word 0x3421D72C
	.word 0x0020F809
	.word 0xAFB00010
	.word 0x24040005
	jal SsSetTickMode                        # 0x0C000000 JMPADDR
	.word 0x00408021
	.word 0x02001021
	.word 0x8FBF0014
	.word 0x8FB00010
	.word 0x03E00008
	.word 0x27BD0018
	.word 0x00000000
	.word 0x00000000

# libps.a(krom2tm2.o)

# Undefined references: Krom2RawAdd2, bcopy, bzero

	.section .text.krom2tm2,"ax",@progbits
	.align 2
.Lkrom2tm2_sect_text:
	.globl Krom2Tim2
Krom2Tim2:
	.word 0x3C0C0001
	.word 0x358C2048
	.word 0x03ACE823
	.word 0x019D6821
	.word 0xADB0FFD8
	.word 0x00808021
	.word 0xADB3FFE4
	.word 0x00A09821
	.word 0xADB5FFEC
	.word 0x00C0A821
	.word 0xADB6FFF0
	.word 0x00E0B021
	.word 0xADB4FFE8
	.word 0x0000A021
	.word 0xADB1FFDC
	.word 0x00008821
	.word 0x27A40010
	.word 0xADBFFFFC
	.word 0xADBEFFF8
	.word 0xADB7FFF4
	.word 0xADB2FFE0
	.word 0x3C170001
	.word 0x02FDB821
	.word 0x8EF72058
	.word 0x3C1E0001
	.word 0x03DDF021
	.word 0x8FDE205C
	.word 0x00000000
	jal bzero                                # 0x0C000000 JMPADDR
	.word 0x24052000
	.word 0x27A42010
	jal bzero                                # 0x0C000000 JMPADDR
	.word 0x3C050001
	.word 0x92050000
	.word 0x00000000
	.word 0x10A0002D
	.word 0x00009021
	.word 0x2A220100
	.word 0x1040002A
	.word 0x30A400FF
	.word 0x2402000A
	.word 0x14820009
	.word 0x00000000
	.word 0x06210002
	.word 0x02201021
	.word 0x2622000F
	.word 0x00021103
	.word 0x24420001
	.word 0x00028900
	j .Lkrom2tm2_sect_text+0x134             # 0x0800004D JMPADDR
	.word 0x26100001
	.word 0x24A2007F
	.word 0x304200FF
	.word 0x2C420018
	.word 0x10400005
	.word 0x00051200
	.word 0x92030001
	.word 0x26100002
	j .Lkrom2tm2_sect_text+0x110             # 0x08000044 JMPADDR
	.word 0x00621825
	jal .Lkrom2tm2_hankanjitrans             # 0x0C000129 JMPADDR
	.word 0x00000000
	.word 0x00401821
	.word 0x3062FFFF
	.word 0x14400002
	.word 0x00000000
	.word 0x2414FFFF
	.word 0x26100001
	jal Krom2RawAdd2                         # 0x0C000000 JMPADDR
	.word 0x3064FFFF
	.word 0x00402021
	.word 0x00112940
	.word 0x27A20010
	.word 0x00452821
	jal bcopy                                # 0x0C000000 JMPADDR
	.word 0x2406001E
	.word 0x26310001
	.word 0x92050000
	.word 0x00000000
	.word 0x14A0FFD6
	.word 0x2A220100
	.word 0x00006821
	.word 0x27AE0010
	.word 0x00004821
	.word 0x00005021
	.word 0x00121040
	.word 0x004E5821
	.word 0x000D1100
	.word 0x004A3021
	.word 0x00D1602A
	.word 0x1180003E
	.word 0x00061140
	.word 0x004E1021
	.word 0x00093840
	.word 0x00E23821
	.word 0x00124040
	.word 0x94E40000
	.word 0x27A62012
	.word 0x000419C2
	.word 0x30630001
	.word 0x00041082
	.word 0x30420010
	.word 0x00621825
	.word 0x000410C0
	.word 0x30420100
	.word 0x00621825
	.word 0x00042200
	.word 0x30841000
	.word 0x00641825
	.word 0xA5632000
	.word 0x94E50000
	.word 0x00C83021
	.word 0x30A3FFFF
	.word 0x000320C2
	.word 0x30840001
	.word 0x00031080
	.word 0x30420010
	.word 0x00822025
	.word 0x000319C0
	.word 0x30630100
	.word 0x00832025
	.word 0x30A50001
	.word 0x00052B00
	.word 0x00852025
	.word 0x27A52014
	.word 0xA4C40000
	.word 0x94E40000
	.word 0x00A82821
	.word 0x00041BC2
	.word 0x00041282
	.word 0x30420010
	.word 0x00621825
	.word 0x00041142
	.word 0x30420100
	.word 0x00621825
	.word 0x30841000
	.word 0x00641825
	.word 0xA4A30000
	.word 0x27A52016
	.word 0x94E40000
	.word 0x00A82821
	.word 0x00041AC2
	.word 0x30630001
	.word 0x00041182
	.word 0x30420010
	.word 0x00621825
	.word 0x00041042
	.word 0x30420100
	.word 0x00621825
	.word 0x00042100
	.word 0x30841000
	.word 0x00641825
	.word 0xA4A30000
	.word 0x254A0001
	.word 0x256B0008
	.word 0x29420010
	.word 0x1440FFBA
	.word 0x26520004
	.word 0x25290001
	.word 0x29220010
	.word 0x1440FFB3
	.word 0x00000000
	.word 0x11800005
	.word 0x2A220010
	.word 0x25AD0001
	.word 0x29A20010
	.word 0x1440FFAC
	.word 0x2A220010
	.word 0x14400008
	.word 0x00118140
	.word 0x2622FFFF
	.word 0x04410003
	.word 0x00021103
	.word 0x2622000E
	.word 0x00021103
	.word 0x24420001
	.word 0x00028240
	.word 0x02602021
	.word 0x26050010
	jal bzero                                # 0x0C000000 JMPADDR
	.word 0x00052880
	.word 0x3C030001
	.word 0x34630010
	.word 0x24020010
	.word 0xAE620000
	.word 0x24020008
	.word 0xAE620004
	.word 0x2402002C
	.word 0xAE620008
	.word 0x001E1400
	.word 0x00571025
	.word 0xAE62000C
	.word 0xAE630010
	.word 0x3C0F0001
	.word 0x01FD7821
	.word 0x8DEF2060
	.word 0x2625FFFF
	.word 0x000F1400
	.word 0x3C0F0001
	.word 0x01FD7821
	.word 0x8DEF2064
	.word 0x00A02021
	.word 0x004F1025
	.word 0xAE620014
	.word 0x26020003
	.word 0x00021080
	.word 0xAE620034
	.word 0x00161400
	.word 0x00551025
	.word 0x04A10002
	.word 0xAE620038
	.word 0x2624000E
	.word 0x00041903
	.word 0x24620001
	.word 0x00022500
	.word 0x2A220011
	.word 0x10400006
	.word 0x00031100
	.word 0x00A21023
	.word 0x24420001
	.word 0x00021080
	j .Lkrom2tm2_sect_text+0x380             # 0x080000E0 JMPADDR
	.word 0x00821025
	.word 0x34820040
	.word 0xAE62003C
	.word 0x2A220010
	.word 0x10400022
	.word 0x00003021
	.word 0x00003821
	.word 0x1A000033
	.word 0x00004821
	.word 0x00112880
	.word 0x27AA0010
	.word 0x27A82012
	.word 0x02602021
	.word 0x04C10002
	.word 0x00C01021
	.word 0x24C2003F
	.word 0x00021183
	.word 0x00021180
	.word 0x00C21023
	.word 0x0045102A
	.word 0x14400003
	.word 0x00000000
	.word 0x24E70001
	.word 0x00073180
	.word 0x25290001
	.word 0x00061040
	.word 0x24C60002
	.word 0x004A1821
	.word 0x01021021
	.word 0x94420000
	.word 0x94632000
	.word 0x00021400
	.word 0x00621825
	.word 0xAC830040
	.word 0x0130102A
	.word 0x1440FFE9
	.word 0x24840004
	j .Lkrom2tm2_sect_text+0x468             # 0x0800011A JMPADDR
	.word 0x02801021
	.word 0x00102840
	.word 0x18A00012
	.word 0x00004821
	.word 0x27A72012
	.word 0x27A60010
	.word 0x94E40000
	.word 0x24E70004
	.word 0x94C32000
	.word 0x000917C2
	.word 0x01221021
	.word 0x25290002
	.word 0x00021043
	.word 0x00021080
	.word 0x00531021
	.word 0x00042400
	.word 0x00641825
	.word 0xAC430040
	.word 0x0125102A
	.word 0x1440FFF2
	.word 0x24C60004
	.word 0x02801021
	.word 0x3C0C0001
	.word 0x358C2048
	.word 0x019D6821
	.word 0x8DBFFFFC
	.word 0x8DBEFFF8
	.word 0x8DB7FFF4
	.word 0x8DB6FFF0
	.word 0x8DB5FFEC
	.word 0x8DB4FFE8
	.word 0x8DB3FFE4
	.word 0x8DB2FFE0
	.word 0x8DB1FFDC
	.word 0x8DB0FFD8
	.word 0x03E00008
	.word 0x03ACE821
.Lkrom2tm2_hankanjitrans:
	.word 0x00001821
	.word 0x2482FFE0
	.word 0x304200FF
	.word 0x2C420010
	.word 0x10400003
	.word 0x00803021
	j .Lkrom2tm2_sect_text+0x558             # 0x08000156 JMPADDR
	.word 0x24030001
	.word 0x2482FFD0
	.word 0x304200FF
	.word 0x2C42000A
	.word 0x10400003
	.word 0x2482FFC6
	j .Lkrom2tm2_sect_text+0x558             # 0x08000156 JMPADDR
	.word 0x00002821
	.word 0x304200FF
	.word 0x2C420007
	.word 0x10400003
	.word 0x2482FFBF
	j .Lkrom2tm2_sect_text+0x558             # 0x08000156 JMPADDR
	.word 0x2403000B
	.word 0x304200FF
	.word 0x2C42001A
	.word 0x10400003
	.word 0x2482FFA5
	j .Lkrom2tm2_sect_text+0x558             # 0x08000156 JMPADDR
	.word 0x24050001
	.word 0x304200FF
	.word 0x2C420006
	.word 0x10400003
	.word 0x2482FF9F
	j .Lkrom2tm2_sect_text+0x558             # 0x08000156 JMPADDR
	.word 0x24030025
	.word 0x304200FF
	.word 0x2C42001A
	.word 0x10400003
	.word 0x2482FF85
	j .Lkrom2tm2_sect_text+0x558             # 0x08000156 JMPADDR
	.word 0x24050002
	.word 0x304200FF
	.word 0x2C420004
	.word 0x14400003
	.word 0x2403003F
	j .Lkrom2tm2_sect_text+0x5B0             # 0x0800016C JMPADDR
	.word 0x00001021
	.word 0x306300FF
	.word 0x10600009
	.word 0x30C200FF
	.word 0x2463001F
	.word 0x00431023
	.word 0x00021040
	lui $at, %hi(.Lkrom2tm2_han_k_table)     # 0x3C010000 REFHI
	.word 0x00220821
	lhu $v0, %lo(.Lkrom2tm2_han_k_table)($at) # 0x942205CC REFLO
	j .Lkrom2tm2_sect_text+0x5B0             # 0x0800016C JMPADDR
	.word 0x00000000
	.word 0x30A300FF
	.word 0x00031880
	.word 0x30C400FF
	lui $v0, %hi(.Lkrom2tm2_han_table)       # 0x3C020000 REFHI
	.word 0x00431021
	lhu $v0, %lo(.Lkrom2tm2_han_table)($v0)  # 0x944205C0 REFLO
	lui $at, %hi(.Lkrom2tm2_sect_data+0x2)   # 0x3C010000 REFHI
	.word 0x00230821
	lhu $v1, %lo(.Lkrom2tm2_sect_data+0x2)($at) # 0x942305C2 REFLO
	.word 0x00441021
	.word 0x00431023
	.word 0x03E00008
	.word 0x00000000
	.word 0x00000000
	.word 0x00000000

	.section .data.krom2tm2,"aw",@progbits
	.align 2
.Lkrom2tm2_sect_data:
.Lkrom2tm2_han_table:
	.word 0x0030824F
	.word 0x00418260
	.word 0x00618281
.Lkrom2tm2_han_k_table:
	.word 0x81498140
	.word 0x81948168
	.word 0x81938190
	.word 0x81668195
	.word 0x816A8169
	.word 0x817B8196
	.word 0x817C8143
	.word 0x815E8144
	.word 0x81478146
	.word 0x81818171
	.word 0x81488172
	.word 0x816D8197
	.word 0x816E818F
	.word 0x8151814F
	.word 0x816F8165
	.word 0x81708162
	.word 0x00008150

# libps.a(EQSF2.O)

	.section .text.eqsf2,"ax",@progbits
	.align 2
.Leqsf2_sect_text:
	.globl __eqsf2
__eqsf2:
	.word 0x10850008
	.word 0x3C037FFF
	.word 0x3463FFFF
	.word 0x00831024
	.word 0x14400005
	.word 0x24020001
	.word 0x00A31024
	.word 0x14400002
	.word 0x24020001
	.word 0x00001021
	.word 0x03E00008
	.word 0x00000000

# libps.a(NESF2.O)

	.section .text.nesf2,"ax",@progbits
	.align 2
.Lnesf2_sect_text:
	.globl __nesf2
__nesf2:
	.word 0x10850008
	.word 0x3C037FFF
	.word 0x3463FFFF
	.word 0x00831024
	.word 0x14400005
	.word 0x24020001
	.word 0x00A31024
	.word 0x14400002
	.word 0x24020001
	.word 0x00001021
	.word 0x03E00008
	.word 0x00000000

# libps.a(LTSF2.O)

	.section .text.ltsf2,"ax",@progbits
	.align 2
.Lltsf2_sect_text:
	.globl __ltsf2
__ltsf2:
	.word 0x00803021
	.word 0x10C50008
	.word 0x3C037FFF
	.word 0x3463FFFF
	.word 0x00C31024
	.word 0x14400006
	.word 0x3C028000
	.word 0x00A31024
	.word 0x14400003
	.word 0x3C028000
	j .Lltsf2_sect_text+0x94                 # 0x08000025 JMPADDR
	.word 0x00001021
	.word 0x00C23824
	.word 0x00A21024
	.word 0x14E20015
	.word 0x0007102B
	.word 0x000615C3
	.word 0x304400FF
	.word 0x000515C3
	.word 0x304300FF
	.word 0x0083102A
	.word 0x1440000D
	.word 0x00000000
	.word 0x1483000C
	.word 0x28C20000
	.word 0x3C02007F
	.word 0x3442FFFF
	.word 0x00C21824
	.word 0x3C040080
	.word 0x00A21024
	.word 0x00441025
	.word 0x00641825
	.word 0x0062182A
	.word 0x10600002
	.word 0x28C20000
	.word 0x2CE20001
	.word 0x00021023
	.word 0x03E00008
	.word 0x00000000
	.word 0x00000000

# libps.a(LESF2.O)

	.section .text.lesf2,"ax",@progbits
	.align 2
.Llesf2_sect_text:
	.globl __lesf2
__lesf2:
	.word 0x00803021
	.word 0x10C50008
	.word 0x3C037FFF
	.word 0x3463FFFF
	.word 0x00C31024
	.word 0x14400006
	.word 0x3C028000
	.word 0x00A31024
	.word 0x14400003
	.word 0x3C028000
	j .Llesf2_sect_text+0x9C                 # 0x08000027 JMPADDR
	.word 0x00001021
	.word 0x00C23824
	.word 0x00A21024
	.word 0x14E20018
	.word 0x2CE20001
	.word 0x000615C3
	.word 0x304400FF
	.word 0x000515C3
	.word 0x304300FF
	.word 0x0083102A
	.word 0x1440000D
	.word 0x00000000
	.word 0x1483000D
	.word 0x3C028000
	.word 0x3C02007F
	.word 0x3442FFFF
	.word 0x00C21824
	.word 0x3C040080
	.word 0x00A21024
	.word 0x00441025
	.word 0x00641825
	.word 0x0043102A
	.word 0x14400003
	.word 0x3C028000
	j .Llesf2_sect_text+0x9C                 # 0x08000027 JMPADDR
	.word 0x0007102B
	.word 0x00C21024
	.word 0x2C420001
	.word 0x03E00008
	.word 0x00000000
	.word 0x00000000
	.word 0x00000000
	.word 0x00000000

# libps.a(GTSF2.O)

	.section .text.gtsf2,"ax",@progbits
	.align 2
.Lgtsf2_sect_text:
	.globl __gtsf2
__gtsf2:
	.word 0x00803021
	.word 0x10C50008
	.word 0x3C037FFF
	.word 0x3463FFFF
	.word 0x00C31024
	.word 0x14400006
	.word 0x3C028000
	.word 0x00A31024
	.word 0x14400003
	.word 0x3C028000
	j .Lgtsf2_sect_text+0x90                 # 0x08000024 JMPADDR
	.word 0x00001021
	.word 0x00C23824
	.word 0x00A21024
	.word 0x14E20015
	.word 0x2CE20001
	.word 0x000615C3
	.word 0x304400FF
	.word 0x000515C3
	.word 0x304300FF
	.word 0x0064102A
	.word 0x1440000D
	.word 0x00000000
	.word 0x1483000C
	.word 0x28C20000
	.word 0x3C02007F
	.word 0x3442FFFF
	.word 0x00C21824
	.word 0x3C040080
	.word 0x00A21024
	.word 0x00441025
	.word 0x00641825
	.word 0x0043102A
	.word 0x10400002
	.word 0x28C20000
	.word 0x2CE20001
	.word 0x03E00008
	.word 0x00000000
	.word 0x00000000
	.word 0x00000000

# libps.a(GESF2.O)

	.section .text.gesf2,"ax",@progbits
	.align 2
.Lgesf2_sect_text:
	.globl __gesf2
__gesf2:
	.word 0x00803021
	.word 0x10C50008
	.word 0x3C037FFF
	.word 0x3463FFFF
	.word 0x00C31024
	.word 0x14400006
	.word 0x3C028000
	.word 0x00A31024
	.word 0x14400003
	.word 0x3C028000
	j .Lgesf2_sect_text+0xA0                 # 0x08000028 JMPADDR
	.word 0x00001021
	.word 0x00C23824
	.word 0x00A21024
	.word 0x14E20018
	.word 0x0007102B
	.word 0x000615C3
	.word 0x304400FF
	.word 0x000515C3
	.word 0x304300FF
	.word 0x0064102A
	.word 0x1440000D
	.word 0x00000000
	.word 0x1483000D
	.word 0x3C028000
	.word 0x3C02007F
	.word 0x3442FFFF
	.word 0x00C21824
	.word 0x3C040080
	.word 0x00A21024
	.word 0x00441025
	.word 0x00641825
	.word 0x0062182A
	.word 0x14600003
	.word 0x3C028000
	j .Lgesf2_sect_text+0x9C                 # 0x08000027 JMPADDR
	.word 0x0007102B
	.word 0x00C21024
	.word 0x2C420001
	.word 0x00021023
	.word 0x03E00008
	.word 0x00000000
	.word 0x00000000
	.word 0x00000000

# libps.a(NEGSF2.O)

	.section .text.negsf2,"ax",@progbits
	.align 2
.Lnegsf2_sect_text:
	.globl __negsf2
__negsf2:
	.word 0x10800003
	.word 0x3C028000
	j .Lnegsf2_sect_text+0x14                # 0x08000005 JMPADDR
	.word 0x00821026
	.word 0x00001021
	.word 0x03E00008
	.word 0x00000000
	.word 0x00000000

# libps.a(SUBSF3.O)

# Undefined references: __addsf3, __negsf2

	.section .text.subsf3,"ax",@progbits
	.align 2
.Lsubsf3_sect_text:
	.globl __subsf3
__subsf3:
	.word 0x3C067FFF
	.word 0x34C6FFFF
	.word 0x27BDFFE8
	.word 0x00A01821
	.word 0x00661024
	.word 0xAFBF0010
	.word 0x14400003
	.word 0x00803821
	j .Lsubsf3_sect_text+0x50                # 0x08000014 JMPADDR
	.word 0x00801021
	.word 0x00E61024
	.word 0x10400006
	.word 0x00000000
	.word 0x3C058000
	jal __addsf3                             # 0x0C000000 JMPADDR
	.word 0x00652826
	j .Lsubsf3_sect_text+0x50                # 0x08000014 JMPADDR
	.word 0x00000000
	jal __negsf2                             # 0x0C000000 JMPADDR
	.word 0x00A02021
	.word 0x8FBF0010
	.word 0x27BD0018
	.word 0x03E00008
	.word 0x00000000

# libps.a(FIXSFSI.O)

# Undefined references: _err_math

	.section .text.fixsfsi,"ax",@progbits
	.align 2
.Lfixsfsi_sect_text:
	.globl __fixsfsi
__fixsfsi:
	.word 0x3C027FFF
	.word 0x3442FFFF
	.word 0x00821024
	.word 0x27BDFFE8
	.word 0x14400004
	.word 0xAFBF0010
	.word 0x3C028000
	j .Lfixsfsi_sect_text+0xA4               # 0x08000029 JMPADDR
	.word 0x00821024
	.word 0x000415C3
	.word 0x304500FF
	.word 0x24A6FF63
	.word 0x18C0000A
	.word 0x3C030080
	.word 0x04800006
	.word 0x24040022
	jal _err_math                            # 0x0C000000 JMPADDR
	.word 0x24050012
	.word 0x3C027FFF
	j .Lfixsfsi_sect_text+0xA4               # 0x08000029 JMPADDR
	.word 0x3442FFFF
	j .Lfixsfsi_sect_text+0xA4               # 0x08000029 JMPADDR
	.word 0x3C028000
	.word 0x3C02007F
	.word 0x3442FFFF
	.word 0x00821024
	.word 0x00431025
	.word 0x000219C0
	.word 0x24A2FF82
	.word 0x2C420020
	.word 0x10400005
	.word 0x00000000
	.word 0x10600003
	.word 0x00061023
	j .Lfixsfsi_sect_text+0x98               # 0x08000026 JMPADDR
	.word 0x00431807
	j .Lfixsfsi_sect_text+0xA4               # 0x08000029 JMPADDR
	.word 0x00001021
	.word 0x04810002
	.word 0x00601021
	.word 0x00021023
	.word 0x8FBF0010
	.word 0x27BD0018
	.word 0x03E00008
	.word 0x00000000
	.word 0x00000000
	.word 0x00000000
	.word 0x00000000

# libps.a(FIXUSFSI.O)

# Undefined references: _err_math

	.section .text.fixusfsi,"ax",@progbits
	.align 2
.Lfixusfsi_sect_text:
	.globl __fixunssfsi
__fixunssfsi:
	.word 0x27BDFFE0
	.word 0xAFB00010
	.word 0x00808021
	.word 0xAFB10014
	.word 0x3C028000
	.word 0x02028824
	.word 0x12200004
	.word 0xAFBF0018
	.word 0x24040021
	jal _err_math                            # 0x0C000000 JMPADDR
	.word 0x24050014
	.word 0x3C027FFF
	.word 0x3442FFFF
	.word 0x02021024
	.word 0x14400003
	.word 0x001015C3
	j .Lfixusfsi_sect_text+0xB4              # 0x0800002D JMPADDR
	.word 0x02201021
	.word 0x304400FF
	.word 0x2485FF62
	.word 0x18A00006
	.word 0x3C02007F
	.word 0x24040022
	jal _err_math                            # 0x0C000000 JMPADDR
	.word 0x24050014
	j .Lfixusfsi_sect_text+0xB4              # 0x0800002D JMPADDR
	.word 0x2402FFFF
	.word 0x3442FFFF
	.word 0x02021024
	.word 0x3C030080
	.word 0x00431025
	.word 0x00021A00
	.word 0x2482FF81
	.word 0x2C420020
	.word 0x10400005
	.word 0x00000000
	.word 0x10600003
	.word 0x00051023
	j .Lfixusfsi_sect_text+0xA8              # 0x0800002A JMPADDR
	.word 0x00431806
	j .Lfixusfsi_sect_text+0xB4              # 0x0800002D JMPADDR
	.word 0x00001021
	.word 0x06010002
	.word 0x00601021
	.word 0x00021023
	.word 0x8FBF0018
	.word 0x8FB10014
	.word 0x8FB00010
	.word 0x03E00008
	.word 0x27BD0020
	.word 0x00000000
	.word 0x00000000

# libps.a(FIXUDFSI.O)

# Undefined references: _dbl_shift, _err_math

	.section .text.fixudfsi,"ax",@progbits
	.align 2
.Lfixudfsi_sect_text:
	.globl __fixunsdfsi
__fixunsdfsi:
	.word 0x27BDFFC8
	.word 0xAFB10024
	.word 0xAFB00020
	.word 0x00A08821
	.word 0xAFB20028
	.word 0x3C028000
	.word 0x02229024
	.word 0x00808021
	.word 0xAFBF0030
	.word 0x12400004
	.word 0xAFB3002C
	.word 0x24040021
	jal _err_math                            # 0x0C000000 JMPADDR
	.word 0x24050013
	.word 0x3C027FFF
	.word 0x3442FFFF
	.word 0x02221024
	.word 0x14400005
	.word 0x00000000
	.word 0x16000004
	.word 0x00111503
	j .Lfixudfsi_sect_text+0xF0              # 0x0800003C JMPADDR
	.word 0x02401021
	.word 0x00111503
	.word 0x305207FF
	.word 0x2653FBE2
	.word 0x1A600005
	.word 0x24040022
	jal _err_math                            # 0x0C000000 JMPADDR
	.word 0x24050013
	j .Lfixudfsi_sect_text+0xF0              # 0x0800003C JMPADDR
	.word 0x2402FFFF
	.word 0x3C02000F
	.word 0x3442FFFF
	.word 0x02221024
	.word 0x3C030010
	.word 0x00431025
	.word 0xAFA2001C
	.word 0xAFB00018
	.word 0x2402000B
	.word 0x8FA60018
	.word 0x8FA7001C
	.word 0xAFA20010
	.word 0x27A40018
	jal _dbl_shift                           # 0x0C000000 JMPADDR
	.word 0x00002821
	.word 0x2642FC01
	.word 0x2C420020
	.word 0x8FA3001C
	.word 0x10400005
	.word 0x00000000
	.word 0x10600003
	.word 0x00131023
	j .Lfixudfsi_sect_text+0xE4              # 0x08000039 JMPADDR
	.word 0x00431806
	j .Lfixudfsi_sect_text+0xF0              # 0x0800003C JMPADDR
	.word 0x00001021
	.word 0x06210002
	.word 0x00601021
	.word 0x00021023
	.word 0x8FBF0030
	.word 0x8FB3002C
	.word 0x8FB20028
	.word 0x8FB10024
	.word 0x8FB00020
	.word 0x03E00008
	.word 0x27BD0038
	.word 0x00000000

# libps.a(FLTSISF.O)

	.section .text.fltsisf,"ax",@progbits
	.align 2
.Lfltsisf_sect_text:
	.globl __floatsisf
__floatsisf:
	.word 0x00003021
	.word 0x14800004
	.word 0x2403009D
	.word 0x00001821
	j .Lfltsisf_sect_text+0xB0               # 0x0800002C JMPADDR
	.word 0x00601021
	.word 0x04810003
	.word 0x3C0200FF
	.word 0x3C068000
	.word 0x00042023
	.word 0x3442FFFF
	.word 0x0044102A
	.word 0x14400008
	.word 0x3C023FFF
	.word 0x3C0500FF
	.word 0x34A5FFFF
	.word 0x00042100
	.word 0x00A4102A
	.word 0x1040FFFD
	.word 0x2463FFFC
	.word 0x3C023FFF
	.word 0x3442FFFF
	.word 0x0044102A
	.word 0x14400007
	.word 0x00000000
	.word 0x3C053FFF
	.word 0x34A5FFFF
	.word 0x00042040
	.word 0x00A4102A
	.word 0x1040FFFD
	.word 0x2463FFFF
	.word 0x24840040
	.word 0x04810004
	.word 0x00000000
	.word 0x24630001
	j .Lfltsisf_sect_text+0x98               # 0x08000026 JMPADDR
	.word 0x00042202
	.word 0x000421C2
	.word 0x000315C0
	.word 0x00C21825
	.word 0x3C02FF7F
	.word 0x3442FFFF
	.word 0x00821024
	.word 0x00621025
	.word 0x03E00008
	.word 0x00000000
	.word 0x00000000
	.word 0x00000000

