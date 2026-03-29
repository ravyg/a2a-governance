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
	"sync"

	"github.com/ravyg/a2a-governance/governance"
)

// AnomalyDetection trips when a transaction value deviates significantly from
// the running mean, using a standard deviation multiplier.
type AnomalyDetection struct {
	// DeviationMultiplier is how many standard deviations from the mean
	// constitutes an anomaly. Defaults to 3.0.
	DeviationMultiplier float64
	// MinSamples is the minimum number of observations before anomaly detection
	// is active. Defaults to 10.
	MinSamples int

	mu    sync.Mutex
	count int
	mean  float64
	m2    float64 // sum of squares of differences from the mean (Welford's)
}

var _ governance.Policy = (*AnomalyDetection)(nil)

func (p *AnomalyDetection) Name() string { return "anomaly_detection" }

func (p *AnomalyDetection) deviationMultiplier() float64 {
	if p.DeviationMultiplier == 0 {
		return 3.0
	}
	return p.DeviationMultiplier
}

func (p *AnomalyDetection) minSamples() int {
	if p.MinSamples == 0 {
		return 10
	}
	return p.MinSamples
}

func (p *AnomalyDetection) Evaluate(_ context.Context, req *governance.RequestContext) (*governance.Evaluation, error) {
	if math.IsNaN(req.TransactionValue) || math.IsInf(req.TransactionValue, 0) {
		return nil, fmt.Errorf("invalid transaction value: %v", req.TransactionValue)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	eval := &governance.Evaluation{
		PolicyName: p.Name(),
		Reason:     governance.ReasonAnomaly,
		Actual:     req.TransactionValue,
	}

	if p.count < p.minSamples() {
		// Not enough data — record and allow.
		p.update(req.TransactionValue)
		eval.Message = fmt.Sprintf("insufficient samples (%d/%d), recording", p.count, p.minSamples())
		return eval, nil
	}

	stddev := p.stddev()
	threshold := p.mean + p.deviationMultiplier()*stddev
	eval.Threshold = threshold

	if stddev > 0 {
		eval.Score = math.Abs(req.TransactionValue-p.mean) / (p.deviationMultiplier() * stddev)
	}

	if req.TransactionValue > threshold {
		eval.Tripped = true
		if stddev > 0 {
			eval.Message = fmt.Sprintf(
				"value %.2f is %.1f std devs from mean %.2f (threshold: %.2f)",
				req.TransactionValue,
				(req.TransactionValue-p.mean)/stddev,
				p.mean,
				threshold,
			)
		} else {
			eval.Message = fmt.Sprintf(
				"value %.2f exceeds mean %.2f (threshold: %.2f, zero variance)",
				req.TransactionValue,
				p.mean,
				threshold,
			)
		}
	} else {
		// Record normal transaction.
		p.update(req.TransactionValue)
	}

	return eval, nil
}

// update adds a value to the running statistics using Welford's online algorithm.
func (p *AnomalyDetection) update(value float64) {
	p.count++
	delta := value - p.mean
	p.mean += delta / float64(p.count)
	delta2 := value - p.mean
	p.m2 += delta * delta2
}

func (p *AnomalyDetection) stddev() float64 {
	if p.count < 2 {
		return 0
	}
	return math.Sqrt(p.m2 / float64(p.count-1))
}

// Reset clears all collected statistics.
func (p *AnomalyDetection) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.count = 0
	p.mean = 0
	p.m2 = 0
}
