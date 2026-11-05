/* r3000.h: the CPU's memory segments, status register and cause codes. */
#ifndef _R3000_H
#define _R3000_H

#define K0BASE 0x80000000
#define K1BASE 0xa0000000
#define K2BASE 0xc0000000

#define SR_CU0 0x10000000
#define SR_CU2 0x40000000
#define SR_BEV 0x00400000
#define SR_TS 0x00200000
#define SR_PE 0x00100000
#define SR_CM 0x00080000
#define SR_PZ 0x00040000
#define SR_SWC 0x00020000
#define SR_ISC 0x00010000
#define SR_IMASK 0x0000ff00
#define SR_IMASKSHIFT 8
#define SR_KUO 0x00000020
#define SR_IEO 0x00000010
#define SR_KUP 0x00000008
#define SR_IEP 0x00000004
#define SR_KUC 0x00000002
#define SR_IEC 0x00000001

#define CAUSE_BD 0x80000000
#define CAUSE_IPMASK 0x0000ff00
#define CAUSE_EXCMASK 0x0000003c
#define EXC_CODE(x) ((x) << 2)

#endif
