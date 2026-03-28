package ap2

import (
	"testing"

	"github.com/ravyg/a2a-governance/governance"
)

func TestFCBStateConstants(t *testing.T) {
	// Verify AP2 FCB states map to governance states.
	if FCBStateClosed != governance.StateClosed {
		t.Errorf("FCBStateClosed = %q, want %q", FCBStateClosed, governance.StateClosed)
	}
	if FCBStateOpen != governance.StateOpen {
		t.Errorf("FCBStateOpen = %q, want %q", FCBStateOpen, governance.StateOpen)
	}
	if FCBStateHalfOpen != governance.StateHalfOpen {
		t.Errorf("FCBStateHalfOpen = %q, want %q", FCBStateHalfOpen, governance.StateHalfOpen)
	}
	if FCBStateTerminated != governance.StateTerminated {
		t.Errorf("FCBStateTerminated = %q, want %q", FCBStateTerminated, governance.StateTerminated)
	}
}

func TestTripConditionTypeValues(t *testing.T) {
	tests := []struct {
		got  TripConditionType
		want string
	}{
		{ConditionValueThreshold, "VALUE_THRESHOLD"},
		{ConditionCumulative, "CUMULATIVE"},
		{ConditionVelocity, "VELOCITY"},
		{ConditionAnomaly, "ANOMALY"},
		{ConditionVendorTrust, "VENDOR_TRUST"},
		{ConditionCredentialCheck, "CREDENTIAL_CHECK"},
		{ConditionCustom, "CUSTOM"},
	}
	for _, tt := range tests {
		if string(tt.got) != tt.want {
			t.Errorf("ConditionType = %q, want %q", tt.got, tt.want)
		}
	}
}

func TestRiskLevelFromScore(t *testing.T) {
	tests := []struct {
		name  string
		score float64
		want  RiskLevel
	}{
		{"zero", 0.0, RiskLevelLow},
		{"low", 0.1, RiskLevelLow},
		{"low boundary", 0.24, RiskLevelLow},
		{"medium threshold", 0.25, RiskLevelMedium},
		{"medium", 0.4, RiskLevelMedium},
		{"medium boundary", 0.49, RiskLevelMedium},
		{"high threshold", 0.50, RiskLevelHigh},
		{"high", 0.6, RiskLevelHigh},
		{"high boundary", 0.74, RiskLevelHigh},
		{"critical threshold", 0.75, RiskLevelCritical},
		{"critical", 0.9, RiskLevelCritical},
		{"max", 1.0, RiskLevelCritical},
		{"over max", 1.5, RiskLevelCritical},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RiskLevelFromScore(tt.score)
			if got != tt.want {
				t.Errorf("RiskLevelFromScore(%f) = %q, want %q", tt.score, got, tt.want)
			}
		})
	}
}

func TestNewRiskPayloadFromEvaluation_Allowed(t *testing.T) {
	result := &governance.EvaluationResult{
		Allowed: true,
		Evaluations: []*governance.Evaluation{
			{
				PolicyName: "value_threshold",
				Reason:     governance.ReasonValueThreshold,
				Tripped:    false,
				Score:      0.1,
				Threshold:  100,
				Actual:     10,
			},
		},
	}

	rp := NewRiskPayloadFromEvaluation(result, governance.StateClosed, "agent-1")

	if rp.AgentID != "agent-1" {
		t.Errorf("AgentID = %q, want %q", rp.AgentID, "agent-1")
	}
	if rp.RiskScore != 0.1 {
		t.Errorf("RiskScore = %f, want 0.1", rp.RiskScore)
	}
	if rp.RiskLevel != RiskLevelLow {
		t.Errorf("RiskLevel = %q, want %q", rp.RiskLevel, RiskLevelLow)
	}
	if rp.FCB == nil {
		t.Fatal("FCB should not be nil")
	}
	if rp.FCB.State != governance.StateClosed {
		t.Errorf("FCB.State = %q, want %q", rp.FCB.State, governance.StateClosed)
	}
	if rp.FCB.EscalationRequired {
		t.Error("EscalationRequired should be false for allowed request")
	}
	if len(rp.FCB.Conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(rp.FCB.Conditions))
	}
	cond := rp.FCB.Conditions[0]
	if cond.ConditionType != ConditionValueThreshold {
		t.Errorf("ConditionType = %q, want %q", cond.ConditionType, ConditionValueThreshold)
	}
	if cond.Triggered {
		t.Error("condition should not be triggered")
	}
}

