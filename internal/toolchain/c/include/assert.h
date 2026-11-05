/* assert.h */
#undef assert
#ifdef NDEBUG
#define assert(e) ((void)0)
#else
int printf(const char *fmt, ...);
void exit(int code);
#define assert(e) ((e) ? (void)0 : (printf("assertion failed: %s, %s line %d\n", #e, __FILE__, __LINE__), exit(1)))
#endif
