/* ctype.h: the classes by arithmetic, since the library has none as functions. */
#ifndef _CTYPE_H
#define _CTYPE_H

int toupper(int c);
int tolower(int c);

#define isdigit(c) ((unsigned)(c) - '0' < 10)
#define isupper(c) ((unsigned)(c) - 'A' < 26)
#define islower(c) ((unsigned)(c) - 'a' < 26)
#define isalpha(c) (isupper(c) || islower(c))
#define isalnum(c) (isalpha(c) || isdigit(c))
#define isspace(c) ((c) == ' ' || (unsigned)(c) - '\t' < 5)
#define isxdigit(c) (isdigit(c) || (unsigned)(c) - 'A' < 6 || (unsigned)(c) - 'a' < 6)
#define isprint(c) ((unsigned)(c) - ' ' < 95)
#define isgraph(c) ((unsigned)(c) - '!' < 94)
#define iscntrl(c) ((unsigned)(c) < 32 || (c) == 127)
#define ispunct(c) (isgraph(c) && !isalnum(c))
#define isascii(c) ((unsigned)(c) < 128)
#define toascii(c) ((c) & 0x7f)

#endif
