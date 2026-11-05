/* stdlib.h */
#ifndef _STDLIB_H
#define _STDLIB_H

#include <stddef.h>

#define RAND_MAX 32767

int abs(int i);
long labs(long i);
int atoi(const char *s);
long atol(const char *s);
double atof(const char *s);
long strtol(const char *s, char **endp, int base);
unsigned long strtoul(const char *s, char **endp, int base);
double strtod(const char *s, char **endp);
int rand(void);
void srand(unsigned int seed);
void *malloc(size_t s);
void *calloc(size_t n, size_t s);
void *realloc(void *p, size_t s);
void free(void *p);
void InitHeap(unsigned long *head, unsigned long size);
void qsort(void *base, size_t n, size_t w, int (*cmp)(const void *, const void *));
void *bsearch(const void *key, const void *base, size_t n, size_t w, int (*cmp)(const void *, const void *));

#endif
