/*
 * libps.h: the Net Yaroze library, for C programs built by yaroze.
 *
 * Written for psxsdk from the published Library Reference and the layouts
 * the yaroze package asserts, not copied from Sony's header. The functions
 * are the resident library's, at the addresses the target's
 * playstation-yaroze-symusr.S gives.
 */
#ifndef _LIBPS_H
#define _LIBPS_H

#include <sys/types.h>
#include <sys/file.h>

/* Video mode, for SetVideoMode and GetVideoMode. */
#define MODE_NTSC 0
#define MODE_PAL 1

/* 1.0 in the GTE's fixed point. */
#define ONE 4096

/* The world, as the parent of a coordinate system. */
#define WORLD ((GsCOORDINATE2 *)0)

/* GsInitGraph's interlace and offset modes. */
#define GsNONINTER 0
#define GsINTER 1
#define GsOFSGTE 0
#define GsOFSGPU 4

/* Attribute bits of objects, sprites and backgrounds. */
#define GsFOG (1 << 3)
#define GsLLMOD (1 << 5)
#define GsLOFF (1 << 6)
#define GsDIV1 (1 << 9)
#define GsDIV2 (2 << 9)
#define GsDIV3 (3 << 9)
#define GsDIV4 (4 << 9)
#define GsDIV5 (5 << 9)
#define GsROTOFF (1 << 27)
#define GsALON (1 << 30)
#define GsDOFF (1U << 31)

/* GsSetLightMode's modes. */
#define GsLMODE_NORMAL 0
#define GsLMODE_FOG 1
#define GsLMODE_LOFF 2

/* The root counters. */
#define RCntCNT2 0xf2000002

/* The sound library. */
#define SSPLAY_PAUSE 0
#define SSPLAY_PLAY 1
#define SSPLAY_INFINITY 0
#define SS_REV_TYPE_OFF 0
#define SS_REV_TYPE_ROOM 1
#define SS_REV_TYPE_STUDIO_A 2
#define SS_REV_TYPE_STUDIO_B 3
#define SS_REV_TYPE_STUDIO_C 4
#define SS_REV_TYPE_HALL 5
#define SS_REV_TYPE_SPACE 6
#define SS_REV_TYPE_ECHO 7
#define SS_REV_TYPE_DELAY 8
#define SS_REV_TYPE_PIPE 9
#define SS_MUTE_OFF 0
#define SS_MUTE_ON 1
#define SS_CD 0
#define SS_SOFF 0
#define SS_SON 1
#define SS_MIX 0
#define SS_REV 1

/* Filling in a rectangle, and clamping. */
#define setRECT(r, _x, _y, _w, _h) ((r)->x = (_x), (r)->y = (_y), (r)->w = (_w), (r)->h = (_h))
#define limitRange(x, l, h) ((x) = ((x) < (l) ? (l) : (x) > (h) ? (h) : (x)))
#define ABS(x) ((x) < 0 ? -(x) : (x))

/* A word of the scratchpad, the 1KB of fast memory in the CPU. */
#define getScratchAddr(offset) ((u_long *)(0x1f800000 + (offset) * 4))

#define GsDivMODE_NDIV 0
#define GsDivMODE_DIV 1

typedef unsigned char PACKET;

typedef struct {
	short x, y;
	short w, h;
} RECT;

typedef struct {
	short vx, vy;
	short vz, pad;
} SVECTOR;

typedef struct {
	long vx, vy;
	long vz, pad;
} VECTOR;

typedef struct {
	unsigned char r, g, b, cd;
} CVECTOR;

typedef struct {
	short m[3][3];
	long t[3];
} MATRIX;

typedef struct {
	u_long tag;
	u_long code[15];
} DR_ENV;

typedef struct {
	RECT clip;
	short ofs[2];
	RECT tw;
	u_short tpage;
	u_char dtd;
	u_char dfe;
	u_char isbg;
	u_char r0, g0, b0;
	DR_ENV dr_env;
} DRAWENV;

typedef struct {
	RECT disp;
	RECT screen;
	u_char isinter;
	u_char isrgb24;
	u_char pad0, pad1;
} DISPENV;

typedef struct {
	u_char minute;
	u_char second;
	u_char sector;
	u_char track;
} CdlLOC;

typedef struct {
	CdlLOC pos;
	u_long size;
	char name[16];
} CdlFILE;

struct DIRENTRY {
	char name[20];
	long attr;
	long size;
	struct DIRENTRY *next;
	long head;
	char system[4];
};

struct EXEC {
	u_long pc0;
	u_long gp0;
	u_long t_addr;
	u_long t_size;
	u_long d_addr;
	u_long d_size;
	u_long b_addr;
	u_long b_size;
	u_long s_addr;
	u_long s_size;
	u_long sp, fp, gp, ret, base;
};

