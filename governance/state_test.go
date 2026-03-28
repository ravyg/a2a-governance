package governance

import "testing"

func TestBreakerState_IsValid(t *testing.T) {
	tests := []struct {
		state BreakerState
		want  bool
	}{
		{StateClosed, true},
		{StateOpen, true},
		{StateHalfOpen, true},
		{StateTerminated, true},
		{"INVALID", false},
		{"", false},
		{"closed", false},
	}
	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			if got := tt.state.IsValid(); got != tt.want {
				t.Errorf("BreakerState(%q).IsValid() = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
}

func TestBreakerState_AllowsTraffic(t *testing.T) {
	tests := []struct {
		state BreakerState
		want  bool
	}{
		{StateClosed, true},
		{StateHalfOpen, true},
		{StateOpen, false},
		{StateTerminated, false},
	}
	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			if got := tt.state.AllowsTraffic(); got != tt.want {
				t.Errorf("BreakerState(%q).AllowsTraffic() = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
}
