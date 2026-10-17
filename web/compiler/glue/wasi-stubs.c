// Stubs for the POSIX functions LLVM's Support library links against and
// WASI does not have. None of them is reached by a compiler that compiles
// and links in-process: there are no subprocesses to run, no signals to
// catch, no users to look up. Each fails the way a system without the
// feature would.
#include <errno.h>
#include <stddef.h>
#include <sys/types.h>

static int fail(void) {
	errno = ENOSYS;
	return -1;
}

// Processes.
pid_t fork(void) { return fail(); }
pid_t vfork(void) { return fail(); }
int execv(const char *path, char *const argv[]) { return fail(); }
int execve(const char *path, char *const argv[], char *const envp[]) { return fail(); }
int execvp(const char *file, char *const argv[]) { return fail(); }
pid_t wait(int *status) { return fail(); }
pid_t waitpid(pid_t pid, int *status, int options) { return fail(); }
pid_t wait4(pid_t pid, int *status, int options, void *rusage) { return fail(); }
int kill(pid_t pid, int sig) { return fail(); }
pid_t getppid(void) { return 1; }
pid_t getsid(pid_t pid) { return 1; }
pid_t setsid(void) { return fail(); }
int pipe(int fds[2]) { return fail(); }
int dup(int fd) { return fail(); }
int dup2(int fd, int fd2) { return fail(); }
unsigned alarm(unsigned seconds) { return 0; }

// Users.
uid_t getuid(void) { return 0; }
gid_t getgid(void) { return 0; }
int fchown(int fd, uid_t uid, gid_t gid) { return fail(); }
int chown(const char *path, uid_t uid, gid_t gid) { return fail(); }
mode_t umask(mode_t mask) { return 022; }
struct passwd;
struct passwd *getpwuid(uid_t uid) { errno = ENOENT; return NULL; }
struct passwd *getpwnam(const char *name) { errno = ENOENT; return NULL; }
int getpwuid_r(uid_t uid, struct passwd *pwd, char *buf, size_t n, struct passwd **result) { *result = NULL; return ENOENT; }
int getpwnam_r(const char *name, struct passwd *pwd, char *buf, size_t n, struct passwd **result) { *result = NULL; return ENOENT; }

// Signals: a handler can be installed and removed, and does nothing, since
// nothing ever raises a signal. The declarations come from wasi-compat.h,
// which every file here is compiled with.
int sigaction(int sig, const struct sigaction *act, struct sigaction *old) { return 0; }
int sigemptyset(sigset_t *set) { return 0; }
int sigfillset(sigset_t *set) { return 0; }
int sigaddset(sigset_t *set, int sig) { return 0; }
int sigdelset(sigset_t *set, int sig) { return 0; }
int sigismember(const sigset_t *set, int sig) { return 0; }
int sigprocmask(int how, const sigset_t *set, sigset_t *old) { return 0; }
int pthread_sigmask(int how, const sigset_t *set, sigset_t *old) { return 0; }
int sigaltstack(const stack_t *ss, stack_t *old) { return 0; }

// Resource limits: nothing is limited.
struct rlimit;
int getrlimit(int resource, struct rlimit *rlim) { return fail(); }
int setrlimit(int resource, const struct rlimit *rlim) { return fail(); }

// Memory advice: there is none to give.
int posix_madvise(void *addr, size_t len, int advice) { return 0; }