typedef struct {
	u_short left;
	u_short right;
} SndVolume;

typedef struct {
	unsigned p : 24;
	unsigned char num : 8;
} GsOT_TAG;

typedef struct {
	u_long length;
	GsOT_TAG *org;
	u_long offset;
	u_long point;
	GsOT_TAG *tag;
} GsOT;

typedef struct {
	VECTOR scale;
	SVECTOR rotate;
	VECTOR trans;
} GsCOORD2PARAM;

typedef struct _GsCOORDINATE2 {
	u_long flg;
	MATRIX coord;
	MATRIX workm;
	GsCOORD2PARAM *param;
	struct _GsCOORDINATE2 *super;
	struct _GsCOORDINATE2 *sub;
} GsCOORDINATE2;

typedef struct {
	MATRIX view;
	GsCOORDINATE2 *super;
} GsVIEW2;

typedef struct {
	long vpx, vpy, vpz;
	long vrx, vry, vrz;
	long rz;
	GsCOORDINATE2 *super;
} GsRVIEW2;

typedef struct {
	long vx, vy, vz;
	u_char r, g, b;
} GsF_LIGHT;

typedef struct {
	short dqa;
	long dqb;
	u_char rfc, gfc, bfc;
} GsFOGPARAM;

typedef struct {
	u_long attribute;
	GsCOORDINATE2 *coord2;
	u_long *tmd;
	u_long id;
} GsDOBJ2;

typedef struct {
	u_long pmode;
	short px, py;
	u_short pw, ph;
	u_long *pixel;
	short cx, cy;
	u_short cw, ch;
	u_long *clut;
} GsIMAGE;

typedef struct {
	u_long attribute;
	short x, y;
	u_short w, h;
	u_short tpage;
	u_char u, v;
	short cx, cy;
	u_char r, g, b;
	short mx, my;
	short scalex, scaley;
	long rotate;
} GsSPRITE;

typedef struct {
	u_char u, v;
	u_short cba;
	u_short flag;
	u_short tpage;
} GsCELL;

typedef struct {
	u_char cellw, cellh;
	u_short ncellw, ncellh;
	GsCELL *base;
	u_short *index;
} GsMAP;

typedef struct {
	u_long attribute;
	short x, y;
	short w, h;
	short scrollx, scrolly;
	u_char r, g, b;
	GsMAP *map;
	short mx, my;
	short scalex, scaley;
	long rotate;
} GsBG;

typedef struct {
	u_long attribute;
	short x0, y0;
	short x1, y1;
	u_char r, g, b;
} GsLINE;

typedef struct {
	u_long attribute;
	short x0, y0;
	short x1, y1;
	u_char r0, g0, b0;
	u_char r1, g1, b1;
} GsGLINE;

typedef struct {
	u_long attribute;
	short x, y;
	u_short w, h;
	u_char r, g, b;
} GsBOXF;

typedef struct {
	short offx, offy;
} _GsPOSITION;

/* The library's variables. */
extern RECT CLIP2;
extern _GsPOSITION POSITION;
extern u_long PSDCNT;
extern short PSDIDX;
extern short PSDOFSX[2];
extern short PSDOFSY[2];
extern DRAWENV GsDRAWENV;
extern DISPENV GsDISPENV;
extern MATRIX GsLSMATRIX;
extern MATRIX GsWSMATRIX;
extern MATRIX GsLIGHTWSMATRIX;
extern MATRIX GsIDMATRIX;
extern MATRIX GsIDMATRIX2;
extern int GsLIGHT_MODE;
extern PACKET *GsOUT_PACKET_P;
extern u_long GsLMODE, GsLIGNR, GsLIOFF, GsNDIV, GsTON, GsDISPON;

/* Moves on to the next frame ID, which restarts at 1 rather than 0. */
#define GsIncFrame() (PSDCNT++, PSDCNT = PSDCNT ? PSDCNT : 1)

/* The GTE and its matrices. */
VECTOR *ApplyMatrix(MATRIX *m, SVECTOR *v0, VECTOR *v1);
VECTOR *ApplyMatrixLV(MATRIX *m, VECTOR *v0, VECTOR *v1);
SVECTOR *ApplyMatrixSV(MATRIX *m, SVECTOR *v0, SVECTOR *v1);
MATRIX *CompMatrix(MATRIX *m0, MATRIX *m1, MATRIX *m2);
MATRIX *MulMatrix0(MATRIX *m0, MATRIX *m1, MATRIX *m2);
void PopMatrix(void);
void PushMatrix(void);
MATRIX *RotMatrix(SVECTOR *r, MATRIX *m);
MATRIX *RotMatrixX(long r, MATRIX *m);
MATRIX *RotMatrixY(long r, MATRIX *m);
MATRIX *RotMatrixZ(long r, MATRIX *m);
MATRIX *ScaleMatrix(MATRIX *m, VECTOR *v);
MATRIX *ScaleMatrixL(MATRIX *m, VECTOR *v);
MATRIX *TransMatrix(MATRIX *m, VECTOR *v);
MATRIX *TransposeMatrix(MATRIX *m0, MATRIX *m1);
void gteMIMefunc(SVECTOR *otp, SVECTOR *dfp, long n, long p);

