package web

import "slices"

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

func uploadOpponent(order []string, newID, pivot string, higher bool, tried []string, draw func(int) int) (id, pool string) {
	pivotRank := slices.Index(order, pivot)
	var eligible, preferred []string
	for rank, id := range order {
		if id == newID || slices.Contains(tried, id) {
			continue
		}
		eligible = append(eligible, id)
		if pivotRank >= 0 && (higher && rank < pivotRank || !higher && rank > pivotRank) {
			preferred = append(preferred, id)
		}
	}
	if len(preferred) > 0 {
		pool := "lower"
		if higher {
			pool = "higher"
		}
		return preferred[draw(len(preferred))], pool
	}
	if len(eligible) > 0 {
		pool := "random"
		if pivotRank >= 0 {
			pool = "fallback"
		}
		return eligible[draw(len(eligible))], pool
	}
	return "", ""
}
