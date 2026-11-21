package web

func randomPair(size int, draw func(int) int) (int, int) {
	if size < 2 {
		return -1, -1
	}
	left := draw(size)
	right := draw(size - 1)
	if right >= left {
		right++
	}
	return left, right
}
