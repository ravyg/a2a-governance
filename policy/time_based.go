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
	"time"

	"github.com/ravyg/a2a-governance/governance"
)

// TimeBased trips when a request arrives outside allowed time windows.
// This is useful for restricting agent operations to business hours or
// preventing overnight automated purchasing.
type TimeBased struct {
	// AllowedStartHour is the earliest hour (0-23) when operations are allowed.
	AllowedStartHour int
	// AllowedEndHour is the latest hour (0-23) when operations are allowed.
	// If AllowedEndHour < AllowedStartHour, the window wraps across midnight.
	AllowedEndHour int
	// AllowedWeekdays restricts which days of the week are allowed.
	// Empty means all days are allowed.
	AllowedWeekdays []time.Weekday
	// Location is the timezone for evaluating time-based rules.
	// Defaults to UTC.
	Location *time.Location
}

var _ governance.Policy = (*TimeBased)(nil)

func (p *TimeBased) Name() string { return "time_based" }

func (p *TimeBased) location() *time.Location {
	if p.Location != nil {
		return p.Location
	}
	return time.UTC
}

func (p *TimeBased) Evaluate(_ context.Context, req *governance.RequestContext) (*governance.Evaluation, error) {
	eval := &governance.Evaluation{
		PolicyName: p.Name(),
		Reason:     governance.ReasonTimeBased,
		Status:     governance.StatusPass,
	}

	ts := req.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}
	ts = ts.In(p.location())

	// Check weekday restriction.
	if len(p.AllowedWeekdays) > 0 {
		weekday := ts.Weekday()
		allowed := false
		for _, d := range p.AllowedWeekdays {
			if d == weekday {
				allowed = true
				break
			}
		}
		if !allowed {
			eval.Tripped = true
			eval.Status = governance.StatusFail
			eval.Score = 1.0
			eval.Message = fmt.Sprintf("day %s is not in allowed weekdays", weekday)
			return eval, nil
		}
	}

	// Check hour restriction.
	hour := ts.Hour()
	if p.AllowedStartHour != p.AllowedEndHour {
		var inWindow bool
		if p.AllowedStartHour < p.AllowedEndHour {
			// Normal window, e.g., 9-17
			inWindow = hour >= p.AllowedStartHour && hour < p.AllowedEndHour
		} else {
			// Wrap-around window, e.g., 22-6 (overnight)
			inWindow = hour >= p.AllowedStartHour || hour < p.AllowedEndHour
		}

		if !inWindow {
			eval.Tripped = true
			eval.Status = governance.StatusFail
			eval.Score = 1.0
			eval.Actual = float64(hour)
			eval.Threshold = float64(p.AllowedStartHour)
			eval.Message = fmt.Sprintf("hour %d is outside allowed window %d:00-%d:00", hour, p.AllowedStartHour, p.AllowedEndHour)
			return eval, nil
		}
	}

	return eval, nil
}
