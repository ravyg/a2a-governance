package policy

import (
	"context"
	"testing"
	"time"

	"github.com/ravyg/a2a-governance/governance"
)

func TestCumulativeSpend_Name(t *testing.T) {
	p := &CumulativeSpend{MaxSpend: 100, Window: time.Hour}
	if got := p.Name(); got != "cumulative_spend" {
		t.Errorf("Name() = %q, want %q", got, "cumulative_spend")
	}
}

func TestCumulativeSpend_BelowLimit(t *testing.T) {
	p := &CumulativeSpend{MaxSpend: 100, Window: time.Hour}
	now := time.Now()

	eval, err := p.Evaluate(context.Background(), &governance.RequestContext{
		TransactionValue: 50,
		Timestamp:        now,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if eval.Tripped {
		t.Error("should not trip below limit")
	}
	if eval.Actual != 50 {
		t.Errorf("Actual = %f, want 50", eval.Actual)
	}
}

func TestCumulativeSpend_ExceedsLimit(t *testing.T) {
	p := &CumulativeSpend{MaxSpend: 100, Window: time.Hour}
	now := time.Now()

	// First: 60 (recorded).
	eval, _ := p.Evaluate(context.Background(), &governance.RequestContext{
		TransactionValue: 60,
		Timestamp:        now,
	})
	if eval.Tripped {
		t.Fatal("first request should not trip")
	}

	// Second: 50, cumulative = 110 > 100.
	eval, _ = p.Evaluate(context.Background(), &governance.RequestContext{
		TransactionValue: 50,
		Timestamp:        now.Add(time.Minute),
	})
	if !eval.Tripped {
		t.Fatal("second request should trip (cumulative 110 > 100)")
	}
	if eval.Score != 110.0/100.0 {
		t.Errorf("Score = %f, want %f", eval.Score, 110.0/100.0)
	}
	if eval.Message == "" {
		t.Error("expected non-empty message when tripped")
	}
}

func TestCumulativeSpend_WindowExpiry(t *testing.T) {
	p := &CumulativeSpend{MaxSpend: 100, Window: time.Hour}
	now := time.Now()

	// Record 80.
	_, _ = p.Evaluate(context.Background(), &governance.RequestContext{
		TransactionValue: 80,
		Timestamp:        now,
	})

	// After window expires, the old record is pruned; 80 again should be fine.
	eval, _ := p.Evaluate(context.Background(), &governance.RequestContext{
		TransactionValue: 80,
		Timestamp:        now.Add(2 * time.Hour),
	})
	if eval.Tripped {
		t.Fatal("should not trip after window expiry")
	}
	if eval.Actual != 80 {
		t.Errorf("Actual = %f, want 80 (old record should be pruned)", eval.Actual)
	}
}

func TestCumulativeSpend_TrippedTransactionNotRecorded(t *testing.T) {
	p := &CumulativeSpend{MaxSpend: 100, Window: time.Hour}
	now := time.Now()

	// Record 90.
	_, _ = p.Evaluate(context.Background(), &governance.RequestContext{
		TransactionValue: 90,
		Timestamp:        now,
	})

	// This trips (90+20=110), should NOT be recorded.
	eval, _ := p.Evaluate(context.Background(), &governance.RequestContext{
		TransactionValue: 20,
		Timestamp:        now.Add(time.Minute),
	})
	if !eval.Tripped {
		t.Fatal("should have tripped")
	}

	// Next request for 5 should see only the 90 (the 20 was not recorded).
	eval, _ = p.Evaluate(context.Background(), &governance.RequestContext{
		TransactionValue: 5,
		Timestamp:        now.Add(2 * time.Minute),
	})
	if eval.Tripped {
		t.Fatal("should not trip (90+5=95 <= 100)")
	}
	if eval.Actual != 95 {
		t.Errorf("Actual = %f, want 95", eval.Actual)
	}
}

func TestCumulativeSpend_Reset(t *testing.T) {
	p := &CumulativeSpend{MaxSpend: 100, Window: time.Hour}
	now := time.Now()

	_, _ = p.Evaluate(context.Background(), &governance.RequestContext{
		TransactionValue: 90,
		Timestamp:        now,
	})

	p.Reset()

	eval, _ := p.Evaluate(context.Background(), &governance.RequestContext{
		TransactionValue: 90,
		Timestamp:        now.Add(time.Minute),
	})
	if eval.Tripped {
		t.Fatal("should not trip after reset")
	}
	if eval.Actual != 90 {
		t.Errorf("Actual = %f, want 90 after reset", eval.Actual)
	}
}
