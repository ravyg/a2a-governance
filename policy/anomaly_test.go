package policy

import (
	"context"
	"math"
	"testing"

	"github.com/ravyg/a2a-governance/governance"
)

func TestAnomalyDetection_Name(t *testing.T) {
	p := &AnomalyDetection{}
	if got := p.Name(); got != "anomaly_detection" {
		t.Errorf("Name() = %q, want %q", got, "anomaly_detection")
	}
}

func TestAnomalyDetection_Defaults(t *testing.T) {
	p := &AnomalyDetection{}
	if p.deviationMultiplier() != 3.0 {
		t.Errorf("default deviationMultiplier = %f, want 3.0", p.deviationMultiplier())
	}
	if p.minSamples() != 10 {
		t.Errorf("default minSamples = %d, want 10", p.minSamples())
	}
}

func TestAnomalyDetection_InsufficientSamples(t *testing.T) {
	p := &AnomalyDetection{MinSamples: 5}

	for i := 0; i < 4; i++ {
		eval, err := p.Evaluate(context.Background(), &governance.RequestContext{
			TransactionValue: float64(100 + i),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if eval.Tripped {
			t.Fatalf("should not trip with insufficient samples (have %d, need 5)", i+1)
		}
		if eval.Message == "" {
			t.Error("expected message about insufficient samples")
		}
	}
}

func TestAnomalyDetection_NormalValues(t *testing.T) {
	p := &AnomalyDetection{MinSamples: 5, DeviationMultiplier: 3.0}

	// Feed 10 values around 100.
	values := []float64{100, 101, 99, 102, 98, 100, 101, 99, 100, 100}
	for _, v := range values {
		eval, err := p.Evaluate(context.Background(), &governance.RequestContext{
			TransactionValue: v,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if eval.Tripped {
			t.Fatalf("value %.2f should not trip (normal range around 100)", v)
		}
	}
}

func TestAnomalyDetection_AnomalousValue(t *testing.T) {
	p := &AnomalyDetection{MinSamples: 5, DeviationMultiplier: 2.0}

	// Feed 10 identical values to get a tight distribution.
	for i := 0; i < 10; i++ {
		_, _ = p.Evaluate(context.Background(), &governance.RequestContext{
			TransactionValue: 100,
		})
	}

	// Feed one more slightly different to have nonzero stddev.
	// Actually, all 100s gives stddev=0, let's use small variation.
	p.Reset()

	// Build a distribution with small variance.
	for i := 0; i < 10; i++ {
		_, _ = p.Evaluate(context.Background(), &governance.RequestContext{
			TransactionValue: 100 + float64(i%2), // 100, 101, 100, 101, ...
		})
	}

	// Now a very large value should trip.
	eval, err := p.Evaluate(context.Background(), &governance.RequestContext{
		TransactionValue: 200,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !eval.Tripped {
		t.Fatal("200 should be anomalous relative to ~100 mean")
	}
	if eval.Message == "" {
		t.Error("expected non-empty message when tripped")
	}
	if eval.Score <= 0 {
		t.Error("expected positive score")
	}
}

func TestAnomalyDetection_WelfordAccuracy(t *testing.T) {
	p := &AnomalyDetection{MinSamples: 5}

	values := []float64{10, 20, 30, 40, 50}
	for _, v := range values {
		_, _ = p.Evaluate(context.Background(), &governance.RequestContext{
			TransactionValue: v,
		})
	}

	// Verify Welford's produces correct mean and stddev.
	expectedMean := 30.0
	if math.Abs(p.mean-expectedMean) > 0.001 {
		t.Errorf("mean = %f, want %f", p.mean, expectedMean)
	}

	// Sample stddev of [10,20,30,40,50]:
	// variance = ((10-30)^2 + (20-30)^2 + (30-30)^2 + (40-30)^2 + (50-30)^2) / (5-1)
	//          = (400+100+0+100+400) / 4 = 1000/4 = 250
	// stddev = sqrt(250) ~= 15.8114
	expectedStddev := math.Sqrt(1000.0 / 4.0)
	actualStddev := p.stddev()
	if math.Abs(actualStddev-expectedStddev) > 0.001 {
		t.Errorf("stddev = %f, want %f", actualStddev, expectedStddev)
	}
}

func TestAnomalyDetection_TrippedValueNotRecorded(t *testing.T) {
	p := &AnomalyDetection{MinSamples: 5, DeviationMultiplier: 2.0}

	for i := 0; i < 10; i++ {
		_, _ = p.Evaluate(context.Background(), &governance.RequestContext{
			TransactionValue: 100 + float64(i%2),
		})
	}

	countBefore := p.count

	// Anomalous value should not be recorded.
	eval, _ := p.Evaluate(context.Background(), &governance.RequestContext{
		TransactionValue: 500,
	})
	if !eval.Tripped {
		t.Fatal("expected trip")
	}
	if p.count != countBefore {
		t.Errorf("count changed from %d to %d; tripped values should not be recorded", countBefore, p.count)
	}
}

func TestAnomalyDetection_Reset(t *testing.T) {
	p := &AnomalyDetection{MinSamples: 5}

	for i := 0; i < 10; i++ {
		_, _ = p.Evaluate(context.Background(), &governance.RequestContext{
			TransactionValue: float64(100 + i),
		})
	}

	p.Reset()

	if p.count != 0 || p.mean != 0 || p.m2 != 0 {
		t.Error("expected all stats to be zero after reset")
	}
}

func TestAnomalyDetection_BelowThreshold_NotTripped(t *testing.T) {
	p := &AnomalyDetection{MinSamples: 5, DeviationMultiplier: 3.0}

	// Wide distribution: mean ~500, large stddev.
	values := []float64{100, 200, 300, 400, 500, 600, 700, 800, 900, 1000}
	for _, v := range values {
		_, _ = p.Evaluate(context.Background(), &governance.RequestContext{
			TransactionValue: v,
		})
	}

	// A value within 3 stddev should not trip.
	eval, _ := p.Evaluate(context.Background(), &governance.RequestContext{
		TransactionValue: 1200,
	})
	// mean = 550, stddev ~= 302.77, threshold = 550 + 3*302.77 = 1458.31
	// 1200 < 1458, so should not trip.
	if eval.Tripped {
		t.Fatalf("1200 should not trip (threshold ~1458), got tripped with threshold %f", eval.Threshold)
	}
}
