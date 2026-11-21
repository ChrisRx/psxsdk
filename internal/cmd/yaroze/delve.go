package yaroze

import (
	"context"
	"debug/elf"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/go-delve/delve/pkg/config"
	"github.com/go-delve/delve/pkg/terminal"
	"github.com/go-delve/delve/service"
	"github.com/go-delve/delve/service/dap"
	"github.com/go-delve/delve/service/debugger"
	"github.com/go-delve/delve/service/rpc2"
	"github.com/go-delve/delve/service/rpccommon"
)

// isGoProgram reports whether the ELF at path is a Go program, which Delve
// debugs: TinyGo's runtime is in it. A C program, or an ECOFF's ELF, is
// left to GDB.
func isGoProgram(path string) bool {
	f, err := elf.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	syms, err := f.Symbols()
	if err != nil {
		return false
	}
	for _, s := range syms {
		if s.Name == "runtime.run" || s.Name == "runtime.alloc" || strings.HasPrefix(s.Name, "runtime.runtimePanic") {
			return true
		}
	}
	return false
}

// delveConfig is the debugger Delve runs for the program elf, served at
// addr: its gdbremote backend dials yaroze's own server there.
func delveConfig(addr, elf string) debugger.Config {
	return debugger.Config{
		Backend:     "gdbremote:" + addr,
		WorkingDir:  ".",
		ExecuteKind: debugger.ExecutingExistingFile,
	}
}

// runDelve runs Delve's command line in this terminal on the program elf,
// served at addr, until the user quits it.
func runDelve(ctx context.Context, addr, elf string) error {
	listener, clientConn := service.ListenerPipe()
	defer listener.Close()
	disconnect := make(chan struct{})
	server := rpccommon.NewServer(&service.Config{
		Listener:       listener,
		ProcessArgs:    []string{elf},
		APIVersion:     2,
		DisconnectChan: disconnect,
		Debugger:       delveConfig(addr, elf),
	})
	if err := server.Run(); err != nil {
		return err
	}
	defer server.Stop()
	// Delve's terminal takes Control-C itself, to stop the program.
	interceptInterrupts(ctx, func() {})
	client := rpc2.NewClientFromConn(clientConn)
	conf, err := config.LoadConfig()
	if err != nil {
		conf = &config.Config{}
	}
	term := terminal.New(client, conf, false)
	if _, err := term.Run(); err != nil {
		return err
	}
	return nil
}

// serveDAP serves Delve's debug adapter protocol on l, for an editor, for
// the program elf served at addr, until the editor disconnects.
func serveDAP(ctx context.Context, l net.Listener, addr, elf string) error {
	defer l.Close()
	disconnect := make(chan struct{})
	server := dap.NewServer(&service.Config{
		Listener:       l,
		ProcessArgs:    []string{elf},
		DisconnectChan: disconnect,
		Debugger:       delveConfig(addr, elf),
	})
	defer server.Stop()
	fmt.Fprintf(os.Stderr, "yaroze: serving the debug adapter protocol on %s for an editor\n", l.Addr())
	server.Run()
	select {
	case <-disconnect:
	case <-ctx.Done():
	}
	return nil
}
