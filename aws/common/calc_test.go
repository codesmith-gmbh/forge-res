package common

import "testing"

func TestCalc(t *testing.T) {
	tests := map[string]int64{
		"x":         1,    // simple Sequence
		"x-1":       0,    // Sequence starting with 0
		"8000 + x":  8001, // Sequence starting with 8001
		"2 * (x-1)": 0,    // even Sequence starting with 0
	}
	for expr, expected := range tests {
		res, err := Eval(expr, 1)
		if err != nil {
			t.Error(err)
		}
		if res != expected {
			t.Errorf("Expecting %d got %d", expected, res)
		}
	}

}
