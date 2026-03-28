package governance

import "testing"

func TestEvaluationResult_String(t *testing.T) {
	tests := []struct {
		name string
		r    *EvaluationResult
		want string
	}{
		{
			name: "allowed with no policies",
			r:    &EvaluationResult{Allowed: true},
			want: "ALLOWED (0 policies evaluated)",
		},
		{
			name: "allowed with two policies",
			r: &EvaluationResult{
				Allowed:     true,
				Evaluations: make([]*Evaluation, 2),
			},
			want: "ALLOWED (2 policies evaluated)",
		},
		{
			name: "blocked with one reason",
			r: &EvaluationResult{
				Allowed:     false,
				TripReasons: []TripReason{ReasonValueThreshold},
				Evaluations: make([]*Evaluation, 1),
			},
			want: "BLOCKED by [VALUE_THRESHOLD] (1 policies evaluated)",
		},
		{
			name: "blocked with multiple reasons",
			r: &EvaluationResult{
				Allowed:     false,
				TripReasons: []TripReason{ReasonValueThreshold, ReasonVelocity},
				Evaluations: make([]*Evaluation, 3),
			},
			want: "BLOCKED by [VALUE_THRESHOLD VELOCITY] (3 policies evaluated)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.String(); got != tt.want {
				t.Errorf("EvaluationResult.String() = %q, want %q", got, tt.want)
			}
		})
	}
}
