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

package governance

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	// ErrCircuitOpen is returned when a request is rejected because the circuit is open.
	ErrCircuitOpen = errors.New("circuit breaker is open")
	// ErrCircuitTerminated is returned when the circuit has been permanently terminated.
	ErrCircuitTerminated = errors.New("circuit breaker is terminated")
)

// BreakerConfig configures a CircuitBreaker.
type BreakerConfig struct {
	// Policies are evaluated on each request. If any policy trips, the circuit opens.
	Policies []Policy

	// CooldownDuration is how long the circuit stays open before transitioning to half-open.
	// Defaults to 30 seconds.
	CooldownDuration time.Duration

	// HalfOpenMaxRequests is the number of probe requests allowed in half-open state.
	// Defaults to 1.
	HalfOpenMaxRequests int

	// ConsecutiveSuccessesToClose is how many successful probes are needed to close the circuit.
	// Defaults to 1.
	ConsecutiveSuccessesToClose int

	// ConsecutiveFailuresToTerminate is how many consecutive trip-to-open transitions
	// cause permanent termination. 0 means never terminate.
	ConsecutiveFailuresToTerminate int

	// OnEscalation is called when the circuit trips and human intervention is needed.
	OnEscalation EscalationHandler

	// OnTransition is called whenever the circuit breaker changes state.
	OnTransition TransitionHook
}

func (c *BreakerConfig) withDefaults() BreakerConfig {
	cfg := *c
	if cfg.CooldownDuration == 0 {
		cfg.CooldownDuration = 30 * time.Second
	}
	if cfg.HalfOpenMaxRequests == 0 {
		cfg.HalfOpenMaxRequests = 1
	}
	if cfg.ConsecutiveSuccessesToClose == 0 {
		cfg.ConsecutiveSuccessesToClose = 1
	}
	return cfg
}

// CircuitBreaker implements a governance-aware circuit breaker state machine.
//
// State transitions:
//
//	CLOSED --[policy trips]--> OPEN --[cooldown expires]--> HALF_OPEN
//	HALF_OPEN --[probe succeeds]--> CLOSED
//	HALF_OPEN --[probe fails]--> OPEN
//	OPEN --[max consecutive failures]--> TERMINATED
type CircuitBreaker struct {
	mu     sync.RWMutex
	config BreakerConfig

	state              BreakerState
	lastTrippedAt      time.Time
	consecutiveTrips   int
	halfOpenSuccesses  int
	halfOpenInFlight   int
	lastEvaluation     *Evaluation
	totalTrips         int64
	totalRequests      int64
	totalBlocked       int64
	now                func() time.Time // for testing
}

// NewCircuitBreaker creates a new circuit breaker with the given configuration.
func NewCircuitBreaker(config BreakerConfig) *CircuitBreaker {
	cfg := config.withDefaults()
	return &CircuitBreaker{
		config: cfg,
		state:  StateClosed,
		now:    time.Now,
	}
}

// State returns the current breaker state.
func (cb *CircuitBreaker) State() BreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.effectiveState()
}

// effectiveState computes the state, accounting for cooldown expiry.
// Caller must hold at least a read lock.
func (cb *CircuitBreaker) effectiveState() BreakerState {
	if cb.state == StateOpen && !cb.lastTrippedAt.IsZero() {
		if cb.now().Sub(cb.lastTrippedAt) >= cb.config.CooldownDuration {
			return StateHalfOpen
		}
	}
	return cb.state
}

