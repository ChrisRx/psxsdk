/*
 * The TinyGo runtime's platform hooks for a Net Yaroze program: what the
 * playstation target's runtime asks of the machine (see "The platform
 * hooks" in TinyGo's src/runtime/runtime_playstation.go), answered by the
 * Net Yaroze library rather than by the BIOS and the hardware, as the
 * target's own weak defaults do. The library is this platform's C library:
 * it owns the console, the vertical blank and the interrupt state, and the
 * runtime goes through it so that the library's view of each stays the
 * true one.
 *
 * These are the same calls whether the library is linked into the program
 * (internal/libps) or resident: the linker resolves them against the one
 * the build has. The link's optimizer puts each into its callers, which
 * HOOK asks of it even where it is told to save space, so a hook costs a
 * program nothing over calling the library directly: not an instruction,
 * and so not a cycle of a program's timing.
 */

#define HOOK __attribute__((always_inline))

extern char _globals_end[], _libps_heap_end[];

void InitHeap(void *head, unsigned long size);
int putchar(int c);
int VSync(int mode);
int EnterCriticalSection(void);
void ExitCriticalSection(void);

/*
 * The start-up handed the library's allocator everything from the end of
 * the image, _globals_end, up to the stack, which is where the Go heap goes too. Hand it
 * only the reserve the link script leaves it below _libps_heap_end, so
 * that its malloc, should anything in the library call it, stops there
 * rather than growing into Go's heap. Nothing has allocated from it yet,
 * so starting it again loses nothing.
 *
 * _libps_heap_end is where the link script ends the library's reserve.
 * The Go heap starts there too, unless the program was built with -hot,
 * which puts the region for its patches between the two.
 */
HOOK void
psx_init(void)
{
	InitHeap(_globals_end, (unsigned long)(_libps_heap_end - _globals_end));
}

HOOK void
psx_putchar(int c)
{
	putchar(c);
}

/*
 * The only clock reachable without installing an interrupt handler is the
 * vertical blank, so time advances in frames: 1/60s on NTSC. VSync(-1)
 * returns the number of vertical blanks since start-up, which is what the
 * library counts, and VSync(0) waits for the next.
 */
#define NS_PER_FRAME (1000 * 1000 * 1000 / 60)

HOOK long long
psx_nanotime(void)
{
	return (long long)VSync(-1) * NS_PER_FRAME;
}

HOOK void
psx_sleep(long long ns)
{
	long long frames;

	for (frames = ns / NS_PER_FRAME; frames > 0; frames--)
		VSync(0);
}

/*
 * The R3000 masks interrupts through the IEc bit of the COP0 status
 * register, but there is no need to touch it directly. The library wraps
 * exactly that in EnterCriticalSection and ExitCriticalSection, and going
 * through it keeps the kernel's own view of the state consistent with
 * ours. EnterCriticalSection reports whether it was the call that masked
 * them, which is what makes nesting work.
 *
 * With the library linked in, these two are in Go, beside the calls they
 * make (internal/libps/kernel.go, which says why).
 */
#ifndef LIBPS_CLEAN
HOOK int
psx_disable_interrupts(void)
{
	return EnterCriticalSection();
}

HOOK void
psx_restore_interrupts(int state)
{
	if (state != 0)
		ExitCriticalSection();
}
#endif
