/* The storage of the Net Yaroze library's variables that programs reach
 * by name, for psxsdk's exp/libps, the library in Go: GsDISPENV and
 * the rest, which the yaroze package declares with go:extern. TinyGo
 * defines no symbol for a Go variable, so the storage is here, under the
 * playstation-yaroze-go target only; under the resident library the names
 * are its own, at fixed addresses. The sizes are the C structs'. */

#define VAR(name, size) unsigned char name[size] __attribute__((aligned(4)))

VAR(GsDISPENV, 20);
VAR(GsDRAWENV, 92);
VAR(GsOUT_PACKET_P, 4);
VAR(POSITION, 4);
VAR(PSDCNT, 4);
VAR(PSDIDX, 4);
VAR(PSDOFSX, 4);
VAR(PSDOFSY, 4);
VAR(GsLIGHTWSMATRIX, 32);
VAR(GsIDMATRIX, 32);
VAR(GsLSMATRIX, 32);
VAR(GsWSMATRIX, 32);
VAR(GsIDMATRIX2, 32);
VAR(CLIP2, 8);
