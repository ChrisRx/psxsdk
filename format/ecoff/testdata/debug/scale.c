/* Built without -g: its function has a procedure descriptor and a symbol,
 * but no stabs and no lines. */
int scale(int v, int by)
{
	int r = v * by;
	return r >> 2;
}
