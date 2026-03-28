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

	"github.com/ravyg/a2a-governance/governance"
)

// VendorTrustScorer is a function that returns a trust score for a vendor (0.0 to 1.0).
type VendorTrustScorer func(ctx context.Context, vendorID string) (float64, error)

// VendorTrust trips when a vendor's trust score falls below a minimum threshold.
type VendorTrust struct {
	// MinTrustScore is the minimum trust score required (0.0 to 1.0).
	MinTrustScore float64
	// Scorer returns the trust score for a given vendor. If nil, a static
	// score registry is used.
	Scorer VendorTrustScorer

	mu     sync.RWMutex
	scores map[string]float64
}

var _ governance.Policy = (*VendorTrust)(nil)

func (p *VendorTrust) Name() string { return "vendor_trust" }

// SetScore registers a static trust score for a vendor.
func (p *VendorTrust) SetScore(vendorID string, score float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.scores == nil {
		p.scores = make(map[string]float64)
	}
	p.scores[vendorID] = score
}

func (p *VendorTrust) Evaluate(ctx context.Context, req *governance.RequestContext) (*governance.Evaluation, error) {
	eval := &governance.Evaluation{
		PolicyName: p.Name(),
		Reason:     governance.ReasonVendorTrust,
		Threshold:  p.MinTrustScore,
	}

	if req.VendorID == "" {
		eval.Message = "no vendor ID provided, skipping"
		return eval, nil
	}

	var score float64
	var err error

	if p.Scorer != nil {
		score, err = p.Scorer(ctx, req.VendorID)
		if err != nil {
			return nil, fmt.Errorf("vendor trust scorer failed for %q: %w", req.VendorID, err)
		}
	} else {
		p.mu.RLock()
		s, ok := p.scores[req.VendorID]
		p.mu.RUnlock()
		if !ok {
			eval.Message = fmt.Sprintf("vendor %q not found in trust registry, allowing", req.VendorID)
			return eval, nil
		}
		score = s
	}

	eval.Actual = score
	eval.Score = 1.0 - score // invert: low trust = high risk

	if score < p.MinTrustScore {
		eval.Tripped = true
		eval.Message = fmt.Sprintf("vendor %q trust score %.2f below minimum %.2f", req.VendorID, score, p.MinTrustScore)
	}

	return eval, nil
}
