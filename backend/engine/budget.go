package engine

import (
	"context"
	"sync/atomic"
)

type executionBudgetContextKey struct{}

type executionBudget struct {
	maxIterations int64
	maxToolCalls  int64
	maxTokens     int64
	iterations    atomic.Int64
	toolCalls     atomic.Int64
	tokens        atomic.Int64
}

func newExecutionBudget(budget Budget) *executionBudget {
	return &executionBudget{
		maxIterations: int64(budget.MaxIterations),
		maxToolCalls:  int64(budget.MaxToolCalls),
		maxTokens:     budget.MaxTokens,
	}
}

func executionBudgetFromContext(ctx context.Context) *executionBudget {
	if ctx == nil {
		return nil
	}
	budget, _ := ctx.Value(executionBudgetContextKey{}).(*executionBudget)
	return budget
}

func (b *executionBudget) addIteration() int64 {
	return b.iterations.Add(1)
}

func (b *executionBudget) addToolCall() int64 {
	return b.toolCalls.Add(1)
}

func (b *executionBudget) addTokens(count int64) int64 {
	if count <= 0 {
		return b.tokens.Load()
	}
	return b.tokens.Add(count)
}
func (b *executionBudget) exceededIterations() bool { return b.iterations.Load() > b.maxIterations }
func (b *executionBudget) exceededToolCalls() bool  { return b.toolCalls.Load() > b.maxToolCalls }
func (b *executionBudget) exceededTokens() bool     { return b.tokens.Load() > b.maxTokens }
