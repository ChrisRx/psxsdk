/* sys/ioctl.h: ioctl's commands. */
#ifndef _SYS_IOCTL_H
#define _SYS_IOCTL_H

#define FIOCNBLOCK (('f' << 8) | 1)
#define FIOCSCAN (('f' << 8) | 2)
#define TIOCRAW (('t' << 8) | 1)
#define TIOCFLUSH (('t' << 8) | 2)
#define TIOCREOPEN (('t' << 8) | 3)
#define TIOCBAUD (('t' << 8) | 4)
#define TIOCEXIT (('t' << 8) | 5)
#define TIOCDTR (('t' << 8) | 6)
#define TIOCRTS (('t' << 8) | 7)
#define TIOCLEN (('t' << 8) | 8)
#define TIOCPARITY (('t' << 8) | 9)
#define TIOSTATUS (('t' << 8) | 10)
#define TIOERRRST (('t' << 8) | 11)
#define TIOEXIST (('t' << 8) | 12)
#define TIORLEN (('t' << 8) | 13)
#define DIOFORMAT (('d' << 8) | 1)

#endif
