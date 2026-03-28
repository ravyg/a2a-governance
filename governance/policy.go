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
	"fmt"
	"time"
)

// TripReason identifies why a circuit breaker tripped.
type TripReason string

const (
	ReasonValueThreshold  TripReason = "VALUE_THRESHOLD"
	ReasonCumulative      TripReason = "CUMULATIVE"
	ReasonVelocity        TripReason = "VELOCITY"
	ReasonAnomaly         TripReason = "ANOMALY"
	ReasonVendorTrust     TripReason = "VENDOR_TRUST"
	ReasonCredentialCheck TripReason = "CREDENTIAL_CHECK"
	ReasonCustom          TripReason = "CUSTOM"
)

// RequestContext carries the metadata of an incoming A2A request
// that policies need for evaluation.
type RequestContext struct {
	// TaskID is the A2A task identifier.
	TaskID string
	// AgentID identifies the agent making the request.
	AgentID string
	// UserID identifies the end user, if known.
	UserID string
	// TenantID identifies the tenant in multi-tenant setups.
	TenantID string
	// TransactionValue is the monetary value of the transaction, if applicable.
	TransactionValue float64
	// Currency is the ISO 4217 currency code.
	Currency string
	// VendorID identifies the vendor/merchant involved.
	VendorID string
	// Metadata holds arbitrary key-value pairs for custom policies.
	Metadata map[string]any
	// Timestamp is when the request was received.
	Timestamp time.Time
}

// Evaluation is the result of evaluating a single policy against a request.
type Evaluation struct {
	// PolicyName identifies which policy produced this evaluation.
	PolicyName string `json:"policy_name"`
	// Reason is the trip condition type.
	Reason TripReason `json:"reason"`
	// Tripped indicates whether this policy's threshold was exceeded.
	Tripped bool `json:"tripped"`
	// Score is an optional numeric score (0.0 = no risk, 1.0 = maximum risk).
	Score float64 `json:"score,omitempty"`
	// Message provides human-readable detail about the evaluation.
	Message string `json:"message,omitempty"`
	// Threshold is the configured threshold that was evaluated against.
	Threshold float64 `json:"threshold,omitempty"`
	// Actual is the actual measured value.
	Actual float64 `json:"actual,omitempty"`
}

// Policy defines the interface for governance policies.
// Each policy evaluates a request and returns an Evaluation.
type Policy interface {
	// Name returns a unique identifier for this policy.
	Name() string
	// Evaluate checks the request against this policy's rules.
	Evaluate(ctx context.Context, req *RequestContext) (*Evaluation, error)
}

// EscalationHandler is called when a circuit breaker trips and human
// intervention is required.
type EscalationHandler func(ctx context.Context, breaker *CircuitBreaker, eval *Evaluation) error

// TransitionHook is called whenever the circuit breaker changes state.
type TransitionHook func(ctx context.Context, from, to BreakerState, eval *Evaluation)

// EvaluationResult aggregates all policy evaluations for a single request.
type EvaluationResult struct {
	// Evaluations contains the result from each policy.
	Evaluations []*Evaluation `json:"evaluations"`
	// Allowed indicates whether the request should proceed.
	Allowed bool `json:"allowed"`
	// TripReasons lists the reasons for any tripped policies.
	TripReasons []TripReason `json:"trip_reasons,omitempty"`
}

// String returns a human-readable summary.
func (r *EvaluationResult) String() string {
	if r.Allowed {
		return fmt.Sprintf("ALLOWED (%d policies evaluated)", len(r.Evaluations))
	}
	return fmt.Sprintf("BLOCKED by %v (%d policies evaluated)", r.TripReasons, len(r.Evaluations))
}
