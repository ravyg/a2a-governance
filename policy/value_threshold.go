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
	"math"

	"github.com/ravyg/a2a-governance/governance"
)

// ValueThreshold trips when a single transaction exceeds a maximum value.
type ValueThreshold struct {
	// MaxValue is the maximum allowed transaction value.
	MaxValue float64
}

var _ governance.Policy = (*ValueThreshold)(nil)

func (p *ValueThreshold) Name() string { return "value_threshold" }

func (p *ValueThreshold) Evaluate(_ context.Context, req *governance.RequestContext) (*governance.Evaluation, error) {
	if math.IsNaN(req.TransactionValue) || math.IsInf(req.TransactionValue, 0) {
		return nil, fmt.Errorf("invalid transaction value: %v", req.TransactionValue)
	}

	eval := &governance.Evaluation{
		PolicyName: p.Name(),
		Reason:     governance.ReasonValueThreshold,
		Threshold:  p.MaxValue,
		Actual:     req.TransactionValue,
	}

	if req.TransactionValue > p.MaxValue {
		eval.Tripped = true
		if p.MaxValue > 0 {
			eval.Score = req.TransactionValue / p.MaxValue
		} else {
			eval.Score = 1.0
		}
		eval.Message = fmt.Sprintf("transaction value %.2f exceeds threshold %.2f", req.TransactionValue, p.MaxValue)
	}

	return eval, nil
}
