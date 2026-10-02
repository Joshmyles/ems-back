package infrastructure

import (
	"math"
	"testing"
)

func TestGini(t *testing.T) {
	if g := gini([]float64{5, 5, 5, 5}); g != 0 {
		t.Errorf("an even load should have gini 0, got %f", g)
	}
	if g := gini([]float64{0, 0, 0, 10}); math.Abs(g-0.75) > 1e-9 {
		t.Errorf("one vehicle doing everything (n=4) should be 0.75, got %f", g)
	}
	if g := gini([]float64{0, 0}); g != 0 {
		t.Errorf("no load should be 0, got %f", g)
	}
}

func TestGroupThousands(t *testing.T) {
	for in, want := range map[float64]string{0: "0", 999: "999", 6710.4: "6,710", 1234567: "1,234,567", -4500: "-4,500"} {
		if got := groupThousands(in); got != want {
			t.Errorf("groupThousands(%v) = %s, want %s", in, got, want)
		}
	}
}
