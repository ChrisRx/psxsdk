/* The program the ECOFF debug information tests read: built by GCC 2.95.3
 * for mipsel-ecoff with -g, beside scale.c without it (build.sh). It links
 * nothing from any SDK, and runs from start.s on its own. */

struct ball {
	int x, y;
	int dx, dy;
};

static struct ball ball = {10, 20, 1, 2};
int frames;
int total;

extern int scale(int v, int by);

static int add(int a, int b)
{
	return a + b;
}

static void move(struct ball *b)
{
	int nx = b->x + b->dx;
	int ny = b->y + b->dy;
	if (nx < 0 || nx > 300)
		b->dx = -b->dx;
	if (ny < 0 || ny > 200)
		b->dy = -b->dy;
	b->x = nx;
	b->y = ny;
}

int tick(int n)
{
	int t = scale(n, 4);
	total = add(total, t);
	move(&ball);
	return total;
}

int main(void)
{
	for (;;) {
		frames++;
		tick(frames);
	}
}
