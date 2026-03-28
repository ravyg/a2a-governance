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

// CumulativeSpend trips when aggregate spend within a time window exceeds a limit.
type CumulativeSpend struct {
	// MaxSpend is the maximum cumulative spend allowed within the window.
	MaxSpend float64
	// Window is the sliding time window for accumulation.
	Window time.Duration

	mu      sync.Mutex
	records []spendRecord
}

type spendRecord struct {
	amount    float64
	timestamp time.Time
}

var _ governance.Policy = (*CumulativeSpend)(nil)

func (p *CumulativeSpend) Name() string { return "cumulative_spend" }

func (p *CumulativeSpend) Evaluate(_ context.Context, req *governance.RequestContext) (*governance.Evaluation, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := req.Timestamp
	if now.IsZero() {
		now = time.Now()
	}

	// Prune records outside the window.
	cutoff := now.Add(-p.Window)
	pruned := p.records[:0]
	for _, r := range p.records {
		if r.timestamp.After(cutoff) {
			pruned = append(pruned, r)
		}
	}
	p.records = pruned

	// Calculate current total including this request.
	var total float64
	for _, r := range p.records {
		total += r.amount
	}
	total += req.TransactionValue

	eval := &governance.Evaluation{
		PolicyName: p.Name(),
		Reason:     governance.ReasonCumulative,
		Threshold:  p.MaxSpend,
		Actual:     total,
	}

	if total > p.MaxSpend {
		eval.Tripped = true
		eval.Score = total / p.MaxSpend
		eval.Message = fmt.Sprintf("cumulative spend %.2f exceeds limit %.2f within %s", total, p.MaxSpend, p.Window)
	} else {
		// Record this transaction.
		p.records = append(p.records, spendRecord{amount: req.TransactionValue, timestamp: now})
	}

	return eval, nil
}

// Reset clears all recorded transactions.
func (p *CumulativeSpend) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.records = nil
}
