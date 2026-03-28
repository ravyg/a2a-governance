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

package governance

// BreakerState represents the current state of a circuit breaker.
type BreakerState string

const (
	// StateClosed indicates normal operation — requests flow through.
	StateClosed BreakerState = "CLOSED"

	// StateOpen indicates the circuit has tripped — requests are blocked.
	StateOpen BreakerState = "OPEN"

	// StateHalfOpen indicates the circuit is probing — limited requests allowed.
	StateHalfOpen BreakerState = "HALF_OPEN"

	// StateTerminated indicates permanent shutdown — no recovery possible.
	StateTerminated BreakerState = "TERMINATED"
)

// IsValid reports whether s is a recognized breaker state.
func (s BreakerState) IsValid() bool {
	switch s {
	case StateClosed, StateOpen, StateHalfOpen, StateTerminated:
		return true
	}
	return false
}

// AllowsTraffic reports whether the state permits requests to flow through.
func (s BreakerState) AllowsTraffic() bool {
	return s == StateClosed || s == StateHalfOpen
}
