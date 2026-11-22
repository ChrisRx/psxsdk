/* The program the ECOFF type and variable tests read: built by GCC 2.95.3
 * for mipsel-ecoff with -g at -O0, so that its locals live on the stack
 * (build.sh). It has one of each kind of type GCC describes for C. */

typedef unsigned char u8;

enum color { RED, GREEN = 5, BLUE };

struct flags {
	unsigned a : 3;
	unsigned b : 5;
	int c;
};

union word {
	int i;
	short h[2];
	u8 b[4];
};

struct node {
	struct node *next;
	int value;
};

struct point {
	short x, y;
};

struct shape {
	struct point pts[3];
	enum color color;
	const char *name;
	float scale;
	double weight;
	long long big;
	unsigned long long ubig;
	struct flags fl;
	union word w;
	int (*fn)(int);
};

struct shape shape = {{{1, 2}, {3, 4}, {5, 6}}, GREEN, "tri", 1.5f, 2.25, -5LL, 7ULL, {5, 17, -3}, {0x01020304}, 0};
static struct node nodes[3];
volatile int ticks;
const int answer = 42;

static int twice(int v)
{
	return v * 2;
}

int walk(struct node *n, int limit)
{
	int sum = 0;
	int count = 0;
	static int calls;
	calls++;
	while (n && count < limit) {
		int v = twice(n->value);
		{
			char tag = 'a' + count;
			sum += v + tag;
		}
		n = n->next;
		count++;
	}
	return sum;
}

int main(void)
{
	int i;
	struct point local = {7, 8};
	shape.fn = twice;
	for (i = 0; i < 3; i++) {
		nodes[i].value = i + 1;
		nodes[i].next = i < 2 ? &nodes[i + 1] : 0;
	}
	for (;;) {
		ticks++;
		shape.pts[0].x = local.x + walk(nodes, 3);
	}
}
