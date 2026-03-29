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

package ap2

import (
	"math"
	"time"

	"github.com/ravyg/a2a-governance/governance"
)

// FCBState represents the AP2 Fiduciary Circuit Breaker state.
// Maps directly to governance.BreakerState for interop.
type FCBState = governance.BreakerState

const (
	FCBStateClosed     = governance.StateClosed
	FCBStateOpen       = governance.StateOpen
	FCBStateHalfOpen   = governance.StateHalfOpen
	FCBStateTerminated = governance.StateTerminated
)

// TripConditionType enumerates the AP2 Section 7.4 trip condition types.
type TripConditionType string

const (
	ConditionValueThreshold  TripConditionType = "VALUE_THRESHOLD"
	ConditionCumulative      TripConditionType = "CUMULATIVE"
	ConditionVelocity        TripConditionType = "VELOCITY"
	ConditionAnomaly         TripConditionType = "ANOMALY"
	ConditionVendorTrust     TripConditionType = "VENDOR_TRUST"
	ConditionCredentialCheck TripConditionType = "CREDENTIAL_CHECK"
	ConditionCustom          TripConditionType = "CUSTOM"
)

// TripConditionResult records the evaluation of a single trip condition
// as defined in AP2 Section 7.4.
type TripConditionResult struct {
	// ConditionType identifies the type of trip condition.
	ConditionType TripConditionType `json:"condition_type"`
	// Triggered indicates whether this condition was triggered.
	Triggered bool `json:"triggered"`
	// Threshold is the configured limit.
	Threshold float64 `json:"threshold,omitempty"`
	// ActualValue is the observed value.
	ActualValue float64 `json:"actual_value,omitempty"`
	// Message provides human-readable context.
	Message string `json:"message,omitempty"`
	// CustomType is the name of a custom condition type when ConditionType is CUSTOM.
	CustomType string `json:"custom_type,omitempty"`
}

// FCBEvaluation captures a complete FCB evaluation as defined in AP2 Section 7.4.
type FCBEvaluation struct {
	// State is the FCB state after evaluation.
	State FCBState `json:"state"`
	// PreviousState is the FCB state before evaluation.
	PreviousState FCBState `json:"previous_state"`
	// Conditions lists all evaluated trip conditions.
	Conditions []TripConditionResult `json:"conditions"`
	// EvaluatedAt is when the evaluation occurred.
	EvaluatedAt time.Time `json:"evaluated_at"`
	// CooldownUntil is when the circuit will transition to half-open (if open).
	CooldownUntil *time.Time `json:"cooldown_until,omitempty"`
	// EscalationRequired indicates whether human intervention is needed.
	EscalationRequired bool `json:"escalation_required"`
	// EscalationReason explains why escalation is needed.
	EscalationReason string `json:"escalation_reason,omitempty"`
}

// RiskPayload is the structured risk signal attached to AP2 mandates,
// providing issuers and payment networks with agent risk assessments.
type RiskPayload struct {
	// FCB is the Fiduciary Circuit Breaker evaluation.
	FCB *FCBEvaluation `json:"fcb,omitempty"`
	// AgentID identifies the agent that produced this risk signal.
	AgentID string `json:"agent_id,omitempty"`
	// SessionID links to the A2A session/task context.
	SessionID string `json:"session_id,omitempty"`
	// RiskScore is an aggregate risk score (0.0 = no risk, 1.0 = maximum risk).
	RiskScore float64 `json:"risk_score"`
	// RiskLevel is a human-readable risk categorization.
	RiskLevel RiskLevel `json:"risk_level"`
	// Metadata holds additional risk context.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// RiskLevel categorizes the overall risk of a transaction.
type RiskLevel string

const (
	RiskLevelLow      RiskLevel = "LOW"
	RiskLevelMedium   RiskLevel = "MEDIUM"
	RiskLevelHigh     RiskLevel = "HIGH"
	RiskLevelCritical RiskLevel = "CRITICAL"
)

// RiskLevelFromScore maps a numeric risk score to a RiskLevel.
// Scores are clamped to [0.0, 1.0]; NaN is treated as Critical.
func RiskLevelFromScore(score float64) RiskLevel {
	if math.IsNaN(score) {
		return RiskLevelCritical
	}
	// Clamp to valid range.
	if score < 0 {
		score = 0
	} else if score > 1.0 {
		score = 1.0
	}
	switch {
	case score < 0.25:
		return RiskLevelLow
	case score < 0.50:
		return RiskLevelMedium
	case score < 0.75:
		return RiskLevelHigh
	default:
		return RiskLevelCritical
	}
}

// NewRiskPayloadFromEvaluation creates a RiskPayload from a governance
// EvaluationResult and circuit breaker state.
func NewRiskPayloadFromEvaluation(result *governance.EvaluationResult, state governance.BreakerState, agentID string) *RiskPayload {
	now := time.Now()

	fcb := &FCBEvaluation{
		State:       state,
		EvaluatedAt: now,
		Conditions:  make([]TripConditionResult, 0, len(result.Evaluations)),
	}

	var maxScore float64
	for _, eval := range result.Evaluations {
		cond := TripConditionResult{
			ConditionType: TripConditionType(eval.Reason),
			Triggered:     eval.Tripped,
			Threshold:     eval.Threshold,
			ActualValue:   eval.Actual,
			Message:       eval.Message,
		}
		fcb.Conditions = append(fcb.Conditions, cond)
		if eval.Score > maxScore {
			maxScore = eval.Score
		}
	}

	if !result.Allowed {
		fcb.EscalationRequired = true
		fcb.EscalationReason = result.String()
	}

	riskScore := maxScore
	if riskScore > 1.0 {
		riskScore = 1.0
	}

	return &RiskPayload{
		FCB:       fcb,
		AgentID:   agentID,
		RiskScore: riskScore,
		RiskLevel: RiskLevelFromScore(riskScore),
	}
}
