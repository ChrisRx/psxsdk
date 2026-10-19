// Sony PlayStation (R3000A), standalone: entry point and the three things
// that cannot be written in Go because they are privileged or they are a
// call into the BIOS.
//
// No automatic delay slot filling. MIPS I executes the instruction after
// every branch and jump regardless of whether the branch is taken, so each
// one below is followed by whatever belongs there, written out.
.set noreorder

// The BIOS exposes its services through three dispatchers in the first page
// of RAM. Loading the table address into $t2 and the function number into
// $t1, then jumping to $t2, is the calling convention; arguments are in the
// usual registers and the result comes back in $v0.
#define BIOS_A0           0xa0
#define std_out_putchar   0x3c

// COP0 register 12 is Status. Bit 0, IEc, is the global interrupt enable.
#define COP0_STATUS       $12
#define SR_IEC            1

	.section .init, "ax", @progbits
	.global _start
	.type _start, @function

// _start is where the loader jumps. It is handed a machine with RAM already
// populated, so there is nothing to copy and nothing to initialise but the
// stack: .bss is cleared by the runtime's preinit, which can see where it
// starts and ends.
//
// The BIOS sets $sp itself from the executable header before jumping here,
// but it is set again from the link script so that the program is correct
// under a loader that does not, such as one that jumps straight to the entry
// point after a serial upload.
_start:
	la	$sp, _stack_top
	move	$fp, $sp
	// Nothing is compiled with small data, so $gp is never used to reach a
	// global. Zero it rather than leave whatever the loader had there.
	move	$gp, $zero

	jal	main
	nop

// main does not return: the runtime calls exit, which hangs. Should it ever
// return anyway, stop here rather than run off into whatever follows.
1:
	b	1b
	nop
	.size _start, . - _start

	.section .text.psx_putchar, "ax", @progbits
	.global psx_putchar
	.type psx_putchar, @function

// psx_putchar(c) writes one byte to the BIOS console, which on a retail
// console goes to the serial port and on an emulator usually goes to a log.
// It is a tail call: the BIOS returns to our caller.
psx_putchar:
	li	$t1, std_out_putchar
	li	$t2, BIOS_A0
	jr	$t2
	nop
	.size psx_putchar, . - psx_putchar

	.section .text.psx_disable_interrupts, "ax", @progbits
	.global psx_disable_interrupts
	.type psx_disable_interrupts, @function

// psx_disable_interrupts masks interrupts and returns the previous state of
// IEc, so a caller can tell whether it was the one that masked them. That is
// what makes nested critical sections work: only the outermost restore
// unmasks.
psx_disable_interrupts:
	mfc0	$t0, COP0_STATUS
	nop
	andi	$v0, $t0, SR_IEC
	li	$t1, ~SR_IEC
	and	$t0, $t0, $t1
	mtc0	$t0, COP0_STATUS
	jr	$ra
	nop
	.size psx_disable_interrupts, . - psx_disable_interrupts

	.section .text.psx_restore_interrupts, "ax", @progbits
	.global psx_restore_interrupts
	.type psx_restore_interrupts, @function

// psx_restore_interrupts(state) unmasks interrupts if state says they were
// enabled when they were masked.
psx_restore_interrupts:
	beq	$a0, $zero, 1f
	nop
	mfc0	$t0, COP0_STATUS
	nop
	ori	$t0, $t0, SR_IEC
	mtc0	$t0, COP0_STATUS
1:
	jr	$ra
	nop
	.size psx_restore_interrupts, . - psx_restore_interrupts
