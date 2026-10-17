// Force-included into every LLVM, clang and lld compilation unit built for
// wasm32-wasi.
//
// LLVM's Support library is written against POSIX and reaches for signals,
// processes and resource limits that WASI does not have, and that wasi-libc
// therefore leaves undeclared. Declaring them here is enough to compile: the
// functions are provided as stubs that fail when the compiler is linked, and
// the code paths that reach them (running subprocesses, catching crashes)
// are not ones a compiler driving itself in-process takes.
#ifndef LLVM_WASI_COMPAT_H
#define LLVM_WASI_COMPAT_H
#ifdef __wasi__

#include <sys/types.h>
#include <signal.h>
#include <stddef.h>
#include <fcntl.h>

#ifdef __cplusplus
extern "C" {
#endif

// unistd.h
unsigned alarm(unsigned);
int dup(int);
int dup2(int, int);
int pipe(int[2]);
pid_t getsid(pid_t);
pid_t setsid(void);
pid_t fork(void);
int execv(const char *, char *const[]);
int execve(const char *, char *const[], char *const[]);
int execvp(const char *, char *const[]);
pid_t getppid(void);
uid_t getuid(void);
gid_t getgid(void);
int fchown(int, uid_t, gid_t);
int chown(const char *, uid_t, gid_t);

// sys/stat.h
mode_t umask(mode_t);

// fcntl.h: record locks
#ifndef F_RDLCK
#define F_RDLCK 0
#define F_WRLCK 1
#define F_UNLCK 2
#endif
#ifndef F_GETLK
#define F_GETLK 5
#define F_SETLK 6
#define F_SETLKW 7
#endif

// signal.h
typedef struct {
	int si_signo;
	int si_errno;
	int si_code;
	pid_t si_pid;
	uid_t si_uid;
	void *si_addr;
} siginfo_t;

struct sigaction {
	union {
		void (*sa_handler)(int);
		void (*sa_sigaction)(int, siginfo_t *, void *);
	} __sa_handler;
	sigset_t sa_mask;
	int sa_flags;
	void (*sa_restorer)(void);
};
#define sa_handler __sa_handler.sa_handler
#define sa_sigaction __sa_handler.sa_sigaction

#define SA_NOCLDSTOP 1
#define SA_NOCLDWAIT 2
#define SA_SIGINFO 4
#define SA_ONSTACK 0x08000000
#define SA_RESTART 0x10000000
#define SA_NODEFER 0x40000000
#define SA_RESETHAND 0x80000000

#define SIG_BLOCK 0
#define SIG_UNBLOCK 1
#define SIG_SETMASK 2

typedef struct sigaltstack {
	void *ss_sp;
	int ss_flags;
	size_t ss_size;
} stack_t;
#define SS_ONSTACK 1
#define SS_DISABLE 2
#define MINSIGSTKSZ 2048
#define SIGSTKSZ 8192

int kill(pid_t, int);
int sigemptyset(sigset_t *);
int sigfillset(sigset_t *);
int sigaddset(sigset_t *, int);
int sigdelset(sigset_t *, int);
int sigismember(const sigset_t *, int);
int sigprocmask(int, const sigset_t *__restrict, sigset_t *__restrict);
int sigaction(int, const struct sigaction *__restrict, struct sigaction *__restrict);
int sigaltstack(const stack_t *__restrict, stack_t *__restrict);
int pthread_sigmask(int, const sigset_t *__restrict, sigset_t *__restrict);

// sys/ioctl.h
struct winsize {
	unsigned short ws_row;
	unsigned short ws_col;
	unsigned short ws_xpixel;
	unsigned short ws_ypixel;
};
#define TIOCGWINSZ 0x5413

// sys/resource.h
typedef unsigned long long rlim_t;
struct rlimit {
	rlim_t rlim_cur;
	rlim_t rlim_max;
};
#define RLIMIT_CPU 0
#define RLIMIT_FSIZE 1
#define RLIMIT_DATA 2
#define RLIMIT_STACK 3
#define RLIMIT_CORE 4
#define RLIMIT_RSS 5
#define RLIMIT_NPROC 6
#define RLIMIT_NOFILE 7
#define RLIMIT_MEMLOCK 8
#define RLIMIT_AS 9
#define RLIM_INFINITY (~0ULL)
int getrlimit(int, struct rlimit *);
int setrlimit(int, const struct rlimit *);

#ifdef __cplusplus
}
#endif

#endif
#endif
