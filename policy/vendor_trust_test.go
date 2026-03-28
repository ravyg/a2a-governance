package policy

import (
	"context"
	"errors"
	"testing"

	"github.com/ravyg/a2a-governance/governance"
)

func TestVendorTrust_Name(t *testing.T) {
	p := &VendorTrust{MinTrustScore: 0.5}
	if got := p.Name(); got != "vendor_trust" {
		t.Errorf("Name() = %q, want %q", got, "vendor_trust")
	}
}

func TestVendorTrust_NoVendorID(t *testing.T) {
	p := &VendorTrust{MinTrustScore: 0.5}

	eval, err := p.Evaluate(context.Background(), &governance.RequestContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if eval.Tripped {
		t.Error("should not trip when no vendor ID")
	}
	if eval.Message == "" {
		t.Error("expected message about missing vendor ID")
	}
}

func TestVendorTrust_StaticRegistry_Trusted(t *testing.T) {
	p := &VendorTrust{MinTrustScore: 0.5}
	p.SetScore("vendor-a", 0.9)

	eval, err := p.Evaluate(context.Background(), &governance.RequestContext{
		VendorID: "vendor-a",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if eval.Tripped {
		t.Error("trusted vendor should not trip")
	}
	if eval.Actual != 0.9 {
		t.Errorf("Actual = %f, want 0.9", eval.Actual)
	}
	if eval.Score < 0.09 || eval.Score > 0.11 { // 1.0 - 0.9, with floating point tolerance
		t.Errorf("Score = %f, want ~0.1", eval.Score)
	}
}

func TestVendorTrust_StaticRegistry_Untrusted(t *testing.T) {
	p := &VendorTrust{MinTrustScore: 0.5}
	p.SetScore("vendor-b", 0.2)

	eval, err := p.Evaluate(context.Background(), &governance.RequestContext{
		VendorID: "vendor-b",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !eval.Tripped {
		t.Error("untrusted vendor (0.2 < 0.5) should trip")
	}
	if eval.Message == "" {
		t.Error("expected non-empty message when tripped")
	}
}

func TestVendorTrust_StaticRegistry_NotFound(t *testing.T) {
	p := &VendorTrust{MinTrustScore: 0.5}
	p.SetScore("vendor-a", 0.9)

	eval, err := p.Evaluate(context.Background(), &governance.RequestContext{
		VendorID: "vendor-unknown",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if eval.Tripped {
		t.Error("unknown vendor should not trip (allowed by default)")
	}
	if eval.Message == "" {
		t.Error("expected message about unknown vendor")
	}
}

func TestVendorTrust_Scorer_Trusted(t *testing.T) {
	p := &VendorTrust{
		MinTrustScore: 0.5,
		Scorer: func(_ context.Context, vendorID string) (float64, error) {
			if vendorID == "vendor-a" {
				return 0.8, nil
			}
			return 0.0, nil
		},
	}

	eval, err := p.Evaluate(context.Background(), &governance.RequestContext{
		VendorID: "vendor-a",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if eval.Tripped {
		t.Error("vendor-a with score 0.8 should not trip")
	}
	if eval.Actual != 0.8 {
		t.Errorf("Actual = %f, want 0.8", eval.Actual)
	}
}

func TestVendorTrust_Scorer_Untrusted(t *testing.T) {
	p := &VendorTrust{
		MinTrustScore: 0.5,
		Scorer: func(_ context.Context, _ string) (float64, error) {
			return 0.3, nil
		},
	}

	eval, err := p.Evaluate(context.Background(), &governance.RequestContext{
		VendorID: "vendor-b",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !eval.Tripped {
		t.Error("vendor with score 0.3 should trip (min 0.5)")
	}
}

func TestVendorTrust_Scorer_Error(t *testing.T) {
	p := &VendorTrust{
		MinTrustScore: 0.5,
		Scorer: func(_ context.Context, _ string) (float64, error) {
			return 0, errors.New("scorer failed")
		},
	}

	_, err := p.Evaluate(context.Background(), &governance.RequestContext{
		VendorID: "vendor-c",
	})
	if err == nil {
		t.Fatal("expected error from failing scorer")
	}
}

func TestVendorTrust_Scorer_OverridesRegistry(t *testing.T) {
	p := &VendorTrust{
		MinTrustScore: 0.5,
		Scorer: func(_ context.Context, _ string) (float64, error) {
			return 0.1, nil // always untrusted
		},
	}
	// Even though static registry has a trusted score, scorer should be used.
	p.SetScore("vendor-a", 0.9)

	eval, err := p.Evaluate(context.Background(), &governance.RequestContext{
		VendorID: "vendor-a",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !eval.Tripped {
		t.Error("scorer should override static registry")
	}
}

func TestVendorTrust_AtThreshold(t *testing.T) {
	p := &VendorTrust{MinTrustScore: 0.5}
	p.SetScore("vendor-a", 0.5)

	eval, err := p.Evaluate(context.Background(), &governance.RequestContext{
		VendorID: "vendor-a",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// score (0.5) is not < 0.5, so should NOT trip.
	if eval.Tripped {
		t.Error("vendor at exact threshold should not trip")
	}
}
