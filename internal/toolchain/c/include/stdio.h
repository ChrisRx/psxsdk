/* stdio.h: printing, which goes to the serial console, and its input. */
#ifndef _STDIO_H
#define _STDIO_H

#include <stddef.h>

#define EOF (-1)
#define BUFSIZ 1024

int printf(const char *fmt, ...);
int sprintf(char *s, const char *fmt, ...);
int puts(const char *s);
int putchar(int c);
int getc(int fd);
int putc(int c, int fd);
int getchar(void);
char *gets(char *s);

#endif
