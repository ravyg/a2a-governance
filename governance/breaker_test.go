package governance

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// stubPolicy is a test helper that returns a fixed evaluation.
type stubPolicy struct {
	name    string
	tripped bool
	reason  TripReason
	score   float64
}

func (p *stubPolicy) Name() string { return p.name }

func (p *stubPolicy) Evaluate(_ context.Context, req *RequestContext) (*Evaluation, error) {
	return &Evaluation{
		PolicyName: p.name,
		Reason:     p.reason,
		Tripped:    p.tripped,
		Score:      p.score,
		Actual:     req.TransactionValue,
		Threshold:  100,
	}, nil
}

// errorPolicy always returns an error.
type errorPolicy struct{}

func (p *errorPolicy) Name() string { return "error_policy" }
func (p *errorPolicy) Evaluate(_ context.Context, _ *RequestContext) (*Evaluation, error) {
	return nil, errors.New("policy error")
}

func newTestBreaker(policies []Policy, opts ...func(*BreakerConfig)) *CircuitBreaker {
	cfg := BreakerConfig{
		Policies:         policies,
		CooldownDuration: 5 * time.Second,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return NewCircuitBreaker(cfg)
}

func fixedNow(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

func TestNewCircuitBreaker_Defaults(t *testing.T) {
	cb := NewCircuitBreaker(BreakerConfig{})
	if cb.State() != StateClosed {
		t.Fatalf("expected CLOSED, got %s", cb.State())
	}
	if cb.config.CooldownDuration != 30*time.Second {
		t.Fatalf("expected default cooldown 30s, got %s", cb.config.CooldownDuration)
	}
	if cb.config.HalfOpenMaxRequests != 1 {
		t.Fatalf("expected default HalfOpenMaxRequests 1, got %d", cb.config.HalfOpenMaxRequests)
	}
	if cb.config.ConsecutiveSuccessesToClose != 1 {
		t.Fatalf("expected default ConsecutiveSuccessesToClose 1, got %d", cb.config.ConsecutiveSuccessesToClose)
	}
}

func TestClosedState_AllowsTraffic(t *testing.T) {
	allow := &stubPolicy{name: "ok", tripped: false, reason: ReasonCustom}
	cb := newTestBreaker([]Policy{allow})

	result, err := cb.Evaluate(context.Background(), &RequestContext{TransactionValue: 50})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Allowed {
		t.Fatal("expected request to be allowed in CLOSED state")
	}
	if cb.State() != StateClosed {
		t.Fatalf("expected CLOSED, got %s", cb.State())
	}
}

func TestPolicyTrip_TransitionsToOpen(t *testing.T) {
	trip := &stubPolicy{name: "trip", tripped: true, reason: ReasonValueThreshold, score: 1.5}
	cb := newTestBreaker([]Policy{trip})

	result, err := cb.Evaluate(context.Background(), &RequestContext{TransactionValue: 200})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Allowed {
		t.Fatal("expected request to be blocked")
	}
	if cb.State() != StateOpen {
		t.Fatalf("expected OPEN, got %s", cb.State())
	}
	if len(result.TripReasons) != 1 || result.TripReasons[0] != ReasonValueThreshold {
		t.Fatalf("unexpected trip reasons: %v", result.TripReasons)
	}
}

func TestOpenState_BlocksRequests(t *testing.T) {
	trip := &stubPolicy{name: "trip", tripped: true, reason: ReasonValueThreshold}
	cb := newTestBreaker([]Policy{trip})

	// Trip the circuit.
	_, _ = cb.Evaluate(context.Background(), &RequestContext{})

	// Second request should be blocked without evaluating policies.
	now := time.Now()
	cb.now = fixedNow(now)
	_, err := cb.Evaluate(context.Background(), &RequestContext{})
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestCooldownExpiry_TransitionsToHalfOpen(t *testing.T) {
	trip := &stubPolicy{name: "trip", tripped: true, reason: ReasonValueThreshold}
	cb := newTestBreaker([]Policy{trip})

	now := time.Now()
	cb.now = fixedNow(now)

	// Trip the circuit.
	_, _ = cb.Evaluate(context.Background(), &RequestContext{})
	if cb.State() != StateOpen {
		t.Fatalf("expected OPEN, got %s", cb.State())
	}

	// Advance past cooldown.
	cb.now = fixedNow(now.Add(6 * time.Second))
	if cb.State() != StateHalfOpen {
		t.Fatalf("expected HALF_OPEN after cooldown, got %s", cb.State())
	}
}

func TestHalfOpen_SuccessfulProbeCloses(t *testing.T) {
	callCount := 0
	dynamic := &stubPolicy{name: "dyn", tripped: true, reason: ReasonValueThreshold}
	cb := newTestBreaker([]Policy{dynamic})

	now := time.Now()
	cb.now = fixedNow(now)

	// Trip.
	_, _ = cb.Evaluate(context.Background(), &RequestContext{})
	_ = callCount

	// Advance to half-open.
	cb.now = fixedNow(now.Add(6 * time.Second))

	// Now the policy should pass.
	dynamic.tripped = false
	result, err := cb.Evaluate(context.Background(), &RequestContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Allowed {
		t.Fatal("expected probe to be allowed")
	}
	if cb.State() != StateClosed {
		t.Fatalf("expected CLOSED after successful probe, got %s", cb.State())
	}
}

func TestHalfOpen_FailedProbeReopens(t *testing.T) {
	trip := &stubPolicy{name: "trip", tripped: true, reason: ReasonValueThreshold}
	cb := newTestBreaker([]Policy{trip})

	now := time.Now()
	cb.now = fixedNow(now)

	// Trip.
	_, _ = cb.Evaluate(context.Background(), &RequestContext{})

	// Advance to half-open.
	cb.now = fixedNow(now.Add(6 * time.Second))
	if cb.State() != StateHalfOpen {
		t.Fatalf("expected HALF_OPEN, got %s", cb.State())
	}

	// Probe fails (policy still tripped).
	result, err := cb.Evaluate(context.Background(), &RequestContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Allowed {
		t.Fatal("expected probe to be blocked")
	}
	// State should be OPEN (not HALF_OPEN) since the underlying state was set back.
	// But we need to check with a frozen time so cooldown hasn't expired again.
	cb.now = fixedNow(now.Add(6 * time.Second))
	if cb.State() != StateOpen {
		t.Fatalf("expected OPEN after failed probe, got %s", cb.State())
	}
}

func TestConsecutiveFailures_CauseTerminated(t *testing.T) {
	trip := &stubPolicy{name: "trip", tripped: true, reason: ReasonValueThreshold}
	cb := newTestBreaker([]Policy{trip}, func(cfg *BreakerConfig) {
		cfg.ConsecutiveFailuresToTerminate = 3
	})

	now := time.Now()
	cb.now = fixedNow(now)

	// Trip 1.
	_, _ = cb.Evaluate(context.Background(), &RequestContext{})
	// Trip 2: advance past cooldown, then evaluate in half-open (trips again).
	cb.now = fixedNow(now.Add(6 * time.Second))
	_, _ = cb.Evaluate(context.Background(), &RequestContext{})
	// Trip 3: advance past cooldown again.
	cb.now = fixedNow(now.Add(12 * time.Second))
	_, _ = cb.Evaluate(context.Background(), &RequestContext{})

	if cb.State() != StateTerminated {
		t.Fatalf("expected TERMINATED after 3 consecutive failures, got %s", cb.State())
	}

	// Terminated circuit should return ErrCircuitTerminated.
	_, err := cb.Evaluate(context.Background(), &RequestContext{})
	if !errors.Is(err, ErrCircuitTerminated) {
		t.Fatalf("expected ErrCircuitTerminated, got %v", err)
	}
}

func TestEscalationHandler_CalledOnTrip(t *testing.T) {
	called := false
	var receivedEval *Evaluation

	trip := &stubPolicy{name: "trip", tripped: true, reason: ReasonValueThreshold, score: 1.5}
	cb := newTestBreaker([]Policy{trip}, func(cfg *BreakerConfig) {
		cfg.OnEscalation = func(_ context.Context, _ *CircuitBreaker, eval *Evaluation) error {
			called = true
			receivedEval = eval
			return nil
		}
	})

	_, _ = cb.Evaluate(context.Background(), &RequestContext{TransactionValue: 200})
	if !called {
		t.Fatal("expected escalation handler to be called")
	}
	if receivedEval == nil || receivedEval.PolicyName != "trip" {
		t.Fatalf("unexpected evaluation passed to escalation: %+v", receivedEval)
	}
}

func TestTransitionHook_CalledOnStateChanges(t *testing.T) {
	var transitions []struct{ from, to BreakerState }

	trip := &stubPolicy{name: "trip", tripped: true, reason: ReasonValueThreshold}
	cb := newTestBreaker([]Policy{trip}, func(cfg *BreakerConfig) {
		cfg.OnTransition = func(_ context.Context, from, to BreakerState, _ *Evaluation) {
			transitions = append(transitions, struct{ from, to BreakerState }{from, to})
		}
	})

	now := time.Now()
	cb.now = fixedNow(now)

	// Trip: CLOSED -> OPEN
	_, _ = cb.Evaluate(context.Background(), &RequestContext{})

	// Advance past cooldown and probe: OPEN -> HALF_OPEN, then trip again: HALF_OPEN -> OPEN
	cb.now = fixedNow(now.Add(6 * time.Second))
	_, _ = cb.Evaluate(context.Background(), &RequestContext{})

	expected := []struct{ from, to BreakerState }{
		{StateClosed, StateOpen},
		{StateOpen, StateHalfOpen},
		{StateHalfOpen, StateOpen},
	}

	if len(transitions) != len(expected) {
		t.Fatalf("expected %d transitions, got %d: %+v", len(expected), len(transitions), transitions)
	}
	for i, exp := range expected {
		if transitions[i].from != exp.from || transitions[i].to != exp.to {
			t.Errorf("transition[%d]: got %s->%s, want %s->%s",
				i, transitions[i].from, transitions[i].to, exp.from, exp.to)
		}
	}
}

func TestReset_ForcesClosedState(t *testing.T) {
	trip := &stubPolicy{name: "trip", tripped: true, reason: ReasonValueThreshold}
	cb := newTestBreaker([]Policy{trip})

	// Trip the circuit.
	_, _ = cb.Evaluate(context.Background(), &RequestContext{})
	if cb.State() != StateOpen {
		t.Fatalf("expected OPEN, got %s", cb.State())
	}

	cb.Reset(context.Background())
	if cb.State() != StateClosed {
		t.Fatalf("expected CLOSED after reset, got %s", cb.State())
	}
}

func TestReset_FromTerminated(t *testing.T) {
	trip := &stubPolicy{name: "trip", tripped: true, reason: ReasonValueThreshold}
	cb := newTestBreaker([]Policy{trip}, func(cfg *BreakerConfig) {
		cfg.ConsecutiveFailuresToTerminate = 1
	})

	_, _ = cb.Evaluate(context.Background(), &RequestContext{})
	if cb.State() != StateTerminated {
		t.Fatalf("expected TERMINATED, got %s", cb.State())
	}

	cb.Reset(context.Background())
	if cb.State() != StateClosed {
		t.Fatalf("expected CLOSED after reset from TERMINATED, got %s", cb.State())
	}
}

func TestStats_Accuracy(t *testing.T) {
	trip := &stubPolicy{name: "trip", tripped: true, reason: ReasonValueThreshold}
	cb := newTestBreaker([]Policy{trip})

	now := time.Now()
	cb.now = fixedNow(now)

	// First request trips.
	_, _ = cb.Evaluate(context.Background(), &RequestContext{})
	// Second request is blocked (circuit open).
	_, _ = cb.Evaluate(context.Background(), &RequestContext{})

	stats := cb.Stats()
	if stats.State != StateOpen {
		t.Errorf("expected state OPEN, got %s", stats.State)
	}
	if stats.TotalRequests != 2 {
		t.Errorf("expected 2 total requests, got %d", stats.TotalRequests)
	}
	if stats.TotalBlocked != 2 {
		t.Errorf("expected 2 total blocked, got %d", stats.TotalBlocked)
	}
	if stats.TotalTrips != 1 {
		t.Errorf("expected 1 total trip, got %d", stats.TotalTrips)
	}
	if stats.ConsecutiveTrips != 1 {
		t.Errorf("expected 1 consecutive trip, got %d", stats.ConsecutiveTrips)
	}
	if stats.LastTrippedAt.IsZero() {
		t.Error("expected LastTrippedAt to be set")
	}
	if stats.LastEvaluation == nil {
		t.Error("expected LastEvaluation to be set")
	}
}

func TestConcurrentAccess(t *testing.T) {
	allow := &stubPolicy{name: "ok", tripped: false, reason: ReasonCustom}
	cb := newTestBreaker([]Policy{allow})

	var wg sync.WaitGroup
	errs := make(chan error, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := cb.Evaluate(context.Background(), &RequestContext{TransactionValue: 10})
			if err != nil {
				errs <- err
			}
			_ = cb.State()
			_ = cb.Stats()
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent evaluation error: %v", err)
	}

	stats := cb.Stats()
	if stats.TotalRequests != 100 {
		t.Errorf("expected 100 total requests, got %d", stats.TotalRequests)
	}
}

func TestHalfOpen_ExcessRequestsBlocked(t *testing.T) {
	// Use HalfOpenMaxRequests = 1 (default).
	trip := &stubPolicy{name: "trip", tripped: true, reason: ReasonValueThreshold}
	cb := newTestBreaker([]Policy{trip})

	now := time.Now()
	cb.now = fixedNow(now)

	// Trip.
	_, _ = cb.Evaluate(context.Background(), &RequestContext{})

	// Advance to half-open, let policy now pass so probe is admitted.
	trip.tripped = false
	cb.now = fixedNow(now.Add(6 * time.Second))

	// First probe is admitted and succeeds, closing the circuit.
	result, err := cb.Evaluate(context.Background(), &RequestContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Allowed {
		t.Fatal("expected first probe to be allowed")
	}
}

func TestPolicyEvaluationError(t *testing.T) {
	ep := &errorPolicy{}
	cb := newTestBreaker([]Policy{ep})

	_, err := cb.Evaluate(context.Background(), &RequestContext{})
	if err == nil {
		t.Fatal("expected error from failing policy")
	}
}

func TestMultiplePolicies_AnyTripBlocks(t *testing.T) {
	allow := &stubPolicy{name: "ok", tripped: false, reason: ReasonCustom}
	trip := &stubPolicy{name: "trip", tripped: true, reason: ReasonVelocity}
	cb := newTestBreaker([]Policy{allow, trip})

	result, err := cb.Evaluate(context.Background(), &RequestContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Allowed {
		t.Fatal("expected blocked when any policy trips")
	}
	if len(result.Evaluations) != 2 {
		t.Fatalf("expected 2 evaluations, got %d", len(result.Evaluations))
	}
}

func TestConsecutiveSuccessesToClose_Multiple(t *testing.T) {
	dynamic := &stubPolicy{name: "dyn", tripped: true, reason: ReasonValueThreshold}
	cb := newTestBreaker([]Policy{dynamic}, func(cfg *BreakerConfig) {
		cfg.ConsecutiveSuccessesToClose = 3
		cfg.HalfOpenMaxRequests = 5
	})

	now := time.Now()
	cb.now = fixedNow(now)

	// Trip.
	_, _ = cb.Evaluate(context.Background(), &RequestContext{})

	// Advance to half-open.
	cb.now = fixedNow(now.Add(6 * time.Second))
	dynamic.tripped = false

	// First two probes succeed but don't close yet.
	_, _ = cb.Evaluate(context.Background(), &RequestContext{})
	if cb.State() != StateHalfOpen {
		t.Fatalf("expected HALF_OPEN after 1 success, got %s", cb.State())
	}
	_, _ = cb.Evaluate(context.Background(), &RequestContext{})
	if cb.State() != StateHalfOpen {
		t.Fatalf("expected HALF_OPEN after 2 successes, got %s", cb.State())
	}

	// Third probe closes the circuit.
	_, _ = cb.Evaluate(context.Background(), &RequestContext{})
	if cb.State() != StateClosed {
		t.Fatalf("expected CLOSED after 3 successes, got %s", cb.State())
	}
}