func TestNewRiskPayloadFromEvaluation_Blocked(t *testing.T) {
	result := &governance.EvaluationResult{
		Allowed: false,
		TripReasons: []governance.TripReason{
			governance.ReasonValueThreshold,
			governance.ReasonVelocity,
		},
		Evaluations: []*governance.Evaluation{
			{
				PolicyName: "value_threshold",
				Reason:     governance.ReasonValueThreshold,
				Tripped:    true,
				Score:      2.0,
				Threshold:  100,
				Actual:     200,
				Message:    "exceeded",
			},
			{
				PolicyName: "velocity",
				Reason:     governance.ReasonVelocity,
				Tripped:    true,
				Score:      1.5,
				Threshold:  10,
				Actual:     15,
				Message:    "rate exceeded",
			},
		},
	}

	rp := NewRiskPayloadFromEvaluation(result, governance.StateOpen, "agent-2")

	// Score is capped at 1.0.
	if rp.RiskScore != 1.0 {
		t.Errorf("RiskScore = %f, want 1.0 (capped)", rp.RiskScore)
	}
	if rp.RiskLevel != RiskLevelCritical {
		t.Errorf("RiskLevel = %q, want %q", rp.RiskLevel, RiskLevelCritical)
	}
	if rp.FCB == nil {
		t.Fatal("FCB should not be nil")
	}
	if !rp.FCB.EscalationRequired {
		t.Error("EscalationRequired should be true for blocked request")
	}
	if rp.FCB.EscalationReason == "" {
		t.Error("EscalationReason should not be empty")
	}
	if len(rp.FCB.Conditions) != 2 {
		t.Fatalf("expected 2 conditions, got %d", len(rp.FCB.Conditions))
	}

	// Both conditions should be triggered.
	for i, cond := range rp.FCB.Conditions {
		if !cond.Triggered {
			t.Errorf("condition[%d] should be triggered", i)
		}
	}
}

func TestNewRiskPayloadFromEvaluation_NoEvaluations(t *testing.T) {
	result := &governance.EvaluationResult{
		Allowed:     true,
		Evaluations: []*governance.Evaluation{},
	}

	rp := NewRiskPayloadFromEvaluation(result, governance.StateClosed, "agent-3")

	if rp.RiskScore != 0 {
		t.Errorf("RiskScore = %f, want 0", rp.RiskScore)
	}
	if rp.RiskLevel != RiskLevelLow {
		t.Errorf("RiskLevel = %q, want %q", rp.RiskLevel, RiskLevelLow)
	}
	if len(rp.FCB.Conditions) != 0 {
		t.Errorf("expected 0 conditions, got %d", len(rp.FCB.Conditions))
	}
}

func TestNewRiskPayloadFromEvaluation_ConditionMapping(t *testing.T) {
	result := &governance.EvaluationResult{
		Allowed: true,
		Evaluations: []*governance.Evaluation{
			{
				PolicyName: "anomaly",
				Reason:     governance.ReasonAnomaly,
				Tripped:    false,
				Score:      0.3,
				Threshold:  150,
				Actual:     120,
				Message:    "within range",
			},
		},
	}

	rp := NewRiskPayloadFromEvaluation(result, governance.StateClosed, "agent-4")

	cond := rp.FCB.Conditions[0]
	if cond.ConditionType != ConditionAnomaly {
		t.Errorf("ConditionType = %q, want %q", cond.ConditionType, ConditionAnomaly)
	}
	if cond.Threshold != 150 {
		t.Errorf("Threshold = %f, want 150", cond.Threshold)
	}
	if cond.ActualValue != 120 {
		t.Errorf("ActualValue = %f, want 120", cond.ActualValue)
	}
	if cond.Message != "within range" {
		t.Errorf("Message = %q, want %q", cond.Message, "within range")
	}
}
