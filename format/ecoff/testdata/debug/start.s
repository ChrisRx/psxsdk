# The programs' entry point, as the SDK's startup.s is: assembly with no
# procedure descriptor, which calls main and stays. main calls __main,
# which runs constructors; like the SDK's, this one has none to run.
	.text
	.globl	_start
_start:
	jal	main
	nop
1:	b	1b
	nop

	.globl	__main
__main:
	jr	$31
	nop
