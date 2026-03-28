// Copyright 2026 Ravish Gupta
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package interceptor

import (
	"context"

	"github.com/a2aproject/a2a-go/a2asrv"

	"github.com/ravyg/a2a-governance/governance"
)

type riskSignalsKey struct{}

// RiskSignals holds governance risk information injected into the request context.
type RiskSignals struct {
	// BreakerState is the current circuit breaker state.
	BreakerState governance.BreakerState `json:"breaker_state"`
	// Stats holds current breaker statistics.
	Stats governance.BreakerStats `json:"stats"`
	// LastEvaluation is the most recent governance evaluation result, if available.
	LastEvaluation *governance.EvaluationResult `json:"last_evaluation,omitempty"`
}

// RiskSignalsFromContext retrieves risk signals from a context.
func RiskSignalsFromContext(ctx context.Context) (*RiskSignals, bool) {
	signals, ok := ctx.Value(riskSignalsKey{}).(*RiskSignals)
	return signals, ok
}

// RiskContextInterceptor implements a2asrv.RequestContextInterceptor and injects
// governance risk signals into the request context before agent execution.
type RiskContextInterceptor struct {
	breaker *governance.CircuitBreaker
}

var _ a2asrv.RequestContextInterceptor = (*RiskContextInterceptor)(nil)

// NewRiskContextInterceptor creates a RequestContextInterceptor that injects
// risk signals into the execution context.
func NewRiskContextInterceptor(breaker *governance.CircuitBreaker) *RiskContextInterceptor {
	return &RiskContextInterceptor{breaker: breaker}
}

func (r *RiskContextInterceptor) Intercept(ctx context.Context, _ *a2asrv.RequestContext) (context.Context, error) {
	stats := r.breaker.Stats()
	signals := &RiskSignals{
		BreakerState: stats.State,
		Stats:        stats,
	}

	// If there's a governance evaluation from the CallInterceptor, attach it.
	if evalResult, ok := GovernanceResultFromContext(ctx); ok {
		signals.LastEvaluation = evalResult
	}

	return context.WithValue(ctx, riskSignalsKey{}, signals), nil
}

// WithRiskContext returns a RequestHandlerOption that injects risk signals
// into the request context.
func WithRiskContext(breaker *governance.CircuitBreaker) a2asrv.RequestHandlerOption {
	return a2asrv.WithRequestContextInterceptor(NewRiskContextInterceptor(breaker))
}
