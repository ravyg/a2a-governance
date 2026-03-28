package policy

import (
	"context"
	"testing"
	"time"

	"github.com/ravyg/a2a-governance/governance"
)

func TestVelocity_Name(t *testing.T) {
	p := &Velocity{MaxRequests: 5, Window: time.Minute}
	if got := p.Name(); got != "velocity" {
		t.Errorf("Name() = %q, want %q", got, "velocity")
	}
}

func TestVelocity_WithinLimit(t *testing.T) {
	p := &Velocity{MaxRequests: 3, Window: time.Minute}
	now := time.Now()

	for i := 0; i < 3; i++ {
		eval, err := p.Evaluate(context.Background(), &governance.RequestContext{
			Timestamp: now.Add(time.Duration(i) * time.Second),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if eval.Tripped {
			t.Fatalf("should not trip on request %d (limit is 3)", i+1)
		}
	}
}

func TestVelocity_ExceedsLimit(t *testing.T) {
	p := &Velocity{MaxRequests: 3, Window: time.Minute}
	now := time.Now()

	// Fill up to limit.
	for i := 0; i < 3; i++ {
		_, _ = p.Evaluate(context.Background(), &governance.RequestContext{
			Timestamp: now.Add(time.Duration(i) * time.Second),
		})
	}

	// 4th request should trip.
	eval, err := p.Evaluate(context.Background(), &governance.RequestContext{
		Timestamp: now.Add(4 * time.Second),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !eval.Tripped {
		t.Fatal("4th request should trip (limit is 3)")
	}
	if eval.Score != 4.0/3.0 {
		t.Errorf("Score = %f, want %f", eval.Score, 4.0/3.0)
	}
	if eval.Actual != 4 {
		t.Errorf("Actual = %f, want 4", eval.Actual)
	}
}

func TestVelocity_WindowExpiry(t *testing.T) {
	p := &Velocity{MaxRequests: 2, Window: time.Minute}
	now := time.Now()

	// Fill to limit.
	for i := 0; i < 2; i++ {
		_, _ = p.Evaluate(context.Background(), &governance.RequestContext{
			Timestamp: now.Add(time.Duration(i) * time.Second),
		})
	}

	// After window expires, old timestamps are pruned.
	eval, _ := p.Evaluate(context.Background(), &governance.RequestContext{
		Timestamp: now.Add(2 * time.Minute),
	})
	if eval.Tripped {
		t.Fatal("should not trip after window expiry")
	}
}

func TestVelocity_TrippedRequestNotRecorded(t *testing.T) {
	p := &Velocity{MaxRequests: 2, Window: time.Minute}
	now := time.Now()

	// Fill to limit.
	_, _ = p.Evaluate(context.Background(), &governance.RequestContext{Timestamp: now})
	_, _ = p.Evaluate(context.Background(), &governance.RequestContext{Timestamp: now.Add(time.Second)})

	// This trips and should NOT be recorded.
	eval, _ := p.Evaluate(context.Background(), &governance.RequestContext{
		Timestamp: now.Add(2 * time.Second),
	})
	if !eval.Tripped {
		t.Fatal("should have tripped")
	}

	// Next request still sees only 2 recorded, so 3rd again trips.
	eval, _ = p.Evaluate(context.Background(), &governance.RequestContext{
		Timestamp: now.Add(3 * time.Second),
	})
	if !eval.Tripped {
		t.Fatal("should still trip (2 recorded + 1 = 3 > 2)")
	}
}

func TestVelocity_Reset(t *testing.T) {
	p := &Velocity{MaxRequests: 2, Window: time.Minute}
	now := time.Now()

	_, _ = p.Evaluate(context.Background(), &governance.RequestContext{Timestamp: now})
	_, _ = p.Evaluate(context.Background(), &governance.RequestContext{Timestamp: now.Add(time.Second)})

	p.Reset()

	eval, _ := p.Evaluate(context.Background(), &governance.RequestContext{
		Timestamp: now.Add(2 * time.Second),
	})
	if eval.Tripped {
		t.Fatal("should not trip after reset")
	}
}
