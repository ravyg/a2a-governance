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

// Package store defines the interface and implementations for persisting
// circuit breaker state and governance event history.
package store

import (
	"context"
	"time"

	"github.com/ravyg/a2a-governance/governance"
)

// GovernanceEvent records a governance evaluation for audit purposes.
type GovernanceEvent struct {
	// ID is a unique event identifier.
	ID string `json:"id"`
	// BreakerID identifies which circuit breaker produced this event.
	BreakerID string `json:"breaker_id"`
	// Timestamp is when the evaluation occurred.
	Timestamp time.Time `json:"timestamp"`
	// State is the breaker state at evaluation time.
	State governance.BreakerState `json:"state"`
	// Result is the evaluation outcome.
	Result *governance.EvaluationResult `json:"result"`
	// RequestContext snapshot (excluding Metadata for storage efficiency).
	TaskID           string  `json:"task_id,omitempty"`
	AgentID          string  `json:"agent_id,omitempty"`
	UserID           string  `json:"user_id,omitempty"`
	TransactionValue float64 `json:"transaction_value,omitempty"`
}

// BreakerSnapshot captures the persisted state of a circuit breaker.
type BreakerSnapshot struct {
	// BreakerID is the unique identifier for this breaker.
	BreakerID string `json:"breaker_id"`
	// State is the current breaker state.
	State governance.BreakerState `json:"state"`
	// LastTrippedAt is when the breaker last tripped.
	LastTrippedAt time.Time `json:"last_tripped_at,omitempty"`
	// ConsecutiveTrips is the current count of consecutive trips.
	ConsecutiveTrips int `json:"consecutive_trips"`
	// Stats holds runtime statistics.
	Stats governance.BreakerStats `json:"stats"`
	// UpdatedAt is when this snapshot was last written.
	UpdatedAt time.Time `json:"updated_at"`
}

// StateStore persists circuit breaker state and governance events.
type StateStore interface {
	// SaveSnapshot persists the current state of a circuit breaker.
	SaveSnapshot(ctx context.Context, snapshot *BreakerSnapshot) error
	// LoadSnapshot retrieves the last persisted state.
	// Returns nil, nil if no snapshot exists.
	LoadSnapshot(ctx context.Context, breakerID string) (*BreakerSnapshot, error)
	// RecordEvent stores a governance evaluation event for audit.
	RecordEvent(ctx context.Context, event *GovernanceEvent) error
	// ListEvents returns governance events for a breaker, ordered by timestamp descending.
	ListEvents(ctx context.Context, breakerID string, limit int) ([]*GovernanceEvent, error)
}
