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

package store

import (
	"context"
	"sync"
)

// DefaultMaxEventsPerBreaker is the default maximum number of governance events
// retained per breaker to prevent unbounded memory growth.
const DefaultMaxEventsPerBreaker = 10_000

// MemoryStore is an in-memory implementation of StateStore suitable for
// testing and single-instance deployments.
type MemoryStore struct {
	mu                 sync.RWMutex
	snapshots          map[string]*BreakerSnapshot
	events             map[string][]*GovernanceEvent
	maxEventsPerBreaker int
}

var _ StateStore = (*MemoryStore)(nil)

// NewMemoryStore creates a new in-memory state store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		snapshots:          make(map[string]*BreakerSnapshot),
		events:             make(map[string][]*GovernanceEvent),
		maxEventsPerBreaker: DefaultMaxEventsPerBreaker,
	}
}

// NewMemoryStoreWithLimit creates a new in-memory state store with a custom
// maximum number of events retained per breaker.
func NewMemoryStoreWithLimit(maxEventsPerBreaker int) *MemoryStore {
	if maxEventsPerBreaker <= 0 {
		maxEventsPerBreaker = DefaultMaxEventsPerBreaker
	}
	return &MemoryStore{
		snapshots:          make(map[string]*BreakerSnapshot),
		events:             make(map[string][]*GovernanceEvent),
		maxEventsPerBreaker: maxEventsPerBreaker,
	}
}

func (s *MemoryStore) SaveSnapshot(_ context.Context, snapshot *BreakerSnapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots[snapshot.BreakerID] = snapshot
	return nil
}

func (s *MemoryStore) LoadSnapshot(_ context.Context, breakerID string) (*BreakerSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap, ok := s.snapshots[breakerID]
	if !ok {
		return nil, nil
	}
	return snap, nil
}

func (s *MemoryStore) RecordEvent(_ context.Context, event *GovernanceEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Prepend for descending timestamp order.
	events := append([]*GovernanceEvent{event}, s.events[event.BreakerID]...)
	// Enforce maximum events per breaker to prevent unbounded memory growth.
	if len(events) > s.maxEventsPerBreaker {
		events = events[:s.maxEventsPerBreaker]
	}
	s.events[event.BreakerID] = events
	return nil
}

func (s *MemoryStore) ListEvents(_ context.Context, breakerID string, limit int) ([]*GovernanceEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	events := s.events[breakerID]
	if limit > 0 && limit < len(events) {
		events = events[:limit]
	}
	return events, nil
}
