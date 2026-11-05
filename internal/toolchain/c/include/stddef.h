/* stddef.h */
#ifndef _STDDEF_H
#define _STDDEF_H

#ifndef _SIZE_T
#define _SIZE_T
typedef unsigned int size_t;
#endif
typedef int ptrdiff_t;
typedef int wchar_t;
#define WEOF (-1)
#ifndef NULL
#define NULL 0
#endif
#define offsetof(t, m) __builtin_offsetof(t, m)

#endif