/* The GPU and video memory. */
int ClearImage(RECT *rect, u_char r, u_char g, u_char b);
int DrawSync(int mode);
u_short GetClut(int x, int y);
u_short GetTPage(int tp, int abr, int x, int y);
long GetVideoMode(void);
int LoadImage(RECT *rect, u_long *p);
int MoveImage(RECT *rect, int x, int y);
DISPENV *PutDispEnv(DISPENV *env);
DRAWENV *PutDrawEnv(DRAWENV *env);
int ResetGraph(int mode);
void SetDispMask(int mask);
long SetVideoMode(long mode);
int StoreImage(RECT *rect, u_long *p);
int VSync(int mode);
int VSyncCallback(void (*func)());

/* The on-screen fonts. */
u_long *FntFlush(int id);
void FntLoad(int tx, int ty);
int FntOpen(int x, int y, int w, int h, int isbg, int n);
int FntPrint();
int KanjiFntClose(void);
u_long *KanjiFntFlush(int id);
int KanjiFntOpen(int x, int y, int w, int h, int dx, int dy, int cx, int cy, int isbg, int n);
int KanjiFntPrint();
int Krom2Tim(u_char *sjis, u_long *taddr, int dx, int dy, int cx, int cy, u_int fg, u_int bg);
int Krom2Tim2(u_char *sjis, u_long *taddr, int dx, int dy, int cdx, int cdy, u_int fg, u_int bg);

/* The extended graphics library. */
void GsClearOt(u_short offset, u_short point, GsOT *otp);
void GsDefDispBuff(u_short x0, u_short y0, u_short x1, u_short y1);
void GsDrawOt(GsOT *otp);
int GsGetActiveBuff(void);
void GsGetLs(GsCOORDINATE2 *coord, MATRIX *m);
void GsGetLw(GsCOORDINATE2 *coord, MATRIX *m);
void GsGetLws(GsCOORDINATE2 *coord2, MATRIX *lw, MATRIX *ls);
void GsGetTimInfo(u_long *tim, GsIMAGE *im);
PACKET *GsGetWorkBase(void);
void GsInit3D(void);
void GsInitCoordinate2(GsCOORDINATE2 *super, GsCOORDINATE2 *base);
void GsInitFixBg16(GsBG *bg, u_long *work);
void GsInitGraph(u_short x_res, u_short y_res, u_short intl, u_short dither, u_short vram);
void GsLinkObject4(u_long tmd, GsDOBJ2 *obj_base, int n);
void GsMapModelingData(u_long *p);
void GsScaleScreen(SVECTOR *scale);
void GsSetAmbient(long r, long g, long b);
void GsSetClip(RECT *clip);
void GsSetClip2D(RECT *rect);
void GsSetDrawBuffClip(void);
void GsSetDrawBuffOffset(void);
int GsSetFlatLight(int id, GsF_LIGHT *lt);
void GsSetFogParam(GsFOGPARAM *fogparam);
void GsSetLightMatrix(MATRIX *mp);
void GsSetLightMode(int mode);
void GsSetLsMatrix(MATRIX *mp);
void GsSetOffset(int offx, int offy);
void GsSetOrign(int x, int y);
void GsSetProjection(long h);
int GsSetRefView2(GsRVIEW2 *pv);
int GsSetView2(GsVIEW2 *pv);
void GsSetWorkBase(PACKET *base_addr);
void GsSortBoxFill(GsBOXF *bp, GsOT *otp, u_short pri);
void GsSortClear(u_char r, u_char g, u_char b, GsOT *otp);
void GsSortFastSprite(GsSPRITE *sp, GsOT *otp, u_short pri);
void GsSortFixBg16(GsBG *bg, u_long *work, GsOT *otp, u_short pri);
void GsSortGLine(GsGLINE *lp, GsOT *otp, u_short pri);
void GsSortLine(GsLINE *lp, GsOT *otp, u_short pri);
void GsSortObject4(GsDOBJ2 *objp, GsOT *otp, int shift, u_long *scratch);
GsOT *GsSortOt(GsOT *ot_src, GsOT *ot_dest);
void GsSortSprite(GsSPRITE *sp, GsOT *otp, u_short pri);
void GsSwapDispBuff(void);

