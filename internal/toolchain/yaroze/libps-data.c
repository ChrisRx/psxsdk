/* The storage of the Net Yaroze library's variables that programs reach
 * by name, for internal/libps, the library in Go: GsDISPENV and
 * the rest, which the yaroze package declares with go:extern. TinyGo
 * defines no symbol for a Go variable, so the storage is here, under the
 * static build only; under the resident library the names
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
VAR(GsLIOFF, 4);
VAR(GsLMODE, 4);
VAR(GsLIGNR, 4);
VAR(GsDISPON, 4);
VAR(GsTON, 4);
VAR(GsLIGHT_MODE, 4);
VAR(GsNDIV, 4);
VAR(math_errno, 4);
