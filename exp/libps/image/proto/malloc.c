/* The library's allocator for C programs: the heap InitHeap gives, a
 * first-fit list of free blocks in address order, merged with their
 * neighbors when freed. A block is a word holding its size in bytes,
 * that word included, and then what malloc returns, so that the first
 * allocation is the heap's start plus 4, as programs have seen it; a free
 * block keeps the next free block in its second word. Sizes are rounded
 * up to words. */
struct block {
	unsigned long size;
	struct block *next; /* while free */
};

static struct block *freelist;

#define HEADER 4ul
#define MIN 8ul

__attribute__((used)) void libps_c_InitHeap(void *head, unsigned long size) {
	struct block *b = (struct block *)(((unsigned long)head + 3) & ~3ul);
	size -= (unsigned long)b - (unsigned long)head;
	size &= ~3ul;
	if (size < MIN) {
		freelist = 0;
		return;
	}
	b->size = size;
	b->next = 0;
	freelist = b;
}

__attribute__((used)) void *libps_c_malloc(unsigned long n) {
	if (n == 0)
		return 0;
	unsigned long need = (n + HEADER + 3) & ~3ul;
	if (need < MIN)
		need = MIN;
	struct block **p = &freelist;
	for (struct block *b = *p; b; p = &b->next, b = *p) {
		if (b->size < need)
			continue;
		if (b->size >= need + MIN) {
			struct block *rest = (struct block *)((char *)b + need);
			rest->size = b->size - need;
			rest->next = b->next;
			*p = rest;
			b->size = need;
		} else {
			*p = b->next;
		}
		return (char *)b + HEADER;
	}
	return 0;
}

__attribute__((used)) void libps_c_free(void *ptr) {
	if (!ptr)
		return;
	struct block *b = (struct block *)((char *)ptr - HEADER);
	struct block *prev = 0, *cur = freelist;
	while (cur && cur < b) {
		prev = cur;
		cur = cur->next;
	}
	b->next = cur;
	if (cur && (char *)b + b->size == (char *)cur) {
		b->size += cur->size;
		b->next = cur->next;
	}
	if (prev) {
		prev->next = b;
		if ((char *)prev + prev->size == (char *)b) {
			prev->size += b->size;
			prev->next = b->next;
		}
	} else {
		freelist = b;
	}
}

void *memset(void *, int, unsigned long);
void *memcpy(void *, const void *, unsigned long);

__attribute__((used)) void *libps_c_calloc(unsigned long n, unsigned long size) {
	unsigned long total = n * size;
	if (size && total / size != n)
		return 0;
	void *p = libps_c_malloc(total);
	if (p)
		memset(p, 0, total);
	return p;
}

__attribute__((used)) void *libps_c_realloc(void *ptr, unsigned long n) {
	if (!ptr)
		return libps_c_malloc(n);
	if (n == 0) {
		libps_c_free(ptr);
		return 0;
	}
	struct block *b = (struct block *)((char *)ptr - HEADER);
	unsigned long have = b->size - HEADER;
	if (have >= n)
		return ptr;
	void *q = libps_c_malloc(n);
	if (q) {
		memcpy(q, ptr, have);
		libps_c_free(ptr);
	}
	return q;
}
