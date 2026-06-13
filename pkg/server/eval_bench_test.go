package server

import "testing"

// BenchmarkEvalCommon exercises the hot softcode-eval shapes: nested
// functions, iteration, registers, string ops.
func BenchmarkEvalCommon(b *testing.B) {
	e := newEvalTestEnv(b)
	exprs := []string{
		"[iter(lnum(1,50), add(##,1))]",
		"[setq(0,hello)][strcat(%q0, world, [ucstr(%q0)])]",
		"[words(iter(lnum(1,30), repeat(x,10)))]",
		"[switch(7,1,a,2,b,3,c,d)][mid(abcdefghij,3,4)]",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, x := range exprs {
			e.eval(x)
		}
	}
}
