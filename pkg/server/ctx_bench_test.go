package server

import (
	"testing"

	"github.com/crystal-mush/gotinymush/pkg/eval"
	"github.com/crystal-mush/gotinymush/pkg/eval/functions"
)

// BenchmarkContextCreation models per-command cost: every dispatched command
// and attribute evaluation builds an EvalContext.
func BenchmarkContextCreation(b *testing.B) {
	e := newEvalTestEnv(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := MakeEvalContextWithGame(e.game, 1, func(c *eval.EvalContext) {
			functions.RegisterAll(c)
		})
		_ = ctx
	}
}

// BenchmarkContextCreationOld measures the pre-optimization path: a fresh
// 550-function table per context.
func BenchmarkContextCreationOld(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ctx := eval.NewEvalContext(nil)
		functions.RegisterAllFresh(ctx)
	}
}
