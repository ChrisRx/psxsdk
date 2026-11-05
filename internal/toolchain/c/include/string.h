/* string.h */
#ifndef _STRING_H
#define _STRING_H

#include <stddef.h>

void *malloc(size_t s);

char *strcat(char *dest, const char *src);
char *strncat(char *dest, const char *src, size_t n);
char *strchr(const char *s, int c);
char *strrchr(const char *s, int c);
int strcmp(const char *a, const char *b);
int strncmp(const char *a, const char *b, size_t n);
char *strcpy(char *dest, const char *src);
char *strncpy(char *dest, const char *src, size_t n);
size_t strlen(const char *s);
size_t strcspn(const char *a, const char *b);
size_t strspn(const char *a, const char *b);
char *strpbrk(const char *a, const char *b);
char *strstr(const char *a, const char *b);
char *strtok(char *a, const char *b);
void *memchr(const void *s, int c, size_t n);
int memcmp(const void *a, const void *b, size_t n);
void *memcpy(void *dest, const void *src, size_t n);
void *memmove(void *dest, const void *src, size_t n);
void *memset(void *s, int c, size_t n);
int bcmp(const void *a, const void *b, size_t n);
void bcopy(const void *src, void *dest, size_t n);
void bzero(void *p, size_t n);

#define strdup(s) strcpy(malloc(strlen(s) + 1), (s))

#endif
