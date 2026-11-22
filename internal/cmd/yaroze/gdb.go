package yaroze

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/ChrisRx/wazero"
	"github.com/ChrisRx/wazero/api"
	"github.com/ChrisRx/wazero/experimental"
	"github.com/ChrisRx/wazero/imports/wasi_snapshot_preview1"
	"github.com/ChrisRx/wazero/sys"
	xcontext "go.chrisrx.dev/x/context"

	"go.chrisrx.dev/psx/internal/gdbserver"
	"go.chrisrx.dev/psx/internal/toolchain"

	"go.chrisrx.dev/pywasi/wasisock"
	"go.chrisrx.dev/pywasi/wasiterm"
)

// runGDB runs GDB's command line on the program elf, served by s at addr,
// until the user quits it, reading commands from stdin and writing to
// stdout and stderr: this terminal's, which GDB edits lines and sets modes
// on. GDB is gdb.wasm, from the
// gdb-wasi project, which toolchain.GDB downloads the first time, run here
// under wazero: WASI with the host's files, and sockets and terminal modes
// from pywasi's host functions, in the "gdbwasi" module it imports them
// from. options are more of GDB's arguments, before it connects.
//
// The terminal's interrupt key reaches this process, not GDB, while the
// program runs: it stops the program, through s, as GDB's own Control-C
// would, and GDB is told it stopped.
func runGDB(ctx context.Context, s *gdbserver.Server, addr, elf string, options []string, stdin io.Reader, stdout, stderr io.Writer) error {
	module, cache, err := toolchain.GDB(ctx, stderr)
	if err != nil {
		return err
	}
	wasm, err := os.ReadFile(module)
	if err != nil {
		return err
	}
	ccache, err := wazero.NewCompilationCacheWithDir(cache)
	if err != nil {
		return err
	}
	r := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().
		WithCompilationCache(ccache).
		WithSkipValidationOnCacheHit(true).
		WithDebugInfoEnabled(false).
		WithCoreFeatures(api.CoreFeaturesV2|experimental.CoreFeaturesExceptionHandling))
	// GDB left blocked when the run is cancelled (see below) holds the
	// runtime, so it is closed only once GDB has ended.
	ended := false
	defer func() {
		if ended {
			r.Close(context.WithoutCancel(ctx))
		}
	}()
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, r); err != nil {
		return err
	}
	host := r.NewHostModuleBuilder("gdbwasi")
	wasisock.Export(host, wasisock.Policy{Enabled: true})
	wasiterm.Export(host)
	if _, err := host.Instantiate(ctx); err != nil {
		return err
	}
	compiled, err := r.CompileModule(ctx, wasm)
	if err != nil {
		return err
	}

	// The guest sees the host's files at the same paths, and starts in
	// this directory. ~/.gdbinit is left out: it is the host GDB's, often
	// for other targets or with Python, which this GDB has not.
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	args := []string{"gdb", "-q", "-nh", "--cd=" + wd}
	if elf == "" {
		// With no symbols, nothing says what the target is.
		args = append(args, "-ex", "set architecture mips:3000")
	}
	args = append(args, options...)
	args = append(args, "-ex", "target remote "+addr)
	if elf != "" {
		if elf, err = filepath.Abs(elf); err != nil {
			return err
		}
		args = append(args, elf)
	}
	cfg := wazero.NewModuleConfig().
		WithName("").
		WithArgs(args...).
		WithFSConfig(wazero.NewFSConfig().WithDirMount("/", "/")).
		WithEnv("TERM", os.Getenv("TERM")).
		WithEnv("HOME", os.Getenv("HOME")).
		WithStdin(stdin).
		WithStdout(stdout).
		WithStderr(stderr).
		WithSysWalltime().
		WithSysNanotime().
		WithSysNanosleep().
		WithRandSource(rand.Reader)

	interceptInterrupts(ctx, func() {
		// Under Sony's monitor nothing can stop a running program: say so,
		// and how to end yaroze instead, rather than let Control-C seem to
		// do nothing.
		if err := s.Interrupt(); err != nil {
			fmt.Fprintf(stderr, "\nyaroze: %v. Press Control-C twice quickly to end yaroze.\n", err)
		}
	})

	ctx, restore := wasiterm.With(ctx, stdin, stdout, stderr)
	defer restore()
	done := make(chan error, 1)
	go func() {
		mod, err := r.InstantiateModule(ctx, compiled, cfg)
		if mod != nil {
			mod.Close(context.WithoutCancel(ctx))
		}
		done <- err
	}()
	select {
	case err = <-done:
		ended = true
	case <-ctx.Done():
		// Two Control-Cs end yaroze, but GDB may be blocked reading the
		// terminal, which nothing interrupts, so it is left behind: the
		// terminal is restored and the process ends.
		return nil
	}
	if exit := (*sys.ExitError)(nil); errors.As(err, &exit) && exit.ExitCode() == 0 {
		return nil
	}
	return err
}

// interceptInterrupts has each Control-C call fn while a debugger runs here,
// rather than end yaroze, which the first interrupt otherwise does (the
// shutdown context it runs under). Two within half a second still end it.
func interceptInterrupts(ctx context.Context, fn func()) {
	xcontext.Intercept(ctx, fn, 500*time.Millisecond)
}
