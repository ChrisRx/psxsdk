/*
 * cteal is the teal probe in C: showcase-gasgar's display with nothing on
 * it, 640 by 480 interlaced from one buffer, ResetGraph(1) at the top of
 * each frame, and a teal clear sent with GsDrawOt. A frame whose clear
 * misses a field shows what was in video memory before. It runs two
 * minutes, then prints END and returns.
 */
#include <libps.h>

static GsOT ot;
static GsOT_TAG tags[1 << 4];
static PACKET packets[4096];

int main(void)
{
	int f;

	printf("start\n");
	ResetGraph(0);
	GsInitGraph(640, 480, 1, 0, 0);
	GsDefDispBuff(0, 0, 0, 0);
	ot.length = 4;
	ot.org = tags;
	SetDispMask(1);
	printf("setup done\n");
	for (f = 0; f < 2 * 60 * 60; f++) {
		if (f % 600 == 0)
			printf("frame %d vsync %d\n", f, VSync(-1));
		VSync(0);
		ResetGraph(1);
		GsClearOt(0, 0, &ot);
		GsSetWorkBase(packets);
		GsSortClear(0, 128, 128, &ot);
		GsDrawOt(&ot);
	}
	printf("END\n");
	return 0;
}
