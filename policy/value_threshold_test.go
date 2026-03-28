package policy

import (
	"context"
	"testing"

	"github.com/ravyg/a2a-governance/governance"
)

func TestValueThreshold_Name(t *testing.T) {
	p := &ValueThreshold{MaxValue: 100}
	if got := p.Name(); got != "value_threshold" {
		t.Errorf("Name() = %q, want %q", got, "value_threshold")
	}
}

func TestValueThreshold_Evaluate(t *testing.T) {
	tests := []struct {
		name      string
		maxValue  float64
		txValue   float64
		wantTrip  bool
		wantScore float64
	}{
		{
			name:     "below threshold",
			maxValue: 100,
			txValue:  50,
			wantTrip: false,
		},
		{
			name:     "at threshold",
			maxValue: 100,
			txValue:  100,
			wantTrip: false,
		},
		{
			name:      "above threshold",
			maxValue:  100,
			txValue:   200,
			wantTrip:  true,
			wantScore: 2.0,
		},
		{
			name:      "slightly above threshold",
			maxValue:  100,
			txValue:   100.01,
			wantTrip:  true,
			wantScore: 100.01 / 100,
		},
		{
			name:     "zero value",
			maxValue: 100,
			txValue:  0,
			wantTrip: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &ValueThreshold{MaxValue: tt.maxValue}
			eval, err := p.Evaluate(context.Background(), &governance.RequestContext{
				TransactionValue: tt.txValue,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if eval.Tripped != tt.wantTrip {
				t.Errorf("Tripped = %v, want %v", eval.Tripped, tt.wantTrip)
			}
			if eval.Tripped && eval.Score != tt.wantScore {
				t.Errorf("Score = %f, want %f", eval.Score, tt.wantScore)
			}
			if eval.PolicyName != "value_threshold" {
				t.Errorf("PolicyName = %q, want %q", eval.PolicyName, "value_threshold")
			}
			if eval.Reason != governance.ReasonValueThreshold {
				t.Errorf("Reason = %q, want %q", eval.Reason, governance.ReasonValueThreshold)
			}
			if eval.Threshold != tt.maxValue {
				t.Errorf("Threshold = %f, want %f", eval.Threshold, tt.maxValue)
			}
			if eval.Actual != tt.txValue {
				t.Errorf("Actual = %f, want %f", eval.Actual, tt.txValue)
			}
			if eval.Tripped && eval.Message == "" {
				t.Error("expected non-empty message when tripped")
			}
		})
	}
}
