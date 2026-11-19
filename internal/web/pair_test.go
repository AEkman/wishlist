package web

import "testing"

func TestRandomPairDrawsFromWholeWishlist(t *testing.T) {
	results := []int{11, 10}
	draw := func(limit int) int {
		t.Helper()
		if len(results) == 0 || results[0] >= limit {
			t.Fatalf("unexpected draw with upper bound %d", limit)
		}
		result := results[0]
		results = results[1:]
		return result
	}
	left, right := randomPair(12, draw)
	if left != 11 || right != 10 {
		t.Fatalf("pair = (%d, %d), want (11, 10)", left, right)
	}
}

func TestUploadOpponentPrefersDirectionAndFallsBack(t *testing.T) {
	order := []string{"bike", "lego", "skates", "ball", "new"}
	drawLast := func(size int) int { return size - 1 }
	for _, tc := range []struct {
		name, pivot string
		higher      bool
		tried       []string
		want, pool  string
	}{
		{name: "first opponent is random", want: "ball", pool: "random"},
		{name: "choose above the new rank", pivot: "ball", higher: true, tried: []string{"skates"}, want: "lego", pool: "higher"},
		{name: "choose below the winner", pivot: "lego", tried: []string{"lego", "ball"}, want: "skates", pool: "lower"},
		{name: "fall back to an untried opponent", pivot: "ball", tried: []string{"ball"}, want: "skates", pool: "fallback"},
		{name: "no opponent remains", tried: []string{"bike", "lego", "skates", "ball"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, pool := uploadOpponent(order, "new", tc.pivot, tc.higher, tc.tried, drawLast)
			if got != tc.want || pool != tc.pool {
				t.Fatalf("opponent = %q from %q, want %q from %q", got, pool, tc.want, tc.pool)
			}
		})
	}
}
