// sys/wait.h for wasm32-wasi, which has no processes and so no header for
// waiting on them. LLVM's Support/Unix includes it unconditionally; the
// functions are stubs that fail.
#ifndef _SYS_WAIT_H
#define _SYS_WAIT_H

#include <sys/types.h>
#include <signal.h>

#ifdef __cplusplus
extern "C" {
#endif

#define WNOHANG 1
#define WUNTRACED 2

#define WEXITSTATUS(s) (((s) & 0xff00) >> 8)
#define WTERMSIG(s) ((s) & 0x7f)
#define WSTOPSIG(s) WEXITSTATUS(s)
#define WIFEXITED(s) (!WTERMSIG(s))
#define WIFSTOPPED(s) ((short)((((s) & 0xffff) * 0x10001U) >> 8) > 0x7f00)
#define WIFSIGNALED(s) (((s) & 0xffff) - 1U < 0xffu)
#define WCOREDUMP(s) ((s) & 0x80)

pid_t wait(int *);
pid_t waitpid(pid_t, int *, int);
struct rusage;
pid_t wait4(pid_t, int *, int, struct rusage *);

#ifdef __cplusplus
}
#endif

#endif
