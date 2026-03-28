package store

import (
	"context"
	"testing"
	"time"

	"github.com/ravyg/a2a-governance/governance"
)

func TestMemoryStore_SaveAndLoadSnapshot(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	snap := &BreakerSnapshot{
		BreakerID:        "breaker-1",
		State:            governance.StateClosed,
		ConsecutiveTrips: 0,
		UpdatedAt:        time.Now(),
	}

	if err := s.SaveSnapshot(ctx, snap); err != nil {
		t.Fatalf("SaveSnapshot error: %v", err)
	}

	loaded, err := s.LoadSnapshot(ctx, "breaker-1")
	if err != nil {
		t.Fatalf("LoadSnapshot error: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if loaded.BreakerID != "breaker-1" {
		t.Errorf("BreakerID = %q, want %q", loaded.BreakerID, "breaker-1")
	}
	if loaded.State != governance.StateClosed {
		t.Errorf("State = %q, want %q", loaded.State, governance.StateClosed)
	}
}

func TestMemoryStore_LoadSnapshot_NotFound(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	snap, err := s.LoadSnapshot(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap != nil {
		t.Fatalf("expected nil snapshot, got %+v", snap)
	}
}

func TestMemoryStore_SaveSnapshot_Overwrite(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	snap1 := &BreakerSnapshot{
		BreakerID:        "breaker-1",
		State:            governance.StateClosed,
		ConsecutiveTrips: 0,
	}
	_ = s.SaveSnapshot(ctx, snap1)

	snap2 := &BreakerSnapshot{
		BreakerID:        "breaker-1",
		State:            governance.StateOpen,
		ConsecutiveTrips: 2,
	}
	_ = s.SaveSnapshot(ctx, snap2)

	loaded, _ := s.LoadSnapshot(ctx, "breaker-1")
	if loaded.State != governance.StateOpen {
		t.Errorf("State = %q, want %q (overwritten)", loaded.State, governance.StateOpen)
	}
	if loaded.ConsecutiveTrips != 2 {
		t.Errorf("ConsecutiveTrips = %d, want 2", loaded.ConsecutiveTrips)
	}
}

func TestMemoryStore_RecordAndListEvents(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()
	now := time.Now()

	events := []*GovernanceEvent{
		{ID: "e1", BreakerID: "b1", Timestamp: now, TaskID: "task-1"},
		{ID: "e2", BreakerID: "b1", Timestamp: now.Add(time.Second), TaskID: "task-2"},
		{ID: "e3", BreakerID: "b1", Timestamp: now.Add(2 * time.Second), TaskID: "task-3"},
	}

	for _, e := range events {
		if err := s.RecordEvent(ctx, e); err != nil {
			t.Fatalf("RecordEvent error: %v", err)
		}
	}

	listed, err := s.ListEvents(ctx, "b1", 0)
	if err != nil {
		t.Fatalf("ListEvents error: %v", err)
	}
	if len(listed) != 3 {
		t.Fatalf("expected 3 events, got %d", len(listed))
	}

	// Events should be in descending order (most recent first).
	if listed[0].ID != "e3" {
		t.Errorf("first event ID = %q, want e3 (most recent)", listed[0].ID)
	}
	if listed[2].ID != "e1" {
		t.Errorf("last event ID = %q, want e1 (oldest)", listed[2].ID)
	}
}

func TestMemoryStore_ListEvents_WithLimit(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_ = s.RecordEvent(ctx, &GovernanceEvent{
			ID:        "e" + string(rune('0'+i)),
			BreakerID: "b1",
		})
	}

	listed, err := s.ListEvents(ctx, "b1", 2)
	if err != nil {
		t.Fatalf("ListEvents error: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("expected 2 events with limit, got %d", len(listed))
	}
}

func TestMemoryStore_ListEvents_NotFound(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	listed, err := s.ListEvents(ctx, "nonexistent", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(listed) != 0 {
		t.Fatalf("expected 0 events, got %d", len(listed))
	}
}

func TestMemoryStore_MultipleBreakers(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	_ = s.SaveSnapshot(ctx, &BreakerSnapshot{BreakerID: "b1", State: governance.StateClosed})
	_ = s.SaveSnapshot(ctx, &BreakerSnapshot{BreakerID: "b2", State: governance.StateOpen})

	_ = s.RecordEvent(ctx, &GovernanceEvent{ID: "e1", BreakerID: "b1"})
	_ = s.RecordEvent(ctx, &GovernanceEvent{ID: "e2", BreakerID: "b2"})
	_ = s.RecordEvent(ctx, &GovernanceEvent{ID: "e3", BreakerID: "b2"})

	snap1, _ := s.LoadSnapshot(ctx, "b1")
	snap2, _ := s.LoadSnapshot(ctx, "b2")
	if snap1.State != governance.StateClosed {
		t.Error("b1 should be CLOSED")
	}
	if snap2.State != governance.StateOpen {
		t.Error("b2 should be OPEN")
	}

	b1Events, _ := s.ListEvents(ctx, "b1", 0)
	b2Events, _ := s.ListEvents(ctx, "b2", 0)
	if len(b1Events) != 1 {
		t.Errorf("b1 events = %d, want 1", len(b1Events))
	}
	if len(b2Events) != 2 {
		t.Errorf("b2 events = %d, want 2", len(b2Events))
	}
}
