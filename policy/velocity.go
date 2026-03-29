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

package policy

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ravyg/a2a-governance/governance"
)

// maxVelocityTimestamps is the maximum number of timestamps retained to prevent
// unbounded memory growth. When exceeded, the oldest timestamps are evicted.
const maxVelocityTimestamps = 100_000

// Velocity trips when the transaction frequency exceeds a rate limit within a window.
type Velocity struct {
	// MaxRequests is the maximum number of requests allowed within the window.
	MaxRequests int
	// Window is the sliding time window for rate measurement.
	Window time.Duration

	mu         sync.Mutex
	timestamps []time.Time
}

var _ governance.Policy = (*Velocity)(nil)

func (p *Velocity) Name() string { return "velocity" }

func (p *Velocity) Evaluate(_ context.Context, req *governance.RequestContext) (*governance.Evaluation, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := req.Timestamp
	if now.IsZero() {
		now = time.Now()
	}

	// Prune timestamps outside window.
	cutoff := now.Add(-p.Window)
	pruned := p.timestamps[:0]
	for _, ts := range p.timestamps {
		if ts.After(cutoff) {
			pruned = append(pruned, ts)
		}
	}
	p.timestamps = pruned

	// Count including this request.
	count := len(p.timestamps) + 1

	eval := &governance.Evaluation{
		PolicyName: p.Name(),
		Reason:     governance.ReasonVelocity,
		Threshold:  float64(p.MaxRequests),
		Actual:     float64(count),
	}

	if count > p.MaxRequests {
		eval.Tripped = true
		if p.MaxRequests > 0 {
			eval.Score = float64(count) / float64(p.MaxRequests)
		} else {
			eval.Score = 1.0
		}
		eval.Message = fmt.Sprintf("request rate %d exceeds limit %d within %s", count, p.MaxRequests, p.Window)
	} else {
		// Record timestamp, enforcing a hard cap to prevent unbounded growth.
		p.timestamps = append(p.timestamps, now)
		if len(p.timestamps) > maxVelocityTimestamps {
			p.timestamps = p.timestamps[len(p.timestamps)-maxVelocityTimestamps:]
		}
	}

	return eval, nil
}

// Reset clears the velocity tracker.
func (p *Velocity) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.timestamps = nil
}
