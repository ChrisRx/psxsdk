package ecoff

func extractBits(num uint32, offset, n int) uint32 {
	return ((1 << n) - 1) & (num >> offset)
}
