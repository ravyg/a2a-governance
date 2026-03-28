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

// Command basic demonstrates a minimal a2a-governance setup with a circuit
// breaker protecting an A2A agent server.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ravyg/a2a-governance/governance"
	"github.com/ravyg/a2a-governance/policy"
)

func main() {
	// Create governance policies.
	breaker := governance.NewCircuitBreaker(governance.BreakerConfig{
		Policies: []governance.Policy{
			&policy.ValueThreshold{MaxValue: 10000},
			&policy.Velocity{MaxRequests: 100, Window: time.Minute},
			&policy.CumulativeSpend{MaxSpend: 50000, Window: 24 * time.Hour},
		},
		CooldownDuration: 30 * time.Second,
		OnEscalation: func(ctx context.Context, cb *governance.CircuitBreaker, eval *governance.Evaluation) error {
			log.Printf("ESCALATION: circuit tripped by %s — %s", eval.PolicyName, eval.Message)
			return nil
		},
		OnTransition: func(ctx context.Context, from, to governance.BreakerState, eval *governance.Evaluation) {
			log.Printf("STATE CHANGE: %s -> %s", from, to)
		},
	})

	ctx := context.Background()

	// Simulate normal transactions.
	for i := 0; i < 5; i++ {
		result, err := breaker.Evaluate(ctx, &governance.RequestContext{
			TaskID:           fmt.Sprintf("task-%d", i),
			AgentID:          "shopping-agent",
			TransactionValue: 500,
			Timestamp:        time.Now(),
		})
		if err != nil {
			log.Fatalf("evaluation error: %v", err)
		}
		fmt.Printf("Request %d: %s\n", i, result)
	}

	// Simulate a high-value transaction that trips the breaker.
	result, err := breaker.Evaluate(ctx, &governance.RequestContext{
		TaskID:           "task-high-value",
		AgentID:          "shopping-agent",
		TransactionValue: 15000,
		Timestamp:        time.Now(),
	})
	if err != nil {
		log.Printf("Expected error: %v", err)
	}
	fmt.Printf("High-value request: %s\n", result)

	// Show stats.
	stats := breaker.Stats()
	fmt.Printf("\nBreaker stats: state=%s requests=%d blocked=%d trips=%d\n",
		stats.State, stats.TotalRequests, stats.TotalBlocked, stats.TotalTrips)
}