/* Sound. */
char SsGetMute(void);
void SsGetMVol(SndVolume *m_vol);
char SsGetSerialAttr(char s_num, char attr);
void SsGetSerialVol(char s_num, SndVolume *s_vol);
short SsIsEos(short access_num, short seq_num);
void SsPlayBack(short access_num, short seq_num, short l_count);
void SsSeqClose(short seq_access_num);
void SsSeqGetVol(short access_num, short seq_num, short *voll, short *volr);
short SsSeqOpen(u_long *addr, short vab_id);
void SsSeqPause(short seq_access_num);
void SsSeqPlay(short seq_access_num, char play_mode, short l_count);
void SsSeqReplay(short seq_access_num);
void SsSeqSetAccelerando(short seq_access_num, long tempo, long v_time);
void SsSeqSetNext(short seq_access_num1, short seq_access_num2);
void SsSeqSetRitardando(short seq_access_num, long tempo, long v_time);
void SsSeqSetVol(short seq_access_num, short voll, short volr);
void SsSeqStop(short seq_access_num);
void SsSetMute(char mode);
void SsSetMVol(short voll, short volr);
void SsSetSerialAttr(char s_num, char attr, char mode);
void SsSetSerialVol(char s_num, short voll, short volr);
void SsSetTempo(short access_num, short seq_num, short tempo);
void SsUtAllKeyOff(short mode);
short SsUtChangePitch(short voice, short vabId, short prog, short old_note, short old_fine, short new_note, short new_fine);
short SsUtGetReverbType(void);
short SsUtGetVVol(short vc, short *voll, short *volr);
short SsUtKeyOff(short voice, short vabId, short prog, short tone, short note);
short SsUtKeyOn(short vabId, short prog, short tone, short note, short fine, short voll, short volr);
short SsUtPitchBend(short voice, short vabId, short prog, short note, short pbend);
void SsUtReverbOff(void);
void SsUtReverbOn(void);
void SsUtSetReverbDelay(short delay);
void SsUtSetReverbDepth(short ldepth, short rdepth);
void SsUtSetReverbFeedback(short feedback);
short SsUtSetReverbType(short type);
short SsUtSetVVol(short vc, short voll, short volr);
void SsVabClose(short vab_id);
short SsVabTransfer(u_char *vh_addr, u_char *vb_addr, short vabid, short i_flag);

/* The CD. */
int CdPlay(int mode, int *tracks, int offset);
int CdReadSync(int mode, u_char *result);
int CdReadFile(char *file, u_long *addr, int nbyte);
struct EXEC *CdReadExec(char *file);
CdlFILE *CdSearchFile(CdlFILE *fp, char *name);

/* The controllers and memory cards. */
void GetPadBuf(volatile u_char **buf1, volatile u_char **buf2);
long TestCard(long chan);

/* Files, the root counters and the kernel. */
void EnterCriticalSection(void);
void ExitCriticalSection(void);
void FlushCache(void);
#define FlushCashe FlushCache
long Exec(struct EXEC *exec, long argc, char **argv);
long Load(char *name, struct EXEC *exec);
long LoadTest(char *name, struct EXEC *exec);
long open(char *name, long mode);
long close(long fd);
long lseek(long fd, long offset, long whence);
long read(long fd, char *buf, long n);
long write(long fd, char *buf, long n);
long format(char *fs);
long rename(char *src, char *dest);
long delete(char *name);
struct DIRENTRY *firstfile(char *name, struct DIRENTRY *dir);
struct DIRENTRY *nextfile(struct DIRENTRY *dir);
long GetRCnt(u_long spec);
long ResetRCnt(u_long spec);
long StartRCnt(u_long spec);
void InitHeap(u_long *head, u_long size);

/* The floating point library, with printing that knows doubles. */
extern int math_errno;
int _get_errno(void);
int printf2(const char *fmt, ...);
int sprintf2(char *s, const char *fmt, ...);
double acos(double x);
double asin(double x);
double atan(double x);
double atan2(double y, double x);
double ceil(double x);
double cos(double x);
double cosh(double x);
double exp(double x);
double floor(double x);
double fmod(double x, double y);
double frexp(double x, int *e);
double hypot(double x, double y);
double ldexp(double x, int e);
double log(double x);
double log10(double x);
double modf(double x, double *i);
double pow(double x, double y);
double sin(double x);
double sinh(double x);
double sqrt(double x);
double tan(double x);
double tanh(double x);
#define fabs(x) ((x) < 0 ? -(x) : (x))

#endif