// Threads: libc++ was built with its threading API on so that LLVM's
// headers compile, and nothing ever starts a thread, so the one thread
// there is owns every lock. wasi-libc provides the rest of pthread.
#include <pthread.h>
int pthread_mutex_lock(pthread_mutex_t *m) { return 0; }
int pthread_mutex_trylock(pthread_mutex_t *m) { return 0; }
int pthread_mutex_unlock(pthread_mutex_t *m) { return 0; }
int pthread_rwlock_rdlock(pthread_rwlock_t *l) { return 0; }
int pthread_rwlock_tryrdlock(pthread_rwlock_t *l) { return 0; }
int pthread_rwlock_wrlock(pthread_rwlock_t *l) { return 0; }
int pthread_rwlock_trywrlock(pthread_rwlock_t *l) { return 0; }
int pthread_rwlock_unlock(pthread_rwlock_t *l) { return 0; }
int pthread_once(pthread_once_t *once, void (*init)(void)) {
	if (*once == 0) {
		*once = 1;
		init();
	}
	return 0;
}
pthread_t pthread_self(void) { static struct { int x; } self; return (pthread_t)&self; }
int pthread_detach(pthread_t t) { return 0; }
int pthread_join(pthread_t t, void **ret) { return fail(); }
int pthread_create(pthread_t *t, const pthread_attr_t *attr, void *(*fn)(void *), void *arg) { errno = EAGAIN; return EAGAIN; }
int pthread_setname_np(pthread_t t, const char *name) { return 0; }
int pthread_getname_np(pthread_t t, char *name, size_t len) { if (len) name[0] = 0; return 0; }

// Thread-specific data, for the one thread.
#define MAX_KEYS 64
static void *key_values[MAX_KEYS];
static unsigned key_count;
int pthread_key_create(pthread_key_t *key, void (*destructor)(void *)) {
	if (key_count == MAX_KEYS) return EAGAIN;
	*key = key_count++;
	return 0;
}
int pthread_key_delete(pthread_key_t key) { return 0; }
void *pthread_getspecific(pthread_key_t key) { return key < MAX_KEYS ? key_values[key] : NULL; }
int pthread_setspecific(pthread_key_t key, const void *value) {
	if (key >= MAX_KEYS) return EINVAL;
	key_values[key] = (void *)value;
	return 0;
}

// Aligned allocation. TinyGo's allocator, which malloc is, hands out
// 16-byte aligned blocks and refuses to free a pointer into the middle of
// one, so alignments up to 16 are plain allocations and anything larger is
// refused outright rather than freed wrongly later. Nothing in LLVM asks
// for more; this is here so the allocator in wasi-libc, which malloc would
// otherwise come from as well, stays out of the link.
#include <stdlib.h>
#include <stdio.h>
void *aligned_alloc(size_t align, size_t size) {
	if (align > 16) {
		fprintf(stderr, "aligned_alloc: alignment %zu is not supported\n", align);
		abort();
	}
	return malloc(size);
}
int posix_memalign(void **res, size_t align, size_t size) {
	if (align > 16) {
		fprintf(stderr, "posix_memalign: alignment %zu is not supported\n", align);
		abort();
	}
	*res = malloc(size);
	return *res ? 0 : ENOMEM;
}

// musl's internal names for the allocator, which wasi-libc's atexit and
// locale code call. They go to TinyGo's allocator like the public ones,
// so that wasi-libc's own allocator is never linked in beside it.
void *__libc_malloc(size_t n) { return malloc(n); }
void *__libc_calloc(size_t m, size_t n) { return calloc(m, n); }
void *__libc_realloc(void *p, size_t n) { return realloc(p, n); }
void __libc_free(void *p) { free(p); }

// Thread-local destructors: with one thread, the thread's exit is the
// program's, so they are ordinary exit handlers.
extern int __cxa_atexit(void (*)(void *), void *, void *);
int __cxa_thread_atexit(void (*dtor)(void *), void *obj, void *dso) {
	return __cxa_atexit(dtor, obj, dso);
}

// Dynamic loading: there are no libraries to load. LLVM reaches for it for
// plugins, which nothing here uses.
void *dlopen(const char *file, int mode) { return NULL; }
void *dlsym(void *handle, const char *name) { return NULL; }
int dlclose(void *handle) { return 0; }
char *dlerror(void) { return "dynamic loading is not supported"; }

// The in-process clang driver TinyGo's builder declares. The shipped build
// leaves clang out and the builder never calls this, reporting the missing
// compiler itself when a C file is not in its cache; the full build, which
// fills that cache, has the real one (TinyGo's builder/clang.cpp).
#ifndef TINYGO_HAVE_CLANG
#include <stdbool.h>
bool tinygo_clang_driver(int argc, char **argv) { return false; }
#endif
