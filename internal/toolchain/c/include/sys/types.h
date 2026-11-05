/* sys/types.h: the short names of the unsigned types. */
#ifndef _SYS_TYPES_H
#define _SYS_TYPES_H

typedef unsigned char u_char;
typedef unsigned short u_short;
typedef unsigned int u_int;
typedef unsigned long u_long;
typedef unsigned short ushort;
typedef unsigned int uint;
typedef unsigned long ulong;
typedef long daddr_t;
typedef char *caddr_t;
typedef long *qaddr_t;
typedef unsigned long ino_t;
typedef long swblk_t;
typedef long off_t;
typedef short dev_t;
typedef long time_t;
typedef unsigned short uid_t;
typedef unsigned short gid_t;
typedef struct _quad {
	long val[2];
} quad;
typedef struct _physadr {
	int r[1];
} *physadr;
typedef struct label_t {
	int val[12];
} label_t;

#define NBBY 8
#define major(x) ((int)(((unsigned)(x) >> 8) & 0xff))
#define minor(x) ((int)((x) & 0xff))
#define makedev(x, y) ((dev_t)(((x) << 8) | (y)))

#ifndef NULL
#define NULL 0
#endif

#ifndef _SIZE_T
#define _SIZE_T
typedef unsigned int size_t;
#endif

#endif
