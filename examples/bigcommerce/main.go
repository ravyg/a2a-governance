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

// Command bigcommerce demonstrates a2a-governance in an ecommerce scenario
// with AP2 risk payloads, vendor trust scoring, and multi-policy evaluation.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/ravyg/a2a-governance/ap2"
	"github.com/ravyg/a2a-governance/governance"
	"github.com/ravyg/a2a-governance/policy"
	"github.com/ravyg/a2a-governance/store"
)

func main() {
	// Set up a vendor trust registry.
	vendorTrust := &policy.VendorTrust{MinTrustScore: 0.7}
	vendorTrust.SetScore("trusted-merchant", 0.95)
	vendorTrust.SetScore("new-merchant", 0.55)
	vendorTrust.SetScore("flagged-merchant", 0.2)

	// Create a governance circuit breaker with commerce-specific policies.
	breaker := governance.NewCircuitBreaker(governance.BreakerConfig{
		Policies: []governance.Policy{
			&policy.ValueThreshold{MaxValue: 5000},
			&policy.CumulativeSpend{MaxSpend: 25000, Window: 24 * time.Hour},
			&policy.Velocity{MaxRequests: 50, Window: time.Hour},
			&policy.AnomalyDetection{DeviationMultiplier: 2.5, MinSamples: 5},
			vendorTrust,
		},
		CooldownDuration:               time.Minute,
		ConsecutiveFailuresToTerminate:  3,
		OnEscalation: func(ctx context.Context, cb *governance.CircuitBreaker, eval *governance.Evaluation) error {
			log.Printf("[ESCALATION] Policy: %s | Reason: %s | %s", eval.PolicyName, eval.Reason, eval.Message)
			return nil
		},
		OnTransition: func(ctx context.Context, from, to governance.BreakerState, eval *governance.Evaluation) {
			log.Printf("[STATE] %s -> %s", from, to)
		},
	})

	// Set up audit store.
	auditStore := store.NewMemoryStore()
	ctx := context.Background()

	// Scenario 1: Normal purchase from trusted merchant.
	fmt.Println("=== Scenario 1: Normal Purchase ===")
	result, _ := breaker.Evaluate(ctx, &governance.RequestContext{
		TaskID:           "order-001",
		AgentID:          "bigcommerce-shopping-agent",
		UserID:           "customer-42",
		TransactionValue: 150.00,
		Currency:         "USD",
		VendorID:         "trusted-merchant",
		Timestamp:        time.Now(),
	})
	fmt.Printf("Result: %s\n", result)

	// Generate AP2 risk payload for the mandate.
	riskPayload := ap2.NewRiskPayloadFromEvaluation(result, breaker.State(), "bigcommerce-shopping-agent")
	mandate := ap2.CartMandate{
		MandateID:   "mandate-001",
		CartID:      "cart-42",
		TotalValue:  150.00,
		Currency:    "USD",
		RiskPayload: riskPayload,
	}
	mandateJSON, _ := json.MarshalIndent(mandate, "", "  ")
	fmt.Printf("Cart Mandate with Risk Payload:\n%s\n\n", mandateJSON)

	// Record audit event.
	_ = auditStore.RecordEvent(ctx, &store.GovernanceEvent{
		ID:               "evt-001",
		BreakerID:        "bigcommerce-default",
		Timestamp:        time.Now(),
		State:            breaker.State(),
		Result:           result,
		TaskID:           "order-001",
		AgentID:          "bigcommerce-shopping-agent",
		TransactionValue: 150.00,
	})

	// Scenario 2: Purchase from untrusted merchant — should trip.
	fmt.Println("=== Scenario 2: Untrusted Vendor ===")
	result2, err := breaker.Evaluate(ctx, &governance.RequestContext{
		TaskID:           "order-002",
		AgentID:          "bigcommerce-shopping-agent",
		UserID:           "customer-42",
		TransactionValue: 200.00,
		Currency:         "USD",
		VendorID:         "flagged-merchant",
		Timestamp:        time.Now(),
	})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
	fmt.Printf("Result: %s\n", result2)

	// Scenario 3: Attempt while circuit is open — should be blocked.
	fmt.Println("\n=== Scenario 3: Circuit Open ===")
	result3, err := breaker.Evaluate(ctx, &governance.RequestContext{
		TaskID:           "order-003",
		AgentID:          "bigcommerce-shopping-agent",
		TransactionValue: 50.00,
		Timestamp:        time.Now(),
	})
	if err != nil {
		fmt.Printf("Blocked: %v\n", err)
	}
	_ = result3

	// Print final stats.
	stats := breaker.Stats()
	fmt.Printf("\n=== Final Stats ===\n")
	fmt.Printf("State: %s | Requests: %d | Blocked: %d | Trips: %d\n",
		stats.State, stats.TotalRequests, stats.TotalBlocked, stats.TotalTrips)

	// Show audit trail.
	events, _ := auditStore.ListEvents(ctx, "bigcommerce-default", 10)
	fmt.Printf("Audit events: %d\n", len(events))
}
