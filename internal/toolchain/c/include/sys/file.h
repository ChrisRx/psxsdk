/* sys/file.h: open's modes. */
#ifndef _SYS_FILE_H
#define _SYS_FILE_H

#define FREAD 0x0001
#define FWRITE 0x0002
#define FNBLOCK 0x0004
#define FRLOCK 0x0010
#define FWLOCK 0x0020
#define FAPPEND 0x0100
#define FCREAT 0x0200
#define FTRUNC 0x0400
#define FSCAN 0x1000
#define FRCOM 0x2000
#define FNBUF 0x4000
#define FASYNC 0x8000

#define O_RDONLY FREAD
#define O_WRONLY FWRITE
#define O_RDWR (FREAD | FWRITE)
#define O_CREAT FCREAT
#define O_NBLOCK FNBLOCK
#define O_NOBUF FNBUF
#define O_NOWAIT FASYNC

#define SEEK_SET 0
#define SEEK_CUR 1
#define SEEK_END 2

#endif