// Evaluate runs all configured policies against the request and returns the aggregated result.
// If the circuit is open or terminated, the request is rejected without evaluating policies.
func (cb *CircuitBreaker) Evaluate(ctx context.Context, req *RequestContext) (*EvaluationResult, error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.totalRequests++

	effective := cb.effectiveState()

	// Handle state transitions from cooldown expiry.
	if effective == StateHalfOpen && cb.state == StateOpen {
		cb.transition(ctx, StateHalfOpen, nil)
	}

	switch effective {
	case StateTerminated:
		cb.totalBlocked++
		return &EvaluationResult{Allowed: false}, ErrCircuitTerminated

	case StateOpen:
		cb.totalBlocked++
		return &EvaluationResult{Allowed: false}, ErrCircuitOpen

	case StateHalfOpen:
		if cb.halfOpenInFlight >= cb.config.HalfOpenMaxRequests {
			cb.totalBlocked++
			return &EvaluationResult{Allowed: false}, ErrCircuitOpen
		}
		cb.halfOpenInFlight++
	}

	// Evaluate all policies.
	result := &EvaluationResult{
		Evaluations: make([]*Evaluation, 0, len(cb.config.Policies)),
		Allowed:     true,
	}

	for _, p := range cb.config.Policies {
		eval, err := p.Evaluate(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("policy %q evaluation failed: %w", p.Name(), err)
		}
		result.Evaluations = append(result.Evaluations, eval)
		if eval.Tripped {
			result.Allowed = false
			result.TripReasons = append(result.TripReasons, eval.Reason)
		}
	}

	// Update state based on evaluation.
	if !result.Allowed {
		cb.trip(ctx, result)
	} else if effective == StateHalfOpen {
		cb.probeSucceeded(ctx)
	}

	return result, nil
}

// trip transitions the circuit to open (or terminated) and triggers escalation.
func (cb *CircuitBreaker) trip(ctx context.Context, result *EvaluationResult) {
	cb.totalTrips++
	cb.totalBlocked++
	cb.consecutiveTrips++
	cb.halfOpenInFlight = 0
	cb.halfOpenSuccesses = 0

	// Find the first tripped evaluation for escalation.
	var triggerEval *Evaluation
	for _, eval := range result.Evaluations {
		if eval.Tripped {
			triggerEval = eval
			break
		}
	}
	cb.lastEvaluation = triggerEval

	// Check for termination threshold.
	if cb.config.ConsecutiveFailuresToTerminate > 0 &&
		cb.consecutiveTrips >= cb.config.ConsecutiveFailuresToTerminate {
		cb.transition(ctx, StateTerminated, triggerEval)
		return
	}

	cb.lastTrippedAt = cb.now()
	cb.transition(ctx, StateOpen, triggerEval)

	// Trigger human escalation.
	if cb.config.OnEscalation != nil && triggerEval != nil {
		// Run escalation outside the lock in production; here we accept the lock
		// is held for simplicity. Production users should use async escalation.
		_ = cb.config.OnEscalation(ctx, cb, triggerEval)
	}
}

// probeSucceeded records a successful probe in half-open state.
func (cb *CircuitBreaker) probeSucceeded(ctx context.Context) {
	cb.halfOpenSuccesses++
	if cb.halfOpenSuccesses >= cb.config.ConsecutiveSuccessesToClose {
		cb.consecutiveTrips = 0
		cb.halfOpenInFlight = 0
		cb.halfOpenSuccesses = 0
		cb.transition(ctx, StateClosed, nil)
	}
}

// transition changes state and fires the hook.
func (cb *CircuitBreaker) transition(ctx context.Context, to BreakerState, eval *Evaluation) {
	from := cb.state
	if from == to {
		return
	}
	cb.state = to
	if cb.config.OnTransition != nil {
		cb.config.OnTransition(ctx, from, to, eval)
	}
}

// Reset forces the circuit back to closed state. Use with caution.
func (cb *CircuitBreaker) Reset(ctx context.Context) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.consecutiveTrips = 0
	cb.halfOpenInFlight = 0
	cb.halfOpenSuccesses = 0
	cb.transition(ctx, StateClosed, nil)
}

// Stats returns current breaker statistics.
func (cb *CircuitBreaker) Stats() BreakerStats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return BreakerStats{
		State:            cb.effectiveState(),
		TotalRequests:    cb.totalRequests,
		TotalBlocked:     cb.totalBlocked,
		TotalTrips:       cb.totalTrips,
		ConsecutiveTrips: cb.consecutiveTrips,
		LastTrippedAt:    cb.lastTrippedAt,
		LastEvaluation:   cb.lastEvaluation,
	}
}

// BreakerStats holds runtime statistics for a circuit breaker.
type BreakerStats struct {
	State            BreakerState `json:"state"`
	TotalRequests    int64        `json:"total_requests"`
	TotalBlocked     int64        `json:"total_blocked"`
	TotalTrips       int64        `json:"total_trips"`
	ConsecutiveTrips int          `json:"consecutive_trips"`
	LastTrippedAt    time.Time    `json:"last_tripped_at,omitempty"`
	LastEvaluation   *Evaluation  `json:"last_evaluation,omitempty"`
}
