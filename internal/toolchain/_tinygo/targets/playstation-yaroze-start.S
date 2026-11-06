# The start-up of a program for the Net Yaroze target: the clean library's own under
# LIBPS_CLEAN, and otherwise the one for a program on the resident library. It stays
# first on the link line, so that _start is at the program's address.

#ifdef LIBPS_CLEAN
#include "playstation-yaroze-clean.S"
#else
#include "playstation-yaroze-resident.S"
#endif
