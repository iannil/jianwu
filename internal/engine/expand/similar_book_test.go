package expand

import (
	"math"
	"testing"
)

func TestCosine(t *testing.T) {
	cases := []struct {
		a, b []float32
		want float64
	}{
		{[]float32{1, 0, 0}, []float32{1, 0, 0}, 1.0},
		{[]float32{1, 0, 0}, []float32{0, 1, 0}, 0.0},
		{[]float32{1, 1, 0}, []float32{1, 0, 0}, 0.7071067811865476},
	}
	for _, c := range cases {
		got := cosine(c.a, c.b)
		if math.Abs(got-c.want) > 1e-9 {
			t.Errorf("cosine(%v, %v) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
